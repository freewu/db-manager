/**
 * Object code generation.
 *
 * One table, view or collection becomes a class, a struct or a record in the
 * language the user picked: one member per column, its comment above it, and a
 * type mapped from the engine's own. The languages themselves live in
 * `languages.ts`; this module is the part that knows about SQL — which of the
 * engine's type names is an integer, and what a column becomes before any
 * language sees it.
 *
 * Generation happens here, in the frontend, for the same reason result export
 * does: the columns are already in hand, so switching language costs nothing and
 * no text has to cross the bridge. Nothing here reads the server or writes to
 * disk.
 */
import type { ColumnInfo, ObjectKind, TableStructure } from '../../api/types'
import { CODE_LANGUAGES } from './languages'
import { ident, type CodeLanguage, type CodegenField, type CodegenInput, type ColumnKind, type RenderContext, type RenderedField } from './types'

export { CODE_LANGUAGES }
export { tokenizeCode } from './highlight'
export type { CodeToken, CodeTokenKind } from './highlight'
export type { CodegenField, CodegenInput, CodeLanguage, ColumnKind } from './types'

/**
 * The language a code window opens in.
 *
 * Java with Lombok because that is where most objects of this kind end up being
 * written; it is a default, not a setting that cannot be changed — the picker in
 * the window and the settings page both move it.
 */
export const DEFAULT_CODE_LANGUAGE = 'java'

/** An object has fields to generate from only if it is one of these. */
const FIELD_KINDS: ObjectKind[] = ['table', 'view', 'materialized_view', 'collection']

/**
 * Whether an object has fields a generator can work from.
 *
 * This asks what the object *is*, never which engine it is on: an engine gives
 * a table, a view and a collection a field list, while a sequence and a stored
 * procedure have none to give, and a "Generate code" that opens onto nothing is
 * worse than not being offered.
 */
export function generatable(kind: ObjectKind): boolean {
  return FIELD_KINDS.includes(kind)
}

/** The language with that id, or the default when the id is not one we know. */
export function codeLanguageById(id: string | undefined): CodeLanguage {
  return (
    CODE_LANGUAGES.find((language) => language.id === id) ??
    CODE_LANGUAGES.find((language) => language.id === DEFAULT_CODE_LANGUAGE) ??
    CODE_LANGUAGES[0]
  )
}

/* --- SQL types ------------------------------------------------------------ */

/**
 * What an engine's type names mean, tried in order against the bare name.
 *
 * The order matters where one name starts another: `bigint` is a long before
 * `int` can see it, `datetime` is a timestamp before `date` can, and a `text`
 * column is a string rather than the bytes its name suggests.
 */
const KIND_PATTERNS: { kind: ColumnKind; match: RegExp }[] = [
  { kind: 'bool', match: /^(bool|boolean|logical)\b/ },
  { kind: 'long', match: /^(bigint|bigserial|int8|int64|long)\b/ },
  { kind: 'int', match: /^(int|integer|int2|int4|int32|serial|smallserial|mediumint|smallint|tinyint|year)\b/ },
  { kind: 'float', match: /^(real|float|float4)\b/ },
  { kind: 'double', match: /^(double|float8|binary_double)\b/ },
  { kind: 'decimal', match: /^(decimal|numeric|number|money|smallmoney|dec)\b/ },
  { kind: 'uuid', match: /^(uuid|uniqueidentifier)\b/ },
  { kind: 'json', match: /^(json|jsonb|object|map)\b/ },
  { kind: 'bytes', match: /^(bytea|blob|binary|varbinary|tinyblob|mediumblob|longblob|image|raw|bindata|bit)\b/ },
  { kind: 'datetime', match: /^(datetime|timestamp|timestamptz|smalldatetime|interval)\b/ },
  { kind: 'date', match: /^date\b/ },
  { kind: 'time', match: /^time/ },
  {
    kind: 'string',
    match:
      /^(char|character|varchar|nvarchar|nchar|varying|text|tinytext|mediumtext|longtext|clob|nclob|string|enum|set|citext|name|xml|objectid|regex|javascript|symbol)\b/
  },
]

/**
 * The kind of value a column holds.
 *
 * The type is read as the engine writes it, with whatever arguments it carries
 * (`varchar(255)`) and whatever follows them (`unsigned`, `without time zone`)
 * cut away first, so one entry covers a whole family of types.
 */
export function kindOfColumn(column: ColumnInfo): ColumnKind {
  const written = (column.columnType || column.dataType || '').toLowerCase()
  // MySQL answers `tinyint(1)` for a boolean and `tinyint` for a number, and the
  // width is the only thing that tells them apart.
  if (/^tinyint\(1\)/.test(written)) return 'bool'
  const bare = written.replace(/\(.*?\)/g, ' ').replace(/\s+/g, ' ').trim()
  // An array of something else. PostgreSQL writes `integer[]`, and a member of
  // it is not the integer the name promises.
  if (bare.includes('[')) return 'unknown'
  for (const { kind, match } of KIND_PATTERNS) {
    if (match.test(bare)) return kind
  }
  return 'unknown'
}

/** The columns of a structure, reduced to what a generator maps. */
export function fieldsOf(structure: TableStructure): CodegenField[] {
  return structure.columns.map((column) => ({
    name: column.name,
    type: column.columnType || column.dataType || '',
    kind: kindOfColumn(column),
    nullable: column.nullable,
    primaryKey: column.primaryKey,
    autoIncrement: column.autoIncrement,
    comment: column.comment || undefined,
  }))
}

/* --- generation ----------------------------------------------------------- */

/**
 * The generated file for one object, in one language.
 *
 * A field's type is looked up by kind, with the language's fallback standing in
 * for a kind it has no entry for, and the language's own NULL treatment applied
 * on top — a language that cannot express one leaves the type alone and the
 * comment says so.
 */
export function generateCode(languageId: string | undefined, input: CodegenInput): string {
  const language = codeLanguageById(languageId)
  const kinds = new Set<ColumnKind>()
  const fields: RenderedField[] = input.fields.map((field) => {
    kinds.add(field.kind)
    const base = language.types[field.kind] ?? language.fallback
    return {
      name: ident(field.name, language.style),
      column: field.name,
      type: field.nullable && language.nullable ? language.nullable(base) : base,
      kind: field.kind,
      nullable: field.nullable,
      primaryKey: field.primaryKey,
      autoIncrement: field.autoIncrement,
      comment: field.comment,
    }
  })
  const context: RenderContext = {
    name: ident(input.object, 'pascal'),
    object: input.object,
    qualified: input.qualified,
    comment: input.comment,
    fields,
    kinds,
    nullable: fields.some((field) => field.nullable),
  }
  return language.render(context)
}

/** The file name the window offers when the generated code is saved. */
export function codeFileName(languageId: string | undefined, object: string): string {
  const language = codeLanguageById(languageId)
  return `${ident(object, language.fileStyle ?? 'pascal')}.${language.ext}`
}
