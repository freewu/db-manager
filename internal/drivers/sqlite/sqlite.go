// Package sqlite implements the SQLite driver.
//
// It uses modernc.org/sqlite, a pure Go translation of SQLite, so the shipped
// binary needs no CGO toolchain and cross-compiles trivially.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverSQLite,
		DisplayName:      "SQLite",
		Implemented:      true,
		Relational:       true,
		SupportsDatabase: false,
		SupportsSchema:   false,
		RequiresFile:     true,
		SupportsDesign:   true,
		// EXPLAIN QUERY PLAN answers with the plan it would follow, and does
		// not run the statement to find out. The bare `EXPLAIN` opcode listing
		// is a different (and much less readable) thing.
		SupportsExplain: true,
		// A SQLite file holds tables and views; its indexes hang off the tables,
		// so the explorer keeps them in the namespace-wide index folder.
		ObjectKinds:     []models.ObjectKind{models.KindTable, models.KindView},
		DefaultDatabase: "main",
		SortOrder:       30,
		Notes:           "SQLite is a single file; attach additional files to browse them side by side.",
	}
}

// Normalize implements drivers.Driver.
func (Driver) Normalize(cfg *models.ConnectionConfig) error {
	if strings.TrimSpace(cfg.FilePath) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "a database file path is required")
	}
	abs, err := filepath.Abs(cfg.FilePath)
	if err == nil {
		cfg.FilePath = abs
	}
	if cfg.Database == "" {
		cfg.Database = "main"
	}
	// Opening a non-existent file in read-write mode silently creates an empty
	// database, which is almost never what the user meant when they mistyped a
	// path. Refuse unless they explicitly asked for creation.
	if !fileExists(cfg.FilePath) && !strings.EqualFold(cfg.Params["mode"], "rwc") {
		return apperr.New(apperr.CodeInvalidConfig,
			"database file %s does not exist (set mode=rwc to create it)", cfg.FilePath)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Open implements drivers.Driver.
func (d Driver) Open(ctx context.Context, cfg models.ConnectionConfig) (drivers.Conn, error) {
	if err := d.Normalize(&cfg); err != nil {
		return nil, err
	}
	conn := sqlbase.NewConn(spec(), cfg)
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func spec() sqlbase.Spec {
	return sqlbase.Spec{
		Info:              Driver{}.Info(),
		SQLDriver:         "sqlite",
		Dialect:           sqlbase.SQLiteDialect{},
		Introspector:      introspector{},
		DSN:               buildDSN,
		BootstrapDatabase: func(models.ConnectionConfig) string { return "main" },
		NativeDDL:         nativeDDL,
		Overview:          overview,
		ExplainSQL:        func(sql string) string { return "EXPLAIN QUERY PLAN " + sql },
	}
}

func buildDSN(cfg models.ConnectionConfig, database string) (string, error) {
	path := cfg.FilePath
	if path == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "a database file path is required")
	}

	q := url.Values{}
	mode := "ro"
	if v, ok := cfg.Params["mode"]; ok && v != "" {
		mode = v
	}
	if !cfg.ReadOnly && mode == "ro" {
		// Read-write is the sensible default for a desktop client unless the
		// user marked the connection read-only.
		mode = "rw"
	}
	q.Set("mode", mode)
	q.Set("_pragma", "busy_timeout(10000)")

	for k, v := range cfg.Params {
		if k == "" || strings.EqualFold(k, "mode") {
			continue
		}
		q.Set(k, v)
	}

	scheme := "file:"
	return scheme + filepath.ToSlash(path) + "?" + q.Encode(), nil
}

// --- introspection ---------------------------------------------------------

type introspector struct{}

func (introspector) Version(ctx context.Context, q sqlbase.Querier) (string, error) {
	var v string
	if err := q.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&v); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read server version")
	}
	return v, nil
}

func (introspector) CurrentDatabase(context.Context, sqlbase.Querier) (string, error) {
	return "main", nil
}

// Databases lists attached database files (main plus anything ATTACHed).
func (introspector) Databases(ctx context.Context, q sqlbase.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list attached databases")
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var (
			seq  int
			name string
			file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return nil, err
		}
		if strings.EqualFold(name, "temp") {
			continue
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// Schemas is empty: SQLite has no schema layer.
func (introspector) Schemas(context.Context, sqlbase.Querier, string) ([]string, error) {
	return []string{}, nil
}

const objectSelect = `
SELECT name, type, COALESCE(sql, '')
FROM %s.sqlite_master
WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%%'`

func (i introspector) Object(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	objs, err := i.Objects(ctx, q, database, schema)
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if o.Name == object {
			return &o, nil
		}
	}
	return nil, apperr.New(apperr.CodeNotFound, "object %s does not exist", object)
}

func (introspector) Objects(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.ObjectInfo, error) {
	// sqlite_master is a table; its name cannot be parameterised, so it is
	// qualified with a quoted attached-database name.
	qualified := "main"
	if database != "" {
		qualified = sqlutil.QuoteDouble(database)
	}
	query := fmt.Sprintf(objectSelect, qualified)

	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	defer rows.Close()

	out := []models.ObjectInfo{}
	for rows.Next() {
		var name, kind, sqlText string
		if err := rows.Scan(&name, &kind, &sqlText); err != nil {
			return nil, err
		}
		obj := models.ObjectInfo{
			Name:     name,
			Database: database,
			Kind:     models.KindTable,
		}
		if strings.EqualFold(kind, "view") {
			obj.Kind = models.KindView
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

func (introspector) Columns(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ColumnInfo, error) {
	// table_xinfo also reports generated/hidden columns; it exists since
	// SQLite 3.26.
	rows, err := q.QueryContext(ctx, "PRAGMA "+pragmaTarget(database)+".table_xinfo("+quotePragmaArg(object)+")")
	if err != nil {
		rows, err = q.QueryContext(ctx, "PRAGMA "+pragmaTarget(database)+".table_info("+quotePragmaArg(object)+")")
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
		}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// table_xinfo: cid, name, type, notnull, dflt_value, pk, hidden
	// table_info : cid, name, type, notnull, dflt_value, pk
	width := len(cols)

	out := []models.ColumnInfo{}
	for rows.Next() {
		holders := make([]any, width)
		ptrs := make([]any, width)
		for i := range holders {
			ptrs[i] = &holders[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		c := models.ColumnInfo{
			Ordinal: int(toInt64(holders[0])),
			Name:    toString(holders[1]),
		}
		declared := strings.ToUpper(toString(holders[2]))
		c.ColumnType = declared
		c.DataType = strings.ToLower(firstWord(declared))
		c.Nullable = toInt64(holders[3]) == 0
		if dv := holders[4]; dv != nil {
			v := toString(dv)
			c.DefaultValue = &v
		}
		pkOrdinal := toInt64(holders[5])
		c.PrimaryKey = pkOrdinal > 0
		// SQLite only auto-increments an INTEGER PRIMARY KEY (rowid alias).
		c.AutoIncrement = c.PrimaryKey && strings.EqualFold(c.DataType, "integer")
		out = append(out, c)
	}
	return out, rows.Err()
}

func pragmaTarget(database string) string {
	if database == "" {
		return "main"
	}
	return database
}

// quotePragmaArg renders a table name for PRAGMA syntax, which accepts either
// a bare identifier or a quoted string.
func quotePragmaArg(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func (introspector) Indexes(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.IndexInfo, error) {
	target := pragmaTarget(database)

	rows, err := q.QueryContext(ctx, "PRAGMA "+target+".index_list("+quotePragmaArg(object)+")")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	var (
		names  []string
		byName = map[string]*models.IndexInfo{}
	)
	for rows.Next() {
		// index_list: seq, name, unique, origin, partial
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		names = append(names, name)
		byName[name] = &models.IndexInfo{
			Name:    name,
			Unique:  unique == 1,
			Primary: origin == "pk",
			Method:  "btree",
			Columns: []string{},
		}
		if partial == 1 {
			byName[name].Comment = "partial index"
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for _, name := range names {
		idx := byName[name]
		info, err := q.QueryContext(ctx, "PRAGMA "+target+".index_info("+quotePragmaArg(name)+")")
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "inspect index %s", name)
		}
		for info.Next() {
			// index_info: seqno, cid, name
			var (
				seqno, cid int
				colName    sql.NullString
			)
			if err := info.Scan(&seqno, &cid, &colName); err != nil {
				info.Close()
				return nil, err
			}
			if colName.Valid {
				idx.Columns = append(idx.Columns, colName.String)
			} else {
				idx.Columns = append(idx.Columns, "(expression)")
			}
		}
		err = info.Err()
		info.Close()
		if err != nil {
			return nil, err
		}
	}

	out := make([]models.IndexInfo, 0, len(names))
	for _, name := range names {
		out = append(out, *byName[name])
	}
	return out, nil
}

// NamespaceIndexes lists the indexes of every table/view in the database.
//
// SQLite has no catalog table that maps indexes to tables in one query (index
// ownership lives in PRAGMA index_list), so this walks the object list. That is
// fine for the sizes SQLite targets, and the explorer loads it lazily.
func (i introspector) NamespaceIndexes(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.IndexEntry, error) {
	objects, err := i.Objects(ctx, q, database, schema)
	if err != nil {
		return nil, err
	}

	out := []models.IndexEntry{}
	for _, object := range objects {
		indexes, err := i.Indexes(ctx, q, database, schema, object.Name)
		if err != nil {
			return nil, err
		}
		for _, idx := range indexes {
			out = append(out, models.IndexEntry{
				Name:     idx.Name,
				Table:    object.Name,
				Database: database,
				Schema:   schema,
				Columns:  idx.Columns,
				Unique:   idx.Unique,
				Primary:  idx.Primary,
				Method:   idx.Method,
			})
		}
	}
	return out, nil
}

func (introspector) ForeignKeys(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ForeignKeyInfo, error) {
	// A SQLite table has no named FK constraint, so rows are grouped by the
	// "id" column, which is stable within one table definition.
	rows, err := q.QueryContext(ctx, "PRAGMA "+pragmaTarget(database)+".foreign_key_list("+quotePragmaArg(object)+")")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list foreign keys")
	}
	defer rows.Close()

	byID := map[int64]*models.ForeignKeyInfo{}
	order := []int64{}
	for rows.Next() {
		// foreign_key_list: id, seq, table, from, to, on_update, on_delete, match
		var (
			id, seq  int64
			refTable string
			from     string
			to       sql.NullString
			onUpdate string
			onDelete string
			match    string
		)
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		fk, ok := byID[id]
		if !ok {
			fk = &models.ForeignKeyInfo{
				Name:              fmt.Sprintf("fk_%s_%d", object, id),
				ReferencedTable:   refTable,
				OnUpdate:          onUpdate,
				OnDelete:          onDelete,
				Columns:           []string{},
				ReferencedColumns: []string{},
			}
			byID[id] = fk
			order = append(order, id)
		}
		fk.Columns = append(fk.Columns, from)
		fk.ReferencedColumns = append(fk.ReferencedColumns, to.String)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.ForeignKeyInfo, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func nativeDDL(ctx context.Context, q sqlbase.Querier, database, schema, object string) (string, error) {
	target := pragmaTarget(database)
	var sqlText sql.NullString
	if err := q.QueryRowContext(ctx,
		"SELECT sql FROM "+sqlutil.QuoteDouble(target)+".sqlite_master WHERE name = ?", object,
	).Scan(&sqlText); err != nil {
		return "", err
	}
	return sqlText.String, nil
}

// --- value helpers for PRAGMA rows (they come back as interface{}) ---------

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case []byte:
		n, _ := strconv.ParseInt(string(t), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}
