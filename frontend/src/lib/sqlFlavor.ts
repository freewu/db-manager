import type { DriverType } from '../api/types'

/**
 * Which SQL flavour a driver speaks.
 *
 * MySQL, TiDB and Apache Doris all speak the MySQL wire protocol against a
 * MySQL-compatible catalog, so they quote identifiers with backticks, write a
 * single `database.object` qualifier, and want the MySQL dialect in the
 * editor. Asking that question by name in a dozen places is how a new engine
 * ends up half-correct — quoted as if it were PostgreSQL in the export
 * script, highlighted as PostgreSQL in the editor — so it is asked here once.
 */
export function isMySQLFamily(driver: DriverType | string | undefined): boolean {
  return driver === 'mysql' || driver === 'tidb' || driver === 'doris'
}
