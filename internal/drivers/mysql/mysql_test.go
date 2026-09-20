package mysql

import (
	"strings"
	"testing"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// MySQL is the engine the shared MySQL-family code was written for, so this
// pins the defaults the connection dialog pre-fills and the DSN the driver
// hands to go-sql-driver.

func TestInfoAdvertisesMySQLEngine(t *testing.T) {
	info := Driver{}.Info()
	if info.Type != models.DriverMySQL {
		t.Fatalf("Info().Type = %q, want %q", info.Type, models.DriverMySQL)
	}
	if info.DefaultPort != 3306 {
		t.Errorf("Info().DefaultPort = %d, want 3306", info.DefaultPort)
	}
	if !info.Relational || !info.SupportsDatabase || !info.SupportsDesign {
		t.Errorf("mysql must be a designable relational engine: %+v", info)
	}
}

func TestNormalizeFillsMySQLDefaults(t *testing.T) {
	cfg := models.ConnectionConfig{}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 3306 || cfg.Username != "root" {
		t.Fatalf("Normalize() = %+v, want host 127.0.0.1 port 3306 user root", cfg)
	}
	if cfg.SSL.Mode != models.SSLDisable {
		t.Errorf("Normalize() SSL.Mode = %q, want %q", cfg.SSL.Mode, models.SSLDisable)
	}
}

func TestNormalizeKeepsExplicitValues(t *testing.T) {
	cfg := models.ConnectionConfig{Host: "db.internal", Port: 3307, Username: "reader"}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.Host != "db.internal" || cfg.Port != 3307 || cfg.Username != "reader" {
		t.Fatalf("Normalize() overwrote explicit values: %+v", cfg)
	}
}

func TestNormalizeRejectsImpossiblePort(t *testing.T) {
	cfg := models.ConnectionConfig{Port: 70000}
	if err := (Driver{}).Normalize(&cfg); err == nil {
		t.Fatal("Normalize() accepted a port above 65535")
	}
}

func TestSpecWiresTheMySQLFamily(t *testing.T) {
	spec := spec()
	if spec.SQLDriver != "mysql" {
		t.Errorf("spec().SQLDriver = %q, want mysql", spec.SQLDriver)
	}
	if spec.Dialect.Name() != models.DriverMySQL {
		t.Errorf("spec().Dialect.Name() = %q, want %q", spec.Dialect.Name(), models.DriverMySQL)
	}
	if spec.Overview == nil || spec.NativeDDL == nil || spec.DSN == nil {
		t.Error("spec() left a hook nil")
	}

	dsn, err := spec.DSN(models.ConnectionConfig{
		Host: "127.0.0.1", Port: 3306, Username: "root", Password: "s3cret",
	}, "book")
	if err != nil {
		t.Fatalf("spec().DSN() error = %v", err)
	}
	for _, want := range []string{"root:s3cret@tcp(127.0.0.1:3306)/book", "charset=utf8mb4", "tls=false"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN = %q, want it to contain %q", dsn, want)
		}
	}
}

// The connection dialog offers the MySQL table designer through the same plan
// as the SQLite and PostgreSQL ones, and TiDB shares the MySQL plan, so the
// dialect must report itself as TiDB while the statements stay MySQL.
func TestMySQLDialectReportsTheEngineItWasBuiltFor(t *testing.T) {
	if name := (sqlbase.MySQLDialect{}).Name(); name != models.DriverMySQL {
		t.Errorf("MySQLDialect{}.Name() = %q, want %q", name, models.DriverMySQL)
	}
	tidb := sqlbase.MySQLDialect{Driver: models.DriverTiDB}
	if tidb.Name() != models.DriverTiDB {
		t.Errorf("MySQLDialect{Driver: tidb}.Name() = %q", tidb.Name())
	}
	if got := tidb.Quote("order"); got != "`order`" {
		t.Errorf("MySQLDialect.Quote() = %q, want backticks", got)
	}
}
