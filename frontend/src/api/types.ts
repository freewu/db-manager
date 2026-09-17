/**
 * Wire types shared with the Go backend.
 *
 * These are hand written on purpose (instead of importing the generated
 * `wailsjs/go/...` bindings) so that `npm run build` works without a Go build
 * step first, and so that the types can use string unions and optional fields
 * the generator cannot express.
 *
 * Keep in sync with internal/models/models.go.
 */

export type DriverType =
  | 'mysql'
  | 'postgres'
  | 'sqlite'
  | 'mongodb'
  | 'oracle'
  | 'sqlserver'

export type SSLMode = 'disable' | 'require' | 'verify-ca' | 'verify-full'

export interface SSLConfig {
  mode: SSLMode
  caFile?: string
  certFile?: string
  keyFile?: string
}

export interface ConnectionConfig {
  id: string
  name: string
  driver: DriverType
  host?: string
  port?: number
  username?: string
  /** Only ever sent to the backend, never received from it. */
  password?: string
  database?: string
  filePath?: string
  params?: Record<string, string>
  ssl?: SSLConfig
  readOnly?: boolean
  color?: string
  /** Persist the password in the profile file. */
  savePassword?: boolean
  /** Read-only hint: a password is already stored for this profile. */
  hasPassword?: boolean
}

export interface DriverInfo {
  type: DriverType
  displayName: string
  defaultPort: number
  implemented: boolean
  relational: boolean
  supportsDatabase: boolean
  supportsSchema: boolean
  requiresFile: boolean
  defaultDatabase?: string
  sortOrder: number
  notes?: string
}

export interface SessionInfo {
  id: string
  name: string
  driver: DriverType
  connectionId?: string
  database?: string
  serverVersion?: string
  readOnly: boolean
  connectedAt: number
}

export interface OpenRequest {
  connectionId?: string
  config?: ConnectionConfig
  password?: string
  database?: string
  readOnly?: boolean
}

export interface TestResult {
  ok: boolean
  message: string
  serverVersion?: string
  latencyMs: number
  databaseCount?: number
  connectedDatabase?: string
}

export type ObjectKind =
  | 'table'
  | 'view'
  | 'materialized_view'
  | 'collection'
  | 'sequence'
  | 'procedure'

export interface ObjectInfo {
  name: string
  schema?: string
  database?: string
  kind: ObjectKind
  comment?: string
  rowEstimate: number
  sizeBytes: number
  engine?: string
}

export interface ColumnInfo {
  name: string
  ordinal: number
  dataType: string
  columnType: string
  nullable: boolean
  defaultValue?: string | null
  primaryKey: boolean
  autoIncrement: boolean
  comment?: string
  charMaxLength?: number | null
  numericPrecision?: number | null
  numericScale?: number | null
}

export interface IndexInfo {
  name: string
  columns: string[]
  unique: boolean
  primary: boolean
  method?: string
  comment?: string
}

/**
 * Namespace-level index row used by the explorer's “Indexes” folder.
 * Unlike {@link IndexInfo} it carries the owning table.
 */
export interface IndexEntry {
  name: string
  table: string
  schema?: string
  database?: string
  columns: string[]
  unique: boolean
  primary: boolean
  method?: string
}

export interface ForeignKeyInfo {
  name: string
  columns: string[]
  referencedSchema?: string
  referencedTable: string
  referencedColumns: string[]
  onUpdate?: string
  onDelete?: string
}

export interface TableStructure {
  object: ObjectInfo
  columns: ColumnInfo[]
  indexes: IndexInfo[]
  foreignKeys: ForeignKeyInfo[]
  ddl: string
}

/* --- ER diagram ---------------------------------------------------------- */

/** One line inside a diagram node. */
export interface GraphColumn {
  name: string
  type: string
  nullable: boolean
  primaryKey: boolean
}

/** One box of the ER diagram. */
export interface GraphNode {
  name: string
  kind: ObjectKind
  comment?: string
  columns: GraphColumn[]
}

/** A foreign key rendered as an arrow. `to` may point outside the namespace. */
export interface GraphEdge {
  from: string
  fromColumns: string[]
  to: string
  toColumns: string[]
  name?: string
  onDelete?: string
  onUpdate?: string
}

/** What the backend hands the ER window: a whole namespace at once. */
export interface SchemaGraph {
  driver?: DriverType
  database: string
  schema: string
  nodes: GraphNode[]
  edges: GraphEdge[]
  warnings: string[]
  truncated: boolean
}

/* --- scripts (DDL editor) ------------------------------------------------ */

/** One statement of a script plus how the app will treat it. */
export interface ScriptStatement {
  index: number
  kind: 'query' | 'ddl' | 'dml' | 'unknown'
  /** Comment-stripped single-line rendering of the statement. */
  preview: string
  destructive: boolean
  reason?: string
}

/** The dry run shown next to the DDL editor before anything runs. */
export interface ScriptAnalysis {
  statements: ScriptStatement[]
  warnings: string[]
  destructive: boolean
  readOnly: boolean
  /** Statements a read-only session would refuse. */
  refused: number
}

/* --- table designer ------------------------------------------------------ */

/**
 * The *desired* definition of a table, as edited in the structure tab.
 *
 * The whole definition is sent rather than a diff: the backend compares it with
 * the live catalog structure and renders the statements that turn one into the
 * other, so the SQL preview and the applied script are the same thing.
 */
export interface TableDesign {
  sessionId: string
  database?: string
  schema?: string
  object: string
  columns: DesignColumn[]
  indexes: DesignIndex[]
}

export interface DesignColumn {
  name: string
  /** The catalog name this field currently has; empty for a new field. */
  originalName?: string
  dataType: string
  nullable: boolean
  /**
   * Raw SQL: `'text'`, `0` or `CURRENT_TIMESTAMP` are emitted verbatim.
   * `null` means “no default”, which is not the same as `DEFAULT ''`.
   */
  defaultValue?: string | null
  primaryKey: boolean
  autoIncrement: boolean
  comment?: string
}

export interface DesignIndex {
  name: string
  originalName?: string
  columns: string[]
  unique: boolean
}

/** What the designer would run, shown before anything is applied. */
export interface DesignPlan {
  statements: string[]
  /** What this engine cannot express, or what the script does beyond the draft. */
  warnings: string[]
  /** True when the script drops a column or an index. */
  destructive: boolean
}

export interface DesignResult {
  plan: DesignPlan
  executed: string[]
  /** Index into `plan.statements` that failed, or -1 when the whole script ran. */
  failedIndex: number
  error?: string
  messages?: string[]
}

export interface ColumnMeta {
  name: string
  databaseType?: string
  nullable: boolean
  isPrimaryKey?: boolean
  editable?: boolean
}

/** A cell value as it arrives from the backend. */
export type CellValue = string | number | boolean | null

export interface QueryResult {
  columns: ColumnMeta[]
  rows: CellValue[][]
  rowCount: number
  affectedRows: number
  lastInsertId: number
  durationMs: number
  truncated: boolean
  hasResultSet: boolean
  sql: string
  statementIndex: number
  statementCount: number
  messages?: string[]
}

export interface SortSpec {
  column: string
  desc: boolean
}

export type FilterOperator =
  | 'eq'
  | 'ne'
  | 'gt'
  | 'gte'
  | 'lt'
  | 'lte'
  | 'contains'
  | 'notContains'
  | 'startsWith'
  | 'endsWith'
  | 'isNull'
  | 'isNotNull'
  | 'in'
  | 'notIn'
  | 'between'

export interface FilterSpec {
  column: string
  operator: FilterOperator
  value?: string
  value2?: string
}

export interface FetchRequest {
  sessionId: string
  database?: string
  schema?: string
  object: string
  limit?: number
  offset?: number
  orderBy?: SortSpec[]
  filters?: FilterSpec[]
  countTotal?: boolean
  timeoutMs?: number
}

export interface FetchResult extends QueryResult {
  total: number
  hasTotal: boolean
}

export interface ExecRequest {
  sessionId: string
  database?: string
  sql: string
  maxRows?: number
  timeoutMs?: number
  readOnly?: boolean
}

/** One component of a row identity (a primary key column and its value). */
export interface KeyValue {
  column: string
  /** Always sent as a string (or null) so no numeric precision is lost. */
  value: string | null
}

/** Inline edit of a single cell. */
export interface CellUpdate {
  sessionId: string
  database?: string
  schema?: string
  object: string
  key: KeyValue[]
  column: string
  value: string | null
}

/** Delete one row identified by its primary key. */
export interface RowDelete {
  sessionId: string
  database?: string
  schema?: string
  object: string
  key: KeyValue[]
}

export interface FileFilter {
  displayName: string
  pattern: string
}

export interface SaveFileRequest {
  defaultFilename: string
  content: string
  filters?: FileFilter[]
}

export interface AppInfo {
  name: string
  version: string
  goVersion: string
  configPath: string
  platform: string
}

/** A named SQL snippet kept in the user's favourites. */
export interface SavedQuery {
  id: string
  name: string
  sql: string
  /** Where the snippet was captured: shown as a hint, never used to block a load. */
  database?: string
  driver?: DriverType
  createdAt: number
  updatedAt: number
}

/** Shape of the generated Wails bridge on `window.go.main.App`. */
export type BackendBridge = Record<string, (...args: unknown[]) => Promise<unknown>>
