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
	DriverTiDB      DriverType = "tidb"
	DriverDoris     DriverType = "doris"
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
// copies and the UI only ever sends a password when the user types one. It is
// also never written to disk in the clear — when the profile opts into
// SavePassword the store keeps a sealed (AES-256-GCM) token instead, and the
// value in memory here stays plain. See internal/secret.
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

// ConnectionGroup is a named folder in the connection explorer.
//
// Groups are one level deep on purpose: a folder that can hold folders turns
// "where is that server" into a search problem, and no engine the explorer talks
// to organises its own objects any deeper than this.
//
// Order places the group among the explorer's top-level entries, which shares
// one sequence with the ungrouped connections (see ConnectionPlacement).
type ConnectionGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

// ConnectionPlacement says where one saved profile sits in the explorer: which
// group holds it (empty means the top level) and its position among its
// siblings.
//
// It is kept apart from ConnectionConfig so that editing a profile — or a
// profile written by a build that never heard of groups — cannot lose the
// arrangement.
type ConnectionPlacement struct {
	ID      string `json:"id"`
	GroupID string `json:"groupId,omitempty"`
	Order   int    `json:"order"`
}

// ConnectionLayout is the connection explorer as the user arranged it: the
// groups, and every profile with its group and position.
//
// Order is authoritative on both sides. Groups and ungrouped profiles share one
// top-level sequence, while a group's members have their own sequence starting
// at zero. Entries that share an order keep the order they were stored in (a
// group winning a tie against a profile), so the same file always draws the
// same tree.
type ConnectionLayout struct {
	Groups []ConnectionGroup     `json:"groups"`
	Items  []ConnectionPlacement `json:"items"`
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

	// SupportsDesign says the table designer can turn a draft of this engine's
	// table into an ALTER script. It is narrower than Relational: an engine
	// whose DDL needs clauses the designer does not model (Doris) still browses
	// tables and edits them through the DDL editor, but gets a read-only field
	// list in place of the designer.
	SupportsDesign bool `json:"supportsDesign"`

	// SupportsExplain says the engine can be asked how it would run a statement
	// without running it, which is what the query window's plan view shows. It
	// is deliberately not `relational`: the flag is set where the driver
	// implements drivers.Explainer, so the window can hide the button instead of
	// offering an action that always fails (a document store has no such
	// statement to send).
	SupportsExplain bool `json:"supportsExplain"`

	// SupportsInsert says generated rows can be appended to a table here, which
	// is what the data generation window writes through. It is set where the
	// driver implements drivers.Inserter. A document store does not: a
	// collection has no column list to fill, so the window says so instead of
	// offering a button that could only fail.
	SupportsInsert bool `json:"supportsInsert"`

	// ObjectKinds are the kinds of object this engine can hold, in the order the
	// explorer draws their folders. Every one of them gets a folder whether or
	// not it holds anything: "Tables (0)" tells the user the database exists and
	// is empty, whereas a folder that appears only once something is in it
	// cannot be told apart from an engine that has no such folder. A kind the
	// engine can never return does not belong in this list — the folder would
	// stay empty forever. It is deliberately not derived from the current
	// object list: an empty database still has to show its folders.
	ObjectKinds []ObjectKind `json:"objectKinds"`
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
type TableDesign struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
	Object    string `json:"object"`

	Columns []DesignColumn `json:"columns"`
	Indexes []DesignIndex  `json:"indexes"`
}

// DesignColumn is one field of a table design.
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

// --- databases -------------------------------------------------------------

// DatabaseCharset is one character set (MySQL family) or encoding (PostgreSQL)
// a server accepts for a new database, together with the collations that go
// with it.
type DatabaseCharset struct {
	Name string `json:"name"`
	// Default marks what the server itself would use, which is also what
	// leaving the clause out means.
	Default bool `json:"default"`
	// Collation is that charset's own default collation (MySQL).
	Collation string `json:"collation,omitempty"`
	// Collations are the ones it can be paired with (MySQL). Empty when the
	// engine pairs the two itself and has nothing to choose.
	Collations []string `json:"collations,omitempty"`
}

// DatabaseOptions is what a driver knows about creating a database on a live
// server: the choices its server offers for the new database's character
// handling, and the sentences that explain the engine's own rules.
//
// It is read when the "New database" window opens, never guessed: MySQL answers
// SHOW CHARACTER SET / SHOW COLLATION, PostgreSQL reports the encodings and
// locales it can be created with. Every list may be empty — Doris has no
// database level character set at all, and MongoDB creates databases by
// naming them — and the window then simply asks for a name.
type DatabaseOptions struct {
	Charsets []DatabaseCharset `json:"charsets"`
	// Collations are the choices that belong to no single charset, which is how
	// PostgreSQL's locales arrive. MySQL pairs them and fills
	// Charsets[].Collations instead.
	Collations []string `json:"collations,omitempty"`
	// CharsetLabel and CollationLabel name the two choices for this engine:
	// PostgreSQL says "Encoding", MySQL says "Character set".
	CharsetLabel   string `json:"charsetLabel,omitempty"`
	CollationLabel string `json:"collationLabel,omitempty"`
	// CollationEditable says the list cannot be complete, so the window also
	// lets the user type a value: PostgreSQL locale names come from the
	// operating system, not from a catalog.
	CollationEditable bool `json:"collationEditable,omitempty"`
	// Hint is the sentence the window shows under the form, for the part of the
	// statement the engine's own rules decide.
	Hint string `json:"hint,omitempty"`
}

// CreateDatabaseRequest is what the "New database" window collected before the
// statement can be rendered.
type CreateDatabaseRequest struct {
	Name      string `json:"name"`
	Charset   string `json:"charset,omitempty"`
	Collation string `json:"collation,omitempty"`
}

// DatabasePlan is the statement that creates the database. Like the designer's
// DesignPlan it is shown verbatim before it runs, so the window never assembles
// SQL of its own.
type DatabasePlan struct {
	Statement string `json:"statement"`
	// Warnings explain what the engine does beyond the statement itself (a
	// MongoDB database comes into being with its first collection).
	Warnings []string `json:"warnings,omitempty"`
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

// --- query plans -----------------------------------------------------------

// ExplainResult is what an engine answers when it is asked how it *would* run a
// statement.
//
// The plan is poured into the same grid shape a result set uses (Columns/Rows)
// because every engine describes it differently: MySQL answers with a table of
// id/select_type/table/rows/Extra, PostgreSQL with a column of indented text,
// SQLite with one row per step of a plan tree. Teaching the window each of
// those shapes would mean the next engine needs a new view; mapping them in the
// driver means the answer is already readable.
type ExplainResult struct {
	// Statement is the exact text that was sent, so a plan can never be read as
	// if it were about other SQL than the one in the editor.
	Statement string `json:"statement"`
	// SQL is the statement being explained, without the EXPLAIN wrapper.
	SQL string `json:"sql"`

	Columns []ColumnMeta `json:"columns"`
	Rows    [][]any      `json:"rows"`

	// Notes say what the plan itself does not: that it is an estimate rather
	// than a measurement, and anything engine specific that would otherwise be
	// misread as a fact about the query.
	Notes      []string `json:"notes,omitempty"`
	DurationMS int64    `json:"durationMs"`
	Truncated  bool     `json:"truncated"`
}

// ExplainRequest asks about one statement. It lives here rather than reusing
// ExecRequest because a plan request has one statement, no row limit and no
// read-only flag: explaining changes nothing, so a read-only session may do it.
type ExplainRequest struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	SQL       string `json:"sql"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
}

// --- scripts (DDL editor) --------------------------------------------------

// ScriptStatement is one statement of a script plus how the app will treat it.
type ScriptStatement struct {
	// Index is the zero-based position of the statement in the script.
	Index int    `json:"index"`
	Kind  string `json:"kind"`
	// Preview is the comment-stripped, single-line rendering of the statement.
	Preview string `json:"preview"`
	// Destructive marks statements that can lose schema or data.
	Destructive bool   `json:"destructive"`
	Reason      string `json:"reason,omitempty"`
}

// ScriptAnalysis is the dry run the DDL editor shows before running anything.
//
// It is produced without contacting the server: the editor uses it to warn
// about destructive statements and to explain what a read-only session would
// refuse, not to validate syntax.
type ScriptAnalysis struct {
	Statements  []ScriptStatement `json:"statements"`
	Warnings    []string          `json:"warnings"`
	Destructive bool              `json:"destructive"`
	ReadOnly    bool              `json:"readOnly"`
	// Refused counts the statements a read-only session would reject.
	Refused int `json:"refused"`
}

// --- ER diagram ------------------------------------------------------------

// SchemaGraph is the ER diagram's input: the objects of one namespace, their
// columns and the foreign keys between them.
type SchemaGraph struct {
	Driver   string      `json:"driver,omitempty"`
	Database string      `json:"database"`
	Schema   string      `json:"schema"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	// Warnings lists objects whose columns or keys could not be read. They
	// still appear in the diagram, just without their details.
	Warnings []string `json:"warnings"`
	// Truncated is set when the namespace holds more objects than the cap.
	Truncated bool `json:"truncated"`
}

// GraphNode is one box of the diagram.
type GraphNode struct {
	Name    string        `json:"name"`
	Kind    ObjectKind    `json:"kind"`
	Comment string        `json:"comment,omitempty"`
	Columns []GraphColumn `json:"columns"`
}

// GraphColumn is one line inside a node.
type GraphColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primaryKey"`
}

// GraphEdge is a foreign key rendered as an arrow.
type GraphEdge struct {
	// From is the child table (the one holding the foreign key).
	From       string   `json:"from"`
	FromColumn []string `json:"fromColumns"`
	// To is the referenced (parent) table. It may point outside the namespace,
	// in which case no node exists for it and the UI draws a stub.
	To       string   `json:"to"`
	ToColumn []string `json:"toColumns"`
	Name     string   `json:"name,omitempty"`
	OnDelete string   `json:"onDelete,omitempty"`
	OnUpdate string   `json:"onUpdate,omitempty"`
}

// --- server runtime overview ------------------------------------------------+-

// OverviewMetric is one labelled number on a status page.
//
// Values are pre-formatted by the driver: the engine knows that "1.5 GiB" and
// "43%" are the readable forms of its own counters, and the UI must not have to
// learn each engine's units.
type OverviewMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	// Hint explains where the number comes from (the variable or view it was
	// read from), shown as a tooltip.
	Hint string `json:"hint,omitempty"`
	// State is "", "good", "warn" or "bad" and only drives colour.
	State string `json:"state,omitempty"`
}

// OverviewGroup is a titled block of metrics.
type OverviewGroup struct {
	Title   string           `json:"title"`
	Note    string           `json:"note,omitempty"`
	Metrics []OverviewMetric `json:"metrics"`
}

// OverviewTable is a small titled table (process list, database sizes, objects).
type OverviewTable struct {
	Title   string     `json:"title"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Note    string     `json:"note,omitempty"`
}

// ServerOverview is the runtime snapshot of one live session.
//
// Exactly one of the engine fields is set, and it always matches Driver. Each
// engine reports different things — MySQL has a process list, PostgreSQL has
// per-database statistics, SQLite has a file on disk — so the payload is a union
// rather than one shape every engine has to fill in. The UI picks the matching
// view instead of branching per field.
type ServerOverview struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	// ServerVersion is the same string the tree shows.
	ServerVersion string `json:"serverVersion"`
	Database      string `json:"database,omitempty"`
	ReadOnly      bool   `json:"readOnly"`
	ConnectedAt   int64  `json:"connectedAt"`
	CollectedAt   int64  `json:"collectedAt"`
	// ElapsedMS is how long the snapshot took; a slow status page is itself a
	// symptom worth showing.
	ElapsedMS int64 `json:"elapsedMs"`
	// Supported is false when the engine has no runtime reporting at all; the UI
	// then shows only the session header and Warnings.
	Supported bool `json:"supported"`
	// Warnings lists sections that could not be read (missing privileges, an
	// older server version). The rest of the page still renders.
	Warnings []string `json:"warnings"`

	MySQL    *MySQLOverview    `json:"mysql,omitempty"`
	Postgres *PostgresOverview `json:"postgres,omitempty"`
	SQLite   *SQLiteOverview   `json:"sqlite,omitempty"`
	Mongo    *MongoOverview    `json:"mongodb,omitempty"`
}

// MySQLOverview is the MySQL/MariaDB status page: global status counters
// grouped by theme, plus the live process list.
type MySQLOverview struct {
	Groups    []OverviewGroup `json:"groups"`
	Processes *OverviewTable  `json:"processes,omitempty"`
}

// PostgresOverview is the PostgreSQL status page: server settings and activity
// counters, the size of every database, and the queries currently running.
type PostgresOverview struct {
	Groups    []OverviewGroup `json:"groups"`
	Databases *OverviewTable  `json:"databases,omitempty"`
	Activity  *OverviewTable  `json:"activity,omitempty"`
}

// SQLiteOverview is the SQLite status page: the file itself, the pragmas that
// describe the database format, and what is stored in it.
type SQLiteOverview struct {
	// Path is the database file, or ":memory:" for a transient database.
	Path string `json:"path"`
	// FileSize is the size on disk in bytes, or -1 when there is no file.
	FileSize int64           `json:"fileSize"`
	Groups   []OverviewGroup `json:"groups"`
	Objects  *OverviewTable  `json:"objects,omitempty"`
	// Attached lists the ATTACHed files, which is SQLite's equivalent of the
	// other databases on a server.
	Attached *OverviewTable `json:"attached,omitempty"`
}

// MongoOverview is the MongoDB status page: server counters (uptime,
// connections, operations, memory, WiredTiger cache) plus the size of every
// database on the deployment.
//
// MongoDB has no process list — it reports a connection count instead — so the
// second block is a table of databases rather than of sessions.
type MongoOverview struct {
	Groups    []OverviewGroup `json:"groups"`
	Databases *OverviewTable  `json:"databases,omitempty"`
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

// RowInsert appends a batch of generated rows to one table.
//
// The values arrive from the data generation window as JSON scalars (strings,
// numbers, booleans, null) in `Columns` order, one slice per row. Drivers bind
// every one of them as a parameter: the only text that reaches the statement is
// an identifier, and identifiers go through the dialect's quoting — so nothing
// the window generates can become SQL.
//
// Rows are sent in batches because one statement per batch costs one round trip
// instead of one per row; the batch size is the caller's business, and the
// service caps it.
type RowInsert struct {
	SessionID string   `json:"sessionId"`
	Database  string   `json:"database,omitempty"`
	Schema    string   `json:"schema,omitempty"`
	Object    string   `json:"object"`
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
}

// RowInsertResult reports what one batch really did.
//
// A statement the engine refused comes back here rather than as an error, so a
// batch that stopped halfway can still say how many rows did land and which row
// stopped it — the same honesty rule DesignResult follows for a half-applied
// script. A request that could not be attempted at all (no session, nothing to
// write) is still an error.
type RowInsertResult struct {
	// Inserted counts the rows this batch really wrote.
	Inserted int64 `json:"inserted"`
	// Failed is the 1-based index of the row that stopped the batch, or 0 when
	// every row went in.
	Failed int `json:"failed,omitempty"`
	// Error is the engine's own message for that row.
	Error string `json:"error,omitempty"`
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

// --- saved queries ---------------------------------------------------------

// SavedQuery is a named SQL snippet kept in the user's favourites.
//
// Favourites are deliberately engine-agnostic: the same snippet can be loaded
// into any query window, so Database and Driver are only hints recorded at save
// time (the UI uses them to show where the snippet came from, never to block a
// load).
type SavedQuery struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	SQL  string `json:"sql"`

	Database string     `json:"database,omitempty"`
	Driver   DriverType `json:"driver,omitempty"`

	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// --- query files -----------------------------------------------------------

// QueryFile is one named script saved next to the connection it belongs to.
//
// Unlike a favourite, which can be loaded anywhere, a query file is bound to a
// connection and a database: it is the file the query window edits, and its
// folder is derived from those two names. SQL is only filled in when the file is
// read; the tree lists names and sizes without pulling every script into memory.
type QueryFile struct {
	ConnectionID string `json:"connectionId"`
	Database     string `json:"database"`
	Name         string `json:"name"`
	// Path is where the file ended up. Shown in the UI (and handed to the file
	// manager) so a user can find the script without trusting our layout.
	Path string `json:"path"`
	// SQL is the script itself, only set by ReadQueryFile.
	SQL string `json:"sql,omitempty"`

	Size      int64 `json:"size"`
	UpdatedAt int64 `json:"updatedAt"`
}

// QueryFileSave is a query window asking for its script to be written.
//
// The name is part of the request because the file it goes to is named after it;
// renaming is a separate call (QueryFileRename), so a save can never move a
// script somewhere a user did not point at.
type QueryFileSave struct {
	ConnectionID string `json:"connectionId"`
	Database     string `json:"database"`
	Name         string `json:"name"`
	SQL          string `json:"sql"`
}

// QueryFileRename moves a script to another name without touching its contents.
// "Rename" here is a move of the file, not a read-and-write: the script on disk
// is the truth, and a rename from the tree must not need to know what is in it.
type QueryFileRename struct {
	ConnectionID string `json:"connectionId"`
	Database     string `json:"database"`
	From         string `json:"from"`
	To           string `json:"to"`
}

// AppInfo is static metadata rendered on the welcome screen.
type AppInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	ConfigPath string `json:"configPath"`
	Platform   string `json:"platform"`
}

// --- data directory --------------------------------------------------------

// DataFileInfo is one entry in the data directory, as the settings page lists it.
type DataFileInfo struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	// Dir marks a folder this build owns rather than a file it wrote. The query
	// files are a tree (one folder per connection and database), so the settings
	// page has to be able to say "3 queries" instead of showing a size.
	Dir bool `json:"dir,omitempty"`
	// Count is how many files the folder holds. Zero for a plain file.
	Count int `json:"count,omitempty"`
}

// DataDirInfo describes where the application keeps its data, so the settings
// page can show the path, what is in it, and whether it is the default location
// or one the user picked.
type DataDirInfo struct {
	Path string `json:"path"`
	// DefaultPath is where the app looks when no directory was chosen. Shown for
	// contrast, and the target of "use the default again".
	DefaultPath string `json:"defaultPath"`
	IsDefault   bool   `json:"isDefault"`
	// Files are the recognised data files that exist right now. An empty list is
	// honest — a fresh install has not written anything yet.
	Files      []DataFileInfo `json:"files"`
	TotalBytes int64          `json:"totalBytes"`
}

// DataDirMoveResult is what happened to the files during a move. It is
// deliberately not a bare bool: a partial outcome has to be reportable, because
// the files that were *not* moved are the ones a user needs to know about.
type DataDirMoveResult struct {
	// Info is the directory now in use.
	Info DataDirInfo `json:"info"`
	// Moved are the file names that were copied to the new directory and
	// verified there.
	Moved []string `json:"moved"`
	// LeftBehind are entries of the old directory that are not files this build
	// writes, so the move did not touch them.
	LeftBehind []string `json:"leftBehind"`
	// Remaining are files that were copied successfully but could not be deleted
	// from the old directory (a lock, a read-only folder). The new directory is
	// the one in use either way; this is only for an honest report.
	Remaining []string `json:"remaining"`
}
