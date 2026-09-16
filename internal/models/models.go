// Package models contains the data transfer objects shared between the Go
// backend and the React frontend. Everything in here is JSON-serialised across
// the Wails bridge, therefore:
//
//   - all fields are exported
//   - json tags use camelCase to match the TypeScript types
//   - timestamps are int64 unix milliseconds (the Wails TS generator has no
//     built-in representation for time.Time)
package models

// DriverType identifies a concrete database driver implementation.
type DriverType string

const (
	DriverMySQL     DriverType = "mysql"
	DriverPostgres  DriverType = "postgres"
	DriverSQLite    DriverType = "sqlite"
	DriverMongoDB   DriverType = "mongodb"
	DriverOracle    DriverType = "oracle"
	DriverSQLServer DriverType = "sqlserver"
)

// SSLMode mirrors the common libpq/MySQL SSL modes.
type SSLMode string

const (
	SSLDisable    SSLMode = "disable"
	SSLRequire    SSLMode = "require"
	SSLVerifyCA   SSLMode = "verify-ca"
	SSLVerifyFull SSLMode = "verify-full"
)

// SSLConfig describes the TLS settings of a connection.
type SSLConfig struct {
	Mode     SSLMode `json:"mode"`
	CAFile   string  `json:"caFile,omitempty"`
	CertFile string  `json:"certFile,omitempty"`
	KeyFile  string  `json:"keyFile,omitempty"`
}

// ConnectionConfig is a saved connection profile.
//
// Password is never sent back to the UI: ListConnections returns redacted
// copies and the UI only ever sends a password when the user types one.
type ConnectionConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Driver   DriverType        `json:"driver"`
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Database string            `json:"database,omitempty"`
	FilePath string            `json:"filePath,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
	SSL      SSLConfig         `json:"ssl,omitempty"`
	ReadOnly bool              `json:"readOnly,omitempty"`
	Color    string            `json:"color,omitempty"`

	// SavePassword controls whether Password is written to disk. It is a
	// profile-level preference, not a secret.
	SavePassword bool `json:"savePassword,omitempty"`
	// HasPassword tells the UI that a password is already stored for this
	// profile (so it can show a "leave empty to reuse" hint).
	HasPassword bool `json:"hasPassword,omitempty"`
}

// Redacted returns a copy of the config that is safe to hand to the frontend.
func (c ConnectionConfig) Redacted() ConnectionConfig {
	c.HasPassword = c.Password != ""
	c.Password = ""
	return c
}

// DriverInfo describes a driver to the UI so it can render the right form and
// decide whether an entry is selectable yet.
type DriverInfo struct {
	Type             DriverType `json:"type"`
	DisplayName      string     `json:"displayName"`
	DefaultPort      int        `json:"defaultPort"`
	Implemented      bool       `json:"implemented"`
	Relational       bool       `json:"relational"`
	SupportsDatabase bool       `json:"supportsDatabase"`
	SupportsSchema   bool       `json:"supportsSchema"`
	RequiresFile     bool       `json:"requiresFile"`
	DefaultDatabase  string     `json:"defaultDatabase,omitempty"`
	SortOrder        int        `json:"sortOrder"`
	Notes            string     `json:"notes,omitempty"`
}

// SessionInfo describes a live (or recently live) connection held by the
// backend.
type SessionInfo struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Driver        DriverType `json:"driver"`
	ConnectionID  string     `json:"connectionId,omitempty"`
	Database      string     `json:"database,omitempty"`
	ServerVersion string     `json:"serverVersion,omitempty"`
	ReadOnly      bool       `json:"readOnly"`
	ConnectedAt   int64      `json:"connectedAt"`
}

// OpenRequest is the payload of App.OpenConnection. Either ConnectionID (a
// stored profile) or Config (an unsaved, ad-hoc connection) must be set.
type OpenRequest struct {
	ConnectionID string            `json:"connectionId,omitempty"`
	Config       *ConnectionConfig `json:"config,omitempty"`
	Password     string            `json:"password,omitempty"`
	Database     string            `json:"database,omitempty"`
	ReadOnly     bool              `json:"readOnly,omitempty"`
}

// TestResult is the outcome of App.TestConnection.
type TestResult struct {
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	ServerVer   string `json:"serverVersion,omitempty"`
	LatencyMS   int64  `json:"latencyMs"`
	Databases   int    `json:"databaseCount,omitempty"`
	ConnectedDB string `json:"connectedDatabase,omitempty"`
}

// ObjectKind classifies a database object in the explorer tree.
type ObjectKind string

const (
	KindTable      ObjectKind = "table"
	KindView       ObjectKind = "view"
	KindMatView    ObjectKind = "materialized_view"
	KindCollection ObjectKind = "collection"
	KindSequence   ObjectKind = "sequence"
	KindProcedure  ObjectKind = "procedure"
)

// ObjectInfo is a table / view / collection summary.
type ObjectInfo struct {
	Name        string     `json:"name"`
	Schema      string     `json:"schema,omitempty"`
	Database    string     `json:"database,omitempty"`
	Kind        ObjectKind `json:"kind"`
	Comment     string     `json:"comment,omitempty"`
	RowEstimate int64      `json:"rowEstimate"`
	SizeBytes   int64      `json:"sizeBytes"`
	Engine      string     `json:"engine,omitempty"`
}

// ColumnInfo describes a single column (or document field).
type ColumnInfo struct {
	Name             string  `json:"name"`
	Ordinal          int     `json:"ordinal"`
	DataType         string  `json:"dataType"`
	ColumnType       string  `json:"columnType"`
	Nullable         bool    `json:"nullable"`
	DefaultValue     *string `json:"defaultValue,omitempty"`
	PrimaryKey       bool    `json:"primaryKey"`
	AutoIncrement    bool    `json:"autoIncrement"`
	Comment          string  `json:"comment,omitempty"`
	CharMaxLength    *int64  `json:"charMaxLength,omitempty"`
	NumericPrecision *int64  `json:"numericPrecision,omitempty"`
	NumericScale     *int64  `json:"numericScale,omitempty"`
}

// IndexInfo describes an index.
type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Method  string   `json:"method,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// IndexEntry is a namespace (database/schema) level index summary used by the
// explorer tree.
//
// IndexInfo always belongs to exactly one object, so it has no need to repeat
// the table name. A database-wide listing does, hence this separate type.
type IndexEntry struct {
	Name     string   `json:"name"`
	Table    string   `json:"table"`
	Schema   string   `json:"schema,omitempty"`
	Database string   `json:"database,omitempty"`
	Columns  []string `json:"columns"`
	Unique   bool     `json:"unique"`
	Primary  bool     `json:"primary"`
	Method   string   `json:"method,omitempty"`
}

// ForeignKeyInfo describes a foreign key constraint.
type ForeignKeyInfo struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referencedSchema,omitempty"`
	ReferencedTable   string   `json:"referencedTable"`
	ReferencedColumns []string `json:"referencedColumns"`
	OnUpdate          string   `json:"onUpdate,omitempty"`
	OnDelete          string   `json:"onDelete,omitempty"`
}

// TableStructure is the full description of one object.
type TableStructure struct {
	Object      ObjectInfo       `json:"object"`
	Columns     []ColumnInfo     `json:"columns"`
	Indexes     []IndexInfo      `json:"indexes"`
	ForeignKeys []ForeignKeyInfo `json:"foreignKeys"`
	DDL         string           `json:"ddl"`
}

// --- table designer --------------------------------------------------------

// TableDesign is the *desired* definition of one table, produced by the table
// designer window.
//
// The designer always sends the whole definition rather than a diff: the
// backend compares it against the live catalog structure and renders the
// statements that turn one into the other. That keeps the dialect knowledge in
// one place and makes the preview and the applied script the exact same code
// path.
//
type TableDesign struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
	Object    string `json:"object"`

	Columns []DesignColumn `json:"columns"`
	Indexes []DesignIndex  `json:"indexes"`
}

// DesignColumn is one field of a table design.
//
type DesignColumn struct {
	// Name is what the column should be called. OriginalName is the catalog
	// name it currently has (empty for a column that is about to be created),
	// so a rename is just a difference between the two.
	Name         string `json:"name"`
	OriginalName string `json:"originalName,omitempty"`

	// DataType is the complete type text as the engine spells it, e.g.
	// "varchar(255)" or "timestamp with time zone".
	DataType string `json:"dataType"`

	Nullable bool `json:"nullable"`
	// DefaultValue is raw SQL (Navicat behaves the same way): the user types
	// 'text', 0 or CURRENT_TIMESTAMP and it is emitted verbatim. nil means "no
	// default", an empty string means DEFAULT ''.
	DefaultValue  *string `json:"defaultValue,omitempty"`
	PrimaryKey    bool    `json:"primaryKey"`
	AutoIncrement bool    `json:"autoIncrement"`
	Comment       string  `json:"comment,omitempty"`
}

// DesignIndex is one index of a table design.
type DesignIndex struct {
	Name         string   `json:"name"`
	OriginalName string   `json:"originalName,omitempty"`
	Columns      []string `json:"columns"`
	Unique       bool     `json:"unique"`
}

// DesignPlan is what the designer shows before anything is applied: the script
// it would run, plus everything the user should know about it.
type DesignPlan struct {
	Statements []string `json:"statements"`
	// Warnings explain what this engine cannot express, or what the script
	// does beyond what was asked (e.g. a primary key column forced NOT NULL).
	Warnings []string `json:"warnings"`
	// Destructive is true when the script drops a column or an index.
	Destructive bool `json:"destructive"`
}

// DesignResult is the outcome of applying a design.
type DesignResult struct {
	Plan     DesignPlan `json:"plan"`
	Executed []string   `json:"executed"`
	// FailedIndex is the index into Plan.Statements that failed, or -1 when the
	// whole script ran.
	FailedIndex int      `json:"failedIndex"`
	Error       string   `json:"error,omitempty"`
	Messages    []string `json:"messages,omitempty"`
}

// ColumnMeta describes one column of a result set.
type ColumnMeta struct {
	Name         string `json:"name"`
	DatabaseType string `json:"databaseType,omitempty"`
	Nullable     bool   `json:"nullable"`
	IsPrimaryKey bool   `json:"isPrimaryKey,omitempty"`
	Editable     bool   `json:"editable,omitempty"`
}

// QueryResult is the envelope returned for every executed statement.
type QueryResult struct {
	Columns      []ColumnMeta `json:"columns"`
	Rows         [][]any      `json:"rows"`
	RowCount     int          `json:"rowCount"`
	AffectedRows int64        `json:"affectedRows"`
	LastInsertID int64        `json:"lastInsertId"`
	DurationMS   int64        `json:"durationMs"`
	Truncated    bool         `json:"truncated"`
	HasResultSet bool         `json:"hasResultSet"`
	SQL          string       `json:"sql"`
	// Batch bookkeeping for multi-statement scripts.
	StatementIndex int      `json:"statementIndex"`
	StatementCount int      `json:"statementCount"`
	Messages       []string `json:"messages,omitempty"`
}

// SortSpec is one ORDER BY entry of a data-grid request.
type SortSpec struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

// FilterSpec is one WHERE entry of a data-grid request. Operator is one of the
// sqlutil.Op* constants.
type FilterSpec struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
	Value2   string `json:"value2,omitempty"`
}

// FetchRequest asks for a page of rows from a single object.
type FetchRequest struct {
	SessionID  string       `json:"sessionId"`
	Database   string       `json:"database,omitempty"`
	Schema     string       `json:"schema,omitempty"`
	Object     string       `json:"object"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	OrderBy    []SortSpec   `json:"orderBy,omitempty"`
	Filters    []FilterSpec `json:"filters,omitempty"`
	CountTotal bool         `json:"countTotal,omitempty"`
	TimeoutMS  int          `json:"timeoutMs,omitempty"`
}

// FetchResult is a page of rows plus optional total count.
type FetchResult struct {
	QueryResult
	Total    int64 `json:"total"`
	HasTotal bool  `json:"hasTotal"`
}

// ExecRequest runs an arbitrary SQL script.
type ExecRequest struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	SQL       string `json:"sql"`
	MaxRows   int    `json:"maxRows,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// KeyValue is one component of a row identity.
//
// Keep this a slice rather than a map: the generated WHERE clause must be
// deterministic, and some engines are picky about the parameter order.
type KeyValue struct {
	Column string `json:"column"`
	Value  any    `json:"value"`
}

// CellUpdate edits a single cell of a single row.
//
// The row is identified by its primary key columns; drivers build a
// parameterised UPDATE, so the value is never interpolated into SQL.
type CellUpdate struct {
	SessionID string     `json:"sessionId"`
	Database  string     `json:"database,omitempty"`
	Schema    string     `json:"schema,omitempty"`
	Object    string     `json:"object"`
	Key       []KeyValue `json:"key"`
	Column    string     `json:"column"`
	Value     any        `json:"value"`
}

// RowDelete removes a single row identified by its primary key.
type RowDelete struct {
	SessionID string     `json:"sessionId"`
	Database  string     `json:"database,omitempty"`
	Schema    string     `json:"schema,omitempty"`
	Object    string     `json:"object"`
	Key       []KeyValue `json:"key"`
}

// SaveFileRequest writes text content to a user chosen path.
type SaveFileRequest struct {
	DefaultFilename string       `json:"defaultFilename"`
	Content         string       `json:"content"`
	Filters         []FileFilter `json:"filters,omitempty"`
}

// FileFilter is a save-dialog file type entry.
type FileFilter struct {
	DisplayName string `json:"displayName"`
	Pattern     string `json:"pattern"`
}

// AppInfo is static metadata rendered on the welcome screen.
type AppInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	ConfigPath string `json:"configPath"`
	Platform   string `json:"platform"`
}
