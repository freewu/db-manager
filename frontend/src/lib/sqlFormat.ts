import { format, type SqlLanguage } from 'sql-formatter'

import type { DriverType } from '../api/types'

/**
 * The sql-formatter grammar name for a driver, or undefined when there is none.
 *
 * This is the one place the frontend spells out a driver name for a *grammar*,
 * which is different from second-guessing a capability: the formatter takes a
 * dialect by name, and the name a parser uses for "Transact-SQL" is not
 * something the backend could hand over. MongoDB answers undefined on purpose —
 * its statements are shell calls, and there is no SQL grammar to lay them out
 * with, so the caller disables the button rather than mangling the script.
 *
 * An unknown driver is formatted as generic SQL, matching the editor, which
 * highlights it as `StandardSQL` for the same reason.
 */
export function formatterDialect(driver: DriverType | undefined): SqlLanguage | undefined {
  switch (driver) {
    case 'mysql':
    // Doris speaks the MySQL dialect (its own EXPLAIN rows included), and the
    // formatter has no grammar further up the tree for it.
    case 'doris':
      return 'mysql'
    case 'tidb':
      return 'tidb'
    case 'postgres':
      return 'postgresql'
    case 'sqlite':
      return 'sqlite'
    case 'sqlserver':
      return 'transactsql'
    case 'oracle':
      return 'plsql'
    case 'mongodb':
      return undefined
    default:
      return 'sql'
  }
}

export interface FormatResult {
  /** The formatted text. Absent when the text could not be formatted. */
  formatted?: string
  /** Why the text could not be formatted. The caller shows this and changes nothing. */
  error?: string
  /** The engine's statements are not SQL, so there is nothing to format them with. */
  unsupported?: boolean
}

/**
 * Re-indents a statement (or a whole script) without changing what it means.
 *
 * Keywords are upper-cased, which is the one opinion in here: everything else
 * about the output is whitespace. A statement the grammar cannot parse comes
 * back as an error and the editor is left exactly as it was — a half-formatted
 * script is worse than an unformatted one.
 */
export function formatSql(sql: string, driver: DriverType | undefined): FormatResult {
  const language = formatterDialect(driver)
  if (!language) return { unsupported: true }
  if (!sql.trim()) return { formatted: sql }

  try {
    return { formatted: format(sql, { language, keywordCase: 'upper', tabWidth: 2 }) }
  } catch (err) {
    return { error: err instanceof Error ? err.message : String(err) }
  }
}
