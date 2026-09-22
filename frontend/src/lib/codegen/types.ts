/**
 * The shapes object code generation works in.
 *
 * A generator never sees an engine: it is handed columns already reduced to a
 * kind and answers with text. Everything engine-specific — which SQL type is an
 * integer, whether a column may hold NULL — is settled before a language is
 * asked anything, which is how the same templates serve every driver this build
 * ships and any it does not.
 */

/** The kind of value a column holds, once the engine's own name is gone. */
export type ColumnKind =
  | 'int'
  | 'long'
  | 'float'
  | 'double'
  | 'decimal'
  | 'bool'
  | 'string'
  | 'bytes'
  | 'date'
  | 'time'
  | 'datetime'
  | 'json'
  | 'uuid'
  /** A type no table maps: engines keep inventing them, and a field is still a
   * field. Every language names its own "something we cannot describe". */
  | 'unknown'

/** One column of the object, reduced to something a generator can map. */
export interface CodegenField {
  /** The column's name as the catalog has it. */
  name: string
  /** The engine's type as written, for the header and for unmapped kinds. */
  type: string
  kind: ColumnKind
  nullable: boolean
  primaryKey: boolean
  autoIncrement: boolean
  comment?: string
}

/** What a generator is asked for. */
export interface CodegenInput {
  /** The object's own name, which the generated type is named after. */
  object: string
  /** Where it came from — `shop.users` — for the header comment. */
  qualified: string
  /** The object's comment, when the engine keeps one. */
  comment?: string
  fields: CodegenField[]
}

/** One field as a language has rendered it: its name, its type, and its kind. */
export interface RenderedField {
  /** The member's name, spelled the way this language spells them. */
  name: string
  /** The column's own name as the catalog has it — what a tag or an annotation
   * has to keep, since a wire format knows nothing about naming conventions. */
  column: string
  /** Its type in this language, with the NULL treatment already applied. */
  type: string
  kind: ColumnKind
  nullable: boolean
  primaryKey: boolean
  autoIncrement: boolean
  comment?: string
}

/** What a `render` is handed. */
export interface RenderContext {
  /** The generated type's name, in the PascalCase most languages want. One
   * whose convention differs (C, Erlang) derives its own from `object`. */
  name: string
  /** The object's own name as the catalog has it. */
  object: string
  /** Where the object came from, for the header comment. */
  qualified: string
  /** The object's comment, when the engine keeps one. */
  comment?: string
  fields: RenderedField[]
  /** The kinds the fields came out as, so a render can pick its imports. */
  kinds: Set<ColumnKind>
  /** Whether any field accepts NULL, for the same reason. */
  nullable: boolean
}

/** The name conventions a language can put a column's name through. */
export type NameStyle = 'same' | 'camel' | 'pascal' | 'snake'

/** One target language: how it names, types and draws an object. */
export interface CodeLanguage {
  /** Stable id: the state file and the window's picker both key on this. */
  id: string
  /** What the picker and the settings page show. */
  label: string
  /** Extension the window offers when the code is saved to a file. */
  ext: string
  /**
   * The name convention the *file* is named in, when it differs from the type's
   * — Erlang insists a module lives in a file named after it.
   */
  fileStyle?: NameStyle
  /** How a column name becomes a member name here. */
  style: NameStyle
  /** The language's type per kind of column. */
  types: Partial<Record<ColumnKind, string>>
  /** What a column of a kind the table above does not name becomes. */
  fallback: string
  /**
   * How this language says "may hold NULL". A language that cannot say it at
   * all leaves this out and lets the field's comment carry the difference.
   */
  nullable?: (type: string) => string
  /** Draws the whole file. */
  render: (context: RenderContext) => string
}

/* --- names ---------------------------------------------------------------- */

/** The seam between the two words of `userName`. */
const CAMEL_BOUNDARY = /([a-z0-9])([A-Z])/g
/** Everything a name can be separated by that is not part of a word. */
const NOT_A_WORD = /[^A-Za-z0-9]+/

/**
 * A name split into the words it is made of.
 *
 * `user_name`, `userName` and `USER_NAME` all come out as the same two words, so
 * a column keeps its meaning whichever convention the table was written in.
 */
export function words(name: string): string[] {
  return name
    .replace(CAMEL_BOUNDARY, '$1 $2')
    .split(NOT_A_WORD)
    .filter(Boolean)
    .map((word) => word.toLowerCase())
}

const capitalize = (word: string) => word.charAt(0).toUpperCase() + word.slice(1)

/** A column name as a member name, spelled the way the language spells them. */
export function ident(name: string, style: NameStyle): string {
  const parts = words(name)
  let rendered = name
  switch (style) {
    case 'same':
      return name
    case 'camel':
      rendered = parts.map((word, index) => (index === 0 ? word : capitalize(word))).join('')
      break
    case 'pascal':
      rendered = parts.map(capitalize).join('')
      break
    case 'snake':
      rendered = parts.join('_')
      break
  }
  if (!rendered) return '_'
  // `2fa` is a column SQL allows and no language does; an underscore keeps the
  // same characters in the same order rather than dropping one of them.
  return /^[A-Za-z_]/.test(rendered) ? rendered : `_${rendered}`
}

/* --- files ---------------------------------------------------------------- */

/**
 * Joins a file's parts into the text the window shows.
 *
 * A part that is not a string is nothing at all — `false` and `undefined` are
 * how a template hands over a section it does not want — while an empty string
 * is a blank line, which is how a template separates two sections. A run of
 * blank lines collapses to one, so a section that is not there never leaves a
 * gap. Nested arrays are flattened, so a list of members can be a part of its
 * own.
 */
export function codeFile(parts: unknown[]): string {
  const text = (parts.flat(3) as unknown[])
    .filter((part): part is string => typeof part === 'string')
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
  return `${text}\n`
}

/** The file's first line: where the code came from. */
export function generatedLine(context: RenderContext, token: string): string {
  return `${token} Generated from ${context.qualified}.`
}

/** A field's comment as a line of its own, in the language's comment syntax. */
export function noteLine(field: RenderedField, token: string): string {
  return field.comment ? `${token} ${field.comment}` : ''
}

/** The width a struct's type column needs for the member names to line up. */
export function typeWidth(fields: RenderedField[]): number {
  return Math.max(0, ...fields.map((field) => field.type.length))
}
