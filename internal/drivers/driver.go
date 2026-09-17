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
	// DeleteRow removes one row identified by its primary key.
	DeleteRow(ctx context.Context, req models.RowDelete) (int64, error)

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
