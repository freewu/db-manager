// Package sqlbase implements the parts of the drivers.Conn contract that are
// identical for every database/sql backed engine.
//
// A concrete driver only supplies a Spec (DSN builder, dialect, introspector)
// and gets pooling, paging, script execution, value coercion and DDL rendering
// for free. Adding PostgreSQL, MySQL, SQLite, SQL Server or Oracle is therefore
// a matter of writing the catalog queries, not of re-implementing the engine.
package sqlbase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// Defaults for paging and pooling.
const (
	DefaultPageSize  = 200
	MaxPageSize      = 5000
	DefaultMaxRows   = 1000
	MaxMaxRows       = 50000
	DefaultTimeoutMS = 60_000

	maxOpenConns = 8
	maxIdleConns = 4
	connMaxLife  = 30 * time.Minute
)

// Querier is the subset of *sql.DB / *sql.Tx the introspectors use. Keeping it
// narrow makes the catalog queries trivially unit-testable with a fake.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Introspector implements the engine specific catalog queries.
type Introspector interface {
	Version(ctx context.Context, q Querier) (string, error)
	// CurrentDatabase returns the catalog the connection is bound to.
	CurrentDatabase(ctx context.Context, q Querier) (string, error)
	// Databases lists catalogs. Engines without the concept return the single
	// synthetic name.
	Databases(ctx context.Context, q Querier) ([]string, error)
	// Schemas lists schemas inside database (may legitimately be empty).
	Schemas(ctx context.Context, q Querier, database string) ([]string, error)
	Object(ctx context.Context, q Querier, database, schema, object string) (*models.ObjectInfo, error)
	Objects(ctx context.Context, q Querier, database, schema string) ([]models.ObjectInfo, error)
	Columns(ctx context.Context, q Querier, database, schema, object string) ([]models.ColumnInfo, error)
	Indexes(ctx context.Context, q Querier, database, schema, object string) ([]models.IndexInfo, error)
	// NamespaceIndexes lists every index of every object in a namespace.
	NamespaceIndexes(ctx context.Context, q Querier, database, schema string) ([]models.IndexEntry, error)
	ForeignKeys(ctx context.Context, q Querier, database, schema, object string) ([]models.ForeignKeyInfo, error)
}

// Spec is the declarative description of a SQL driver.
type Spec struct {
	Info         models.DriverInfo
	SQLDriver    string
	Dialect      drivers.Dialect
	Introspector Introspector

	// DSN builds the connection string for a specific database. database is
	// already resolved to a concrete catalog name.
	DSN func(cfg models.ConnectionConfig, database string) (string, error)

	// BootstrapDatabase returns the catalog to connect to when the user did
	// not pick one (PostgreSQL needs "postgres", MySQL can use "", ...).
	BootstrapDatabase func(cfg models.ConnectionConfig) string

	// NativeDDL optionally returns the engine's own CREATE statement.
	NativeDDL func(ctx context.Context, q Querier, database, schema, object string) (string, error)
}

// Conn is the shared drivers.Conn implementation.
type Conn struct {
	spec Spec
	cfg  models.ConnectionConfig

	mu     sync.Mutex
	pools  map[string]*sql.DB
	closed bool
}

// NewConn builds the shared connection wrapper.
func NewConn(spec Spec, cfg models.ConnectionConfig) *Conn {
	return &Conn{spec: spec, cfg: cfg, pools: map[string]*sql.DB{}}
}

var _ drivers.Conn = (*Conn)(nil)

// Spec exposes the driver spec (used by engine specific extras).
func (c *Conn) Spec() Spec { return c.spec }

// Config returns the normalised configuration.
func (c *Conn) Config() models.ConnectionConfig { return c.cfg }

// Dialect implements drivers.Conn.
func (c *Conn) Dialect() drivers.Dialect { return c.spec.Dialect }

// DB returns a pooled handle for the requested catalog, opening it on demand.
//
// PostgreSQL cannot query across databases, so a "switch database" in the UI
// becomes a brand new pool here. MySQL and SQLite reuse a single pool.
func (c *Conn) DB(ctx context.Context, database string) (*sql.DB, error) {
	name := c.resolveDatabase(database)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, apperr.New(apperr.CodeInternal, "connection is closed")
	}
	if db, ok := c.pools[name]; ok {
		return db, nil
	}

	dsn, err := c.spec.DSN(c.cfg, name)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(c.spec.SQLDriver, dsn)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeConnectionFail, err, "open %s connection", c.spec.Info.DisplayName)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLife)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeConnectionFail, err, "connect to %s", c.spec.Info.DisplayName)
	}
	c.pools[name] = db
	return db, nil
}

func (c *Conn) resolveDatabase(database string) string {
	if database != "" {
		return database
	}
	if c.cfg.Database != "" {
		return c.cfg.Database
	}
	if c.spec.BootstrapDatabase != nil {
		return c.spec.BootstrapDatabase(c.cfg)
	}
	return ""
}

// Ping implements drivers.Conn.
func (c *Conn) Ping(ctx context.Context) error {
	db, err := c.DB(ctx, "")
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		return apperr.Wrap(apperr.CodeConnectionFail, err, "ping")
	}
	return nil
}

// Close implements drivers.Conn and releases every pool.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var firstErr error
	for name, db := range c.pools {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(c.pools, name)
	}
	return firstErr
}

// Version implements drivers.Conn.
func (c *Conn) Version(ctx context.Context) (string, error) {
	db, err := c.DB(ctx, "")
	if err != nil {
		return "", err
	}
	return c.spec.Introspector.Version(ctx, db)
}

// CurrentDatabase implements drivers.Conn.
func (c *Conn) CurrentDatabase(ctx context.Context) (string, error) {
	db, err := c.DB(ctx, "")
	if err != nil {
		return "", err
	}
	return c.spec.Introspector.CurrentDatabase(ctx, db)
}

// Databases implements drivers.Conn.
func (c *Conn) Databases(ctx context.Context) ([]string, error) {
	db, err := c.DB(ctx, "")
	if err != nil {
		return nil, err
	}
	return c.spec.Introspector.Databases(ctx, db)
}

// Schemas implements drivers.Conn.
func (c *Conn) Schemas(ctx context.Context, database string) ([]string, error) {
	db, err := c.DB(ctx, database)
	if err != nil {
		return nil, err
	}
	return c.spec.Introspector.Schemas(ctx, db, c.resolveDatabase(database))
}

// Objects implements drivers.Conn.
func (c *Conn) Objects(ctx context.Context, database, schema string) ([]models.ObjectInfo, error) {
	db, err := c.DB(ctx, database)
	if err != nil {
		return nil, err
	}
	return c.spec.Introspector.Objects(ctx, db, c.resolveDatabase(database), schema)
}

// Structure implements drivers.Conn.
func (c *Conn) Structure(ctx context.Context, database, schema, object string) (*models.TableStructure, error) {
	db, err := c.DB(ctx, database)
	if err != nil {
		return nil, err
	}
	resolved := c.resolveDatabase(database)

	obj, err := c.spec.Introspector.Object(ctx, db, resolved, schema, object)
	if err != nil {
		return nil, err
	}
	cols, err := c.spec.Introspector.Columns(ctx, db, resolved, schema, object)
	if err != nil {
		return nil, err
	}
	idx, err := c.spec.Introspector.Indexes(ctx, db, resolved, schema, object)
	if err != nil {
		return nil, err
	}
	fks, err := c.spec.Introspector.ForeignKeys(ctx, db, resolved, schema, object)
	if err != nil {
		return nil, err
	}

	structure := &models.TableStructure{
		Object:      *obj,
		Columns:     cols,
		Indexes:     idx,
		ForeignKeys: fks,
	}

	if c.spec.NativeDDL != nil {
		if ddl, err := c.spec.NativeDDL(ctx, db, resolved, schema, object); err == nil {
			structure.DDL = ddl
		}
	}
	if structure.DDL == "" {
		structure.DDL = RenderCreateTable(c.spec.Dialect, *obj, cols, idx, fks)
	}
	return structure, nil
}

// Indexes implements drivers.Conn.
func (c *Conn) Indexes(ctx context.Context, database, schema string) ([]models.IndexEntry, error) {
	db, err := c.DB(ctx, database)
	if err != nil {
		return nil, err
	}
	return c.spec.Introspector.NamespaceIndexes(ctx, db, c.resolveDatabase(database), schema)
}

// Fetch implements drivers.Conn.
func (c *Conn) Fetch(ctx context.Context, req drivers.FetchRequest) (*models.FetchResult, error) {
	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return nil, err
	}
	resolved := c.resolveDatabase(req.Database)
	d := c.spec.Dialect

	limit := req.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	ctx, cancel := withTimeout(ctx, req.TimeoutMS)
	defer cancel()

	where, args, err := sqlutil.BuildWhere(d, req.Filters, 1)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "invalid filter")
	}

	// Stable paging: without an explicit sort a LIMIT/OFFSET query may return
	// rows in arbitrary order (and repeat or skip rows across pages), so fall
	// back to the primary key when the caller did not ask for anything.
	pk := c.primaryKey(ctx, db, resolved, req.Schema, req.Object)

	orderBy := ""
	if len(req.OrderBy) > 0 {
		orderBy = sqlutil.BuildOrderBy(d, req.OrderBy)
	}
	if orderBy == "" && len(pk) > 0 {
		specs := make([]models.SortSpec, 0, len(pk))
		for _, col := range pk {
			specs = append(specs, models.SortSpec{Column: col})
		}
		orderBy = sqlutil.BuildOrderBy(d, specs)
	}

	target := d.Qualify(resolved, req.Schema, req.Object)
	query := "SELECT * FROM " + target + where + orderBy + d.LimitOffset(limit, req.Offset)

	started := time.Now()
	result, err := scanQuery(ctx, db, query, args, limit)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "fetch rows")
	}
	result.DurationMS = time.Since(started).Milliseconds()
	result.SQL = query

	// Flag the key columns so the data grid can offer inline editing and build
	// the identifying WHERE clause without another catalog round trip.
	for i := range result.Columns {
		for _, name := range pk {
			if strings.EqualFold(result.Columns[i].Name, name) {
				result.Columns[i].IsPrimaryKey = true
			}
		}
	}

	out := &models.FetchResult{QueryResult: *result}
	if req.CountTotal {
		if total, err := c.count(ctx, db, target, where, args); err == nil {
			out.Total = total
			out.HasTotal = true
		}
		// A failed count is not fatal: the page is still useful.
	}
	return out, nil
}

// Execute implements drivers.Conn.
func (c *Conn) Execute(ctx context.Context, req drivers.ExecRequest) (*models.QueryResult, error) {
	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return nil, err
	}

	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	if maxRows > MaxMaxRows {
		maxRows = MaxMaxRows
	}

	statements := sqlutil.SplitStatements(req.SQL)
	if len(statements) == 0 {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no statement to execute")
	}

	ctx, cancel := withTimeout(ctx, req.TimeoutMS)
	defer cancel()

	var (
		lastQuery *models.QueryResult
		lastExec  *models.QueryResult
		messages  []string
	)

	for i, stmt := range statements {
		isQuery := sqlutil.IsQueryStatement(stmt)
		if req.ReadOnly && !isQuery {
			return nil, apperr.New(apperr.CodeReadOnly, "connection is read-only, refusing to run statement %d", i+1)
		}

		started := time.Now()
		if isQuery {
			res, err := scanQuery(ctx, db, stmt, nil, maxRows)
			if err != nil {
				return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "statement %d failed", i+1)
			}
			res.DurationMS = time.Since(started).Milliseconds()
			res.SQL = stmt
			messages = append(messages, fmt.Sprintf("#%d: %d row(s) in %dms", i+1, res.RowCount, res.DurationMS))
			lastQuery = res
			continue
		}

		execRes, err := db.ExecContext(ctx, stmt)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "statement %d failed", i+1)
		}
		res := &models.QueryResult{
			SQL:          stmt,
			HasResultSet: false,
			Columns:      []models.ColumnMeta{},
			Rows:         [][]any{},
		}
		if n, err := execRes.RowsAffected(); err == nil {
			res.AffectedRows = n
		}
		if id, err := execRes.LastInsertId(); err == nil {
			res.LastInsertID = id
		}
		res.DurationMS = time.Since(started).Milliseconds()
		messages = append(messages, fmt.Sprintf("#%d: %d row(s) affected in %dms", i+1, res.AffectedRows, res.DurationMS))
		lastExec = res
	}

	// Prefer the last statement that produced a result set: that is what a
	// user running a script wants to look at.
	final := lastQuery
	if final == nil {
		final = lastExec
	}
	if final == nil {
		final = &models.QueryResult{Columns: []models.ColumnMeta{}, Rows: [][]any{}}
	}
	final.StatementCount = len(statements)
	final.StatementIndex = len(statements) - 1
	final.Messages = messages
	return final, nil
}

// primaryKey returns the PK column names of an object, or nil.
func (c *Conn) primaryKey(ctx context.Context, q Querier, database, schema, object string) []string {
	cols, err := c.spec.Introspector.Columns(ctx, q, database, schema, object)
	if err != nil {
		return nil
	}
	out := make([]string, 0, 2)
	for _, col := range cols {
		if col.PrimaryKey {
			out = append(out, col.Name)
		}
	}
	return out
}

func (c *Conn) count(ctx context.Context, q Querier, target, where string, args []any) (int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM " + target + where
	if err := q.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// withTimeout applies the per-request timeout, defaulting to a minute so a
// forgotten "SELECT *" on a huge table cannot wedge the UI forever.
func withTimeout(ctx context.Context, ms int) (context.Context, context.CancelFunc) {
	if ms <= 0 {
		ms = DefaultTimeoutMS
	}
	return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
}
