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

// --- table copy ------------------------------------------------------------

// CopyTableRequest asks for one table to be duplicated into a new one.
//
// WithData is the whole difference between the two entries the explorer's copy
// menu offers: "structure only" writes the fields and indexes of an empty table,
// "structure and data" appends the rows. Target is the new table's name, which
// is the only thing the window has to ask for — every other fact about the copy
// is read from the table being copied, at the moment the script is rendered.
//
// The result of planning one is a DesignPlan and the result of running one is a
// DesignResult: a copy is a CREATE TABLE plus (optionally) an INSERT, so it is
// previewed, run statement by statement and reported on exactly like a draft of
// the table designer.
type CopyTableRequest struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
	// Object is the table being copied, Target the one being created.
	Object   string `json:"object"`
	Target   string `json:"target"`
	WithData bool   `json:"withData"`
}

// --- table operations ------------------------------------------------------

// TableOpRequest names the one table the explorer's menu is about to empty or
// remove.
//
// Both of those are a single statement about a table that already exists, so
// neither needs a payload beyond the name: what the statement says about the
// fields, the counter or the indexes is read from the live catalog at the moment
// it is rendered, exactly like a copy's CREATE is. A plan for one is a
// DesignPlan and a run is a DesignResult, like every other statement this
// application writes — which is also where both are marked as destructive
// before anything has run.
type TableOpRequest struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
	// Object is the table being emptied or removed.
	Object string `json:"object"`
}

// --- database comparison ---------------------------------------------------

// CompareSide names one namespace to compare: the session it is read through,
// and the database (with the schema inside it, where the engine has them).
//
// A side is read every time the comparison runs, never carried across the
// bridge as data: what a table looks like is the one fact that must not be a
// snapshot taken when the window was opened.
type CompareSide struct {
	SessionID string `json:"sessionId"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
}

// CompareRequest asks for two namespaces to be compared.
//
// Left is the reference and Right is the one the generated script would bring
// in line with it; which is which only matters to the sync, never to the
// comparison itself. Both sides travel by session rather than by connection,
// because a session already says which profile, which server and which login
// the namespace is reached through.
type CompareRequest struct {
	Left  CompareSide `json:"left"`
	Right CompareSide `json:"right"`
}

// SyncDatabaseRequest asks for the script that turns Right into Left.
//
// DropExtra is the one thing the user has to decide: a table that exists only on
// the right is left alone unless it is asked for, because "make B look like A"
// and "delete everything B has that A does not" are different promises.
type SyncDatabaseRequest struct {
	Left      CompareSide `json:"left"`
	Right     CompareSide `json:"right"`
	DropExtra bool        `json:"dropExtra"`
}

// DiffStatus is how one compared thing stands across the two sides.
type DiffStatus string

const (
	// DiffSame is a table that is on both sides with the same shape, or a
	// field / index that matches.
	DiffSame DiffStatus = "same"
	// DiffAdded is something the left has and the right does not: a table to
	// create, a field or index to add.
	DiffAdded DiffStatus = "added"
	// DiffRemoved is something the right has and the left does not: a table,
	// field or index that the left side does not know about.
	DiffRemoved DiffStatus = "removed"
	// DiffChanged is something both sides have, spelled differently.
	DiffChanged DiffStatus = "changed"
)

// DiffField is one attribute that differs, already spelled for reading: a type,
// a nullability, a default. Both sides are text because the two engines may
// phrase the same fact differently and comparing the texts is what found the
// difference in the first place.
type DiffField struct {
	Field string `json:"field"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// DiffItem is one compared thing inside a table: a field or an index.
//
// Fields is empty for an item that is only on one side (nothing to compare it
// with) and for one that matches; Summary carries the same fact as one short
// sentence, which is what a list row shows without expanding.
type DiffItem struct {
	Name    string      `json:"name"`
	Kind    string      `json:"kind"`
	Status  DiffStatus  `json:"status"`
	Fields  []DiffField `json:"fields,omitempty"`
	Summary string      `json:"summary"`
}

// TableDiff is one table's comparison.
//
// Kind is the object kind (a table, for now): the comparison is about tables
// because those are the objects a script can be generated for.
type TableDiff struct {
	Name    string     `json:"name"`
	Kind    ObjectKind `json:"kind"`
	Status  DiffStatus `json:"status"`
	Columns []DiffItem `json:"columns"`
	Indexes []DiffItem `json:"indexes"`
	Summary string     `json:"summary"`
}

// SchemaCompare is the comparison of two namespaces, table by table.
//
// Everything the window shows is here, and only here: the two sides it was
// asked about, the names to call them, and one entry per table. Counts are left
// out on purpose — the window can count what it was given, and a number that
// travels separately from the list it counts is a number that can disagree with
// it.
type SchemaCompare struct {
	Left       CompareSide `json:"left"`
	Right      CompareSide `json:"right"`
	LeftLabel  string      `json:"leftLabel"`
	RightLabel string      `json:"rightLabel"`
	Driver     DriverType  `json:"driver"`
	Tables     []TableDiff `json:"tables"`
	// Warnings say what the comparison itself could not look at, so a table
	// that matches is not read as a promise that the two are identical.
	Warnings []string `json:"warnings"`
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
	// SQL is the statement as it was written, which is what the change log keeps
	// — a preview is for showing next to an editor, not for recording what was
	// run.
	//
	// It never crosses the bridge: the window that asked for the analysis already
	// holds the whole script, so sending the statements back would double the
	// message for nothing.
	SQL string `json:"-"`
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
	// Schema and Object are the place the script was written against, when it has
	// one: the DDL editor of a table runs that table's script. They are not sent
	// to the engine — the script names what it touches itself — but they are what
	// the change log records as the object of the statements it ran, since the
	// window knows and the text would have to be parsed to find out.
	Schema    string `json:"schema,omitempty"`
	Object    string `json:"object,omitempty"`
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

// RowUpdate edits several columns of a single row at once.
//
// It is one statement with one WHERE clause, which is what makes the row detail
// layer safe to use on a table whose primary key is a compound: the row is
// identified once, and the values are bound — the same guarantee CellUpdate
// gives, for the case where more than one field changed.
//
// Values uses KeyValue because a change is exactly a column and a value, and the
// key is exactly a list of them.
type RowUpdate struct {
	SessionID string     `json:"sessionId"`
	Database  string     `json:"database,omitempty"`
	Schema    string     `json:"schema,omitempty"`
	Object    string     `json:"object"`
	Key       []KeyValue `json:"key"`
	Values    []KeyValue `json:"values"`
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
	// SkipErrors keeps the batch going past a row the engine refuses, counting
	// it instead of stopping at it. Off — the default — a refusal ends the
	// batch, because a mock that keeps producing rows the engine will not take
	// is something to be seen and fixed rather than run over.
	SkipErrors bool `json:"skipErrors,omitempty"`
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
	// Skipped counts the rows a skip-errors batch passed over. It stays 0 on a
	// batch that was told to stop, which never gets past the row it stopped at.
	Skipped int64 `json:"skipped,omitempty"`
	// Failed is the 1-based index of the row that stopped the batch, or 0 when
	// nothing stopped it: every row went in, or the batch was told to keep going
	// past the rows the engine refused.
	Failed int `json:"failed,omitempty"`
	// Error is the engine's own message for the row that stopped the batch, or —
	// when the batch was told to keep going — for the first row it passed over.
	// It is empty when nothing was refused.
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

// --- custom mock placeholders ----------------------------------------------

// MockPlaceholder is one placeholder the user defined for the data generation
// window, kept as a file of its own under the `.mock` folder of the data
// directory.
//
// Template is a mock template in the same syntax the window's mock column takes,
// written out of the built-in placeholders (@string, @natural, …) and the other
// custom ones; Name is what a template then says as `@name`. The name is also
// the file name, so it is the identity of the entry rather than a label stuck on
// it — which is why the settings page only lets it be typed while creating one.
//
// Nothing here is rendered by the backend: the engine that reads a template lives
// in the window, so Go stores the text and the window is the one that judges it.
type MockPlaceholder struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description,omitempty"`
	UpdatedAt   int64  `json:"updatedAt"`
	// Broken explains why the file could not be read. The entry is still listed,
	// so a placeholder that was hand-edited into something unparseable can be
	// seen — and deleted — from the settings page instead of quietly vanishing.
	Broken string `json:"broken,omitempty"`
}

// --- change log ------------------------------------------------------------

// Where a logged statement came from: which window of the application asked for
// it to run. The value travels with the entry, so the log keeps saying where a
// statement was written long after that window is closed.
const (
	// ChangeSourceScript is a window that runs text the user wrote: the query
	// window, the DDL editor, a database being created.
	ChangeSourceScript = "script"
	// ChangeSourceDesign is the structure page saving a draft of an existing
	// table.
	ChangeSourceDesign = "design"
	// ChangeSourceCreate is the table designer creating one.
	ChangeSourceCreate = "create"
	// ChangeSourceGrid is the data grid editing or deleting a row: a change
	// asked for from the result table, not from a script or a design.
	ChangeSourceGrid = "grid"
	// ChangeSourceCopy is the explorer duplicating a table: one CREATE TABLE,
	// the indexes that go with it, and the rows when they were asked for.
	ChangeSourceCopy = "copy"
	// ChangeSourceExplorer is the explorer's own table menu emptying or removing
	// a table: a change asked for from the tree, without a window showing the
	// table being opened at all.
	ChangeSourceExplorer = "explorer"
	// ChangeSourceCompare is the comparison page applying a script it generated
	// from the difference between two databases: several tables in one run,
	// which is why the entry points at no single object.
	ChangeSourceCompare = "compare"
	// ChangeSourceDataGen is the data generation window appending rows it made
	// up: one entry per batch, because a batch is what the engine is handed.
	ChangeSourceDataGen = "datagen"
)

// ChangeLogEntry is one statement this application ran that changed schema or
// data.
//
// The log is a record of what the program did to the databases it was pointed
// at: every statement it executed other than a read, when it ran it, where, and
// what came of it. It is written by the layer that executes statements, not by
// the windows, so a statement cannot be run through the application without
// leaving a line behind — and the whole "Structure Sync" slot is answered by
// reading the file rather than by contacting an engine.
//
// Table is the object the statement was applied to, which is *context* rather
// than something parsed out of the text: the structure page knows the table it
// saved, the DDL editor knows the table whose script it runs, and a query window
// knows only the database. It is empty for a statement that creates a table — a
// table that does not exist yet is not an object an entry can point at, and its
// name is written in the statement itself.
//
// Error is the message the run ended with, and is empty when it finished. A
// script goes to the engine in one call, so a run that stops halfway leaves the
// same message on each of its statements: the entry says what the run said, and
// never more than the engine did.
//
// Rows is how much data the statement changed: the count the engine reported as
// affected, which is rows inserted, updated or deleted. It is zero for anything
// that cannot report one — DDL, and a script holding more than one write
// statement, where no count can be attributed to a single line.
type ChangeLogEntry struct {
	Version int   `json:"version"`
	At      int64 `json:"at"`

	Connection ChangeLogConnection `json:"connection"`
	Database   string              `json:"database,omitempty"`
	Schema     string              `json:"schema,omitempty"`
	Table      string              `json:"table,omitempty"`

	// Kind is the statement's own leading keyword, lowercased ("create",
	// "alter", "drop", "insert", …), so the list can be scanned by what was
	// done rather than by a category.
	Kind string `json:"kind"`
	// Source is one of the ChangeSource values above.
	Source    string `json:"source"`
	Statement string `json:"statement"`
	// Rows is how much data the statement changed, as the engine counted it.
	Rows  int64  `json:"rows,omitempty"`
	Error string `json:"error,omitempty"`
}

// ChangeLogConnection is what an entry remembers about the connection it ran on.
//
// It is copied at write time rather than looked up when the log is read: profiles
// get renamed, re-pointed and deleted, and a log that changes its mind about
// where a statement ran is not a log. Nothing secret is kept — the address and
// the user name are what a reader needs to tell two servers apart.
type ChangeLogConnection struct {
	ID      string     `json:"id,omitempty"`
	Name    string     `json:"name,omitempty"`
	Driver  DriverType `json:"driver,omitempty"`
	Address string     `json:"address,omitempty"`
	User    string     `json:"user,omitempty"`
}

// ChangeLog is one page of one log file, newest entry first, plus the file it
// came from and the files there are to choose between.
//
// Total is how many entries the file holds, which is what lets the window say
// "the newest 200 of 1,248" instead of pretending a page is the whole story.
// Files travels with the page because the two are read together: a window that
// knows which entries it has but not which files exist cannot offer the reader
// the older ones.
type ChangeLog struct {
	// File is the file these entries were read from: the live log, or an archive
	// a rotation moved aside.
	File    string           `json:"file"`
	Entries []ChangeLogEntry `json:"entries"`
	Total   int              `json:"total"`
	Files   []ChangeLogFile  `json:"files"`
}

// ChangeLogFile is one file the change log is spread over.
//
// The live file is appended to and rotated away from a whole file at a time, so
// the log is a shelf rather than a single document: reading it means picking a
// file, and this is what the window picks from.
type ChangeLogFile struct {
	// Name is the file name inside the log folder: `<yyyymmdd>.log` for the file
	// being written, `<yyyymmdd>-<n>.log` for one that was rotated out of the way.
	// A log written by an older build is named as it was written — the file, or
	// `changelog.jsonl` — and is listed from the data directory itself.
	Name string `json:"name"`
	// Archived marks a file that is complete and will not be written to again.
	Archived bool `json:"archived"`
	// At is when the file was last written, and zero for the one being written
	// right now.
	At int64 `json:"at,omitempty"`
	// Bytes is the file's size on disk.
	Bytes int64 `json:"bytes"`
	// Entries is how many statements it holds. Counting them means reading it,
	// which is why this is one call rather than a separate listing: the reader
	// has to read the file it opens anyway.
	Entries int `json:"entries"`
}

// ChangeLogSettings is how the change log is kept.
//
// The log is rotated rather than trimmed: at MaxEntries the file is moved aside
// whole and a new one is started, so nothing a user has ever been shown
// disappears on its own. The three bounds travel with the value so the settings
// page validates against the same numbers the backend does, instead of
// restating them and drifting.
type ChangeLogSettings struct {
	// MaxEntries is how many statements the live file holds before it is archived.
	MaxEntries int `json:"maxEntries"`
	// Default, Min and Max describe what MaxEntries may be. They are what the
	// store uses; the settings page only shows them.
	Default int `json:"default"`
	Min     int `json:"min"`
	Max     int `json:"max"`
}

// DataGenSettings is how much one data generation run may write.
//
// It is a ceiling on the Rows box of the data generation window rather than a
// batch size: the window still sends small batches, reports what landed as it
// goes, and stops the moment it is told to. The three bounds travel with the
// value so the settings page validates against the same numbers the backend
// does, instead of restating them and drifting.
type DataGenSettings struct {
	// MaxRows is the most rows one run may write.
	MaxRows int `json:"maxRows"`
	// Default, Min and Max describe what MaxRows may be. They are what the store
	// uses; the settings page only shows them.
	Default int `json:"default"`
	Min     int `json:"min"`
	Max     int `json:"max"`
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
