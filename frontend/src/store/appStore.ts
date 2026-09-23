/**
 * Global application state.
 *
 * Scope decisions:
 *   - Connections, sessions, workspace tabs and the explorer cache live here
 *     because several panes need to read them.
 *   - Per-tab editor/grid state stays inside the tab components (Ant Design
 *     keeps inactive tab panes mounted, so it survives tab switches) which
 *     avoids a giant store and needless re-renders of the whole app.
 */
import { create } from 'zustand'

import { api, toMessage } from '../api/client'
import type {
  AppInfo,
  ConnectionConfig,
  ConnectionGroup,
  ConnectionLayout,
  DataDirInfo,
  DataDirMoveResult,
  DriverInfo,
  IndexEntry,
  MockPlaceholder,
  ObjectInfo,
  ObjectKind,
  OpenRequest,
  QueryFile,
  QueryFileRename,
  QueryFileSave,
  SavedQuery,
  SessionInfo,
  TableDesign,
  TableStructure,
} from '../api/types'
import type { ConnectionDraft } from '../connection/shared'
import { arrangementOf, layoutOf, moveEntry, type DropTarget } from '../lib/explorer'
import {
  databaseKey,
  FOLDER_LABEL,
  indexesKey,
  namespaceKey,
  objectsKey,
  queriesKey,
} from '../lib/tree'
import { designFrom, emptyStructure, newTableDesign } from '../lib/design'
import { DEFAULT_CODE_LANGUAGE, codeLanguageById } from '../lib/codegen'
import {
  parseThemeMode,
  resolveTheme,
  watchSystemTheme,
  type ResolvedTheme,
  type ThemeMode,
} from '../lib/theme'

export type { ResolvedTheme, ThemeMode } from '../lib/theme'

export type TabKind =
  | 'query'
  | 'table'
  | 'newtable'
  | 'objects'
  | 'ddl'
  | 'codegen'
  | 'datagen'
  | 'er'
  | 'runtime'
  | 'changelog'

/**
 * The id of a saved-script window.
 *
 * Derived from the file rather than random, unlike a scratchpad query tab: a
 * script is one thing, and opening it twice has to bring back the window that is
 * already editing it. The parts cannot collide because the separator cannot
 * appear in a connection id (a UUID) or in a name the backend accepted.
 *
 * A rename deliberately leaves this key alone. It is the identity of the *window*
 * — which must survive a rename, or the pane would be torn down and its unsaved
 * text with it — while the file it points at is carried in `tab.queryFile`.
 */
const queryFileId = (connectionId: string, database: string, name: string) =>
  `queryFile:${connectionId}:${database}:${name}`

/** Whether a tab is the window over one particular saved script. */
const isQueryFileTab = (tab: WorkspaceTab, connectionId: string, database: string, name: string) =>
  tab.kind === 'query' &&
  tab.queryFile?.connectionId === connectionId &&
  tab.queryFile.database === database &&
  tab.queryFile.name === name

/** Sub-views of a table/view window (Navicat-style bottom tab strip). */
export type TableView = 'data' | 'structure' | 'indexes' | 'foreignKeys' | 'ddl'

/** Object-list windows: one per folder of the explorer tree. */
export type ListScope = ObjectKind | 'index'

/**
 * A place in the explorer that holds objects: a schema where the engine has
 * them, the database itself where it does not.
 *
 * `schema` is left off for a database that is not a namespace of its own —
 * PostgreSQL's databases hold schemas, so picking one says where the user is
 * without saying which list to open. The ribbon says as much instead of
 * guessing a schema.
 */
export interface NamespaceScope {
  sessionId: string
  database: string
  schema?: string
}

/**
 * A folder of the explorer whose object list just came to the front.
 *
 * A list window is always scoped to one namespace and one folder of it, so
 * bringing one up says where the user is: the tree is asked to open that
 * namespace up and point at that folder. `schema` is the namespace the list is
 * scoped to, which is the database itself on an engine that has no schemas.
 */
export interface RevealTarget {
  sessionId: string
  database: string
  schema: string
  kind: ListScope
}

export interface WorkspaceTab {
  id: string
  kind: TabKind
  sessionId: string
  title: string
  database?: string
  schema?: string
  object?: string
  objectKind?: ObjectKind
  /** Which sub-view of a table window is active. */
  view?: TableView
  /** Object-list windows only: which folder the list is scoped to. */
  list?: ListScope
  /**
   * A query window that is bound to a saved script rather than being a
   * scratchpad: the tab edits that file, and saving writes it back.
   */
  queryFile?: { connectionId: string; database: string; name: string }
  /**
   * Whether a query window holds edits that are not on disk yet. Kept on the tab
   * because the tab label is what shows it, and the label is drawn by the store.
   */
  dirty?: boolean
}

/**
 * Table designer state, kept per table window so switching between the Data and
 * Structure sub-tabs does not throw the draft away.
 */
export interface DesignState {
  /** The definition as the user has it right now. */
  draft: TableDesign
  /** The catalog structure the draft was prefilled from. */
  baseline: TableStructure
}

interface TreeCache {
  databases: Record<string, string[]>
  schemas: Record<string, string[]>
  objects: Record<string, ObjectInfo[]>
  indexes: Record<string, IndexEntry[]>
  /**
   * Saved scripts, per database. An empty array means "read, and there are
   * none", which is a different answer from "not read yet" (absent).
   */
  queries: Record<string, QueryFile[]>
  loaded: Record<string, boolean>
  loading: Record<string, boolean>
  errors: Record<string, string>
}

const emptyTree = (): TreeCache => ({
  databases: {},
  schemas: {},
  objects: {},
  indexes: {},
  queries: {},
  loaded: {},
  loading: {},
  errors: {},
})

interface AppState {
  boot: 'loading' | 'ready' | 'failed'
  bootError?: string
  appInfo?: AppInfo
  drivers: DriverInfo[]
  connections: ConnectionConfig[]
  /**
   * The explorer's arrangement: which groups exist, what sits in them and in
   * what order. Derived from the profiles on the backend (see
   * internal/service/layout.go), so it is always refreshed together with them.
   */
  connectionLayout: ConnectionLayout
  sessions: SessionInfo[]
  activeSessionId?: string
  /**
   * The connection node the explorer is focused on — the same id the tree
   * stores in its `connection` nodes: a saved profile's id, or the session's own
   * id when there is no profile behind it (ad-hoc, or its profile was deleted).
   *
   * Set whenever a session is focused (see `focused`), and additionally when a
   * saved profile that nobody has opened yet is picked in the tree, since that
   * one has no session to hand the focus to. The ribbon speaks about this
   * connection: pick a closed profile and *Open* lights up, pick an open one
   * and *Close* does.
   */
  activeConnectionId?: string
  /**
   * The namespace the explorer is focused on: the schema where the engine has
   * them, the database where it does not. Written by the tree's selection (see
   * `setActiveNamespace`) and read by the ribbon's *Table* / *View* buttons,
   * which list the objects of the place the user is standing in.
   */
  activeNamespace?: NamespaceScope
  /**
   * A place in the explorer to open up and point at, if one was asked for.
   *
   * Written when a list window comes to the front — by the ribbon that opens it
   * and by the tab strip — and consumed by the tree, which is the pane that owns
   * `expandedKeys` / `selectedKeys`. It is deliberately one-way: the request
   * never writes `activeNamespace`, so the ribbon still follows the tree and
   * only the tree's own selection says where the user stands (see Round 8).
   */
  reveal?: RevealTarget
  tabs: WorkspaceTab[]
  activeTabId?: string
  /** The preference: light, dark, or follow the system. */
  theme: ThemeMode
  /** Which language the code window opens in; the settings page moves it. */
  codegenLanguage: string
  /** What is actually painted — `theme` with `system` resolved. */
  resolvedTheme: ResolvedTheme
  /** Where the app keeps its data, as the settings page shows it. */
  dataDir?: DataDirInfo
  tree: TreeCache
  designs: Record<string, DesignState>
  savedQueries: SavedQuery[]
  /**
   * The placeholders the user defined, as the data generation window and the
   * settings page read them. They live in the data directory, so moving the data
   * directory moves them too.
   */
  mockPlaceholders: MockPlaceholder[]
  editorOpen: boolean
  editorDraft?: ConnectionDraft

  bootstrap: () => Promise<void>
  setTheme: (theme: ThemeMode) => void
  /** Sets the language a new code window starts in, and stores it. */
  setCodegenLanguage: (language: string) => void
  /** Re-reads the data directory (the settings page calls this after a move). */
  refreshDataDir: () => Promise<DataDirInfo>
  /**
   * Moves the app's data into `dir` (empty = the default) and switches the
   * running app over to it. Returns what happened to each file.
   */
  moveDataDir: (dir: string) => Promise<DataDirMoveResult>
  /** Takes the driver the user picked from the "new connection" menu. */
  openConnectionEditor: (draft?: ConnectionDraft) => void
  closeConnectionEditor: () => void

  refreshSavedQueries: () => Promise<void>
  saveSavedQuery: (query: SavedQuery) => Promise<SavedQuery>
  deleteSavedQuery: (id: string) => Promise<void>
  /** Re-reads the custom mock placeholders; returns what is there now. */
  refreshMockPlaceholders: () => Promise<MockPlaceholder[]>
  /** Creates or updates one custom placeholder, keyed by its name. */
  saveMockPlaceholder: (placeholder: MockPlaceholder) => Promise<MockPlaceholder>
  deleteMockPlaceholder: (name: string) => Promise<void>

  refreshConnections: () => Promise<void>
  /** Re-reads the arrangement alone, for changes the profiles do not show
   * (deleting a group hands its connections back to the top level). */
  refreshConnectionLayout: () => Promise<void>
  saveConnection: (cfg: ConnectionConfig, groupId?: string) => Promise<ConnectionConfig>
  deleteConnection: (id: string) => Promise<void>

  /**
   * Moves an explorer entry — a connection or a group — to where a drop asked
   * for it and stores the result. Rejects without changing anything the
   * backend refused, so what is drawn is always what is stored.
   */
  moveConnection: (dragId: string, target: DropTarget) => Promise<void>
  createConnectionGroup: (name: string) => Promise<ConnectionGroup>
  renameConnectionGroup: (id: string, name: string) => Promise<void>
  /** Removes a group; the connections in it go back to the top level. */
  deleteConnectionGroup: (id: string) => Promise<void>

  openConnection: (req: OpenRequest) => Promise<SessionInfo>
  closeSession: (sessionId: string) => Promise<void>
  setActiveSession: (sessionId: string) => void
  /** Marks which connection the explorer is focused on (see `activeConnectionId`). */
  setActiveConnection: (connectionId: string) => void
  /**
   * Marks which namespace the explorer is focused on (see `activeNamespace`).
   * Called with no argument for a node that is not in one — a connection node
   * is about the connection, not about any database.
   */
  setActiveNamespace: (scope?: NamespaceScope) => void
  /** Drops a request once the tree has carried it out, or given up on it. */
  clearReveal: () => void

  loadDatabases: (sessionId: string) => Promise<void>
  loadSchemas: (sessionId: string, database: string) => Promise<void>
  loadObjects: (sessionId: string, database: string, schema: string) => Promise<void>
  loadIndexes: (sessionId: string, database: string, schema: string) => Promise<void>
  /** Reads the saved scripts of one database (the tree's Queries folder). */
  loadQueryFiles: (sessionId: string, database: string) => Promise<QueryFile[]>
  /**
   * Writes a script through to its file and refreshes the folder it lives in.
   * Returns the file as it now is on disk.
   */
  saveQueryFile: (save: QueryFileSave) => Promise<QueryFile>
  /** Moves a script to another name, and re-points an open tab at it. */
  renameQueryFile: (rename: QueryFileRename) => Promise<QueryFile>
  /**
   * Creates a script holding `sql`, named as asked. Refuses (through the
   * backend) a name that is already taken.
   */
  createQueryFile: (
    sessionId: string,
    database: string,
    name: string,
    sql: string,
  ) => Promise<QueryFile>
  /**
   * Binds a scratchpad window to the file it has just been saved as.
   *
   * The window keeps what is typed in it (its id does not change, so the pane is
   * not rebuilt) and from here on it is a window *over a file*: the tree row it
   * now has and the next Save both mean that name, and closing it with unsaved
   * edits asks first.
   */
  adoptQueryFile: (tabId: string, file: QueryFile) => void
  /**
   * Re-reads one database's script folder by the connection it belongs to.
   *
   * Saving and renaming answer with the *connection*, not the session, and the
   * tree cache is keyed by session — this is the one place that has to translate
   * between the two.
   */
  reloadQueryFolder: (connectionId: string, database: string) => Promise<void>
  deleteQueryFile: (sessionId: string, database: string, name: string) => Promise<void>
  /** Marks a query window's edits as saved, or not saved. */
  setTabDirty: (tabId: string, dirty: boolean) => void
  invalidateSession: (sessionId: string) => void

  /**
   * Opens a query window over one database.
   *
   * `schema` is the namespace inside that database whose table names the window
   * completes; it is left out where the caller only knows the database (a
   * connection node, the tab strip), and the window then falls back to what the
   * explorer has already loaded (see `catalogOf` in lib/tree.ts).
   */
  openQueryTab: (sessionId: string, database?: string, schema?: string) => void
  /**
   * Opens a saved script, or brings its window back to the front when it is
   * already open. One tab per file: the file is the thing being edited, and two
   * windows over one file would fight over it.
   */
  openQueryFileTab: (sessionId: string, database: string, name: string) => void
  openObjectsTab: (
    sessionId: string,
    database: string,
    schema: string,
    list: ListScope,
  ) => void
  openTableTab: (
    sessionId: string,
    database: string,
    schema: string,
    object: ObjectInfo,
    view?: TableView,
  ) => void
  /** Opens the table designer for a table that does not exist yet. */
  openNewTableTab: (sessionId: string, database: string, schema: string) => void
  /**
   * Opens the DDL editor. With `object` it starts from that object's live
   * definition; without it, from a template for a new object.
   */
  openDdlTab: (
    sessionId: string,
    database: string,
    schema: string,
    object?: string,
  ) => void
  /**
   * Opens the code window of an object: its fields as a class, struct or record
   * in the language that window's own picker shows. One window per object, so
   * going back and forth does not pile up copies of the same file.
   */
  openCodegenTab: (
    sessionId: string,
    database: string,
    schema: string,
    object: ObjectInfo,
  ) => void
  /**
   * Opens the data generation window of a connection: pick one of its tables on
   * the left, describe what each column gets on the right, and insert the rows.
   *
   * One window per connection, not per table, because its state — the mocks
   * written for a table — is worth keeping while moving around that connection.
   * Opening it again with a table pointed at moves the existing window's target
   * instead of adding a copy, which is what makes "Data generation" on a table
   * do the useful thing rather than open a second window on the same engine.
   */
  openDataGenTab: (
    sessionId: string,
    database?: string,
    schema?: string,
    object?: string,
  ) => void
  /** Opens the ER diagram of one namespace (schema, or database when there is
   * no schema layer). */
  openErTab: (sessionId: string, database: string, schema: string) => void
  /** Opens the live status page of a connection (one per session). */
  openRuntimeTab: (sessionId: string) => void
  /**
   * Opens the change log: what this application has run against the databases
   * it was pointed at, newest first.
   */
  openChangeLog: () => void
  setTabView: (tabId: string, view: TableView) => void

  /**
   * Makes sure a table window has a draft. An existing draft is kept unless
   * `reset` is set, so a reload never silently discards edits.
   */
  ensureDesign: (tabId: string, structure: TableStructure, reset?: boolean) => void
  updateDesign: (tabId: string, draft: TableDesign) => void
  dropDesign: (tabId: string) => void

  closeTab: (tabId: string) => void
  closeAllTabs: () => void
  setActiveTab: (tabId: string) => void

  driverOf: (sessionId: string) => DriverInfo | undefined
  sessionOf: (sessionId: string) => SessionInfo | undefined
}

const STATE_KEY = 'ui'

/**
 * Stores the UI preferences.
 *
 * `SaveState` replaces the whole file rather than merging into it, so every
 * preference has to be written every time one of them changes — one writer for
 * all of them is what keeps a theme change from dropping the language, and the
 * other way round.
 */
function saveUi(state: Pick<AppState, 'theme' | 'codegenLanguage'>): void {
  // Fire and forget: a failed preference write must not disturb the UI.
  void api
    .saveState({ [STATE_KEY]: { theme: state.theme, codegenLanguage: state.codegenLanguage } })
    .catch(() => undefined)
}

/** Copy of an object without one key (drafts must die with their window). */
function withoutKey<T>(source: Record<string, T>, key: string): Record<string, T> {
  if (!source[key]) return source
  const out = { ...source }
  delete out[key]
  return out
}

/** The profile a live session came from, or the session's own id when there is
 * none (an ad-hoc connection, or one whose profile was deleted). */
const connectionOfSession = (sessions: SessionInfo[], sessionId: string): string | undefined => {
  const session = sessions.find((s) => s.id === sessionId)
  return session ? session.connectionId ?? session.id : undefined
}

/**
 * The patch that hands the focus to a session.
 *
 * Two ids move together here. `activeSessionId` is the live session the status
 * bar describes and the query tabs belong to; `activeConnectionId` is the
 * connection node the explorer is working on, which is *not* always the same
 * thing — a profile nobody has opened yet has no session to point at. Focusing
 * a session always focuses its connection node, so the two never drift apart on
 * their own.
 */
const focused = (state: AppState, sessionId: string) => ({
  activeSessionId: sessionId,
  activeConnectionId: connectionOfSession(state.sessions, sessionId) ?? state.activeConnectionId,
})

/**
 * The explorer request that belongs to bringing a window to the front.
 *
 * An object list *is* one folder of one namespace, so the tree is asked to show
 * the place it came from; every other kind of window leaves the explorer where
 * the user put it.
 */
const revealOf = (tab: WorkspaceTab): RevealTarget | undefined => {
  if (tab.kind !== 'objects' || !tab.database || !tab.schema || !tab.list) return undefined
  return { sessionId: tab.sessionId, database: tab.database, schema: tab.schema, kind: tab.list }
}

export const useAppStore = create<AppState>((set, get) => ({
  boot: 'loading',
  drivers: [],
  connections: [],
  connectionLayout: { groups: [], items: [] },
  sessions: [],
  tabs: [],
  theme: 'light',
  resolvedTheme: 'light',
  codegenLanguage: DEFAULT_CODE_LANGUAGE,
  tree: emptyTree(),
  designs: {},
  savedQueries: [],
  mockPlaceholders: [],
  editorOpen: false,

  openConnectionEditor(draft) {
    set({ editorOpen: true, editorDraft: draft })
  },

  closeConnectionEditor() {
    set({ editorOpen: false, editorDraft: undefined })
  },

  async bootstrap() {
    try {
      const [
        appInfo,
        drivers,
        connections,
        connectionLayout,
        savedQueries,
        mockPlaceholders,
        persisted,
        dataDir,
      ] = await Promise.all([
          api.appInfo(),
          api.listDrivers(),
          api.listConnections(),
          // A missing or unreadable layout file just means "no arrangement".
          api.listConnectionLayout().catch(() => ({ groups: [], items: [] }) as ConnectionLayout),
          // A missing or unreadable favourites file must not block startup.
          api.listSavedQueries().catch(() => [] as SavedQuery[]),
          // Nor may a placeholder file someone hand-edited into nonsense: the
          // mock column simply has no custom entries until it is fixed.
          api.listMockPlaceholders().catch(() => [] as MockPlaceholder[]),
          api.loadState().catch(() => ({}) as Record<string, unknown>),
          // The data directory is only reported by the settings page; failing to
          // read it must not keep the app from starting.
          api.getDataDir().catch(() => undefined),
        ])

      const stored = persisted?.[STATE_KEY] as
        | { theme?: unknown; codegenLanguage?: unknown }
        | undefined
      // Navicat's classic look is light; dark stays one toggle away.
      const theme = parseThemeMode(stored?.theme)
      // An id from a version that had different languages is one this build may
      // not know, and `codeLanguageById` answers with the default for it.
      const codegenLanguage = codeLanguageById(
        typeof stored?.codegenLanguage === 'string' ? stored.codegenLanguage : undefined,
      ).id

      // The OS can be switched to dark while the app is running, and "follow the
      // system" has to follow it live — the listener stays installed for the
      // whole session and only ever moves when the preference is `system`,
      // because `resolveTheme` ignores the system for the other two.
      watchSystemTheme((systemDark) => {
        set({ resolvedTheme: resolveTheme(get().theme, systemDark) })
      })

      set({
        boot: 'ready',
        appInfo,
        drivers,
        connections,
        connectionLayout,
        savedQueries,
        mockPlaceholders,
        dataDir,
        theme,
        resolvedTheme: resolveTheme(theme),
        codegenLanguage,
      })
    } catch (error) {
      set({ boot: 'failed', bootError: toMessage(error) })
    }
  },

  setTheme(theme) {
    // The preference and the painted value are kept apart: a `system` choice
    // passes the current system answer through, and the listener above keeps it
    // up to date from then on.
    set({ theme, resolvedTheme: resolveTheme(theme) })
    saveUi(get())
  },

  setCodegenLanguage(codegenLanguage) {
    set({ codegenLanguage })
    saveUi(get())
  },

  async refreshDataDir() {
    const dataDir = await api.getDataDir()
    set({ dataDir })
    return dataDir
  },

  async moveDataDir(dir) {
    const result = await api.moveDataDirectory(dir)
    set({ dataDir: result.info })
    // The welcome pane and the About tab show the path the app is working in,
    // and it has just changed; re-reading the info is cheaper than patching a
    // copy of it that would then disagree with the backend.
    try {
      set({ appInfo: await api.appInfo() })
    } catch {
      // Cosmetic only: the move itself already succeeded.
    }
    return result
  },

  async refreshConnections() {
    // The arrangement is derived from the profiles, so the two are always read
    // together: a profile added or removed anywhere shows up in both.
    const [connections, connectionLayout] = await Promise.all([
      api.listConnections(),
      api.listConnectionLayout(),
    ])
    set({ connections, connectionLayout })
  },

  async refreshConnectionLayout() {
    set({ connectionLayout: await api.listConnectionLayout() })
  },

  async moveConnection(dragId, target) {
    const before = get().connectionLayout
    const entries = arrangementOf(get().connections, before)
    // Drawn at once — a drag that waited for a round trip would feel broken —
    // and rolled back if the backend refused it, so the tree never shows an
    // arrangement that was not stored.
    set({ connectionLayout: layoutOf(moveEntry(entries, dragId, target)) })
    try {
      set({ connectionLayout: await api.saveConnectionLayout(get().connectionLayout) })
    } catch (error) {
      set({ connectionLayout: before })
      throw error
    }
  },

  async createConnectionGroup(name) {
    const created = await api.saveConnectionGroup({ id: '', name, order: 0 })
    // The backend appends a new group, so the answer settles where it landed.
    set((state) => ({
      connectionLayout: {
        ...state.connectionLayout,
        groups: [...state.connectionLayout.groups, created],
      },
    }))
    return created
  },

  async renameConnectionGroup(id, name) {
    const existing = get().connectionLayout.groups.find((group) => group.id === id)
    // A rename keeps the position it had; sending it back is what makes the
    // request say what it means.
    const saved = await api.saveConnectionGroup({ id, name, order: existing?.order ?? 0 })
    set((state) => ({
      connectionLayout: {
        ...state.connectionLayout,
        groups: state.connectionLayout.groups.map((group) => (group.id === saved.id ? saved : group)),
      },
    }))
  },

  async deleteConnectionGroup(id) {
    await api.deleteConnectionGroup(id)
    // The connections of a deleted group are handed back to the top level by the
    // backend; asking it where everything landed beats repeating that rule here.
    await get().refreshConnectionLayout()
  },

  async refreshSavedQueries() {
    const savedQueries = await api.listSavedQueries()
    set({ savedQueries })
  },

  async saveSavedQuery(query) {
    const saved = await api.saveSavedQuery(query)
    await get().refreshSavedQueries()
    return saved
  },

  async deleteSavedQuery(id) {
    await api.deleteSavedQuery(id)
    await get().refreshSavedQueries()
  },

  async refreshMockPlaceholders() {
    const mockPlaceholders = await api.listMockPlaceholders()
    set({ mockPlaceholders })
    return mockPlaceholders
  },

  async saveMockPlaceholder(placeholder) {
    const saved = await api.saveMockPlaceholder(placeholder)
    // Re-read rather than patch the list: the name is the file name, so a save
    // can replace one entry with another (a differently-cased spelling) and the
    // backend is the one that knows which entries exist now.
    await get().refreshMockPlaceholders()
    return saved
  },

  async deleteMockPlaceholder(name) {
    await api.deleteMockPlaceholder(name)
    await get().refreshMockPlaceholders()
  },

  async saveConnection(cfg, groupId) {
    const saved = await api.saveConnection(cfg)
    await get().refreshConnections()
    // A connection created from a group's menu belongs in that group: a new
    // profile is appended to the top level, so this is a move, not a flag.
    if (groupId) await get().moveConnection(saved.id, { t: 'inside', groupId })
    return saved
  },

  async deleteConnection(id) {
    await api.deleteConnection(id)
    await get().refreshConnections()
  },

  async openConnection(req) {
    const session = await api.openConnection(req)
    set((state) => ({
      sessions: [...state.sessions.filter((s) => s.id !== session.id), session],
      activeSessionId: session.id,
      // An ad-hoc session is its own connection node, exactly like the tree
      // draws it.
      activeConnectionId: session.connectionId ?? session.id,
    }))
    void get().loadDatabases(session.id)
    return session
  },

  async closeSession(sessionId) {
    try {
      await api.closeConnection(sessionId)
    } finally {
      set((state) => {
        const sessions = state.sessions.filter((s) => s.id !== sessionId)
        const tabs = state.tabs.filter((t) => t.sessionId !== sessionId)
        const gone = new Set(
          state.tabs.filter((t) => t.sessionId === sessionId).map((t) => t.id),
        )
        const designs: Record<string, DesignState> = {}
        for (const [key, value] of Object.entries(state.designs)) {
          if (!gone.has(key)) designs[key] = value
        }
        const activeTabId =
          state.activeTabId && tabs.some((t) => t.id === state.activeTabId)
            ? state.activeTabId
            : tabs[tabs.length - 1]?.id
        const activeSessionId =
          state.activeSessionId === sessionId
            ? sessions[sessions.length - 1]?.id
            : state.activeSessionId
        // The namespace note goes with the session it pointed at: the database
        // node it came from is gone from the tree, and the ribbon must not offer
        // to list the objects of a connection that is no longer open.
        const activeNamespace =
          state.activeNamespace?.sessionId === sessionId ? undefined : state.activeNamespace
        return { sessions, tabs, designs, activeTabId, activeSessionId, activeNamespace }
      })
    }
  },

  setActiveSession(sessionId) {
    set({ activeSessionId: sessionId })
  },

  setActiveConnection(connectionId) {
    set({ activeConnectionId: connectionId })
  },

  setActiveNamespace(scope) {
    set({ activeNamespace: scope })
  },

  clearReveal() {
    set({ reveal: undefined })
  },

  async loadDatabases(sessionId) {
    set((state) => ({
      tree: {
        ...state.tree,
        loading: { ...state.tree.loading, [sessionId]: true },
        errors: { ...state.tree.errors, [sessionId]: '' },
      },
    }))
    try {
      const databases = await api.listDatabases(sessionId)
      set((state) => ({
        tree: {
          ...state.tree,
          databases: { ...state.tree.databases, [sessionId]: databases },
          loaded: { ...state.tree.loaded, [sessionId]: true },
          loading: { ...state.tree.loading, [sessionId]: false },
        },
      }))
    } catch (error) {
      set((state) => ({
        tree: {
          ...state.tree,
          loading: { ...state.tree.loading, [sessionId]: false },
          errors: { ...state.tree.errors, [sessionId]: toMessage(error) },
        },
      }))
    }
  },

  async loadSchemas(sessionId, database) {
    const key = databaseKey(sessionId, database)
    set((state) => ({
      tree: {
        ...state.tree,
        loading: { ...state.tree.loading, [key]: true },
        errors: { ...state.tree.errors, [key]: '' },
      },
    }))
    try {
      const schemas = await api.listSchemas(sessionId, database)
      set((state) => ({
        tree: {
          ...state.tree,
          schemas: { ...state.tree.schemas, [key]: schemas },
          loaded: { ...state.tree.loaded, [key]: true },
          loading: { ...state.tree.loading, [key]: false },
        },
      }))
    } catch (error) {
      set((state) => ({
        tree: {
          ...state.tree,
          loading: { ...state.tree.loading, [key]: false },
          errors: { ...state.tree.errors, [key]: toMessage(error) },
        },
      }))
    }
  },

  async loadObjects(sessionId, database, schema) {
    const ns = namespaceKey(sessionId, database, schema)
    set((state) => ({
      tree: {
        ...state.tree,
        loading: { ...state.tree.loading, [ns]: true },
        errors: { ...state.tree.errors, [ns]: '' },
      },
    }))
    try {
      const objects = await api.listObjects(sessionId, database, schema)
      set((state) => ({
        tree: {
          ...state.tree,
          objects: { ...state.tree.objects, [objectsKey(sessionId, database, schema)]: objects },
          loaded: { ...state.tree.loaded, [ns]: true },
          loading: { ...state.tree.loading, [ns]: false },
        },
      }))
    } catch (error) {
      set((state) => ({
        tree: {
          ...state.tree,
          loading: { ...state.tree.loading, [ns]: false },
          errors: { ...state.tree.errors, [ns]: toMessage(error) },
        },
      }))
    }
  },

  async loadIndexes(sessionId, database, schema) {
    const ns = indexesKey(sessionId, database, schema)
    set((state) => ({
      tree: {
        ...state.tree,
        loading: { ...state.tree.loading, [ns]: true },
        errors: { ...state.tree.errors, [ns]: '' },
      },
    }))
    try {
      const indexes = await api.listIndexes(sessionId, database, schema)
      set((state) => ({
        tree: {
          ...state.tree,
          indexes: { ...state.tree.indexes, [ns]: indexes },
          loaded: { ...state.tree.loaded, [ns]: true },
          loading: { ...state.tree.loading, [ns]: false },
        },
      }))
    } catch (error) {
      set((state) => ({
        tree: {
          ...state.tree,
          loading: { ...state.tree.loading, [ns]: false },
          errors: { ...state.tree.errors, [ns]: toMessage(error) },
        },
      }))
    }
  },

  async loadQueryFiles(sessionId, database) {
    const key = queriesKey(sessionId, database)
    const connectionId = get().sessions.find((s) => s.id === sessionId)?.connectionId
    if (!connectionId) {
      // An ad-hoc session has no profile, so it has no folder to keep scripts
      // in. The tree does not draw the folder in that case either; this is the
      // guard for a request that raced the session going away.
      set((state) => ({
        tree: {
          ...state.tree,
          queries: { ...state.tree.queries, [key]: [] },
          loaded: { ...state.tree.loaded, [key]: true },
          errors: { ...state.tree.errors, [key]: '' },
        },
      }))
      return []
    }
    set((state) => ({
      tree: {
        ...state.tree,
        loading: { ...state.tree.loading, [key]: true },
        errors: { ...state.tree.errors, [key]: '' },
      },
    }))
    try {
      const files = await api.listQueryFiles(connectionId, database)
      set((state) => ({
        tree: {
          ...state.tree,
          queries: { ...state.tree.queries, [key]: files },
          loaded: { ...state.tree.loaded, [key]: true },
          loading: { ...state.tree.loading, [key]: false },
        },
      }))
      return files
    } catch (error) {
      const message = toMessage(error)
      set((state) => ({
        tree: {
          ...state.tree,
          loading: { ...state.tree.loading, [key]: false },
          errors: { ...state.tree.errors, [key]: message },
        },
      }))
      throw error
    }
  },

  async saveQueryFile(save) {
    const file = await api.saveQueryFile(save)
    // The folder's listing is what the tree draws, so it is re-read rather than
    // patched by hand: the backend is the one that decides what the name on disk
    // ended up being.
    await get().reloadQueryFolder(save.connectionId, save.database)
    return file
  },

  async renameQueryFile(rename) {
    const file = await api.renameQueryFile(rename)
    await get().reloadQueryFolder(rename.connectionId, rename.database)
    set((state) => ({
      // An open window follows its file: it keeps what is typed in it (the key is
      // untouched, so the pane is not rebuilt) and its next save lands on the new
      // name. A window that holds nothing yet is left alone — reopening the tree
      // row reads the file under its new name anyway.
      tabs: state.tabs.map((tab) =>
        isQueryFileTab(tab, rename.connectionId, rename.database, rename.from)
          ? { ...tab, title: rename.to, queryFile: { ...tab.queryFile!, name: rename.to } }
          : tab,
      ),
    }))
    return file
  },

  async createQueryFile(sessionId, database, name, sql) {
    const connectionId = get().sessions.find((s) => s.id === sessionId)?.connectionId
    if (!connectionId) throw new Error('This session has no saved connection profile.')
    const file = await api.createQueryFile(connectionId, database, name, sql)
    await get().reloadQueryFolder(connectionId, database)
    return file
  },

  adoptQueryFile(tabId, file) {
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === tabId
          ? {
              ...tab,
              title: file.name,
              dirty: false,
              queryFile: {
                connectionId: file.connectionId,
                database: file.database,
                name: file.name,
              },
            }
          : tab,
      ),
    }))
  },

  async deleteQueryFile(sessionId, database, name) {
    const connectionId = get().sessions.find((s) => s.id === sessionId)?.connectionId
    if (!connectionId) return
    await api.deleteQueryFile(connectionId, database, name)
    await get().reloadQueryFolder(connectionId, database)
    // Closing the window is the caller's business: the tree menu asks first, and
    // a delete that was refused must not have closed anything already.
  },

  async reloadQueryFolder(connectionId, database) {
    const sessionId = get().sessions.find((s) => s.connectionId === connectionId)?.id
    if (!sessionId) {
      // The session is gone (disconnected while the window was open): there is
      // no folder left to refresh, and inventing a key would leave a cache entry
      // nothing can ever read or clear.
      return
    }
    await get().loadQueryFiles(sessionId, database)
  },

  setTabDirty(tabId, dirty) {
    set((state) => ({
      tabs: state.tabs.map((tab) => (tab.id === tabId ? { ...tab, dirty } : tab)),
    }))
  },

  invalidateSession(sessionId) {
    // Every cache key starts with the session id, so a prefix match is enough.
    const belongs = (key: string) =>
      key === sessionId || key.startsWith(sessionId + '\u0000')
    const filter = <T,>(source: Record<string, T>): Record<string, T> => {
      const out: Record<string, T> = {}
      for (const [key, value] of Object.entries(source)) {
        if (!belongs(key)) out[key] = value
      }
      return out
    }
    set((state) => ({
      tree: {
        databases: filter(state.tree.databases),
        schemas: filter(state.tree.schemas),
        objects: filter(state.tree.objects),
        indexes: filter(state.tree.indexes),
        queries: filter(state.tree.queries),
        loaded: filter(state.tree.loaded),
        loading: filter(state.tree.loading),
        errors: filter(state.tree.errors),
      },
    }))
  },

  openQueryTab(sessionId, database, schema) {
    const id = `query:${sessionId}:${Math.random().toString(36).slice(2, 10)}`
    const tab: WorkspaceTab = {
      id,
      kind: 'query',
      sessionId,
      title: 'Query',
      database,
      schema,
    }
    set((state) => ({
      tabs: [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
    }))
  },

  openQueryFileTab(sessionId, database, name) {
    const connectionId = get().sessions.find((s) => s.id === sessionId)?.connectionId
    if (!connectionId) return
    const tab: WorkspaceTab = {
      id: queryFileId(connectionId, database, name),
      kind: 'query',
      sessionId,
      title: name,
      database,
      queryFile: { connectionId, database, name },
    }
    set((state) => {
      // Already open — under this name now, or under the name it had before a
      // rename: the window that is there is the file, so it comes back to the
      // front instead of a second one being opened over it.
      const existing = state.tabs.find((t) => isQueryFileTab(t, connectionId, database, name))
      return {
        tabs: existing ? state.tabs : [...state.tabs, tab],
        activeTabId: existing ? existing.id : tab.id,
        ...focused(state, sessionId),
      }
    })
  },

  openObjectsTab(sessionId, database, schema, list) {
    const id = `objects:${sessionId}:${database}:${schema}:${list}`
    const tab: WorkspaceTab = {
      id,
      kind: 'objects',
      sessionId,
      title: list === 'index' ? 'Indexes' : FOLDER_LABEL[list],
      database,
      schema,
      list,
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
      // Opening (or re-using) a list window is also a request to show where it
      // came from, so the ribbon and the tree agree without the ribbon writing
      // the tree's own focus.
      reveal: { sessionId, database, schema, kind: list },
    }))
  },

  openTableTab(sessionId, database, schema, object, view) {
    const id = `table:${sessionId}:${database}:${schema}:${object.name}`
    const tab: WorkspaceTab = {
      id,
      kind: 'table',
      sessionId,
      title: object.name,
      database,
      schema,
      object: object.name,
      objectKind: object.kind,
      view: view ?? 'data',
    }
    set((state) => {
      const existing = state.tabs.find((t) => t.id === id)
      // Re-opening an already open object just brings it back to the requested
      // sub-view instead of duplicating the tab.
      const tabs = existing
        ? state.tabs.map((t) => (t.id === id ? { ...t, view: view ?? t.view ?? 'data' } : t))
        : [...state.tabs, tab]
      return {
        tabs,
        activeTabId: id,
        ...focused(state, sessionId),
      }
    })
  },

  openNewTableTab(sessionId, database, schema) {
    // Every "new table" window is its own draft, so it gets its own id instead
    // of being deduplicated like a table that has a name to key on.
    const id = `newtable:${sessionId}:${database}:${schema}:${Math.random().toString(36).slice(2, 10)}`
    const tab: WorkspaceTab = {
      id,
      kind: 'newtable',
      sessionId,
      title: 'New table',
      database,
      schema,
    }
    const driver = get().driverOf(sessionId)?.type
    set((state) => ({
      tabs: [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
      designs: {
        ...state.designs,
        [id]: {
          draft: newTableDesign(sessionId, database, schema, driver),
          baseline: emptyStructure(database, schema),
        },
      },
    }))
  },

  openDdlTab(sessionId, database, schema, object) {
    const id = `ddl:${sessionId}:${database}:${schema}:${object ?? '*'}`
    const tab: WorkspaceTab = {
      id,
      kind: 'ddl',
      sessionId,
      title: object ? `${object} DDL` : 'DDL script',
      database,
      schema,
      object,
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
    }))
  },

  openCodegenTab(sessionId, database, schema, object) {
    const id = `codegen:${sessionId}:${database}:${schema}:${object.name}`
    const tab: WorkspaceTab = {
      id,
      kind: 'codegen',
      sessionId,
      title: `${object.name} code`,
      database,
      schema,
      object: object.name,
      objectKind: object.kind,
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
    }))
  },

  openDataGenTab(sessionId, database, schema, object) {
    const id = `datagen:${sessionId}`
    set((state) => {
      const existing = state.tabs.find((tab) => tab.id === id)
      if (existing) {
        // Moving to another database moves to another place: pointing the
        // window at the old namespace or table would leave it aimed at
        // something that is no longer in the tree beside it.
        const samePlace = database === undefined || database === existing.database
        return {
          tabs: state.tabs.map((tab) =>
            tab.id === id
              ? {
                  ...tab,
                  database: database ?? tab.database,
                  schema: samePlace ? (schema ?? tab.schema) : schema,
                  object: samePlace ? (object ?? tab.object) : object,
                }
              : tab,
          ),
          activeTabId: id,
          ...focused(state, sessionId),
        }
      }
      const tab: WorkspaceTab = {
        id,
        kind: 'datagen',
        sessionId,
        title: 'Data generation',
        database,
        schema,
        object,
      }
      return {
        tabs: [...state.tabs, tab],
        activeTabId: id,
        ...focused(state, sessionId),
      }
    })
  },

  openErTab(sessionId, database, schema) {
    const id = `er:${sessionId}:${database}:${schema}`
    const tab: WorkspaceTab = {
      id,
      kind: 'er',
      sessionId,
      title: schema ? `ER · ${schema}` : 'ER diagram',
      database,
      schema,
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
    }))
  },

  openRuntimeTab(sessionId) {
    // One page per connection: its whole point is to be the place you look at
    // when you double-click the connection, so a second tab would only be a
    // stale copy of the same numbers.
    const id = `runtime:${sessionId}`
    const tab: WorkspaceTab = {
      id,
      kind: 'runtime',
      sessionId,
      title: 'Runtime',
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
      ...focused(state, sessionId),
    }))
  },

  openChangeLog() {
    // One window, not one per connection: the log covers every connection this
    // application has touched, so a copy per session would be the same file read
    // twice. It is also the one window that opens with nothing connected.
    const id = 'changelog'
    const tab: WorkspaceTab = {
      id,
      kind: 'changelog',
      sessionId: '',
      title: 'Change Log',
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.id === id) ? state.tabs : [...state.tabs, tab],
      activeTabId: id,
    }))
  },

  setTabView(tabId, view) {
    set((state) => ({
      tabs: state.tabs.map((tab) => (tab.id === tabId ? { ...tab, view } : tab)),
    }))
  },

  ensureDesign(tabId, structure, reset = false) {
    set((state) => {
      if (!reset && state.designs[tabId]) return state
      const sessionId = state.tabs.find((tab) => tab.id === tabId)?.sessionId ?? ''
      return {
        designs: {
          ...state.designs,
          [tabId]: { draft: designFrom(structure, sessionId), baseline: structure },
        },
      }
    })
  },

  updateDesign(tabId, draft) {
    set((state) => {
      const current = state.designs[tabId]
      if (!current) return state
      return { designs: { ...state.designs, [tabId]: { ...current, draft } } }
    })
  },

  dropDesign(tabId) {
    set((state) => {
      if (!state.designs[tabId]) return state
      const designs = { ...state.designs }
      delete designs[tabId]
      return { designs }
    })
  },

  closeTab(tabId) {
    set((state) => {
      const index = state.tabs.findIndex((t) => t.id === tabId)
      if (index < 0) return state
      const tabs = state.tabs.filter((t) => t.id !== tabId)
      let activeTabId = state.activeTabId
      if (state.activeTabId === tabId) {
        activeTabId = tabs[Math.min(index, tabs.length - 1)]?.id
      }
      return { tabs, activeTabId, designs: withoutKey(state.designs, tabId) }
    })
  },

  closeAllTabs() {
    set({ tabs: [], activeTabId: undefined, designs: {} })
  },

  setActiveTab(tabId) {
    set((state) => {
      const tab = state.tabs.find((t) => t.id === tabId)
      if (!tab) return { activeTabId: tabId }
      const reveal = revealOf(tab)
      return {
        activeTabId: tabId,
        ...focused(state, tab.sessionId),
        // Switching to a list window points the explorer at its folder as well.
        ...(reveal ? { reveal } : {}),
      }
    })
  },

  driverOf(sessionId) {
    const session = get().sessions.find((s) => s.id === sessionId)
    if (!session) return undefined
    return get().drivers.find((d) => d.type === session.driver)
  },

  sessionOf(sessionId) {
    return get().sessions.find((s) => s.id === sessionId)
  },
}))
