import type { DriverInfo, DriverType } from '../api/types'

/**
 * What the UI is allowed to show for a driver.
 *
 * The backend registry already answers this: `DriverInfo` carries the
 * capabilities, and the frontend must not second-guess it by name. Asking
 * "is this relational" as `driver !== 'mongodb'` would work exactly once and
 * then rot the moment a second document store is added, so every surface that
 * is SQL-only (the table designer, ER diagrams, the DDL editor, `CREATE
 * DATABASE`) asks these helpers instead.
 */
export interface Capabilities {
  /**
   * Tables with typed columns, keys and DDL. False for a document store, whose
   * objects are collections with no schema to design.
   */
  relational: boolean
  /**
   * The server holds several namespaces, so "New database" is offered. This is
   * *not* `relational`: MongoDB has no CREATE DATABASE and no tables, yet it
   * holds databases, and its driver renders the `use <name>` that selects one.
   * Engines whose databases are files (SQLite) say `supportsDatabase: false`.
   */
  createDatabase: boolean
  /** Objects live directly under the database (no schema level in between). */
  flatNamespace: boolean
  /**
   * The table designer can edit this engine's tables. False for engines whose
   * DDL needs more than the designer models, which then stays read-only.
   */
  designable: boolean
}

/**
 * Capabilities of a driver, defaulting to the relational behaviour for a
 * driver that is not in the registry (an ad-hoc connection to a type this build
 * does not know).
 */
export function capabilitiesOf(driver: DriverInfo | undefined): Capabilities {
  return {
    relational: driver ? driver.relational : true,
    createDatabase: driver ? driver.supportsDatabase : true,
    flatNamespace: !(driver?.supportsSchema ?? false),
    // A driver we ship no record for is treated as a plain SQL engine, which is
    // what every build so far has been.
    designable: driver ? driver.supportsDesign : true,
  }
}

/** Looks a driver up in the registry by type. */
export function findDriver(
  drivers: DriverInfo[],
  type: DriverType | undefined,
): DriverInfo | undefined {
  return drivers.find((driver) => driver.type === type)
}
