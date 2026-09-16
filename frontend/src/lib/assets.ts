import mongoIcon from '@asserts/icon/mongoDB.png'
import mysqlIcon from '@asserts/icon/MYSQL.png'
import postgresIcon from '@asserts/icon/postgresql.png'
import sqliteIcon from '@asserts/icon/SQLite.png'
import logo from '@asserts/logo.png'

import type { DriverType } from '../api/types'

/**
 * Brand artwork.
 *
 * The files live in the repository-level `asserts` folder (`@asserts`), which
 * is also the source for the packaged desktop icon, so there is exactly one
 * copy of every image.
 */
export const appLogo = logo

const DRIVER_ICONS: Partial<Record<DriverType, string>> = {
  mysql: mysqlIcon,
  postgres: postgresIcon,
  sqlite: sqliteIcon,
  mongodb: mongoIcon,
}

/** Vendor logo for a driver, or `undefined` when we ship no artwork for it. */
export function driverIcon(type: DriverType | undefined): string | undefined {
  return type ? DRIVER_ICONS[type] : undefined
}

/** Same as {@link driverIcon}, but never empty — falls back to the app logo. */
export function driverIconOrLogo(type: DriverType | undefined): string {
  return driverIcon(type) ?? logo
}
