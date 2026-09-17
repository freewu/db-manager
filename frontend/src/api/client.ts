/**
 * Thin typed bridge over the Wails bindings.
 *
 * Every backend method is reachable through `api.*`, which keeps all the
 * `window.go` plumbing in one file and gives the rest of the app real types.
 */
import type {
  AppInfo,
  BackendBridge,
  CellUpdate,
  ConnectionConfig,
  DesignPlan,
  DesignResult,
  DriverInfo,
  ExecRequest,
  FetchRequest,
  FetchResult,
  IndexEntry,
  ObjectInfo,
  OpenRequest,
  QueryResult,
  RowDelete,
  SaveFileRequest,
  SavedQuery,
  ScriptAnalysis,
  SessionInfo,
  TableDesign,
  TableStructure,
  TestResult,
} from './types'

/** Wails injects `window.go.main.App` once the webview has booted. */
function bridge(): BackendBridge | undefined {
  const w = window as unknown as { go?: { main?: { App?: BackendBridge } } }
  return w.go?.main?.App
}

/** True when the page runs inside the Wails webview. */
export function isDesktop(): boolean {
  return bridge() !== undefined
}

/**
 * Human readable backend errors.
 *
 * Wails rejects the promise with the Go error string, which is already
 * sanitised on the backend; we only normalise non-Error rejections.
 */
export class BackendError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'BackendError'
  }
}

export function toMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  if (error == null) return 'Unknown error'
  try {
    return JSON.stringify(error)
  } catch {
    return String(error)
  }
}

async function invoke<T>(method: string, ...args: unknown[]): Promise<T> {
  const b = bridge()
  if (!b) {
    throw new BackendError(
      'Backend bridge is unavailable. Start the app with `wails dev` or `wails build` instead of opening the page directly.',
    )
  }
  const fn = b[method]
  if (typeof fn !== 'function') {
    throw new BackendError(`Unknown backend method: ${method}`)
  }
  try {
    return (await fn(...args)) as T
  } catch (error) {
    throw new BackendError(toMessage(error))
  }
}

export const api = {
  // --- application --------------------------------------------------------
  appInfo: () => invoke<AppInfo>('AppInfo'),
  listDrivers: () => invoke<DriverInfo[]>('ListDrivers'),
  loadState: () => invoke<Record<string, unknown>>('LoadState'),
  saveState: (state: Record<string, unknown>) => invoke<void>('SaveState', state),
  revealInExplorer: (path: string) => invoke<void>('RevealInExplorer', path),

  // --- query favourites ---------------------------------------------------
  listSavedQueries: () => invoke<SavedQuery[]>('ListSavedQueries'),
  saveSavedQuery: (query: SavedQuery) => invoke<SavedQuery>('SaveSavedQuery', query),
  deleteSavedQuery: (id: string) => invoke<void>('DeleteSavedQuery', id),

  // --- profiles -----------------------------------------------------------
  listConnections: () => invoke<ConnectionConfig[]>('ListConnections'),
  saveConnection: (cfg: ConnectionConfig) =>
    invoke<ConnectionConfig>('SaveConnection', cfg),
  deleteConnection: (id: string) => invoke<void>('DeleteConnection', id),
  testConnection: (cfg: ConnectionConfig) => invoke<TestResult>('TestConnection', cfg),

  // --- sessions -----------------------------------------------------------
  openConnection: (req: OpenRequest) => invoke<SessionInfo>('OpenConnection', req),
  closeConnection: (sessionId: string) => invoke<void>('CloseConnection', sessionId),
  listSessions: () => invoke<SessionInfo[]>('ListSessions'),

  // --- metadata -----------------------------------------------------------
  listDatabases: (sessionId: string) => invoke<string[]>('ListDatabases', sessionId),
  listSchemas: (sessionId: string, database: string) =>
    invoke<string[]>('ListSchemas', sessionId, database),
  listObjects: (sessionId: string, database: string, schema: string) =>
    invoke<ObjectInfo[]>('ListObjects', sessionId, database, schema),
  listIndexes: (sessionId: string, database: string, schema: string) =>
    invoke<IndexEntry[]>('ListIndexes', sessionId, database, schema),
  getStructure: (sessionId: string, database: string, schema: string, object: string) =>
    invoke<TableStructure>('GetStructure', sessionId, database, schema, object),

  // --- table designer -----------------------------------------------------
  planTableDesign: (design: TableDesign) =>
    invoke<DesignPlan>('PlanTableDesign', design),
  applyTableDesign: (design: TableDesign) =>
    invoke<DesignResult>('ApplyTableDesign', design),

  // --- data ---------------------------------------------------------------
  fetchRows: (req: FetchRequest) => invoke<FetchResult>('FetchRows', req),
  executeSql: (req: ExecRequest) => invoke<QueryResult>('ExecuteSQL', req),
  /** Dry run for the DDL editor: statement kinds + destructive warnings. */
  analyzeSql: (sessionId: string, sql: string) =>
    invoke<ScriptAnalysis>('AnalyzeSQL', sessionId, sql),
  updateCell: (req: CellUpdate) => invoke<number>('UpdateCell', req),
  deleteRow: (req: RowDelete) => invoke<number>('DeleteRow', req),

  // --- files --------------------------------------------------------------
  saveTextFile: (req: SaveFileRequest) => invoke<string>('SaveTextFile', req),
  pickFile: (title: string, patterns: string[]) =>
    invoke<string>('PickFile', title, patterns),
}

/**
 * Subset of the Wails runtime used by the UI. Guarded so the app degrades
 * gracefully when opened in a plain browser during frontend-only development.
 */
export const runtime = {
  setTitle(title: string) {
    const r = (window as unknown as { runtime?: { WindowSetTitle?: (t: string) => void } })
      .runtime
    r?.WindowSetTitle?.(title)
  },
  quit() {
    const r = (window as unknown as { runtime?: { Quit?: () => void } }).runtime
    r?.Quit?.()
  },
  onEvent(name: string, callback: (...args: unknown[]) => void): () => void {
    const r = (
      window as unknown as {
        runtime?: {
          EventsOn?: (n: string, cb: (...a: unknown[]) => void) => () => void
          EventsOff?: (n: string) => void
        }
      }
    ).runtime
    if (!r?.EventsOn) return () => undefined
    const off = r.EventsOn(name, callback)
    return typeof off === 'function' ? off : () => r.EventsOff?.(name)
  },
}
