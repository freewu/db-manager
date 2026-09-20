package doris

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dbmanager/internal/drivers/sqltest"
	"dbmanager/internal/models"
)

// Doris shares the MySQL DSN builder, so the only thing worth pinning there is
// what Doris changes: the port it suggests, the user, and the client-side
// parameter expansion its front end needs.

func TestInfoAdvertisesDoris(t *testing.T) {
	info := Driver{}.Info()
	if info.Type != models.DriverDoris {
		t.Fatalf("Info().Type = %q, want %q", info.Type, models.DriverDoris)
	}
	if info.DefaultPort != 9030 {
		t.Errorf("Info().DefaultPort = %d, want 9030", info.DefaultPort)
	}
	if !info.Relational || !info.SupportsDatabase {
		t.Errorf("doris is a relational engine with databases: %+v", info)
	}
	// A Doris table needs a data model and a distribution clause, so the
	// designer is off while the DDL editor stays available.
	if info.SupportsDesign {
		t.Errorf("doris must not advertise the table designer: %+v", info)
	}
	if info.Notes == "" {
		t.Error("Info().Notes is empty: the UI shows it to explain the read-only designer")
	}
}

func TestNormalizeFillsDorisDefaults(t *testing.T) {
	cfg := models.ConnectionConfig{}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 9030 || cfg.Username != "root" {
		t.Fatalf("Normalize() = %+v, want host 127.0.0.1 port 9030 user root", cfg)
	}
	if err := (Driver{}).Normalize(&models.ConnectionConfig{Port: 0xffff + 1}); err == nil {
		t.Fatal("Normalize() accepted a port above 65535")
	}
}

func TestSpecUsesTheMySQLEngineWithDorisNames(t *testing.T) {
	spec := spec()
	if spec.SQLDriver != "mysql" {
		t.Errorf("spec().SQLDriver = %q, want the MySQL driver", spec.SQLDriver)
	}
	if spec.Dialect.Name() != models.DriverDoris {
		t.Errorf("spec().Dialect.Name() = %q, want %q", spec.Dialect.Name(), models.DriverDoris)
	}
	if spec.Overview == nil || spec.NativeDDL == nil {
		t.Error("spec() left a hook nil")
	}

	dsn, err := spec.DSN(models.ConnectionConfig{Host: "fe.internal", Port: 9030, Username: "root"}, "")
	if err != nil {
		t.Fatalf("spec().DSN() error = %v", err)
	}
	for _, want := range []string{"root@tcp(fe.internal:9030)/", "interpolateParams=true"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN = %q, want it to contain %q", dsn, want)
		}
	}
}

// The status page has two halves that can fail on their own: the MySQL
// variables Doris answers with a subset of, and the Doris statements. Neither
// may take the other down.
func TestOverviewAddsTheDorisSections(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW GLOBAL STATUS",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{{"Uptime", "120"}, {"Threads_connected", "3"}, {"Questions", "10"}},
		},
		sqltest.Expectation{
			Match:   "SHOW GLOBAL VARIABLES",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{{"version", "5.7.99 Doris"}, {"max_connections", "100"}},
		},
		sqltest.Expectation{
			Match:   "SHOW FULL PROCESSLIST",
			Columns: []string{"Id", "User", "Command"},
			Rows:    [][]any{{"1", "root", "Query"}},
		},
		sqltest.Expectation{
			Match:   "SHOW FRONTENDS",
			Columns: []string{"Name", "IP", "Alive", "Join", "Version"},
			Rows: [][]any{
				{"fe-1", "10.0.0.1", "true", "true", "2.1.0"},
				{"fe-2", "10.0.0.2", "false", "true", "2.1.0"},
			},
		},
		sqltest.Expectation{
			Match:   "SHOW BACKENDS",
			Columns: []string{"BackendId", "IP", "Alive", "Version"},
			Rows:    [][]any{{"10001", "10.0.0.11", "true", "2.1.0"}},
		},
		sqltest.Expectation{
			Match:   "SELECT COUNT(*), IFNULL(SUM(TABLE_ROWS), 0), IFNULL(SUM(DATA_LENGTH), 0), IFNULL(SUM(INDEX_LENGTH), 0) FROM information_schema.TABLES",
			Columns: []string{"count", "rows", "data", "index"},
			Rows:    [][]any{{12, 4000, 1024, 512}},
		},
	)

	page, err := overview(context.Background(), handle.DB, models.ConnectionConfig{})
	if err != nil {
		t.Fatalf("overview() error = %v", err)
	}
	titles := groupTitles(page)
	for _, want := range []string{"Server", "Connections", "Queries", "Cluster", "Storage"} {
		if !containsString(titles, want) {
			t.Errorf("groups = %v, want %q among them", titles, want)
		}
	}
	if containsString(titles, "InnoDB and caches") {
		t.Error("doris has no InnoDB buffer pool: the group should not be there")
	}
	if page.MySQL.Processes == nil || len(page.MySQL.Processes.Rows) != 1 {
		t.Errorf("process list = %+v, want one session", page.MySQL.Processes)
	}
	if !hasMetric(page, "Front end instances", "2") {
		t.Errorf("cluster group = %+v, want two front ends", group(page, "Cluster"))
	}
	if !hasMetric(page, "Back end instances", "1") {
		t.Errorf("cluster group = %+v, want one back end", group(page, "Cluster"))
	}
	if !hasMetric(page, "Tables", "12") || !hasMetric(page, "Rows", "4,000") {
		t.Errorf("storage group = %+v, want the table count and row count", group(page, "Storage"))
	}
}

// A front end that is not alive is the one number on this page worth colouring.
func TestOverviewFlagsADeadFrontEnd(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW GLOBAL STATUS",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{},
		},
		sqltest.Expectation{
			Match:   "SHOW GLOBAL VARIABLES",
			Columns: []string{"Variable_name", "Value"},
			Rows:    [][]any{},
		},
		sqltest.Expectation{
			Match:   "SHOW FRONTENDS",
			Columns: []string{"Name", "Alive", "Join", "Version"},
			Rows:    [][]any{{"fe-1", "false", "true", "2.1.0"}},
		},
		sqltest.Expectation{Match: "SHOW BACKENDS", Err: errors.New("not readable")},
		sqltest.Expectation{
			Match:   "SELECT COUNT(*)",
			Columns: []string{"count", "rows", "data", "index"},
			Rows:    [][]any{{0, 0, 0, 0}},
		},
	)

	page, err := overview(context.Background(), handle.DB, models.ConnectionConfig{})
	if err != nil {
		t.Fatalf("overview() error = %v", err)
	}
	cluster := group(page, "Cluster")
	if cluster == nil {
		t.Fatal("the cluster group is missing")
	}
	found := false
	for _, metric := range cluster.Metrics {
		if metric.Label == "Front ends not up" {
			found = true
			if metric.State != "warn" {
				t.Errorf("metric = %+v, want a warning state", metric)
			}
		}
	}
	if !found {
		t.Errorf("cluster metrics = %+v, want the dead front end reported", cluster.Metrics)
	}
	if len(page.Warnings) == 0 {
		t.Error("a failing SHOW BACKENDS should be reported as a warning")
	}
}

// Some Doris builds do not answer the MySQL status variables at all. The Doris
// half of the page must still be there, with a note saying what was missing.
func TestOverviewSurvivesWithoutTheMySQLVariables(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{Match: "SHOW GLOBAL STATUS", Err: errors.New("unsupported statement")},
		sqltest.Expectation{
			Match:   "SHOW FRONTENDS",
			Columns: []string{"Name", "Alive", "Version"},
			Rows:    [][]any{{"fe-1", "true", "2.1.0"}},
		},
		sqltest.Expectation{
			Match:   "SHOW BACKENDS",
			Columns: []string{"BackendId", "Alive", "Version"},
			Rows:    [][]any{{"10001", "true", "2.1.0"}},
		},
		sqltest.Expectation{
			Match:   "SELECT COUNT(*)",
			Columns: []string{"count", "rows", "data", "index"},
			Rows:    [][]any{{3, 30, 64, 16}},
		},
	)

	page, err := overview(context.Background(), handle.DB, models.ConnectionConfig{})
	if err != nil {
		t.Fatalf("overview() error = %v", err)
	}
	if group(page, "Cluster") == nil || group(page, "Storage") == nil {
		t.Fatalf("groups = %v, want the Doris sections", groupTitles(page))
	}
	if len(page.Warnings) == 0 {
		t.Error("the unreadable status variables should be reported as a warning")
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
