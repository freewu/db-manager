// Package drivers defines the pluggable database driver contract.
//
// Design goals
//
//  1. Phase 1 ships MySQL, PostgreSQL and SQLite. Phase 2 adds MongoDB,
//     Oracle and SQL Server without touching the service or UI layers.
//  2. The contract is intentionally *not* SQL-specific: MongoDB implements the
//     same Conn interface by mapping documents onto the ObjectInfo/ColumnInfo
//     model (database -> collection -> field) and by translating a small query
//     language into filters in its own Execute implementation.
//  3. Metadata access is split into small calls so the UI can lazily expand a
//     tree without fetching the whole catalog.
package drivers

import (
	"context"
	"sort"
	"sync"

	"dbmanager/internal/models"
)

// Driver is the entry point of a database integration.
type Driver interface {
	// Info describes the driver for the UI.
	Info() models.DriverInfo
	// Normalize validates and fills in defaults (port, host, ...). It must be
	// safe to call on a partially filled config.
	Normalize(cfg *models.ConnectionConfig) error
	// Open establishes a connection.
	Open(ctx context.Context, cfg models.ConnectionConfig) (Conn, error)
}

// Conn is a live connection to one server.
//
// Implementations must be safe for concurrent use: the UI fires several
// metadata queries in parallel while a query is still running.
type Conn interface {
	Ping(ctx context.Context) error
	Close() error

	// Version returns the human readable server version.
	Version(ctx context.Context) (string, error)
	// CurrentDatabase returns the database the session is bound to.
	CurrentDatabase(ctx context.Context) (string, error)

	// Databases lists catalog names. Drivers without the concept return a
	// single synthetic entry.
	Databases(ctx context.Context) ([]string, error)
	// Schemas lists schemas inside a database (may be empty).
	Schemas(ctx context.Context, database string) ([]string, error)
	// Objects lists tables/views/collections in a namespace.
	Objects(ctx context.Context, database, schema string) ([]models.ObjectInfo, error)
	// Structure describes one object in full.
	Structure(ctx context.Context, database, schema, object string) (*models.TableStructure, error)
	// Indexes lists every index in a namespace. It powers the explorer's
	// "Indexes" folder without one catalog round trip per table.
	Indexes(ctx context.Context, database, schema string) ([]models.IndexEntry, error)

	// Fetch returns a page of rows from a single object.
	Fetch(ctx context.Context, req FetchRequest) (*models.FetchResult, error)
	// Execute runs an arbitrary script.
	Execute(ctx context.Context, req ExecRequest) (*models.QueryResult, error)

	// UpdateCell changes one column of one row identified by its primary key.
	// The statement is always parameterised; values are never interpolated.
	UpdateCell(ctx context.Context, req models.CellUpdate) (int64, error)
	// UpdateRow changes several columns of one row identified by its primary
	// key, in one statement.
	UpdateRow(ctx context.Context, req models.RowUpdate) (int64, error)
	// DeleteRow removes one row identified by its primary key.
	DeleteRow(ctx context.Context, req models.RowDelete) (int64, error)

	// PlanRowUpdate and PlanRowDelete render the statement those two would run,
	// without running it: what the row detail layer shows before it is applied.
	// They are pure, so the previewed text is the statement the run uses — the
	// service renders it once and hands the same text to the engine and to the
	// change log.
	PlanRowUpdate(req models.RowUpdate) (string, error)
	PlanRowDelete(req models.RowDelete) (string, error)

	// Dialect exposes the SQL flavour for identifier quoting and pagination.
	Dialect() Dialect
}

// Grapher is an optional Conn capability: describe a whole namespace (objects,
// their columns and the foreign keys between them) in one call.
//
// The ER diagram window needs exactly this. Drivers that cannot describe
// relationships simply do not implement Grapher, and the service falls back to
// reading each object's Structure — same result, more round trips. Adding the
// method to Conn instead would have forced every future driver to fake it.
type Grapher interface {
	Graph(ctx context.Context, database, schema string) (*models.SchemaGraph, error)
}

// Overviewer is an optional Conn capability: report how the server is doing
// right now (uptime, connections, cache hit rates, running queries).
//
// Every engine answers a different question, so the return value is the union
// models.ServerOverview: the driver fills in its own field and nothing else.
// Drivers without runtime reporting leave Spec.Overview nil; Overview then
// returns apperr.CodeUnsupported and the service turns that into a page that
// says so.
type Overviewer interface {
	Overview(ctx context.Context) (*models.ServerOverview, error)
}

// Analyzer is an optional Conn capability: describe what a script would do
// without running it.
//
// The DDL editor shows that dry run before anything is executed. For SQL the
// default is a keyword heuristic in sqlutil; a driver whose statements are not
// SQL implements this instead, so the editor keeps working on a document store
// (where no keyword table could recognise `db.orders.drop()`).
type Analyzer interface {
	AnalyzeScript(script string, readOnly bool) models.ScriptAnalysis
}

// Explainer is an optional Conn capability: how the engine would run a
// statement, without running it.
//
// The query window's "Plan" view calls this. Keeping it apart from Execute is
// what lets the promise in its name be kept literally — no driver is ever asked
// to execute anything in order to produce a plan, so PostgreSQL's EXPLAIN
// ANALYZE (which *does* run the statement and then throws the rows away) is
// never emitted. An engine whose statements cannot be described in one generic
// SQL statement does not implement this, and the window says so instead of
// showing an empty grid.
type Explainer interface {
	Explain(ctx context.Context, req ExplainRequest) (*models.ExplainResult, error)
}

// Inserter is an optional Conn capability: append a batch of generated rows to
// a table.
//
// The data generation window produces values in the frontend (placeholders,
// Chinese name tables, Luhn checksums are presentation, not driver work); what
// needs an engine is writing them down. The values may only travel as bind
// parameters — nothing the window generates is SQL — and a batch the server
// refuses must come back as a partial count rather than as an error, so the
// window can say which row failed instead of blaming the whole run. Whether a
// refusal ends the batch or is counted over is the caller's choice, and it is
// carried by the request (`RowInsert.SkipErrors`).
//
// A document store does not implement this: a collection has no column list to
// fill, so there is nothing for the window to offer.
type Inserter interface {
	InsertRows(ctx context.Context, req models.RowInsert) (models.RowInsertResult, error)
	// PlanRowInsert is the statement InsertRows would run, for the window to show
	// before a run starts. The values are made up while the run goes on, so what
	// it renders is the statement's shape — the table, the columns, and that the
	// values arrive as parameters — through the same renderer the run uses.
	PlanRowInsert(req models.RowInsert) (string, error)
}

// RowStreamer is an optional Conn capability: read a whole object's rows, one
// at a time.
//
// The export window reads a table exactly once and writes what it reads to a
// file, so the rows must not be collected first: a table with ten million rows
// is an ordinary thing to export, and reading it page by page would ask the
// server to skip everything it has already sent (quadratic work for a linear
// question) while this side held a copy of the whole table. Streaming asks the
// engine once and hands each row to the caller, who writes it down and forgets
// it.
//
// A driver whose rows cannot be read as one cursor simply does not implement
// this, and the export window says so instead of writing a different set of
// rows than the one that was asked for.
type RowStreamer interface {
	// StreamRows calls each once per row, in the order the engine returns them.
	// An error from each stops the read and comes back unchanged, so a caller's
	// own decision (the user pressed Stop) is not dressed up as a failure of the
	// query; an error from the engine comes back wrapped in apperr like every
	// other read.
	//
	// The caller's context is the only deadline: a scan that takes a minute is
	// not a failure, it is a large table, and the caller is the one who knows
	// whether to wait (it has a Stop button) — so no timeout is invented here.
	StreamRows(ctx context.Context, req StreamRequest, each func(row []any) error) error
}

// StreamRequest is the driver-level request for one object's rows.
type StreamRequest struct {
	Database string
	Schema   string
	Object   string
	// Columns selects the fields to read, in this order. Empty means every
	// column, in the order the catalog declares them — which is what a row's
	// values are lined up with, so a caller that names its columns elsewhere
	// (a CSV header, an INSERT's column list) passes the same list here.
	Columns []string
}

// ExplainRequest is the driver-level explain request (no session ids).
type ExplainRequest struct {
	Database  string
	SQL       string
	TimeoutMS int
}

// DatabaseCreator is an optional Conn capability: how this engine creates a
// database, and what its server accepts for the new one's character set.
//
// MySQL family and PostgreSQL answer it, MongoDB answers it with `use`, and
// SQLite does not implement it at all — a file is not a server that holds
// databases, so the UI has no "New database" item for it. Engines that have
// databases but nothing to choose about them (Doris) implement it and return
// options with a hint and no lists.
type DatabaseCreator interface {
	// DatabaseOptions reports what the driver knows about CREATE DATABASE on
	// this server. Asking the server is the point: character sets and
	// collations differ per engine and per version, so none of them may be
	// hardcoded in the UI.
	DatabaseOptions(ctx context.Context) (*models.DatabaseOptions, error)
	// CreateDatabase renders the statement that creates the database. It never
	// runs it: the window shows the statement first and executes that exact
	// string through Execute.
	CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error)
}

// FetchRequest is the driver-level page request (no session ids).
type FetchRequest struct {
	Database   string
	Schema     string
	Object     string
	Limit      int
	Offset     int
	OrderBy    []models.SortSpec
	Filters    []models.FilterSpec
	CountTotal bool
	TimeoutMS  int
}

// ExecRequest is the driver-level script request.
type ExecRequest struct {
	Database  string
	SQL       string
	MaxRows   int
	TimeoutMS int
	ReadOnly  bool
}

// Dialect encapsulates the syntactic differences between SQL engines.
type Dialect interface {
	// Name is the driver type name.
	Name() models.DriverType
	// Quote wraps an identifier in the engine's quote characters.
	Quote(ident string) string
	// Qualify builds a fully qualified object reference.
	Qualify(database, schema, object string) string
	// Placeholder renders the nth (1-based) bind parameter.
	Placeholder(n int) string
	// LimitOffset renders the pagination suffix. Engines that need a
	// different formulation (Oracle's FETCH FIRST, SQL Server's OFFSET ...
	// ROWS FETCH NEXT) implement it here.
	LimitOffset(limit, offset int) string
	// SupportsLimitOffset reports whether server-side paging is available.
	SupportsLimitOffset() bool
}

// --- registry --------------------------------------------------------------

var (
	regMu    sync.RWMutex
	registry = map[models.DriverType]Driver{}
)

// Register makes a driver available to the application. It is intended to be
// called from init() functions and panics on duplicate registration, which can
// only be a programming error.
func Register(d Driver) {
	info := d.Info()
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := registry[info.Type]; exists {
		panic("drivers: duplicate registration for " + string(info.Type))
	}
	registry[info.Type] = d
}

// Get looks a driver up by type.
func Get(t models.DriverType) (Driver, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	d, ok := registry[t]
	return d, ok
}

// MustGet is Get with a panic on miss; for internal call sites that already
// validated the type.
func MustGet(t models.DriverType) Driver {
	d, ok := Get(t)
	if !ok {
		panic("drivers: unknown driver " + string(t))
	}
	return d
}

// All returns every registered driver sorted for stable UI ordering.
func All() []Driver {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Driver, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Info(), out[j].Info()
		if a.SortOrder != b.SortOrder {
			return a.SortOrder < b.SortOrder
		}
		return a.DisplayName < b.DisplayName
	})
	return out
}

// Infos returns the UI metadata of every registered driver.
func Infos() []models.DriverInfo {
	all := All()
	out := make([]models.DriverInfo, 0, len(all))
	for _, d := range all {
		out = append(out, d.Info())
	}
	return out
}
