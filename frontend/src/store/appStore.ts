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
  DriverInfo,
  IndexEntry,
  ObjectInfo,
  ObjectKind,
  OpenRequest,
  SavedQuery,
  SessionInfo,
  TableDesign,
  TableStructure,
} from '../api/types'
import type { ConnectionDraft } from '../connection/shared'
import { databaseKey, FOLDER_LABEL, indexesKey, namespaceKey, objectsKey } from '../lib/tree'
import { designFrom } from '../lib/design'

export type TabKind = 'query' | 'table' | 'objects' | 'ddl' | 'er' | 'runtime'

/** Sub-views of a table/view window (Navicat-style bottom tab strip). */
export type TableView = 'data' | 'structure' | 'indexes' | 'foreignKeys' | 'ddl'

/** Object-list windows: one per folder of the explorer tree. */
export type ListScope = ObjectKind | 'index'

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
}

export type ThemeMode = 'light' | 'dark'


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
  loaded: Record<string, boolean>
  loading: Record<string, boolean>
  errors: Record<string, string>
}

const emptyTree = (): TreeCache => ({
  databases: {},
  schemas: {},
  objects: {},
  indexes: {},
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
  sessions: SessionInfo[]
  activeSessionId?: string
  tabs: WorkspaceTab[]
  activeTabId?: string
  theme: ThemeMode
  tree: TreeCache
  designs: Record<string, DesignState>
  savedQueries: SavedQuery[]
  editorOpen: boolean
  editorDraft?: ConnectionDraft
  /** Side panel that picks which driver a new connection is for. */
  pickerOpen: boolean

  bootstrap: () => Promise<void>
  setTheme: (theme: ThemeMode) => void
  openConnectionPicker: () => void
  closeConnectionPicker: () => void
  openConnectionEditor: (draft?: ConnectionDraft) => void
  closeConnectionEditor: () => void

  refreshSavedQueries: () => Promise<void>
  saveSavedQuery: (query: SavedQuery) => Promise<SavedQuery>
  deleteSavedQuery: (id: string) => Promise<void>

  refreshConnections: () => Promise<void>
  saveConnection: (cfg: ConnectionConfig) => Promise<ConnectionConfig>
  deleteConnection: (id: string) => Promise<void>

  openConnection: (req: OpenRequest) => Promise<SessionInfo>
  closeSession: (sessionId: string) => Promise<void>
  setActiveSession: (sessionId: string) => void

  loadDatabases: (sessionId: string) => Promise<void>
  loadSchemas: (sessionId: string, database: string) => Promise<void>
  loadObjects: (sessionId: string, database: string, schema: string) => Promise<void>
  loadIndexes: (sessionId: string, database: string, schema: string) => Promise<void>
  invalidateSession: (sessionId: string) => void

  openQueryTab: (sessionId: string, database?: string) => void
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
  /** Opens the ER diagram of one namespace (schema, or database when there is
   * no schema layer). */
  openErTab: (sessionId: string, database: string, schema: string) => void
  /** Opens the live status page of a connection (one per session). */
  openRuntimeTab: (sessionId: string) => void
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

/** Copy of an object without one key (drafts must die with their window). */
function withoutKey<T>(source: Record<string, T>, key: string): Record<string, T> {
  if (!source[key]) return source
  const out = { ...source }
  delete out[key]
  return out
}

export const useAppStore = create<AppState>((set, get) => ({
  boot: 'loading',
  drivers: [],
  connections: [],
  sessions: [],
  tabs: [],
  theme: 'light',
  tree: emptyTree(),
  designs: {},
  savedQueries: [],
  editorOpen: false,
  pickerOpen: false,

  openConnectionPicker() {
    set({ pickerOpen: true })
  },

  closeConnectionPicker() {
    set({ pickerOpen: false })
  },

  // Opening the editor always closes the picker: picking a driver is the only
  // thing the picker does, and a panel left standing behind the modal would
  // swallow the next click.
  openConnectionEditor(draft) {
    set({ pickerOpen: false, editorOpen: true, editorDraft: draft })
  },

  closeConnectionEditor() {
    set({ editorOpen: false, editorDraft: undefined })
  },

  async bootstrap() {
    try {
      const [appInfo, drivers, connections, savedQueries, persisted] = await Promise.all([
        api.appInfo(),
        api.listDrivers(),
        api.listConnections(),
        // A missing or unreadable favourites file must not block startup.
        api.listSavedQueries().catch(() => [] as SavedQuery[]),
        api.loadState().catch(() => ({}) as Record<string, unknown>),
      ])

      const storedTheme = persisted?.[STATE_KEY] as { theme?: ThemeMode } | undefined
      // Navicat's classic look is light; dark stays one toggle away.
      const theme: ThemeMode = storedTheme?.theme === 'dark' ? 'dark' : 'light'

      set({
        boot: 'ready',
        appInfo,
        drivers,
        connections,
        savedQueries,
        theme,
      })
    } catch (error) {
      set({ boot: 'failed', bootError: toMessage(error) })
    }
  },

  setTheme(theme) {
    set({ theme })
    // Fire and forget: a failed preference write must not disturb the UI.
    void api
      .saveState({ [STATE_KEY]: { theme } })
      .catch(() => undefined)
  },

  async refreshConnections() {
    const connections = await api.listConnections()
    set({ connections })
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

  async saveConnection(cfg) {
    const saved = await api.saveConnection(cfg)
    await get().refreshConnections()
    return saved
  },

  async deleteConnection(id) {
    await api.deleteConnection(id)
    await get().refreshConnections()
  },

  async openConnection(req) {
    const session = await api.openConnection(req)
    set((state) => {
      const others = state.sessions.filter((s) => s.id !== session.id)
      return {
        sessions: [...others, session],
        activeSessionId: session.id,
      }
    })
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
        return { sessions, tabs, designs, activeTabId, activeSessionId }
      })
    }
  },

  setActiveSession(sessionId) {
    set({ activeSessionId: sessionId })
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
        loaded: filter(state.tree.loaded),
        loading: filter(state.tree.loading),
        errors: filter(state.tree.errors),
      },
    }))
  },

  openQueryTab(sessionId, database) {
    const id = `query:${sessionId}:${Math.random().toString(36).slice(2, 10)}`
    const tab: WorkspaceTab = {
      id,
      kind: 'query',
      sessionId,
      title: 'Query',
      database,
    }
    set((state) => ({
      tabs: [...state.tabs, tab],
      activeTabId: id,
      activeSessionId: sessionId,
    }))
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
      activeSessionId: sessionId,
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
        activeSessionId: sessionId,
      }
    })
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
      activeSessionId: sessionId,
    }))
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
      activeSessionId: sessionId,
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
      activeSessionId: sessionId,
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
      return { activeTabId: tabId, activeSessionId: tab?.sessionId ?? state.activeSessionId }
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
