/**
 * Tree node identities for the object explorer.
 *
 * Keys must survive a JSON round trip because Ant Design's Tree only gives us
 * the key back in its callbacks. JSON keeps arbitrary identifiers (dots,
 * spaces, unicode) unambiguous.
 */
import type { ObjectKind } from '../api/types'

export type NodeRef =
  | { t: 'connection'; connectionId: string }
  | { t: 'session'; sessionId: string }
  /** A user-made folder holding connections; it belongs to no session. */
  | { t: 'group'; groupId: string }
  | { t: 'db'; sessionId: string; database: string }
  | { t: 'schema'; sessionId: string; database: string; schema: string }
  | {
      t: 'folder'
      sessionId: string
      database: string
      schema: string
      kind: ObjectKind
    }
  | {
      t: 'indexFolder'
      sessionId: string
      database: string
      schema: string
    }
  | {
      t: 'index'
      sessionId: string
      database: string
      schema: string
      index: string
      table: string
    }
  | {
      t: 'object'
      sessionId: string
      database: string
      schema: string
      object: string
      kind: ObjectKind
    }
  /**
   * The folder of saved scripts, under a *database*.
   *
   * A script is scoped to the database it was written for, not to a schema
   * inside it, so this folder hangs off the database node even where the object
   * folders live under schemas (PostgreSQL) — that deviation is deliberate, and
   * is what makes the same script reachable from both schemas of one database.
   */
  | { t: 'queries'; sessionId: string; database: string }
  | { t: 'queryFile'; sessionId: string; database: string; name: string }

export function encodeNode(ref: NodeRef): string {
  return JSON.stringify(ref)
}

export function decodeNode(key: string): NodeRef | null {
  try {
    const parsed = JSON.parse(key) as NodeRef
    if (parsed && typeof parsed === 'object' && 't' in parsed) return parsed
    return null
  } catch {
    return null
  }
}

/** Separator that cannot appear in an identifier. */
const SEP = '\u0000'

export const namespaceKey = (sessionId: string, database: string, schema: string) =>
  [sessionId, database, schema].join(SEP)

export const databaseKey = (sessionId: string, database: string) =>
  [sessionId, database].join(SEP)

export const objectsKey = (sessionId: string, database: string, schema: string) =>
  [sessionId, database, schema, 'objects'].join(SEP)

export const indexesKey = (sessionId: string, database: string, schema: string) =>
  [sessionId, database, schema, 'indexes'].join(SEP)

/** The saved scripts of one database; they have no schema of their own. */
export const queriesKey = (sessionId: string, database: string) =>
  [sessionId, database, 'queries'].join(SEP)

/** Object kinds the explorer groups into folders. */
export const FOLDER_ORDER: ObjectKind[] = [
  'table',
  'view',
  'materialized_view',
  'collection',
  'sequence',
  'procedure',
]

export const FOLDER_LABEL: Record<ObjectKind, string> = {
  table: 'Tables',
  view: 'Views',
  materialized_view: 'Materialized views',
  collection: 'Collections',
  sequence: 'Sequences',
  procedure: 'Procedures',
}

/** Singular noun used when a single object is referenced in the UI. */
export const KIND_SINGULAR: Record<ObjectKind, string> = {
  table: 'Table',
  view: 'View',
  materialized_view: 'Materialized view',
  collection: 'Collection',
  sequence: 'Sequence',
  procedure: 'Procedure',
}
