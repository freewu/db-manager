// Package postgres implements the PostgreSQL driver on top of pgx's
// database/sql adapter.
package postgres

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

const defaultPort = 5432

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverPostgres,
		DisplayName:      "PostgreSQL",
		DefaultPort:      defaultPort,
		Implemented:      true,
		Relational:       true,
		SupportsDatabase: true,
		SupportsSchema:   true,
		SupportsDesign:   true,
		SortOrder:        20,
		DefaultDatabase:  "postgres",
		Notes:            "PostgreSQL cannot query across databases; the app opens a separate pool per database.",
	}
}

// Normalize implements drivers.Driver.
func (Driver) Normalize(cfg *models.ConnectionConfig) error {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return apperr.New(apperr.CodeInvalidConfig, "port %d is out of range", cfg.Port)
	}
	if cfg.Username == "" {
		cfg.Username = "postgres"
	}
	if cfg.Database == "" {
		cfg.Database = "postgres"
	}
	if cfg.SSL.Mode == "" {
		cfg.SSL.Mode = models.SSLDisable
	}
	return nil
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
		Info:         Driver{}.Info(),
		SQLDriver:    "pgx",
		Dialect:      sqlbase.PostgresDialect{},
		Introspector: introspector{},
		DSN:          buildDSN,
		BootstrapDatabase: func(cfg models.ConnectionConfig) string {
			if cfg.Database != "" {
				return cfg.Database
			}
			return "postgres"
		},
		// PostgreSQL has no SHOW CREATE TABLE; the generic renderer is used.
		Overview: overview,
		// CREATE DATABASE has its own shape here (encoding + locale + template0),
		// so PostgreSQL renders it itself; see database.go.
		DatabaseOptions: DatabaseOptions,
		CreateDatabase:  CreateDatabase,
	}
}

func buildDSN(cfg models.ConnectionConfig, database string) (string, error) {
	if database == "" {
		database = "postgres"
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   "/" + database,
	}
	if cfg.Username != "" {
		if cfg.Password != "" {
			u.User = url.UserPassword(cfg.Username, cfg.Password)
		} else {
			u.User = url.User(cfg.Username)
		}
	}

	q := url.Values{}
	mode := string(cfg.SSL.Mode)
	if mode == "" {
		mode = string(models.SSLDisable)
	}
	q.Set("sslmode", mode)
	q.Set("connect_timeout", "15")
	q.Set("application_name", "db-manager")
	if cfg.SSL.CAFile != "" {
		q.Set("sslrootcert", cfg.SSL.CAFile)
	}
	if cfg.SSL.CertFile != "" {
		q.Set("sslcert", cfg.SSL.CertFile)
	}
	if cfg.SSL.KeyFile != "" {
		q.Set("sslkey", cfg.SSL.KeyFile)
	}
	for k, v := range cfg.Params {
		if k == "" || isReservedParam(k) {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func isReservedParam(k string) bool {
	switch strings.ToLower(k) {
	case "sslmode", "sslrootcert", "sslcert", "sslkey", "connect_timeout", "application_name":
		return true
	}
	return false
}

// --- introspection ---------------------------------------------------------

type introspector struct{}

const objectSelect = `
SELECT c.relname,
       c.relkind::text,
       COALESCE(obj_description(c.oid, 'pg_class'), ''),
       GREATEST(c.reltuples, 0)::bigint,
       CASE WHEN c.relkind IN ('r', 'p', 'm', 'f') THEN pg_total_relation_size(c.oid) ELSE 0 END,
       COALESCE(am.amname, '')
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_am am ON am.oid = c.relam
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')`

func (introspector) Version(ctx context.Context, q sqlbase.Querier) (string, error) {
	var v string
	if err := q.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&v); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read server version")
	}
	return v, nil
}

func (introspector) CurrentDatabase(ctx context.Context, q sqlbase.Querier) (string, error) {
	var name string
	if err := q.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read current database")
	}
	return name, nil
}

func (introspector) Databases(ctx context.Context, q sqlbase.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT datname FROM pg_database
WHERE NOT datistemplate AND datallowconn
ORDER BY datname`)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (introspector) Schemas(ctx context.Context, q sqlbase.Querier, database string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT nspname FROM pg_namespace
WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'
ORDER BY nspname`)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list schemas")
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (i introspector) Object(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" AND n.nspname = $1 AND c.relname = $2", schemaOf(schema), object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "describe object")
	}
	defer rows.Close()

	objs, err := scanObjects(rows, database, schema)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "object %s.%s does not exist", schemaOf(schema), object)
	}
	return &objs[0], nil
}

func (i introspector) Objects(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx,
		objectSelect+" AND n.nspname = $1 ORDER BY c.relname", schemaOf(schema))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	defer rows.Close()
	return scanObjects(rows, database, schema)
}

func schemaOf(schema string) string {
	if schema == "" {
		return "public"
	}
	return schema
}

func scanObjects(rows *sql.Rows, database, schema string) ([]models.ObjectInfo, error) {
	out := []models.ObjectInfo{}
	for rows.Next() {
		var (
			name, relkind, comment, method string
			rowEstimate                    int64
			size                           sql.NullInt64
		)
		if err := rows.Scan(&name, &relkind, &comment, &rowEstimate, &size, &method); err != nil {
			return nil, err
		}
		out = append(out, models.ObjectInfo{
			Name:        name,
			Schema:      schemaOf(schema),
			Database:    database,
			Kind:        kindFromRelkind(relkind),
			Comment:     comment,
			RowEstimate: rowEstimate,
			SizeBytes:   size.Int64,
			Engine:      method,
		})
	}
	return out, rows.Err()
}

func kindFromRelkind(kind string) models.ObjectKind {
	switch kind {
	case "v":
		return models.KindView
	case "m":
		return models.KindMatView
	case "r", "p", "f":
		return models.KindTable
	default:
		return models.KindTable
	}
}

func (introspector) Columns(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ColumnInfo, error) {
	const query = `
SELECT a.attname,
       a.attnum::int,
       COALESCE(ic.data_type, format_type(a.atttypid, a.atttypmod)),
       format_type(a.atttypid, a.atttypmod),
       NOT a.attnotnull,
       pg_get_expr(ad.adbin, ad.adrelid),
       COALESCE(col_description(a.attrelid, a.attnum), ''),
       (a.attidentity IN ('a', 'd')),
       ic.character_maximum_length,
       ic.numeric_precision,
       ic.numeric_scale,
       (pkc.column_name IS NOT NULL)
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN information_schema.columns ic
       ON ic.table_schema = n.nspname AND ic.table_name = c.relname AND ic.column_name = a.attname
LEFT JOIN (
    SELECT kcu.column_name
    FROM information_schema.table_constraints tc
    JOIN information_schema.key_column_usage kcu
      ON kcu.constraint_name = tc.constraint_name
     AND kcu.table_schema = tc.table_schema
     AND kcu.table_name = tc.table_name
    WHERE tc.constraint_type = 'PRIMARY KEY'
      AND tc.table_schema = $1
      AND tc.table_name = $2
) pkc ON pkc.column_name = a.attname
WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`

	rows, err := q.QueryContext(ctx, query, schemaOf(schema), object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
	}
	defer rows.Close()

	out := []models.ColumnInfo{}
	for rows.Next() {
		var (
			c             models.ColumnInfo
			nullable      bool
			def           sql.NullString
			comment       string
			isIdentity    bool
			charLen, prec sql.NullInt64
			scale         sql.NullInt64
			primary       bool
		)
		if err := rows.Scan(&c.Name, &c.Ordinal, &c.DataType, &c.ColumnType, &nullable, &def,
			&comment, &isIdentity, &charLen, &prec, &scale, &primary); err != nil {
			return nil, err
		}
		c.Nullable = nullable
		c.PrimaryKey = primary
		c.Comment = comment
		if def.Valid {
			v := def.String
			c.DefaultValue = &v
		}
		c.CharMaxLength = nullableInt(charLen)
		c.NumericPrecision = nullableInt(prec)
		c.NumericScale = nullableInt(scale)
		// identity columns, or a nextval() default (serial / bigserial).
		c.AutoIncrement = isIdentity || (def.Valid && strings.Contains(def.String, "nextval("))
		out = append(out, c)
	}
	return out, rows.Err()
}

func (introspector) Indexes(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.IndexInfo, error) {
	const query = `
SELECT i.relname,
       ix.indisunique,
       ix.indisprimary,
       am.amname,
       COALESCE(obj_description(i.oid, 'pg_class'), ''),
       k.ord::int,
       pg_get_indexdef(i.oid, k.ord::int, true)
FROM pg_index ix
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN pg_am am ON am.oid = i.relam
JOIN LATERAL generate_series(1, ix.indnkeyatts::int) AS k(ord) ON true
WHERE n.nspname = $1 AND t.relname = $2
ORDER BY i.relname, k.ord`

	rows, err := q.QueryContext(ctx, query, schemaOf(schema), object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	byName := map[string]*models.IndexInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, method, comment, column string
			unique, primary               bool
			ord                           int
		)
		if err := rows.Scan(&name, &unique, &primary, &method, &comment, &ord, &column); err != nil {
			return nil, err
		}
		idx, ok := byName[name]
		if !ok {
			idx = &models.IndexInfo{
				Name:    name,
				Unique:  unique,
				Primary: primary,
				Method:  method,
				Comment: comment,
				Columns: []string{},
			}
			byName[name] = idx
			order = append(order, name)
		}
		idx.Columns = append(idx.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.IndexInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

func (introspector) NamespaceIndexes(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.IndexEntry, error) {
	const query = `
SELECT t.relname,
       i.relname,
       ix.indisunique,
       ix.indisprimary,
       am.amname,
       k.ord::int,
       pg_get_indexdef(i.oid, k.ord::int, true)
FROM pg_index ix
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN pg_am am ON am.oid = i.relam
JOIN LATERAL generate_series(1, ix.indnkeyatts::int) AS k(ord) ON true
WHERE n.nspname = $1
ORDER BY t.relname, i.relname, k.ord`

	rows, err := q.QueryContext(ctx, query, schemaOf(schema))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	type key struct{ table, name string }
	byKey := map[key]*models.IndexEntry{}
	order := []key{}
	for rows.Next() {
		var (
			table, name, method, column string
			unique, primary           bool
			ord                       int
		)
		if err := rows.Scan(&table, &name, &unique, &primary, &method, &ord, &column); err != nil {
			return nil, err
		}
		k := key{table: table, name: name}
		entry, ok := byKey[k]
		if !ok {
			entry = &models.IndexEntry{
				Name:     name,
				Table:    table,
				Database: database,
				Schema:   schemaOf(schema),
				Unique:   unique,
				Primary:  primary,
				Method:   method,
				Columns:  []string{},
			}
			byKey[k] = entry
			order = append(order, k)
		}
		entry.Columns = append(entry.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.IndexEntry, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

func (introspector) ForeignKeys(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ForeignKeyInfo, error) {
	const query = `
SELECT con.conname,
       att.attname,
       fn.nspname,
       fcl.relname,
       fatt.attname,
       con.confupdtype::text,
       con.confdeltype::text
FROM pg_constraint con
JOIN pg_class cl ON cl.oid = con.conrelid
JOIN pg_namespace n ON n.oid = cl.relnamespace
JOIN pg_class fcl ON fcl.oid = con.confrelid
JOIN pg_namespace fn ON fn.oid = fcl.relnamespace
JOIN LATERAL generate_subscripts(con.conkey, 1) AS k(ord) ON true
JOIN pg_attribute att ON att.attrelid = con.conrelid AND att.attnum = con.conkey[k.ord]
JOIN pg_attribute fatt ON fatt.attrelid = con.confrelid AND fatt.attnum = con.confkey[k.ord]
WHERE con.contype = 'f' AND n.nspname = $1 AND cl.relname = $2
ORDER BY con.conname, k.ord`

	rows, err := q.QueryContext(ctx, query, schemaOf(schema), object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list foreign keys")
	}
	defer rows.Close()

	byName := map[string]*models.ForeignKeyInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, column, refSchema, refTable, refColumn string
			upd, del                                     string
		)
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn, &upd, &del); err != nil {
			return nil, err
		}
		fk, ok := byName[name]
		if !ok {
			fk = &models.ForeignKeyInfo{
				Name:              name,
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				OnUpdate:          actionName(upd),
				OnDelete:          actionName(del),
				Columns:           []string{},
				ReferencedColumns: []string{},
			}
			byName[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, column)
		fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.ForeignKeyInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// actionName maps PostgreSQL's single character referential action codes.
func actionName(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return ""
	}
}

func nullableInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
