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
  /** A server that holds several namespaces, so `CREATE DATABASE` makes sense. */
  creatableDatabase: boolean
  /** Objects live directly under the database (no schema level in between). */
  flatNamespace: boolean
}

/**
 * Capabilities of a driver, defaulting to the relational behaviour for a
 * driver that is not in the registry (an ad-hoc connection to a type this build
 * does not know).
 */
export function capabilitiesOf(driver: DriverInfo | undefined): Capabilities {
  return {
    relational: driver ? driver.relational : true,
    creatableDatabase: driver
      ? driver.supportsDatabase && !driver.requiresFile && driver.relational
      : true,
    flatNamespace: !(driver?.supportsSchema ?? false),
  }
}

/** Looks a driver up in the registry by type. */
export function findDriver(
  drivers: DriverInfo[],
  type: DriverType | undefined,
): DriverInfo | undefined {
  return drivers.find((driver) => driver.type === type)
}
