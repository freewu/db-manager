package tidb

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// The tests in this file talk to a real TiDB cluster. They are skipped unless
// DMB_TEST_TIDB_HOST is set, so `go test ./...` stays green without one:
//
//	DMB_TEST_TIDB_HOST=127.0.0.1 DMB_TEST_TIDB_PORT=4000 DMB_TEST_TIDB_USERNAME=root \
//	go test ./internal/drivers/tidb/ -run Integration -v
//
// The user needs to create and drop databases; everything the tests create is
// dropped again.

func testConfig(t *testing.T) models.ConnectionConfig {
	t.Helper()
	host := os.Getenv("DMB_TEST_TIDB_HOST")
	if host == "" {
		t.Skip("set DMB_TEST_TIDB_HOST to run the TiDB integration tests")
	}
	port := defaultPort
	if raw := os.Getenv("DMB_TEST_TIDB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("bad DMB_TEST_TIDB_PORT %q: %v", raw, err)
		}
		port = parsed
	}
	return models.ConnectionConfig{
		Name:     "integration",
		Driver:   models.DriverTiDB,
		Host:     host,
		Port:     port,
		Username: os.Getenv("DMB_TEST_TIDB_USERNAME"),
		Password: os.Getenv("DMB_TEST_TIDB_PASSWORD"),
	}
}

// testConn opens a session and drops the scratch database afterwards, so a
// failed run leaves nothing behind.
func testConn(t *testing.T) (drivers.Conn, models.ConnectionConfig, string) {
	t.Helper()
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opened, err := (Driver{}).Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })

	database := fmt.Sprintf("dmb_test_%d", time.Now().UnixNano())
	if _, err := opened.Execute(ctx, drivers.ExecRequest{SQL: "CREATE DATABASE " + database}); err != nil {
		t.Fatalf("create %s: %v", database, err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := opened.Execute(cleanup, drivers.ExecRequest{SQL: "DROP DATABASE IF EXISTS " + database}); err != nil {
			t.Logf("cleanup: drop %s: %v", database, err)
		}
	})
	return opened, cfg, database
}

// scratchTable creates one table and returns its name.
func scratchTable(t *testing.T, conn drivers.Conn, database string) string {
	t.Helper()
	ctx := context.Background()
	script := fmt.Sprintf(`CREATE TABLE %s.orders (
		id BIGINT NOT NULL AUTO_INCREMENT,
		title VARCHAR(120) NULL COMMENT 'what the customer sees',
		amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY idx_title (title)
	) COMMENT 'one row per order'`, database)
	if _, err := conn.Execute(ctx, drivers.ExecRequest{SQL: script}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	insert := fmt.Sprintf(`INSERT INTO %s.orders (title, amount) VALUES ('A-1', 12.50), ('A-2', 3.00), (NULL, 0.00)`, database)
	if _, err := conn.Execute(ctx, drivers.ExecRequest{SQL: insert}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return "orders"
}

func TestIntegrationServerInfo(t *testing.T) {
	conn, _, database := testConn(t)
	ctx := context.Background()

	if err := conn.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	version, err := conn.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version == "" {
		t.Error("the server reported no version")
	}
	t.Logf("server version: %s", version)

	names, err := conn.Databases(ctx)
	if err != nil {
		t.Fatalf("databases: %v", err)
	}
	found := false
	for _, name := range names {
		switch name {
		case "information_schema", "mysql", "performance_schema", "metrics_schema":
			t.Errorf("%s is a system schema and should be hidden", name)
		}
		if name == database {
			found = true
		}
	}
	if !found {
		t.Errorf("%s is missing from %v", database, names)
	}

	if schemas, err := conn.Schemas(ctx, database); err != nil || len(schemas) != 0 {
		t.Errorf("schemas = %v / %v, want an empty list: TiDB has no separate schema layer", schemas, err)
	}
}

func TestIntegrationStructureAndIndexes(t *testing.T) {
	conn, _, database := testConn(t)
	table := scratchTable(t, conn, database)
	ctx := context.Background()

	objects, err := conn.Objects(ctx, database, "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != table {
		t.Fatalf("objects = %+v, want one %s", objects, table)
	}
	if objects[0].Comment != "one row per order" {
		t.Errorf("comment = %q, want the table comment", objects[0].Comment)
	}
	if objects[0].Kind != models.KindTable {
		t.Errorf("kind = %q, want a table", objects[0].Kind)
	}

	structure, err := conn.Structure(ctx, database, "", table)
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	columns := map[string]models.ColumnInfo{}
	for _, column := range structure.Columns {
		columns[column.Name] = column
	}
	if len(columns) != 4 {
		t.Fatalf("columns = %+v, want four", columns)
	}
	if id := columns["id"]; !id.PrimaryKey || !id.AutoIncrement || id.Nullable {
		t.Errorf("id = %+v, want a non-null auto-increment primary key", id)
	}
	if title := columns["title"]; !title.Nullable || title.Comment != "what the customer sees" {
		t.Errorf("title = %+v, want a nullable commented column", title)
	}
	if amount := columns["amount"]; amount.DefaultValue == nil || !strings.HasPrefix(*amount.DefaultValue, "0.00") {
		t.Errorf("amount = %+v, want the default TiDB reported", amount)
	}

	indexes := map[string]models.IndexInfo{}
	for _, index := range structure.Indexes {
		indexes[index.Name] = index
	}
	if pk, ok := indexes["PRIMARY"]; !ok || !pk.Unique || len(pk.Columns) != 1 || pk.Columns[0] != "id" {
		t.Errorf("PRIMARY = %+v, want a unique index on id", indexes["PRIMARY"])
	}
	if ix, ok := indexes["idx_title"]; !ok || !ix.Unique || ix.Columns[0] != "title" {
		t.Errorf("idx_title = %+v, want a unique index on title", indexes["idx_title"])
	}

	if structure.DDL == "" || !strings.Contains(strings.ToUpper(structure.DDL), "CREATE TABLE") {
		t.Errorf("ddl = %q, want TiDB's own CREATE TABLE", structure.DDL)
	}

	entries, err := conn.Indexes(ctx, database, "")
	if err != nil {
		t.Fatalf("indexes: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("indexes = %+v, want the two indexes of %s", entries, table)
	}
}

func TestIntegrationFetchAndExecute(t *testing.T) {
	conn, _, database := testConn(t)
	table := scratchTable(t, conn, database)
	ctx := context.Background()

	result, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database,
		Object:   table,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("fetched %d rows, want 3", len(result.Rows))
	}

	// The designer runs its own queries through Execute, so a plain UPDATE has
	// to work as well.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf("UPDATE %s.orders SET amount = 99.00 WHERE title = 'A-1'", database),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := conn.Fetch(ctx, drivers.FetchRequest{Database: database, Object: table, Limit: 10})
	if err != nil {
		t.Fatalf("fetch after update: %v", err)
	}
	found := false
	for _, row := range updated.Rows {
		for _, cell := range row {
			if fmt.Sprint(cell) == "99.00" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("rows = %+v, want the updated amount", updated.Rows)
	}
}

// overviewer is the optional Conn capability the status page needs.
type overviewer interface {
	Overview(ctx context.Context) (*models.ServerOverview, error)
}

func TestIntegrationOverview(t *testing.T) {
	conn, _, _ := testConn(t)
	ctx := context.Background()

	page, err := conn.(overviewer).Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if page.MySQL == nil || len(page.MySQL.Groups) == 0 {
		t.Fatalf("overview = %+v, want the MySQL-compatible groups", page)
	}
	for _, group := range page.MySQL.Groups {
		if group.Title == "InnoDB and caches" {
			t.Error("TiDB has no InnoDB buffer pool: the group should not be there")
		}
	}
	// CLUSTER_INFO needs the PROCESS privilege; when it is missing the page
	// says so instead of failing.
	if page.MySQL.Processes == nil && len(page.Warnings) == 0 {
		t.Error("either the process list or a warning about it is expected")
	}
}
