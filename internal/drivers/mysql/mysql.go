// Package mysql implements the MySQL / MariaDB driver.
package mysql

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

const defaultPort = 3306

// systemDatabases are the schemas every MySQL server ships with. They are
// hidden from the explorer as long as the server has something else to show;
// see visibleDatabases.
var systemDatabases = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"mysql":              true,
	"sys":                true,
}

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverMySQL,
		DisplayName:      "MySQL / MariaDB",
		DefaultPort:      defaultPort,
		Implemented:      true,
		Relational:       true,
		SupportsDatabase: true,
		SupportsSchema:   false,
		SortOrder:        10,
		DefaultDatabase:  "",
		Notes:            "MySQL treats schemas as databases; the explorer shows a single level.",
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
		cfg.Username = "root"
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
		SQLDriver:    "mysql",
		Dialect:      sqlbase.MySQLDialect{},
		Introspector: introspector{},
		DSN:          buildDSN,
		BootstrapDatabase: func(models.ConnectionConfig) string {
			// Connecting without a default schema is valid and lets the user
			// browse every database they can access.
			return ""
		},
		NativeDDL: nativeDDL,
	}
}

func buildDSN(cfg models.ConnectionConfig, database string) (string, error) {
	mc := driver.NewConfig()
	mc.User = cfg.Username
	mc.Passwd = cfg.Password
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mc.DBName = database
	mc.ParseTime = true
	mc.Loc = time.Local
	mc.Timeout = 15 * time.Second
	mc.AllowNativePasswords = true
	mc.Params = map[string]string{"charset": "utf8mb4"}

	for k, v := range cfg.Params {
		if k == "" || isReservedParam(k) {
			continue
		}
		mc.Params[k] = v
	}

	tlsName, err := tlsConfigName(cfg.SSL)
	if err != nil {
		return "", err
	}
	mc.TLSConfig = tlsName

	// FormatDSN never includes the password when it is empty, but when set it
	// is embedded; callers must treat the result as a secret.
	return mc.FormatDSN(), nil
}

func isReservedParam(k string) bool {
	switch strings.ToLower(k) {
	case "tls", "charset", "parsetime", "loc", "timeout", "readtimeout", "writetimeout":
		return true
	}
	return false
}

// tlsConfigName maps our SSL settings onto go-sql-driver's TLS registry.
func tlsConfigName(ssl models.SSLConfig) (string, error) {
	switch ssl.Mode {
	case "", models.SSLDisable:
		return "false", nil
	case models.SSLRequire:
		// Encrypted transport, no certificate verification: the historical
		// meaning of "require" in MySQL.
		return "skip-verify", nil
	case models.SSLVerifyCA, models.SSLVerifyFull:
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if ssl.Mode == models.SSLVerifyFull {
			tlsCfg.ServerName = ""
		} else {
			// verify-ca: verify the chain but not the hostname.
			tlsCfg.InsecureSkipVerify = true
			tlsCfg.VerifyPeerCertificate = verifyChainOnly
		}
		if ssl.CAFile != "" {
			pem, err := os.ReadFile(ssl.CAFile)
			if err != nil {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "read CA file")
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return "", apperr.New(apperr.CodeInvalidConfig, "CA file %s contains no usable certificate", ssl.CAFile)
			}
			tlsCfg.RootCAs = pool
		}
		if ssl.CertFile != "" && ssl.KeyFile != "" {
			pair, err := tls.LoadX509KeyPair(ssl.CertFile, ssl.KeyFile)
			if err != nil {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "load client certificate")
			}
			tlsCfg.Certificates = []tls.Certificate{pair}
		}

		// Registering under a deterministic name keeps repeated connects from
		// leaking registry entries.
		sum := sha1.Sum([]byte(string(ssl.Mode) + "|" + ssl.CAFile + "|" + ssl.CertFile + "|" + ssl.KeyFile))
		name := "dbm-" + hex.EncodeToString(sum[:8])
		if err := driver.RegisterTLSConfig(name, tlsCfg); err != nil {
			// A duplicate registration is fine: the config is equivalent.
			if !strings.Contains(err.Error(), "already registered") {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "register TLS config")
			}
		}
		return name, nil
	default:
		return "", apperr.New(apperr.CodeInvalidConfig, "unsupported SSL mode %q", ssl.Mode)
	}
}

func verifyChainOnly(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	certs := make([]*x509.Certificate, 0, len(rawCerts))
	for _, raw := range rawCerts {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return fmt.Errorf("no certificate presented by server")
	}
	pool := x509.NewCertPool()
	for _, c := range certs[1:] {
		pool.AddCert(c)
	}
	_, err := certs[0].Verify(x509.VerifyOptions{Roots: pool, Intermediates: pool})
	return err
}

// --- introspection ---------------------------------------------------------

type introspector struct{}

const objectSelect = `
SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_COMMENT, ''), IFNULL(TABLE_ROWS, 0),
       IFNULL(DATA_LENGTH, 0) + IFNULL(INDEX_LENGTH, 0), IFNULL(ENGINE, '')
FROM information_schema.TABLES`

func (introspector) Version(ctx context.Context, q sqlbase.Querier) (string, error) {
	var v string
	if err := q.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read server version")
	}
	return v, nil
}

func (introspector) CurrentDatabase(ctx context.Context, q sqlbase.Querier) (string, error) {
	var name sql.NullString
	if err := q.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read current database")
	}
	return name.String, nil
}

func (introspector) Databases(ctx context.Context, q sqlbase.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT SCHEMA_NAME FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	defer rows.Close()

	all := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		all = append(all, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return visibleDatabases(all), nil
}

// visibleDatabases hides the schemas MySQL ships with — they are noise once the
// server has real databases. A brand new server has nothing else though, and an
// explorer with no children reads as a broken connection, so in that case the
// system schemas are kept (they are still browsable, and the alternative is an
// empty tree with nothing to click).
func visibleDatabases(all []string) []string {
	out := make([]string, 0, len(all))
	for _, name := range all {
		if systemDatabases[strings.ToLower(name)] {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 && len(all) > 0 {
		return all
	}
	return out
}

// Schemas is empty: MySQL has no schema layer below the database.
func (introspector) Schemas(context.Context, sqlbase.Querier, string) ([]string, error) {
	return []string{}, nil
}

func (i introspector) Object(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "describe object")
	}
	defer rows.Close()
	objs, err := scanObjects(rows, database, schema)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "object %s.%s does not exist", database, object)
	}
	return &objs[0], nil
}

func (i introspector) Objects(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME", database)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	defer rows.Close()
	return scanObjects(rows, database, schema)
}

func scanObjects(rows *sql.Rows, database, schema string) ([]models.ObjectInfo, error) {
	out := []models.ObjectInfo{}
	for rows.Next() {
		var (
			name, tableType, comment, engine string
			rowEstimate, size                sql.NullInt64
		)
		if err := rows.Scan(&name, &tableType, &comment, &rowEstimate, &size, &engine); err != nil {
			return nil, err
		}
		out = append(out, models.ObjectInfo{
			Name:        name,
			Schema:      schema,
			Database:    database,
			Kind:        kindFromTableType(tableType),
			Comment:     comment,
			RowEstimate: rowEstimate.Int64,
			SizeBytes:   size.Int64,
			Engine:      engine,
		})
	}
	return out, rows.Err()
}

func kindFromTableType(t string) models.ObjectKind {
	switch strings.ToUpper(t) {
	case "VIEW":
		return models.KindView
	case "BASE TABLE":
		return models.KindTable
	default:
		return models.KindTable
	}
}

func (introspector) Columns(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ColumnInfo, error) {
	const query = `
SELECT COLUMN_NAME, ORDINAL_POSITION, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
       COLUMN_KEY, EXTRA, IFNULL(COLUMN_COMMENT, ''),
       CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
	}
	defer rows.Close()

	out := []models.ColumnInfo{}
	for rows.Next() {
		var (
			c                     models.ColumnInfo
			isNullable, columnKey string
			def                   sql.NullString
			extra, comment        string
			charLen, prec, scale  sql.NullInt64
		)
		if err := rows.Scan(&c.Name, &c.Ordinal, &c.DataType, &c.ColumnType, &isNullable, &def,
			&columnKey, &extra, &comment, &charLen, &prec, &scale); err != nil {
			return nil, err
		}
		c.Nullable = strings.EqualFold(isNullable, "YES")
		c.PrimaryKey = strings.EqualFold(columnKey, "PRI")
		c.AutoIncrement = strings.Contains(strings.ToLower(extra), "auto_increment")
		c.Comment = comment
		if def.Valid {
			v := def.String
			c.DefaultValue = &v
		}
		c.CharMaxLength = nullableInt(charLen)
		c.NumericPrecision = nullableInt(prec)
		c.NumericScale = nullableInt(scale)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (introspector) Indexes(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.IndexInfo, error) {
	const query = `
SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(INDEX_TYPE, ''), IFNULL(INDEX_COMMENT, '')
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	byName := map[string]*models.IndexInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, indexType, comment string
			nonUnique                int
			seq                      int
			column                   sql.NullString
		)
		if err := rows.Scan(&name, &nonUnique, &seq, &column, &indexType, &comment); err != nil {
			return nil, err
		}
		idx, ok := byName[name]
		if !ok {
			idx = &models.IndexInfo{
				Name:    name,
				Unique:  nonUnique == 0,
				Primary: strings.EqualFold(name, "PRIMARY"),
				Method:  indexType,
				Comment: comment,
				Columns: []string{},
			}
			byName[name] = idx
			order = append(order, name)
		}
		if column.Valid && column.String != "" {
			idx.Columns = append(idx.Columns, column.String)
		} else {
			// Functional index: STATISTICS has no column name.
			idx.Columns = append(idx.Columns, "(expression)")
		}
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
SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(INDEX_TYPE, '')
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`

	rows, err := q.QueryContext(ctx, query, database)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	type key struct{ table, name string }
	byKey := map[key]*models.IndexEntry{}
	order := []key{}
	for rows.Next() {
		var (
			table, name, indexType string
			nonUnique, seq         int
			column                 sql.NullString
		)
		if err := rows.Scan(&table, &name, &nonUnique, &seq, &column, &indexType); err != nil {
			return nil, err
		}
		k := key{table: table, name: name}
		entry, ok := byKey[k]
		if !ok {
			entry = &models.IndexEntry{
				Name:     name,
				Table:    table,
				Database: database,
				Schema:   schema,
				Unique:   nonUnique == 0,
				Primary:  strings.EqualFold(name, "PRIMARY"),
				Method:   indexType,
				Columns:  []string{},
			}
			byKey[k] = entry
			order = append(order, k)
		}
		if column.Valid && column.String != "" {
			entry.Columns = append(entry.Columns, column.String)
		} else {
			entry.Columns = append(entry.Columns, "(expression)")
		}
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
SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME,
       k.REFERENCED_COLUMN_NAME, r.UPDATE_RULE, r.DELETE_RULE
FROM information_schema.KEY_COLUMN_USAGE k
JOIN information_schema.REFERENTIAL_CONSTRAINTS r
  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME
WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL
ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list foreign keys")
	}
	defer rows.Close()

	byName := map[string]*models.ForeignKeyInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, column, refSchema, refTable, refColumn, onUpdate, onDelete string
		)
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn, &onUpdate, &onDelete); err != nil {
			return nil, err
		}
		fk, ok := byName[name]
		if !ok {
			fk = &models.ForeignKeyInfo{
				Name:              name,
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				OnUpdate:          onUpdate,
				OnDelete:          onDelete,
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

func nativeDDL(ctx context.Context, q sqlbase.Querier, database, schema, object string) (string, error) {
	target := sqlutil.Qualify(sqlutil.QuoteBacktick, database, object)

	rows, err := q.QueryContext(ctx, "SHOW CREATE TABLE "+target)
	if err != nil {
		// Views need their own statement.
		rows, err = q.QueryContext(ctx, "SHOW CREATE VIEW "+target)
		if err != nil {
			return "", err
		}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", sql.ErrNoRows
	}
	holders := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range holders {
		ptrs[i] = &holders[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	// The DDL is always the second column ("Create Table" / "Create View").
	if len(holders) >= 2 {
		if s, ok := holders[1].(string); ok {
			return s, nil
		}
		if b, ok := holders[1].([]byte); ok {
			return string(b), nil
		}
	}
	return "", nil
}

func nullableInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
