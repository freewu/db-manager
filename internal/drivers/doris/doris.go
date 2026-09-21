// Package doris implements the Apache Doris driver.
//
// Doris answers the MySQL wire protocol, so the DSN builder and the SHOW-based
// helpers come from mysqlcompat and the table designer is deliberately off
// (see Info). Its catalog, however, is not MySQL's: information_schema exists
// but is thin and version-dependent, while SHOW DATABASES / SHOW FULL TABLES /
// SHOW FULL COLUMNS / SHOW INDEX are the statements Doris documents and keeps
// stable. The introspector in this package therefore reads the SHOW forms and
// uses information_schema only where it can enrich them.
package doris

import (
	"context"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

const defaultPort = 9030

// systemSchemas are the databases a Doris cluster creates for itself. Doris
// hides some of them from SHOW DATABASES already; filtering again costs
// nothing and covers the versions that list them.
var systemSchemas = map[string]bool{
	"information_schema": true,
	"__internal_schema":  true,
	"mysql":              true,
	"sys":                true,
}

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverDoris,
		DisplayName:      "Apache Doris",
		DefaultPort:      defaultPort,
		Implemented:      true,
		Relational:       true,
		SupportsDatabase: true,
		SupportsSchema:   false,
		// A Doris table needs a data model and a distribution clause
		// (DUPLICATE/UNIQUE/AGGREGATE KEY, DISTRIBUTED BY, ...), which the
		// designer does not model. Tables are still browsable and editable
		// through the DDL editor, including CREATE TABLE.
		SupportsDesign:  false,
		SortOrder:       46,
		DefaultDatabase: "",
		Notes:           "MySQL protocol, MPP analytics: the designer is read-only here because Doris DDL needs distribution clauses.",
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
		Dialect:      sqlbase.MySQLDialect{Driver: models.DriverDoris},
		Introspector: newIntrospector(),
		DSN:          mysqlcompat.DSNWith(DSNOptions),
		BootstrapDatabase: func(models.ConnectionConfig) string {
			// Doris accepts a session without a default database, which is what
			// lets the explorer list every database at once.
			return ""
		},
		NativeDDL: mysqlcompat.NativeDDL,
		Overview:  overview,
		// Doris creates databases but has nothing to choose about them; see
		// database.go.
		DatabaseOptions: DatabaseOptions,
		CreateDatabase:  CreateDatabase,
	}
}

// newIntrospector is the Doris catalog reader. The system-schema list belongs
// to the engine, so it is wired here rather than left for the caller to
// remember.
func newIntrospector() introspector {
	return introspector{Introspector: mysqlcompat.Introspector{SystemSchemas: systemSchemas}}
}

// DSNOptions are the driver options Doris needs: it expands placeholders on the
// client, so a query never depends on Doris' prepared-statement support, which
// differs between releases.
var DSNOptions = mysqlcompat.DSNOptions{InterpolateParams: true}
