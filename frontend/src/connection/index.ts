import type { DriverType } from '../api/types'
import { MysqlConnect } from './MysqlConnect'
import { PostgresConnect } from './PostgresConnect'
import { SqliteConnect } from './SqliteConnect'
import type { DriverForm } from './shared'

/**
 * One page per driver.
 *
 * A driver without an entry here has no form yet, which the picker shows as
 * "planned" instead of opening a page that cannot save anything. Adding a
 * driver means: add the file next to this one, register it here, and (if it is
 * not one of the three) the explorer/query surfaces start working as soon as
 * the backend driver does.
 */
const DRIVER_FORMS: Partial<Record<DriverType, DriverForm>> = {
  mysql: MysqlConnect,
  postgres: PostgresConnect,
  sqlite: SqliteConnect,
}

/** The page for a driver, or `undefined` when we ship no form for it yet. */
export function driverForm(type: DriverType | undefined): DriverForm | undefined {
  return type ? DRIVER_FORMS[type] : undefined
}

/** One line describing the page, for the "new connection" picker. */
export function driverSummary(type: DriverType | undefined): string | undefined {
  return driverForm(type)?.summary
}
