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
  ChangeLog,
  ChangeLogSettings,
  ConnectionConfig,
  ConnectionGroup,
  ConnectionLayout,
  CreateDatabaseRequest,
  DataDirInfo,
  DataDirMoveResult,
  DatabaseOptions,
  DatabasePlan,
  DesignPlan,
  DesignResult,
  DriverInfo,
  ExecRequest,
  ExplainRequest,
  ExplainResult,
  FetchRequest,
  FetchResult,
  IndexEntry,
  MockPlaceholder,
  ObjectInfo,
  OpenRequest,
  QueryResult,
  RowDelete,
  RowInsert,
  RowInsertResult,
  RowUpdate,
  SaveFileRequest,
  SavedQuery,
  QueryFile,
  QueryFileSave,
  QueryFileRename,
  SchemaGraph,
  ScriptAnalysis,
  ServerOverview,
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

  // --- settings -----------------------------------------------------------
  /** Where the app keeps its data, and what is in there right now. */
  getDataDir: () => invoke<DataDirInfo>('GetDataDir'),
  /** Native folder chooser; an empty string means the user cancelled. */
  pickDataDirectory: (title: string) => invoke<string>('PickDataDirectory', title),
  /** Moves the data into `dir` (empty = the default directory) and switches over. */
  moveDataDirectory: (dir: string) => invoke<DataDirMoveResult>('MoveDataDirectory', dir),

  // --- query favourites ---------------------------------------------------
  listSavedQueries: () => invoke<SavedQuery[]>('ListSavedQueries'),
  saveSavedQuery: (query: SavedQuery) => invoke<SavedQuery>('SaveSavedQuery', query),
  deleteSavedQuery: (id: string) => invoke<void>('DeleteSavedQuery', id),

  // --- saved query files --------------------------------------------------
  /** The scripts saved for one connection and database, without their text. */
  listQueryFiles: (connectionId: string, database: string) =>
    invoke<QueryFile[]>('ListQueryFiles', connectionId, database),
  readQueryFile: (connectionId: string, database: string, name: string) =>
    invoke<QueryFile>('ReadQueryFile', connectionId, database, name),
  saveQueryFile: (save: QueryFileSave) => invoke<QueryFile>('SaveQueryFile', save),
  /** Creates a script holding `sql`; refuses a name that is already taken. */
  createQueryFile: (connectionId: string, database: string, name: string, sql: string) =>
    invoke<QueryFile>('CreateQueryFile', connectionId, database, name, sql),
  /** Moves a script to another name without touching the script itself. */
  renameQueryFile: (rename: QueryFileRename) =>
    invoke<QueryFile>('RenameQueryFile', rename),
  deleteQueryFile: (connectionId: string, database: string, name: string) =>
    invoke<void>('DeleteQueryFile', connectionId, database, name),

  // --- custom mock placeholders -------------------------------------------
  /** Every placeholder the user defined, for the mock picker's Custom tab. */
  listMockPlaceholders: () => invoke<MockPlaceholder[]>('ListMockPlaceholders'),
  /** Creates or updates one, keyed by its name. */
  saveMockPlaceholder: (placeholder: MockPlaceholder) =>
    invoke<MockPlaceholder>('SaveMockPlaceholder', placeholder),
  deleteMockPlaceholder: (name: string) => invoke<void>('DeleteMockPlaceholder', name),

  // --- change log ---------------------------------------------------------
  /**
   * One file of the change log, newest entry first.
   *
   * It is read from the data directory rather than from a server, so it opens
   * with nothing connected — which is when a change usually needs looking up.
   * A limit of 0 asks for the default page, and an empty file name asks for the
   * file being written; the answer carries the files to choose between, so the
   * window never has to guess what else is there.
   */
  listChangeLog: (file = '', limit = 0) => invoke<ChangeLog>('ListChangeLog', file, limit),
  /** How many statements one log file holds before it is archived. */
  changeLogSettings: () => invoke<ChangeLogSettings>('ChangeLogSettings'),
  /** Saves it, and answers what is in force — the backend is what decides. */
  saveChangeLogSettings: (settings: ChangeLogSettings) =>
    invoke<ChangeLogSettings>('SaveChangeLogSettings', settings),

  // --- profiles -----------------------------------------------------------
  listConnections: () => invoke<ConnectionConfig[]>('ListConnections'),
  saveConnection: (cfg: ConnectionConfig) =>
    invoke<ConnectionConfig>('SaveConnection', cfg),
  deleteConnection: (id: string) => invoke<void>('DeleteConnection', id),
  testConnection: (cfg: ConnectionConfig) => invoke<TestResult>('TestConnection', cfg),

  // --- explorer arrangement -----------------------------------------------
  /** The connection tree as the user left it (groups, membership, order). */
  listConnectionLayout: () => invoke<ConnectionLayout>('ListConnectionLayout'),
  saveConnectionLayout: (layout: ConnectionLayout) =>
    invoke<ConnectionLayout>('SaveConnectionLayout', layout),
  saveConnectionGroup: (group: ConnectionGroup) =>
    invoke<ConnectionGroup>('SaveConnectionGroup', group),
  deleteConnectionGroup: (id: string) => invoke<void>('DeleteConnectionGroup', id),

  // --- sessions -----------------------------------------------------------
  openConnection: (req: OpenRequest) => invoke<SessionInfo>('OpenConnection', req),
  closeConnection: (sessionId: string) => invoke<void>('CloseConnection', sessionId),
  listSessions: () => invoke<SessionInfo[]>('ListSessions'),

  // --- metadata -----------------------------------------------------------
  listDatabases: (sessionId: string) => invoke<string[]>('ListDatabases', sessionId),
  /** What this server accepts for a new database (charsets/encodings). */
  databaseOptions: (sessionId: string) =>
    invoke<DatabaseOptions>('DatabaseOptions', sessionId),
  /** The CREATE DATABASE statement for the dialog, rendered by the driver. */
  planCreateDatabase: (sessionId: string, req: CreateDatabaseRequest) =>
    invoke<DatabasePlan>('PlanCreateDatabase', sessionId, req),
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
  /** The same pair for a table that does not exist yet. */
  planCreateTable: (design: TableDesign) =>
    invoke<DesignPlan>('PlanCreateTable', design),
  applyCreateTable: (design: TableDesign) =>
    invoke<DesignResult>('ApplyCreateTable', design),

  // --- data ---------------------------------------------------------------
  fetchRows: (req: FetchRequest) => invoke<FetchResult>('FetchRows', req),
  executeSql: (req: ExecRequest) => invoke<QueryResult>('ExecuteSQL', req),
  /**
   * How the engine would run one statement. Nothing is executed, which is why
   * this is safe on a read-only session and on a statement that writes.
   */
  explainSql: (req: ExplainRequest) => invoke<ExplainResult>('ExplainSQL', req),
  /** Objects, columns and foreign keys of a namespace, for the ER diagram. */
  getSchemaGraph: (sessionId: string, database: string, schema: string) =>
    invoke<SchemaGraph>('GetSchemaGraph', sessionId, database, schema),
  /** Dry run for the DDL editor: statement kinds + destructive warnings. */
  analyzeSql: (sessionId: string, sql: string) =>
    invoke<ScriptAnalysis>('AnalyzeSQL', sessionId, sql),
  /** Live state of one connection: uptime, sessions, cache, running queries. */
  getServerOverview: (sessionId: string) =>
    invoke<ServerOverview>('GetServerOverview', sessionId),
  updateCell: (req: CellUpdate) => invoke<number>('UpdateCell', req),
  deleteRow: (req: RowDelete) => invoke<number>('DeleteRow', req),
  /**
   * The statement an edit of this row would run, rendered by the engine that
   * would run it. Nothing is executed, so it is safe to ask before agreeing.
   */
  planRowUpdate: (req: RowUpdate) => invoke<string>('PlanRowUpdate', req),
  /** Applies an edit of several columns of one row, as a single statement. */
  updateRow: (req: RowUpdate) => invoke<number>('UpdateRow', req),
  /**
   * Appends one batch of generated rows. A batch the engine refuses comes back
   * as a result naming the row that stopped it — or counting the rows it was
   * asked to skip — not as a rejection.
   */
  insertRows: (req: RowInsert) => invoke<RowInsertResult>('InsertRows', req),

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
