import type { DriverType } from '../api/types'
import { MongodbConnect } from './MongodbConnect'
import { MysqlConnect } from './MysqlConnect'
import { PostgresConnect } from './PostgresConnect'
import { SqliteConnect } from './SqliteConnect'
import type { DriverForm } from './shared'

/**
 * One page per driver.
 *
 * A driver without an entry here has no form yet, which the picker shows as
 * "planned" instead of opening a page that cannot save anything. Adding a
 * driver means: add the file next to this one and register it here. The
 * explorer, grid and query surfaces are driver agnostic, and the handful of
 * places that are not (the designer, the DDL templates) ask the driver's
 * capabilities rather than its name.
 */
const DRIVER_FORMS: Partial<Record<DriverType, DriverForm>> = {
  mysql: MysqlConnect,
  postgres: PostgresConnect,
  sqlite: SqliteConnect,
  mongodb: MongodbConnect,
}

/** The page for a driver, or `undefined` when we ship no form for it yet. */
export function driverForm(type: DriverType | undefined): DriverForm | undefined {
  return type ? DRIVER_FORMS[type] : undefined
}

/** One line describing the page, for the "new connection" picker. */
export function driverSummary(type: DriverType | undefined): string | undefined {
  return driverForm(type)?.summary
}
