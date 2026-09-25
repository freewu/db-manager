#!/usr/bin/env node
/**
 * The message tables behind the interface, and the tool that fills them.
 *
 * The interface is written in one language in the source (English) and read in
 * three (`frontend/src/lib/i18n`). Every user-visible string therefore lives in
 * exactly one place: `frontend/src/lib/i18n/messages/<area>.ts`, one file per
 * source file, each line holding the three spellings of one message. This script
 * moves strings out of the code and into those files, and puts a translation
 * back in.
 *
 * Subcommands:
 *
 *   extract [--report] <file…>   rewrite the named source files: every
 *                                user-visible string becomes `t('area.key')` and
 *                                the area file is created or updated (existing
 *                                translations are kept). `--report` also prints
 *                                the strings it left alone, with the syntax they
 *                                sat in — the list to read when hunting for the
 *                                sentence that is still English.
 *   index                        regenerate `messages/index.ts` from the areas
 *   export [--all]               print `{ "key": "English" }` as JSON, the input
 *                                for a translator (only untranslated keys unless
 *                                `--all`)
 *   fill <file.json>             merge `{ "key": ["简体中文", "繁體中文"] }` back
 *                                into the area files
 *   list                         how much of each area is translated, worst first
 *
 * Why a codemod rather than editing by hand: there are several hundred strings,
 * and the cost of a hand edit is not the edit itself but the two that silently do
 * not happen. Re-running `extract` is also what keeps a new component honest —
 * `--report` turns "the string I forgot" into a line of output rather than into
 * an English sentence in the middle of a Chinese window.
 *
 * The TypeScript compiler is borrowed from the frontend's node_modules: this
 * repository has no root-level dependencies, and the alternative (a regex over
 * .tsx) cannot tell a tooltip from a CSS class name.
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import ts from '../frontend/node_modules/typescript/lib/typescript.js'

const here = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(here, '..')
const srcDir = path.join(root, 'frontend/src')
const messagesDir = path.join(srcDir, 'lib/i18n/messages')
const indexPath = path.join(messagesDir, 'index.ts')

/**
 * Attribute and property names whose value is read by a person.
 *
 * Everything else is a class name, an identifier, an enum member or a lookup
 * key, and translating one of those is a bug that only shows up at runtime — so
 * this is an allow list, not a list of exceptions.
 */
const TEXT_NAMES = new Set([
  'title',
  'label',
  'description',
  'placeholder',
  'tooltip',
  'content',
  'message',
  'okText',
  'cancelText',
  'subTitle',
  'help',
  'hint',
  'extra',
  'emptyText',
  'footer',
  'header',
  'summary',
  'text',
  'prompt',
  'note',
  'tip',
  'caption',
  'confirmText',
  'addonAfter',
  'addonBefore',
  'suffix',
  'prefix',
  'aria-label',
  // A hint written for a form field that is not the field's `placeholder`:
  // the connection forms name their own, because the host is one of several.
  'hostPlaceholder',
])

/** The objects whose method calls take a sentence to show the user. */
const TOAST_HOLDERS = new Set(['message', 'notification', 'modal', 'Modal'])

/**
 * Local helpers that take a word as an argument, and which argument it is.
 *
 * These build a piece of the interface out of a sentence — a tree row that
 * says *Loading indexes…*, a button that copies and says *Script* — so a string
 * handed to one of them is read by a person just as much as a `title` is. The
 * list is by name because there is nothing else to go on: a call is not a prop,
 * and the argument is a plain `string`.
 */
const TEXT_CALLS = {
  placeholderNode: [1],
  emptyNode: [1, 2],
  placeholder: [1],
  copy: [1],
  driverLabel: [1],
}

/**
 * Leading words that mean "this string is SQL", not prose.
 *
 * Matched in capitals only: `Create copy` and `Delete this query` are sentences
 * that happen to start with a word SQL also uses, and a statement written for a
 * human to read is upper-cased. Case-insensitive matching here would quietly
 * leave every sentence that starts with Create, Delete, Show or Comment behind.
 */
const SQL_LEAD =
  /^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|TRUNCATE|WITH|PRAGMA|SHOW|EXPLAIN|EXEC|GRANT|REVOKE|COMMENT)\b/

/** A name in capitals (`utf8` upper-cased, `PK`, `UTF-8`) is an identifier. */
const SCREAMING = /^[A-Z0-9_-]+$/

const ENTITIES = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#39;': "'",
  '&apos;': "'",
  '&nbsp;': ' ',
  '&mdash;': '—',
  '&ndash;': '–',
  '&hellip;': '…',
  '&middot;': '·',
  '&times;': '×',
}

// -------------------------------------------------------------------- text --

/** JSX collapses whitespace; a message should hold the collapsed reading. */
function tidy(text, trim = true) {
  let out = text
  for (const [entity, plain] of Object.entries(ENTITIES)) {
    out = out.split(entity).join(plain)
  }
  out = out.replace(/\s+/g, ' ')
  return trim ? out.trim() : out
}

/**
 * Whether a string is something a person reads.
 *
 * A text position holds prose even when it is one lowercase word — `connected`,
 * `read-only`, `ascending` are all labels — so the only strings ruled out here
 * are the ones that cannot be prose: no letters at all, a statement, or a name
 * written in capitals (`PRIMARY KEY`, `UTF-8`, `ASC`).
 */
function isReadable(text) {
  if (!/\p{L}/u.test(text)) return false
  if (SQL_LEAD.test(text)) return false
  // One letter is a separator being assembled (`\`${n} tab${"s"}\``), not a word.
  const letters = text.match(/[A-Za-z]/g)?.length ?? 0
  if (letters < 2 && !/[^\x00-\x7F]/.test(text)) return false
  return !SCREAMING.test(text)
}

// ------------------------------------------------------------------- areas --

/**
 * The area a source file's messages live in: `components/Foo.tsx` → `foo`.
 *
 * A file one directory deeper keeps its directory (`components/overview/
 * shared.tsx` → `overviewShared`), because two files called `shared.tsx` are two
 * sets of messages and their keys have to be able to say which is which. The
 * `components` directory itself says nothing, so it is dropped. A name that
 * already ends with its directory (`overview/MongoOverview.tsx`) is trimmed, so
 * the key reads `overviewMongo` rather than `overviewMongoOverview`.
 */
function areaOf(file) {
  const rel = path.relative(srcDir, file).replace(/\\/g, '/')
  const parts = rel.split('/')
  const base = path.basename(rel).replace(/\.(tsx?|jsx?)$/, '')
  const group = parts.length > 1 ? parts[parts.length - 2] : ''
  const kept = group && base.toLowerCase().endsWith(group.toLowerCase()) ? base.slice(0, -group.length) : base
  const dirs = parts
    .slice(0, -1)
    .filter((part) => part !== 'components')
    .map(upperFirst)
  return lowerFirst(dirs.join('') + upperFirst(kept))
}

const lowerFirst = (text) => text.charAt(0).toLowerCase() + text.slice(1)
const upperFirst = (text) => text.charAt(0).toUpperCase() + text.slice(1)

/** The message key for one piece of text, inside an area. */
function keyOf(area, text) {
  // `{name}` placeholders name nothing a key should carry, and a whole sentence
  // would make an unreadable key: words are added while the key stays short
  // enough to read, and the key never stops in the middle of one.
  const words = text
    .replace(/\{[^}]*\}/g, ' ')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
    .split(' ')
    .filter(Boolean)
  const kept = []
  for (const word of words) {
    if (kept.length && [...kept, word].join('-').length > 48) break
    kept.push(word)
  }
  return `${area}.${kept.join('-') || 'text'}`
}

/** The identifier an area file exports: `query-pane` → `queryPane`. */
const identifierOf = (area) => area.replace(/-(\w)/g, (_, ch) => ch.toUpperCase())

const areaPath = (area) => path.join(messagesDir, `${area}.ts`)

function allAreas() {
  if (!fs.existsSync(messagesDir)) return []
  return fs
    .readdirSync(messagesDir)
    .filter((name) => name.endsWith('.ts') && name !== 'index.ts')
    .sort()
}

/** One message line: `'a.b': ['English', '简体中文', '繁體中文']`. */
function lineOf(key, entry) {
  const columns = [entry[0], entry[1] ?? '', entry[2] ?? ''].map(quoted)
  const oneLine = `  '${key}': [${columns.join(', ')}],`
  if (oneLine.length <= 100) return oneLine
  return `  '${key}': [\n${columns.map((column) => `    ${column},`).join('\n')}\n  ],`
}

function quoted(text) {
  return `'${text.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n')}'`
}

/** One area file: keys in order, three columns each. */
class Table {
  constructor(area, source) {
    this.area = area
    this.source = source
    this.entries = new Map()
  }

  /** Adds a message, reusing an existing line when the English text matches. */
  add(english) {
    const base = keyOf(this.area, english)
    let key = base
    let suffix = 2
    while (this.entries.has(key) && this.entries.get(key)[0] !== english) {
      key = `${base}-${suffix++}`
    }
    if (this.entries.has(key)) return key
    this.entries.set(key, [english, '', ''])
    return key
  }
}

const HEADER = [
  "import type { AreaMessages } from '../types'",
  '',
  '// source: %SOURCE%',
  '//',
  '// Written by `node scripts/i18n.mjs extract`. The first column is the text the',
  '// code was written with and the other two are its translations; a correction',
  '// belongs here. The marker line above is read back by the script, so it stays.',
  '',
]

function writeTable(table) {
  const lines = HEADER.map((line) =>
    line.replace('%SOURCE%', path.relative(root, table.source).replace(/\\/g, '/')),
  )
  lines.push(`export const ${identifierOf(table.area)} = {`)
  for (const [key, entry] of table.entries) lines.push(lineOf(key, entry))
  lines.push('} satisfies AreaMessages', '')
  fs.mkdirSync(messagesDir, { recursive: true })
  fs.writeFileSync(areaPath(table.area), lines.join('\n'))
}

/** Reads an area file back, so a second `extract` keeps what was translated. */
function readTable(file) {
  const area = path.basename(file).replace(/\.ts$/, '')
  const text = fs.readFileSync(file, 'utf8')
  const source = /^\/\/ source: (.+)$/m.exec(text)?.[1]
  const table = new Table(area, source ? path.join(root, source) : file)
  const sf = ts.createSourceFile(file, text, ts.ScriptTarget.ES2022, true, ts.ScriptKind.TS)
  const walk = (node) => {
    if (ts.isPropertyAssignment(node) && ts.isStringLiteral(node.name) && ts.isArrayLiteralExpression(node.initializer)) {
      const columns = node.initializer.elements.map((element) =>
        ts.isStringLiteral(element) || ts.isNoSubstitutionTemplateLiteral(element) ? element.text : '',
      )
      table.entries.set(node.name.text, [columns[0] ?? '', columns[1] ?? '', columns[2] ?? ''])
    }
    node.forEachChild(walk)
  }
  walk(sf)
  return table
}

// ------------------------------------------------------------------ reading --

/**
 * The nearest ancestor that is not a parenthesis, a conditional, a cast or the
 * braces around a JSX child.
 *
 * Skipping those is what recognises `hint={busy ? 'Saving…' : 'Save'}` as a hint
 * in both branches and `{'Open'}` as the text of an element. A choice written
 * with `??`, `||` or `&&` is skipped the same way, because it is one: `{tip ??
 * 'Create one with …'}` is the tip of the element it sits in, whichever branch
 * the reader gets.
 */
function makeHolder() {
  return function holderOf(node) {
    let current = node
    for (;;) {
      const parent = current.parent
      if (!parent) return undefined
      if (
        ts.isParenthesizedExpression(parent) ||
        ts.isConditionalExpression(parent) ||
        ts.isAsExpression(parent) ||
        ts.isNonNullExpression(parent) ||
        ts.isJsxExpression(parent) ||
        // `a ?? b`, `a || b`, `a && b` — but not `a + b`, which is assembling
        // text rather than choosing between two of them.
        (ts.isBinaryExpression(parent) &&
          parent.operatorToken.kind !== ts.SyntaxKind.PlusToken &&
          parent.operatorToken.kind !== ts.SyntaxKind.PlusEqualsToken &&
          (parent.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken ||
            parent.operatorToken.kind === ts.SyntaxKind.BarBarToken ||
            parent.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken))
      ) {
        current = parent
        continue
      }
      return parent
    }
  }
}

/**
 * Whether `container` is `node` or one of its parents.
 *
 * A string that sits in a conditional is still the argument of the call that
 * holds the conditional: `t(a ? 'x' : 'y')` is two strings, both of them already
 * looked up.
 */
function holds(container, node) {
  for (let current = node; current; current = current.parent) {
    if (current === container) return true
  }
  return false
}

/**
 * How a string met in this position is read, or `undefined` when this position
 * is not one a person reads.
 */
function readingOf(node, holder) {
  if (ts.isJsxAttribute(holder) && TEXT_NAMES.has(holder.name.getText())) return 'text'
  if (ts.isJsxElement(holder) || ts.isJsxFragment(holder)) return 'text'
  if (ts.isPropertyAssignment(holder)) {
    const name = holder.name.getText().replace(/^['"]|['"]$/g, '')
    return TEXT_NAMES.has(name) ? 'text' : undefined
  }
  if (ts.isCallExpression(holder)) {
    const indices = TEXT_CALLS[holder.expression.getText()]
    if (indices) {
      const at = holder.arguments.findIndex((argument) => holds(argument, node))
      if (at >= 0 && indices.includes(at)) return 'text'
    }
  }
  if (ts.isCallExpression(holder) && holder.arguments[0] && holds(holder.arguments[0], node) && ts.isPropertyAccessExpression(holder.expression)) {
    const receiver = holder.expression.expression
    return ts.isIdentifier(receiver) && TOAST_HOLDERS.has(receiver.text) ? 'text' : undefined
  }
  if (ts.isReturnStatement(holder)) return returnsWords(holder) ? 'text' : undefined
  return undefined
}

/**
 * Whether a `return 'something'` is a word rather than a value.
 *
 * Only a component returns words: `BootFailure` returning `'The backend failed
 * to start.'` is text, while `randomHex()` returning `'ab'` is data. The two are
 * told apart by the name the function is written under — components are named
 * with a capital, helpers are not — so a `return` inside anything else is left
 * alone.
 */
function returnsWords(node) {
  for (let current = node.parent; current; current = current.parent) {
    const named =
      ts.isFunctionDeclaration(current) || ts.isFunctionExpression(current)
        ? current.name?.text
        : undefined
    if (named) return /^\p{Lu}/u.test(named)
    if (ts.isArrowFunction(current) || ts.isFunctionExpression(current)) {
      const parent = current.parent
      if (parent && ts.isVariableDeclaration(parent) && ts.isIdentifier(parent.name)) {
        return /^\p{Lu}/u.test(parent.name.text)
      }
      return false
    }
    if (ts.isMethodDeclaration(current) || ts.isGetAccessorDeclaration(current)) return false
    if (ts.isSourceFile(current)) break
  }
  return false
}

// ------------------------------------------------------------------ rewrite --

/** Turns `` `Only ${count} rows` `` into its text and the names to fill in. */
function templateOf(node) {
  /** Is there a word written into the expression, rather than a value read from it? */
  const holdsLiteral = (expression) => {
    if (ts.isStringLiteral(expression) || ts.isNoSubstitutionTemplateLiteral(expression)) return true
    let found = false
    expression.forEachChild((child) => {
      if (!found && holdsLiteral(child)) found = true
    })
    return found
  }
  const params = []
  const used = new Set()
  let static_ = node.head.text
  let text = node.head.text
  for (const span of node.templateSpans) {
    const source = span.expression.getText()
    // A constant in capitals (`PROJECT_URL`, `JUST_VERSION`) is a value being
    // assembled into a URL or a command, not a word woven into a sentence.
    if (/\b[A-Z][A-Z0-9_]{2,}\b/.test(source)) return undefined
    // The name of a placeholder is the last word of the expression — `count`
    // from `row.count`, `rows.length` from the length of a list. An expression
    // with no name at all cannot be filled in and is left for a hand edit.
    const words = [...source.matchAll(/[A-Za-z_$][\w$]*/g)].map((match) => match[0])
    let name = words[words.length - 1]
    if (!name || /[`{}]/.test(source)) return undefined
    // `op === 'drop' ? 'Dropping' : 'Emptying'` is a word the sentence needs, not
    // a value to fill in: taking the last word of the expression would name the
    // placeholder after one of the branches and leave both behind in English.
    // A string literal anywhere in the expression means this is that case.
    if (holdsLiteral(span.expression)) return undefined
    if (used.has(name)) {
      let suffix = 2
      while (used.has(`${name}${suffix}`)) suffix++
      name = `${name}${suffix}`
    }
    used.add(name)
    params.push([name, source])
    text += `{${name}}${span.literal.text}`
    static_ += span.literal.text
  }
  // `{label} {value}` is a pair being put side by side, not a sentence: with no
  // words of its own there is nothing for a translation to say.
  if (!/[A-Za-z]/.test(static_)) return undefined
  return { text: tidy(text, false), params }
}

function scriptKindOf(file) {
  if (file.endsWith('.tsx')) return ts.ScriptKind.TSX
  if (file.endsWith('.jsx')) return ts.ScriptKind.JSX
  if (file.endsWith('.js')) return ts.ScriptKind.JS
  return ts.ScriptKind.TS
}

/** Adds `import { t } from '<i18n>'` after the last import, if it is missing. */
function withImport(source, sf, file) {
  // Resolved rather than pattern-matched: `lib/about.ts` reaches the same module
  // as `components/Foo.tsx` does, one as `./i18n` and the other as `../lib/i18n`,
  // and a second import of it is a syntax error the type checker would catch
  // only after the file had been written.
  const target = path.join(srcDir, 'lib/i18n')
  const already = sf.statements.some(
    (statement) =>
      ts.isImportDeclaration(statement) &&
      ts.isStringLiteral(statement.moduleSpecifier) &&
      path.resolve(path.dirname(file), statement.moduleSpecifier.text) === target,
  )
  if (already) return source
  let relative = path.relative(path.dirname(file), path.join(srcDir, 'lib/i18n')).replace(/\\/g, '/')
  if (!relative.startsWith('.')) relative = `./${relative}`
  const line = `import { t } from '${relative}'`
  const imports = sf.statements.filter((statement) => ts.isImportDeclaration(statement))
  if (imports.length === 0) return `${line}\n\n${source}`
  const end = imports[imports.length - 1].getEnd()
  return `${source.slice(0, end)}\n${line}${source.slice(end)}`
}

/** Rewrites one file and returns what it changed. */
function extractFile(file, report, dry) {
  const raw = fs.readFileSync(file, 'utf8')
  const sf = ts.createSourceFile(file, raw, ts.ScriptTarget.ES2022, true, scriptKindOf(file))
  const area = areaOf(file)
  const table = fs.existsSync(areaPath(area)) ? readTable(areaPath(area)) : new Table(area, file)
  table.area = area
  table.source = file

  const holderOf = makeHolder()
  const lineAt = (node) => sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1
  /** Prose the extractor did not take, for a human to look at. */
  const prose = (text) => /\p{Lu}/u.test(text) && /[a-z]/.test(text)
  /** A module specifier is a path, never a sentence. */
  const isPath = (holder) =>
    ts.isImportDeclaration(holder) || ts.isExportDeclaration(holder) || ts.isImportTypeNode(holder)
  /** A key that is already looked up through `t`/`tn` is done. */
  const isLookup = (node, text, holder) => {
    if (
      ts.isCallExpression(holder) &&
      ts.isIdentifier(holder.expression) &&
      (holder.expression.text === 't' || holder.expression.text === 'tn') &&
      holder.arguments[0] &&
      holds(holder.arguments[0], node)
    ) {
      return true
    }
    // A map of keys (`Record<string, MessageKey>`) is looked up where it is used,
    // so the key written next to a name is already a key and not a sentence.
    if (ts.isPropertyAssignment(holder) && holder.initializer && holds(holder.initializer, node)) {
      return /^[a-z][A-Za-z0-9]*\.[a-z0-9-]+$/.test(text)
    }
    return false
  }
  /** JSX attributes hold `{…}`, everywhere else the call stands on its own. */
  const call = (node, expression) =>
    ts.isJsxAttribute(node.parent) ? `{${expression}}` : expression

  const edits = []
  const visit = (node) => {
    if (ts.isJsxText(node)) {
      const text = tidy(node.text)
      if (text.length > 1 && isReadable(text)) {
        // Only the words themselves are replaced: the whitespace around them is
        // what keeps the element's closing tag on its own line.
        const start = node.end - node.text.length
        const lead = node.text.length - node.text.trimStart().length
        const trail = node.text.length - node.text.trimEnd().length
        edits.push({ start: start + lead, end: node.end - trail, text: `{t('${table.add(text)}')}` })
      }
      return
    }

    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      const text = tidy(node.text)
      if (!text) return
      const holder = holderOf(node)
      const reading = isReadable(text) ? readingOf(node, holder) : undefined
      if (!reading) {
        if (report && prose(text) && !isPath(holder) && !isLookup(node, text, holder)) {
          report.push(`left  ${path.relative(root, file)}:${lineAt(node)}  ${describe(holder)}  ${JSON.stringify(text)}`)
        }
        return
      }
      edits.push({ start: node.getStart(sf), end: node.getEnd(), text: call(node, `t('${table.add(text)}')`) })
      return
    }

    if (ts.isTemplateExpression(node)) {
      const holder = holderOf(node)
      if (!readingOf(node, holder)) {
        if (report && prose(node.head.text)) {
          report.push(`left  ${path.relative(root, file)}:${lineAt(node)}  ${describe(holder)}  ${JSON.stringify(tidy(node.getText(sf)))}`)
        }
        return
      }
      const built = templateOf(node)
      if (!built || !isReadable(built.text)) {
        if (report) {
          report.push(`skip  ${path.relative(root, file)}:${lineAt(node)}  ${JSON.stringify(node.getText(sf))}`)
        }
        return
      }
      const params = built.params.length
        ? `, { ${built.params.map(([name, source]) => (name === source ? name : `${name}: ${source}`)).join(', ')} }`
        : ''
      edits.push({
        start: node.getStart(sf),
        end: node.getEnd(),
        text: call(node, `t('${table.add(built.text)}'${params})`),
      })
    }
  }
  const visitAll = (node) => {
    visit(node)
    node.forEachChild(visitAll)
  }
  visitAll(sf)

  if (edits.length === 0) return 0

  let out = raw
  let last = Infinity
  for (const edit of edits.sort((a, b) => b.start - a.start)) {
    if (edit.end > last) continue
    out = out.slice(0, edit.start) + edit.text + out.slice(edit.end)
    last = edit.start
  }
  out = withImport(out, sf, file)
  if (!dry) {
    fs.writeFileSync(file, out)
    writeTable(table)
  }
  return edits.length
}

/** The kind of syntax a string sat in, for the report. */
function describe(holder) {
  if (!holder) return 'nowhere'
  if (ts.isJsxAttribute(holder)) return `jsx:${holder.name.getText()}`
  if (ts.isPropertyAssignment(holder)) return `prop:${holder.name.getText().replace(/^['"]|['"]$/g, '')}`
  if (ts.isCallExpression(holder)) return `call:${holder.expression.getText().slice(0, 24)}`
  if (ts.isReturnStatement(holder)) return 'return'
  if (ts.isJsxElement(holder) || ts.isJsxFragment(holder)) return 'jsx-child'
  if (ts.isBinaryExpression(holder)) {
    const operator = holder.operatorToken.getText()
    return operator === '+' ? 'concat' : `binary:${operator}`
  }
  if (ts.isArrayLiteralExpression(holder)) return 'array'
  if (ts.isVariableDeclaration(holder)) return 'variable'
  if (ts.isParameter(holder)) return 'parameter'
  if (ts.isJSDoc(holder)) return 'jsdoc'
  return ts.SyntaxKind[holder.kind]
}

// -------------------------------------------------------------- index file --

/**
 * Every source file that may hold a key, with the message tables left out.
 *
 * An area table holds every key twice — once as its own property name and once
 * as the text it was written from — so reading them back would make every key
 * look used.
 */
function sourceFiles() {
  const files = []
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name)
      if (entry.isDirectory()) {
        if (entry.name !== 'i18n') walk(full)
        continue
      }
      if (/\.(ts|tsx)$/.test(entry.name)) files.push(full)
    }
  }
  walk(srcDir)
  return files
}

/**
 * The keys no source file mentions any more, as `area.ts` → keys.
 *
 * A key is a string literal wherever it is used — `t('settingsPane.save')`, a
 * `MessageKey` table entry, the ternary in a title — so "does any file contain
 * this text" is the whole test. The exception is a key built by concatenation,
 * which this script has never produced and the type checker would refuse.
 */
function unusedKeys() {
  const haystack = sourceFiles()
    .map((file) => fs.readFileSync(file, 'utf8'))
    .join('\n')
  const found = new Map()
  for (const name of allAreas()) {
    const table = readTable(path.join(messagesDir, name))
    for (const key of table.entries.keys()) {
      if (haystack.includes(key)) continue
      const area = name.replace(/\.ts$/, '')
      if (!found.has(area)) found.set(area, [])
      found.get(area).push(key)
    }
  }
  return found
}

/**
 * Drops the keys nothing reads, or lists them with `--dry`.
 *
 * A key goes unused when a sentence is rewritten or a component is reworded; a
 * stale row is not an error, but it is one more line for the translator to read
 * and one more chance to translate something that is not shown anywhere. Prune
 * only ever removes, and it prints what it removes, so the diff says whether it
 * guessed right.
 */
function prune(args) {
  const dry = args.includes('--dry')
  const found = unusedKeys()
  let total = 0
  for (const [area, keys] of found) {
    total += keys.length
    if (dry) {
      console.log(`${area}: ${keys.length}`)
      for (const key of keys) console.log(`  ${key}`)
      continue
    }
    const file = path.join(messagesDir, `${area}.ts`)
    const table = readTable(file)
    for (const key of keys) table.entries.delete(key)
    writeTable(table)
    console.log(`${area}: dropped ${keys.length}`)
  }
  console.log(`${total} unused keys${dry ? ' (dry run)' : ' dropped'}`)
}

/**
 * Regenerates `messages/index.ts`.
 *
 * Generated rather than written by hand because the alternative is a list every
 * new area has to be added to by hand, and a line missing from it reads as "this
 * file's messages are all gone" (everything falls back to English) rather than as
 * an error.
 */
function writeIndex() {
  const areas = allAreas().map((name) => name.replace(/\.ts$/, ''))
  const lines = [
    '// Generated by `node scripts/i18n.mjs index` — do not edit.',
    '//',
    '// Every message in the interface, gathered from the area files beside this',
    '// one. Each key carries its area as a prefix, so a line can be traced back to',
    '// the file it came from without a lookup table.',
    '',
  ]
  for (const area of areas) lines.push(`import { ${identifierOf(area)} } from './${area}'`)
  lines.push('', '/** Every message, keyed by `<area>.<slug>`. */', 'export const MESSAGES = {')
  for (const area of areas) lines.push(`  ...${identifierOf(area)},`)
  lines.push(
    '}',
    '',
    '/** The keys a `t` call may use; the type checker knows every one of them. */',
    'export type MessageKey = keyof typeof MESSAGES',
    '',
  )
  fs.writeFileSync(indexPath, lines.join('\n'))
  return areas.length
}

// ---------------------------------------------------------------- commands --

function extract(args) {
  const report = args.includes('--report')
  // A dry run answers "what would this do" without touching the tree, which is
  // the only way to look at a whole directory before deciding to sweep it.
  const dry = args.includes('--dry')
  const files = args.filter((arg) => !arg.startsWith('--'))
  const notes = []
  let total = 0
  for (const file of files) {
    const before = fs.readFileSync(file, 'utf8')
    total += extractFile(file, notes, dry)
    if (!dry && fs.readFileSync(file, 'utf8') !== before) console.log(`rewrote ${path.relative(root, file)}`)
  }
  if (dry) {
    console.log(`${total} strings would be rewritten`)
  } else {
    console.log(`${writeIndex()} area files, ${total} strings rewritten`)
  }
  if (report) for (const note of notes) console.log(`  ${note}`)
}

/**
 * Checks every translation against the English it was written from.
 *
 * Two things go wrong in a translation and neither is a type error: a
 * placeholder loses its braces (the message then shows `{name}` as literal
 * text) and a plural separator disappears (the second form is then never used).
 * Both are mechanical, so both are checked here — key by key, column by column.
 */
function verify() {
  const placeholders = (text) => [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort().join(',')
  let checked = 0
  let empty = 0
  const problems = []
  for (const name of allAreas()) {
    const area = name.replace(/\.ts$/, '')
    const table = readTable(path.join(messagesDir, name))
    for (const [key, columns] of table.entries) {
      const [english, zhCN, zhTW] = columns
      if (!zhCN || !zhTW) {
        empty += 1
        problems.push(`EMPTY  ${area}  ${key}`)
        continue
      }
      checked += 1
      for (const [language, translated] of [
        ['zh-CN', zhCN],
        ['zh-TW', zhTW],
      ]) {
        if (placeholders(english) !== placeholders(translated)) {
          problems.push(`PARAM  ${language}  ${key}\n       en: ${english}\n       tr: ${translated}`)
        }
        if (english.split('|').length !== translated.split('|').length) {
          problems.push(`FORMS  ${language}  ${key}\n       en: ${english}\n       tr: ${translated}`)
        }
      }
    }
  }
  if (!checked && !empty) {
    console.log('nothing to verify')
    return
  }
  console.log(`${checked} translated, ${empty} still in English`)
  for (const problem of problems) console.log(problem)
  if (problems.length) process.exitCode = 1
}

function exportMessages(all) {
  const out = {}
  for (const name of allAreas()) {
    const table = readTable(path.join(messagesDir, name))
    for (const [key, entry] of table.entries) {
      if (all || !entry[1] || !entry[2]) out[key] = entry[0]
    }
  }
  console.log(JSON.stringify(out, null, 2))
}

function fill(file) {
  const wanted = JSON.parse(fs.readFileSync(file, 'utf8'))
  let filled = 0
  for (const name of allAreas()) {
    const file_ = path.join(messagesDir, name)
    const table = readTable(file_)
    let touched = false
    for (const [key, entry] of table.entries) {
      const columns = wanted[key]
      if (!columns) continue
      if (columns[0] || columns[1]) filled++
      if (columns[0]) entry[1] = columns[0]
      if (columns[1]) entry[2] = columns[1]
      touched = true
      delete wanted[key]
    }
    if (touched) writeTable(table)
  }
  console.log(`${filled} messages filled`)
  const unknown = Object.keys(wanted)
  if (unknown.length) {
    console.log(`${unknown.length} keys matched nothing:`)
    for (const key of unknown.slice(0, 20)) console.log(`  ${key}`)
  }
}

function list() {
  const rows = allAreas().map((name) => {
    const table = readTable(path.join(messagesDir, name))
    const entries = [...table.entries.values()]
    const done = entries.filter((entry) => entry[1] && entry[2]).length
    return { name: name.replace(/\.ts$/, ''), done, total: entries.length }
  })
  rows.sort((a, b) => a.done / a.total - b.done / b.total)
  for (const row of rows) {
    console.log(`${((row.done / row.total) * 100).toFixed(0).padStart(3)}%  ${row.done}/${row.total}  ${row.name}`)
  }
  const done = rows.reduce((sum, row) => sum + row.done, 0)
  const total = rows.reduce((sum, row) => sum + row.total, 0)
  console.log(`${((done / total) * 100).toFixed(1)}%  ${done}/${total}  all`)
}

const [command, ...rest] = process.argv.slice(2)
switch (command) {
  case 'extract':
    extract(rest)
    break
  case 'index':
    console.log(`${writeIndex()} area files`)
    break
  case 'export':
    exportMessages(rest.includes('--all'))
    break
  case 'fill':
    fill(rest[0])
    break
  case 'list':
    list()
    break
  case 'prune':
    prune(rest)
    break
  case 'verify':
    verify()
    break
  default:
    console.log(
      'usage: i18n.mjs extract [--report] <file…> | index | export [--all] | fill <file.json> | list | prune [--dry] | verify',
    )
    process.exit(command ? 1 : 0)
}
