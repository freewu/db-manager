// Package tidb implements the TiDB driver.
//
// TiDB speaks the MySQL wire protocol and reads its own catalog through
// MySQL's information_schema, so browsing, editing and the table designer all
// come from mysqlcompat and sqlbase. What is TiDB-specific — the port, the
// system schemas it ships with, and the cluster topology behind
// information_schema.CLUSTER_INFO — lives here.
package tidb

import (
	"context"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

const defaultPort = 4000

// systemSchemas are the databases a TiDB cluster creates for itself. `test` is
// deliberately not in the list: it is where users put their first table.
var systemSchemas = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"mysql":              true,
	"metrics_schema":     true,
	"sys":                true,
}

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverTiDB,
		DisplayName:      "TiDB",
		DefaultPort:      defaultPort,
		Implemented:      true,
		Relational:       true,
		SupportsDatabase: true,
		SupportsSchema:   false,
		SupportsDesign:   true,
		// A plain EXPLAIN is answered by this whole family (see
		// mysqlcompat.ExplainSQL) and never runs the statement.
		SupportsExplain: true,
		ObjectKinds:     []models.ObjectKind{models.KindTable, models.KindView},
		SortOrder:       45,
		DefaultDatabase: "",
		Notes:           "MySQL-compatible distributed SQL: the explorer reads the MySQL catalog, the status page adds the cluster topology.",
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
		Dialect:      sqlbase.MySQLDialect{Driver: models.DriverTiDB},
		Introspector: mysqlcompat.Introspector{SystemSchemas: systemSchemas},
		DSN:          mysqlcompat.DSN,
		BootstrapDatabase: func(models.ConnectionConfig) string {
			// TiDB accepts a session without a default schema, which is what
			// lets the explorer list every database at once.
			return ""
		},
		NativeDDL: mysqlcompat.NativeDDL,
		Overview:  overview,
		// TiDB answers SHOW CHARACTER SET / SHOW COLLATION with the set it
		// supports and takes the same CREATE DATABASE clauses as MySQL, so the
		// shared implementation is the whole story.
		DatabaseOptions: mysqlcompat.DatabaseOptions,
		ExplainSQL:      mysqlcompat.ExplainSQL,
		CreateDatabase:  mysqlcompat.CreateDatabase,
	}
}
