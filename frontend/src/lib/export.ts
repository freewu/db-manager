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

/** Serialises a result set as an INSERT script (best effort, data only). */
export function toInsertScript(
  table: string,
  columns: ColumnMeta[],
  rows: CellValue[][],
  driver: string,
): string {
  const names = columns.map((c) => c.name)
  const literal = (value: CellValue): string => {
    if (value === null || value === undefined) return 'NULL'
    if (typeof value === 'number') return String(value)
    if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE'
    return `'${String(value).replace(/'/g, "''")}'`
  }
  const target = driver === 'mysql' ? `\`${table}\`` : `"${table}"`
  const columnList = names
    .map((n) => (driver === 'mysql' ? `\`${n}\`` : `"${n}"`))
    .join(', ')
  return rows
    .map(
      (row) =>
        `INSERT INTO ${target} (${columnList}) VALUES (${row.map(literal).join(', ')});`,
    )
    .join('\n')
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
