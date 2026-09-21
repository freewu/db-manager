// Package mysql implements the MySQL / MariaDB driver.
//
// It is the reference engine of the MySQL family: the catalog queries, the DSN
// builder and the SHOW-based helpers live in mysqlcompat, and this package adds
// what belongs to MySQL itself — the registry entry, the defaults (port 3306,
// user root) and the status page built from SHOW GLOBAL STATUS.
package mysql

import (
	"context"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

const defaultPort = 3306

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
		SupportsDesign:   true,
		// MySQL keeps views next to tables in the catalog; there is no third
		// object kind the explorer would be honest about drawing.
		ObjectKinds:     []models.ObjectKind{models.KindTable, models.KindView},
		SortOrder:       10,
		DefaultDatabase: "",
		Notes:           "MySQL treats schemas as databases; the explorer shows a single level.",
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
		Introspector: mysqlcompat.Introspector{},
		DSN:          mysqlcompat.DSN,
		BootstrapDatabase: func(models.ConnectionConfig) string {
			// Connecting without a default schema is valid and lets the user
			// browse every database they can access.
			return ""
		},
		NativeDDL: mysqlcompat.NativeDDL,
		Overview:  overview,
		// The character sets and collations come from SHOW CHARACTER SET /
		// SHOW COLLATION on this server; see mysqlcompat/database.go.
		DatabaseOptions: mysqlcompat.DatabaseOptions,
		CreateDatabase:  mysqlcompat.CreateDatabase,
	}
}
