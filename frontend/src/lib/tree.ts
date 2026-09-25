/**
 * Tree node identities for the object explorer.
 *
 * Keys must survive a JSON round trip because Ant Design's Tree only gives us
 * the key back in its callbacks. JSON keeps arbitrary identifiers (dots,
 * spaces, unicode) unambiguous.
 */
import type { ObjectInfo, ObjectKind } from '../api/types'
import type { MessageKey } from './i18n'

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

/**
 * What the explorer calls each kind of folder, and one object of that kind.
 *
 * The words themselves live in the message tables and these name the message:
 * written here they would be read once, when the module is first loaded, and
 * would then stay in whichever language that happened to be. Callers wrap them
 * in `t(…)` where they are drawn.
 */
export const FOLDER_LABEL: Record<ObjectKind, MessageKey> = {
  table: 'libTree.tables',
  view: 'libTree.views',
  materialized_view: 'libTree.materialized-views',
  collection: 'libTree.collections',
  sequence: 'libTree.sequences',
  procedure: 'libTree.procedures',
}

/** Singular noun used when a single object is referenced in the UI. */
export const KIND_SINGULAR: Record<ObjectKind, MessageKey> = {
  table: 'libTree.table',
  view: 'libTree.view',
  materialized_view: 'libTree.materialized-view',
  collection: 'libTree.collection',
  sequence: 'libTree.sequence',
  procedure: 'libTree.procedure',
}

/**
 * The slice of the explorer's cache the catalog lookup reads.
 *
 * Named structurally rather than as the store's `TreeCache`, so this module
 * does not have to know what else the cache holds.
 */
export interface TreeCatalog {
  objects: Record<string, ObjectInfo[]>
  schemas: Record<string, string[]>
}

/**
 * The objects a query window should complete table names from.
 *
 * A window is scoped to one database, and to a schema inside it when the engine
 * has schemas and the caller knew which one — that one namespace is the whole
 * answer. Without a schema (a window opened from a database node, from a saved
 * script, or from the tab strip on PostgreSQL) the objects live in the schemas,
 * so every schema whose list the explorer already holds counts. Opening a
 * window is not a reason to run a catalog query per schema, so the schemas
 * nobody has browsed are left out rather than fetched; the one namespace the
 * window *does* name is fetched by the window itself (see QueryPane).
 */
export function catalogOf(
  tree: TreeCatalog,
  scope: { sessionId: string; database?: string; schema?: string },
  hasSchemas: boolean,
): ObjectInfo[] {
  const { sessionId, database } = scope
  if (!database) return []
  // An engine without schemas puts its objects straight in the database, which
  // is the namespace the tree keys them under as well.
  const namespace = scope.schema ?? (hasSchemas ? undefined : database)
  const schemas = namespace ? [namespace] : (tree.schemas[databaseKey(sessionId, database)] ?? [])
  const objects: ObjectInfo[] = []
  for (const schema of schemas) {
    objects.push(...(tree.objects[objectsKey(sessionId, database, schema)] ?? []))
  }
  return objects
}
