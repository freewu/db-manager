#!/usr/bin/env node
//
// Check the introduction site in docs/.
//
//   node scripts/docs-check.mjs
//
// The site is a handful of static files (index.html, site.css, i18n.js,
// site.js, version.json) with no build step, so nothing else would notice a
// language that lost a key, a `data-i18n` pointing at a key that no longer
// exists, or a screenshot that was renamed. This script is that notice: it
// fails, with the list of problems, in CI and before a commit.
//
// `docs/` is what gets published, and it is also what you get by opening
// docs/index.html — so every reference it makes has to stay inside it. A `../`
// reference would work from a checkout and 404 once the directory is published,
// which is why one is a failure here rather than a path to resolve.
//
// Only the Node standard library is used, so it runs anywhere `node` does.

import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'
import vm from 'node:vm'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const docs = join(root, 'docs')

const problems = []
const notes = []

function read(path) {
  return readFileSync(path, 'utf8')
}

function fail(message) {
  problems.push(message)
}

function report(label, summary) {
  if (problems.length) {
    console.error(`${label}: ${problems.length} problem(s)\n`)
    for (const problem of problems) console.error(`  - ${problem}`)
    process.exit(1)
  }
  if (notes.length) console.log(notes.map((note) => `note: ${note}`).join('\n'))
  console.log(`${label}: ok — ${summary}`)
}

/**
 * Every reference to a local file in a document, as written: `src`/`href`
 * attributes, `url(…)` in a stylesheet, and single-quoted strings that name a
 * file (`fetch('version.json')`). Absolute URLs, fragments and data URIs are none
 * of our business. Quoted *double* strings are left alone: they are what an
 * attribute value already looks like, and reporting those twice would only
 * make the list harder to read.
 */
function localRefs(text) {
  return [
    ...[...text.matchAll(/(?:src|href)="([^"]+)"/g)].map((m) => m[1]),
    ...[...text.matchAll(/url\(\s*['"]?([^'")]+)['"]?\s*\)/g)].map((m) => m[1]),
    ...[...text.matchAll(/'([A-Za-z0-9_./-]+\.(?:json|png|jpe?g|svg|webp|css|js|html))'/g)].map(
      (m) => m[1],
    ),
  ].filter((url) => url && !/^(https?:|\/\/|#|mailto:|data:)/.test(url))
}

/* ------------------------------------------------------------ dictionaries */

const context = { window: {} }
vm.createContext(context)
vm.runInContext(read(join(docs, 'i18n.js')), context, { filename: 'docs/i18n.js' })

const site = context.window.DM_DOCS

if (!site || !site.messages) {
  console.error('docs/i18n.js did not define window.DM_DOCS.messages')
  process.exit(1)
}

const languages = Object.keys(site.messages)
if (languages.length < 2) fail(`only ${languages.length} language(s) defined`)

const fallback = site.fallbackLang || site.defaultLang
if (!site.messages[fallback]) fail(`the fallback language "${fallback}" has no dictionary`)
if (!site.messages[site.defaultLang]) fail(`the default language "${site.defaultLang}" has no dictionary`)

// The language buttons in the markup have to lead somewhere, and every
// dictionary has to be reachable from them.
// Strip comments: the markup documents the conventions in a comment, and the
// keys "documented" there are placeholders, not real keys.
const html = read(join(docs, 'index.html')).replace(/<!--[\s\S]*?-->/g, '')
const js = read(join(docs, 'site.js'))

const buttonCodes = [...html.matchAll(/class="langs"[\s\S]*?<\/div>/g)]
  .flatMap((block) => [...block[0].matchAll(/data-lang="([^"]+)"/g)])
  .map((m) => m[1])
for (const code of languages) {
  if (!buttonCodes.includes(code)) fail(`docs/index.html has no button for the "${code}" dictionary`)
}
// The codes baked into the markup (the switcher) come from the dictionary list.
for (const entry of site.languages) {
  if (!site.messages[entry.code]) fail(`language "${entry.code}" is listed but has no dictionary`)
  if (!buttonCodes.includes(entry.code)) fail(`language "${entry.code}" is listed but has no button`)
}

/* ---------------------------------------------------------------- keys */

const base = site.messages[fallback]
const baseKeys = Object.keys(base).sort()
const known = new Set(baseKeys)

for (const code of languages) {
  if (code === fallback) continue
  const keys = new Set(Object.keys(site.messages[code]))
  for (const key of baseKeys) {
    if (!keys.has(key)) fail(`${code}: missing key "${key}"`)
  }
  for (const key of keys) {
    if (!known.has(key)) fail(`${code}: key "${key}" is not in ${fallback}`)
  }
}

for (const code of languages) {
  for (const [key, value] of Object.entries(site.messages[code])) {
    if (typeof value !== 'string') fail(`${code}: "${key}" is not a string`)
    else if (value.trim() === '') fail(`${code}: "${key}" is empty`)
  }
}

/* ------------------------------------------------------------- the markup */

function checkKeys(list, where) {
  for (const key of list) {
    if (!known.has(key)) fail(`${where} asks for the missing key "${key}"`)
  }
}

checkKeys(
  [...html.matchAll(/data-i18n="([^"]+)"/g)].map((m) => m[1]),
  'docs/index.html (data-i18n)',
)
checkKeys(
  [...html.matchAll(/data-i18n-html="([^"]+)"/g)].map((m) => m[1]),
  'docs/index.html (data-i18n-html)',
)
checkKeys(
  [...html.matchAll(/data-i18n-attr="([^"]+)"/g)]
    .flatMap((m) => m[1].split(';'))
    .map((pair) => pair.slice(pair.indexOf(':') + 1).trim())
    .filter(Boolean),
  'docs/index.html (data-i18n-attr)',
)
checkKeys(
  // `t('shot.' + id + '.title')` builds its key at run time; only literal keys
  // are checked here (the ids themselves are checked further down).
  [...js.matchAll(/\bt\('([^']+)'(?!\s*\+)/g)].map((m) => m[1]),
  'docs/site.js',
)

// Screenshots: the ids in the markup have to have copy, and the files have to
// be there. `shot.<id>.title` is built from the id at run time, so the checker
// is the only thing that can see a typo in it.
const shotIds = [
  ...new Set(
    [...html.matchAll(/data-shot(?:-link)?="([^"]+)"/g)].map((m) => m[1]),
  ),
].sort()
for (const id of shotIds) {
  for (const part of ['title', 'desc', 'chips']) {
    if (!known.has(`shot.${id}.${part}`)) fail(`screenshot "${id}" has no shot.${id}.${part}`)
  }
}
// And the other way round: an image nothing shows is a file that was dropped
// from the page by accident.
const images = readdirSync(join(docs, 'images')).filter((name) => name.endsWith('.png'))
for (const name of images) {
  const id = name.replace(/\.png$/, '')
  if (!shotIds.includes(id)) fail(`docs/images/${name} is not shown anywhere on the page`)
}

/* ----------------------------------------------------------------- files */

const referenced = new Set()
const documents = [
  ['docs/index.html', html],
  ['docs/site.js', js],
  ['docs/site.css', read(join(docs, 'site.css'))],
]
for (const [name, text] of documents) {
  for (const url of localRefs(text)) {
    if (referenced.has(url)) continue
    referenced.add(url)
    // Everything the page needs is beside it — the engine marks in engine/, the
    // screenshots in images/, the version in version.json — so a reference that
    // climbs out of docs/ is a file the published site would not have.
    if (url.startsWith('../')) {
      fail(`${name} points outside docs/: ${url}`)
      continue
    }
    if (!existsSync(join(docs, url.split('#')[0]))) {
      fail(`${name} points at ${url}, which does not exist`)
    }
  }
}

/* ---------------------------------------------------------------- report */

report(
  'docs',
  `${baseKeys.length} keys in ${languages.length} languages, ` +
    `${shotIds.length} screenshots, ${referenced.size} references, all inside docs/`,
)
