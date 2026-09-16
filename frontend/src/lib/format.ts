/** Presentation helpers shared across panes. */

import type { CellValue, ColumnMeta } from '../api/types'

/** Formats a byte count using binary units. */
export function formatBytes(bytes: number | null | undefined): string {
  // Negative values are the "unknown" sentinel some engines use.
  if (bytes == null || bytes < 0 || Number.isNaN(bytes)) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let value = bytes / 1024
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[unitIndex]}`
}

/** Formats a row count, tolerating the -1 sentinel some engines return. */
export function formatCount(count: number | null | undefined): string {
  if (count == null || count < 0) return '—'
  return count.toLocaleString()
}

export function formatDuration(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

export function formatTimestamp(ms: number | null | undefined): string {
  if (!ms) return '—'
  return new Date(ms).toLocaleString()
}

/** Renders a cell value for display in a table cell. */
export function displayValue(value: CellValue): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value)
}

/** Truncates long values so a grid row stays one line tall. */
export function truncate(value: string, max = 200): string {
  if (value.length <= max) return value
  return `${value.slice(0, max)}…`
}

/**
 * Renders a value as a SQL literal.
 *
 * Only used to build statements the user reviews before running (INSERT/UPDATE
 * previews) — the app never concatenates user data into executed SQL.
 */
export function sqlLiteral(value: CellValue): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'number') return String(value)
  if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE'
  return `'${String(value).replace(/'/g, "''")}'`
}

/** Quotes an identifier for a specific engine. */
export function quoteIdent(name: string, driver: string): string {
  switch (driver) {
    case 'mysql':
      return `\`${name.replace(/`/g, '``')}\``
    case 'sqlserver':
      return `[${name.replace(/]/g, ']]')}]`
    default:
      return `"${name.replace(/"/g, '""')}"`
  }
}

/** Builds the fully qualified name of an object for a specific engine. */
export function qualifiedName(
  driver: string,
  database: string | undefined,
  schema: string | undefined,
  object: string,
): string {
  const parts: string[] = []
  if (driver === 'mysql' && database) parts.push(quoteIdent(database, driver))
  if (driver !== 'mysql' && driver !== 'sqlite' && schema) parts.push(quoteIdent(schema, driver))
  if (driver === 'sqlite' && database && database !== 'main') {
    parts.push(quoteIdent(database, driver))
  }
  parts.push(quoteIdent(object, driver))
  return parts.join('.')
}

/** Picks a sensible editor height for a value preview. */
export function isLongValue(value: CellValue): boolean {
  return typeof value === 'string' && value.length > 120
}

/** Case-insensitive column lookup used when matching result columns to keys. */
export function findColumn(
  columns: ColumnMeta[],
  name: string,
): ColumnMeta | undefined {
  const lower = name.toLowerCase()
  return columns.find((c) => c.name.toLowerCase() === lower)
}
