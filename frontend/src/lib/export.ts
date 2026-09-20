/**
 * Result set serialisation.
 *
 * Export happens in the frontend so that no result data has to travel back
 * across the bridge; the backend only writes the finished string to disk.
 */
import type { CellValue, ColumnMeta, QueryResult } from '../api/types'

function csvCell(value: CellValue): string {
  if (value === null || value === undefined) return ''
  const text = typeof value === 'string' ? value : String(value)
  if (/[",\r\n]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

/** Serialises a result set as RFC 4180 CSV. */
export function toCSV(columns: ColumnMeta[], rows: CellValue[][]): string {
  const header = columns.map((c) => csvCell(c.name)).join(',')
  const body = rows.map((row) => row.map(csvCell).join(','))
  return [header, ...body].join('\r\n')
}

/** Serialises a result set as pretty printed JSON objects. */
export function toJSON(columns: ColumnMeta[], rows: CellValue[][]): string {
  const objects = rows.map((row) => {
    const record: Record<string, CellValue> = {}
    columns.forEach((column, index) => {
      record[column.name] = row[index] ?? null
    })
    return record
  })
  return JSON.stringify(objects, null, 2)
}

/**
 * Where an export is going: the object plus the namespace that contains it.
 *
 * The namespace travels separately instead of being baked into a qualified
 * name because no level of it can be recovered by splitting that name — a
 * MongoDB collection may contain a dot, and a database may need quoting.
 */
export interface ExportTarget {
  driver: string
  database?: string
  schema?: string
  object: string
}

/** Serialises a result set as an INSERT script (best effort, data only). */
export function toInsertScript(
  target: ExportTarget,
  columns: ColumnMeta[],
  rows: CellValue[][],
): string {
  if (target.driver === 'mongodb') {
    return toInsertScriptMongo(target, columns, rows)
  }

  const { driver } = target
  const table = qualify(target)
  const names = columns.map((c) => c.name)
  const literal = (value: CellValue): string => {
    if (value === null || value === undefined) return 'NULL'
    if (typeof value === 'number') return String(value)
    if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE'
    return `'${String(value).replace(/'/g, "''")}'`
  }
  const quotedTable = driver === 'mysql' ? `\`${table}\`` : `"${table}"`
  const columnList = names
    .map((n) => (driver === 'mysql' ? `\`${n}\`` : `"${n}"`))
    .join(', ')
  return rows
    .map(
      (row) =>
        `INSERT INTO ${quotedTable} (${columnList}) VALUES (${row.map(literal).join(', ')});`,
    )
    .join('\n')
}

/** `db.collection` for a document store, `schema.table` for the SQL engines. */
function qualify(target: ExportTarget): string {
  const { driver, database, schema, object } = target
  const parts: string[] = []
  if (driver === 'mysql' && database) parts.push(database)
  if (driver !== 'mysql' && driver !== 'sqlite' && schema) parts.push(schema)
  if (driver === 'sqlite' && database && database !== 'main') parts.push(database)
  parts.push(object)
  return parts.join('.')
}

/**
 * Serialises a result set as one `insertMany` call.
 *
 * `db.getCollection(<name>)` is used rather than `db.<name>` so that a
 * collection whose name is not a plain identifier still produces a runnable
 * script.
 */
function toInsertScriptMongo(
  target: ExportTarget,
  columns: ColumnMeta[],
  rows: CellValue[][],
): string {
  const name = target.object
  const collection = `db.getCollection(${JSON.stringify(name)})`
  const documents = rows.map((row) => {
    const fields = columns.map((column, index) => {
      const value = row[index] ?? null
      return `  ${JSON.stringify(column.name)}: ${mongoLiteral(column, value)}`
    })
    return `{\n${fields.join(',\n')}\n}`
  })
  return [
    `// ${rows.length} document(s) from ${qualify(target)}`,
    `${collection}.insertMany([`,
    documents.map((doc) => doc.replace(/^/gm, '  ')).join(',\n'),
    '])',
    '',
  ].join('\n')
}

/**
 * Renders one cell as the literal `insertMany` should carry.
 *
 * The grid shows every BSON value as text, so the type the sampler inferred for
 * the column plus the shape of the text decide how it is written back. The rule
 * is conservative on purpose: anything that is not recognised ends up as a
 * JSON string, because a script that inserts a string where a date belonged is
 * recoverable, while a script that fails to parse is not.
 */
function mongoLiteral(column: ColumnMeta, value: CellValue): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'boolean') return String(value)
  const type = (column.databaseType ?? '').toLowerCase()
  const has = (name: string) => type === name || type.split('|').some((part) => part.trim() === name)
  const text = String(value)

  if (typeof value === 'number') {
    return has('decimal') ? `{"$numberDecimal":${JSON.stringify(text)}}` : text
  }

  // ObjectID and dates lose their BSON identity in the grid: an id is a bare
  // hex string and a date is ISO text, so both have to be rebuilt.
  if (has('objectid') && /^[0-9a-f]{24}$/i.test(text)) {
    return `{"$oid":${JSON.stringify(text)}}`
  }
  if (has('date')) {
    const when = new Date(text)
    if (!Number.isNaN(when.getTime())) {
      return `{"$date":${JSON.stringify(when.toISOString())}}`
    }
  }
  const stamp = /^Timestamp\((\d+),\s*(\d+)\)$/.exec(text)
  if (has('timestamp') && stamp) {
    return `{"$timestamp":{"t":${stamp[1]},"i":${stamp[2]}}}`
  }
  if (has('regex')) return mongoRegexLiteral(text)

  // Documents, arrays and the remaining BSON types are already extended JSON.
  if (EXTENDED_TYPES.some(has) && isJSON(text)) return text
  return JSON.stringify(text)
}

/**
 * BSON types whose text form is extended JSON. ObjectID, date, decimal and
 * timestamp are absent because their text form is not: those are rebuilt above.
 */
const EXTENDED_TYPES = ['object', 'array', 'bindata', 'javascript', 'symbol', 'minkey', 'maxkey', 'undefined']

/** Rebuilds a regex cell, which the grid shows as `/pattern/options`. */
function mongoRegexLiteral(text: string): string {
  const match = /^\/(.*)\/([a-z]*)$/.exec(text)
  const pattern = match ? match[1] : text
  const options = match ? match[2] : ''
  return `{"$regularExpression":{"pattern":${JSON.stringify(pattern)},"options":${JSON.stringify(options)}}}`
}

/** True when the text is a JSON document or array the shell can carry as-is. */
function isJSON(text: string): boolean {
  if (!/^[[{]/.test(text)) return false
  try {
    JSON.parse(text)
    return true
  } catch {
    return false
  }
}

/** Convenience wrapper around a QueryResult. */
export function resultToCSV(result: QueryResult): string {
  return toCSV(result.columns, result.rows)
}

export function resultToJSON(result: QueryResult): string {
  return toJSON(result.columns, result.rows)
}

/** Triggers a browser download; used as a fallback outside the desktop shell. */
export function downloadText(filename: string, content: string, mime: string): void {
  const blob = new Blob([content], { type: mime })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(url)
}
