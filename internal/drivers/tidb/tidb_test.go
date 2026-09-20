package tidb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqltest"
	"dbmanager/internal/models"
)

// TiDB is MySQL on the wire and in the catalog, so the tests here pin the parts
// that are TiDB: the defaults, the schemas it hides, and the cluster section of
// the status page.

func TestInfoAdvertisesTiDB(t *testing.T) {
	info := Driver{}.Info()
	if info.Type != models.DriverTiDB {
		t.Fatalf("Info().Type = %q, want %q", info.Type, models.DriverTiDB)
	}
	if info.DefaultPort != 4000 {
		t.Errorf("Info().DefaultPort = %d, want 4000", info.DefaultPort)
	}
	if !info.Relational || !info.SupportsDatabase || !info.SupportsDesign {
		t.Errorf("tidb should be a designable relational engine: %+v", info)
	}
}

func TestNormalizeFillsTiDBDefaults(t *testing.T) {
	cfg := models.ConnectionConfig{}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 4000 || cfg.Username != "root" {
		t.Fatalf("Normalize() = %+v, want host 127.0.0.1 port 4000 user root", cfg)
	}
	if cfg.SSL.Mode != models.SSLDisable {
		t.Errorf("Normalize() SSL.Mode = %q, want %q", cfg.SSL.Mode, models.SSLDisable)
	}
}

func TestSpecReusesTheMySQLMachinery(t *testing.T) {
	spec := spec()
	if spec.SQLDriver != "mysql" {
		t.Errorf("spec().SQLDriver = %q, want the MySQL driver", spec.SQLDriver)
	}
	// The dialect keeps saying TiDB so that generated DDL (and the designer's
	// warnings) name the engine the user connected to.
	if spec.Dialect.Name() != models.DriverTiDB {
		t.Errorf("spec().Dialect.Name() = %q, want %q", spec.Dialect.Name(), models.DriverTiDB)
	}
	if spec.Overview == nil || spec.NativeDDL == nil {
		t.Error("spec() left a hook nil")
	}

	dsn, err := spec.DSN(models.ConnectionConfig{Host: "tidb.internal", Port: 4000, Username: "root"}, "")
	if err != nil {
		t.Fatalf("spec().DSN() error = %v", err)
	}
	if !strings.Contains(dsn, "root@tcp(tidb.internal:4000)/") {
		t.Errorf("DSN = %q, want the TiDB address", dsn)
	}
	if strings.Contains(dsn, "interpolateParams") {
		t.Errorf("DSN = %q, want server-side prepared statements, as for MySQL", dsn)
	}
}

func TestIntrospectorHidesTheTiDBSchemas(t *testing.T) {
	reader, ok := spec().Introspector.(mysqlcompat.Introspector)
	if !ok {
		t.Fatalf("spec().Introspector = %T, want the shared MySQL reader", spec().Introspector)
	}
	in := []string{"information_schema", "metrics_schema", "mysql", "performance_schema", "sys", "test"}
	got := reader.FilterDatabases(in)
	if len(got) != 1 || got[0] != "test" {
		t.Fatalf("FilterDatabases(%v) = %v, want [test]", in, got)
	}
}

func TestOverviewAddsTheClusterSection(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW GLOBAL STATUS",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{{"Uptime", "3600"}, {"Threads_connected", "4"}, {"Questions", "900"}},
		},
		sqltest.Expectation{
			Match:   "SHOW GLOBAL VARIABLES",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{{"version_comment", "TiDB Server (Apache License 2.0)"}, {"max_connections", "0"}},
		},
		sqltest.Expectation{
			Match:   "SHOW FULL PROCESSLIST",
			Columns: []string{"Id", "User", "Command"},
			Rows:    [][]any{{"1", "root", "Query"}},
		},
		sqltest.Expectation{
			Match:   "SELECT TYPE, INSTANCE, STATUS, VERSION FROM information_schema.CLUSTER_INFO",
			Columns: []string{"TYPE", "INSTANCE", "STATUS", "VERSION"},
			Rows: [][]any{
				{"tidb", "10.0.0.1:4000", "Up", "8.1.0"},
				{"tikv", "10.0.0.11:20160", "Up", "8.1.0"},
				{"tikv", "10.0.0.12:20160", "Up", "8.1.0"},
				{"pd", "10.0.0.21:2379", "Down", "8.1.0"},
			},
		},
	)

	page, err := overview(context.Background(), handle.DB, models.ConnectionConfig{})
	if err != nil {
		t.Fatalf("overview() error = %v", err)
	}
	titles := groupTitles(page)
	for _, want := range []string{"Server", "Connections", "Queries", "Cluster"} {
		if !containsString(titles, want) {
			t.Errorf("groups = %v, want %q among them", titles, want)
		}
	}
	// TiDB stores rows in TiKV: an InnoDB buffer pool group would be zeros.
	if containsString(titles, "InnoDB and caches") {
		t.Error("tidb has no InnoDB buffer pool: the group should not be there")
	}
	for label, value := range map[string]string{"TiKV instances": "2", "TiDB instances": "1", "PD instances": "1"} {
		if !hasMetric(page, label, value) {
			t.Errorf("cluster group = %+v, want %s %s", group(page, "Cluster").Metrics, label, value)
		}
	}
	if !hasWarn(page, "Not Up") {
		t.Errorf("cluster group = %+v, want the down instance flagged", group(page, "Cluster").Metrics)
	}
}

// Reading the topology needs a privilege; without it the rest of the page still
// renders and says what was missing.
func TestOverviewWithoutClusterInfo(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW GLOBAL STATUS",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{{"Uptime", "60"}},
		},
		sqltest.Expectation{
			Match:   "SHOW GLOBAL VARIABLES",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{},
		},
		sqltest.Expectation{Match: "SHOW FULL PROCESSLIST", Err: errors.New("Access denied")},
		sqltest.Expectation{
			Match: "SELECT TYPE, INSTANCE, STATUS, VERSION FROM information_schema.CLUSTER_INFO",
			Err:   errors.New("SELECT command denied to user 'app'"),
		},
	)

	page, err := overview(context.Background(), handle.DB, models.ConnectionConfig{})
	if err != nil {
		t.Fatalf("overview() error = %v", err)
	}
	if group(page, "Cluster") == nil {
		t.Fatalf("groups = %v, want the cluster section even when it is empty", groupTitles(page))
	}
	if len(page.Warnings) != 2 {
		t.Errorf("warnings = %v, want the failed process list and cluster info reported", page.Warnings)
	}
}

// --- helpers ---------------------------------------------------------------

func groupTitles(page *models.ServerOverview) []string {
	titles := make([]string, 0, len(page.MySQL.Groups))
	for _, g := range page.MySQL.Groups {
		titles = append(titles, g.Title)
	}
	return titles
}

func group(page *models.ServerOverview, title string) *models.OverviewGroup {
	for i := range page.MySQL.Groups {
		if page.MySQL.Groups[i].Title == title {
			return &page.MySQL.Groups[i]
		}
	}
	return nil
}

func hasMetric(page *models.ServerOverview, label, value string) bool {
	for _, g := range page.MySQL.Groups {
		for _, metric := range g.Metrics {
			if metric.Label == label && metric.Value == value {
				return true
			}
		}
	}
	return false
}

func hasWarn(page *models.ServerOverview, label string) bool {
	for _, g := range page.MySQL.Groups {
		for _, metric := range g.Metrics {
			if metric.Label == label && metric.State == "warn" {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
