package doris

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

// The tests in this file talk to a real Doris cluster. They are skipped unless
// DMB_TEST_DORIS_HOST is set, so `go test ./...` stays green without one:
//
//	DMB_TEST_DORIS_HOST=127.0.0.1 DMB_TEST_DORIS_PORT=9030 DMB_TEST_DORIS_USERNAME=root \
//	go test ./internal/drivers/doris/ -run Integration -v
//
// The user needs to create and drop databases; everything the tests create is
// dropped again.

func testConfig(t *testing.T) models.ConnectionConfig {
	t.Helper()
	host := os.Getenv("DMB_TEST_DORIS_HOST")
	if host == "" {
		t.Skip("set DMB_TEST_DORIS_HOST to run the Doris integration tests")
	}
	port := defaultPort
	if raw := os.Getenv("DMB_TEST_DORIS_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("bad DMB_TEST_DORIS_PORT %q: %v", raw, err)
		}
		port = parsed
	}
	return models.ConnectionConfig{
		Name:     "integration",
		Driver:   models.DriverDoris,
		Host:     host,
		Port:     port,
		Username: os.Getenv("DMB_TEST_DORIS_USERNAME"),
		Password: os.Getenv("DMB_TEST_DORIS_PASSWORD"),
	}
}

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

// scratchTable creates a duplicate-key table — the shape Doris wants — with one
// comment and a distribution clause, and returns its name.
func scratchTable(t *testing.T, conn drivers.Conn, database string) string {
	t.Helper()
	ctx := context.Background()
	script := fmt.Sprintf(`CREATE TABLE %s.orders (
		id BIGINT NOT NULL COMMENT 'order id',
		title VARCHAR(120) NULL COMMENT 'what the customer sees',
		amount DECIMAL(10,2) NULL DEFAULT "0.00"
	) ENGINE=OLAP
	DUPLICATE KEY(id)
	COMMENT 'one row per order'
	DISTRIBUTED BY HASH(id) BUCKETS 1
	PROPERTIES ("replication_allocation" = "tag.location.default: 1")`, database)
	if _, err := conn.Execute(ctx, drivers.ExecRequest{SQL: script}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	insert := fmt.Sprintf(`INSERT INTO %s.orders (id, title, amount) VALUES (1, 'A-1', 12.50), (2, 'A-2', 3.00)`, database)
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

	// SHOW DATABASES is the listing Doris documents; the introspector reads the
	// first column of it.
	names, err := conn.Databases(ctx)
	if err != nil {
		t.Fatalf("databases: %v", err)
	}
	found := false
	for _, name := range names {
		switch name {
		case "information_schema", "__internal_schema", "mysql":
			t.Errorf("%s is a system database and should be hidden", name)
		}
		if name == database {
			found = true
		}
	}
	if !found {
		t.Errorf("%s is missing from %v", database, names)
	}

	if schemas, err := conn.Schemas(ctx, database); err != nil || len(schemas) != 0 {
		t.Errorf("schemas = %v / %v, want an empty list: Doris has no schema layer", schemas, err)
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
	if objects[0].Kind != models.KindTable {
		t.Errorf("kind = %q, want a table", objects[0].Kind)
	}
	// SHOW FULL TABLES has no comment; the introspector enriches it from
	// information_schema when the version there has the column.
	if objects[0].Comment != "" && objects[0].Comment != "one row per order" {
		t.Errorf("comment = %q, want the table comment or nothing", objects[0].Comment)
	}

	structure, err := conn.Structure(ctx, database, "", table)
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	columns := map[string]models.ColumnInfo{}
	for _, column := range structure.Columns {
		columns[column.Name] = column
	}
	if len(columns) != 3 {
		t.Fatalf("columns = %+v, want three", columns)
	}
	if id := columns["id"]; id.DataType != "bigint" || !id.PrimaryKey || id.Comment != "order id" {
		t.Errorf("id = %+v, want a bigint key column with its comment", id)
	}
	if title := columns["title"]; !title.Nullable || title.DataType != "varchar" || title.CharMaxLength == nil || *title.CharMaxLength != 120 {
		t.Errorf("title = %+v, want a nullable varchar(120)", title)
	}
	if amount := columns["amount"]; amount.NumericPrecision == nil || *amount.NumericPrecision != 10 {
		t.Errorf("amount = %+v, want decimal(10,2)", amount)
	}

	// Doris has no foreign keys; saying so is part of the contract.
	if len(structure.ForeignKeys) != 0 {
		t.Errorf("foreign keys = %+v, want none", structure.ForeignKeys)
	}

	// SHOW CREATE TABLE is what the DDL tab shows, and it is what a user would
	// edit by hand because the designer is off for Doris.
	if structure.DDL == "" || !strings.Contains(strings.ToUpper(structure.DDL), "CREATE TABLE") {
		t.Errorf("ddl = %q, want Doris' own CREATE TABLE", structure.DDL)
	}

	entries, err := conn.Indexes(ctx, database, "")
	if err != nil {
		t.Fatalf("indexes: %v", err)
	}
	// A duplicate-key table reports its key as the primary index.
	for _, entry := range entries {
		if entry.Table != table {
			t.Errorf("index %+v belongs to another table", entry)
		}
	}
}

func TestIntegrationFetchAndExecute(t *testing.T) {
	conn, _, database := testConn(t)
	table := scratchTable(t, conn, database)
	ctx := context.Background()

	result, err := conn.Fetch(ctx, drivers.FetchRequest{Database: database, Object: table, Limit: 10})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("fetched %d rows, want 2", len(result.Rows))
	}

	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf("UPDATE %s.orders SET amount = 99.00 WHERE id = 1", database),
	}); err != nil {
		t.Fatalf("update: %v", err)
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
		t.Fatalf("overview = %+v, want the Doris groups", page)
	}
	titles := make([]string, 0, len(page.MySQL.Groups))
	for _, group := range page.MySQL.Groups {
		titles = append(titles, group.Title)
		if group.Title == "InnoDB and caches" {
			t.Error("Doris has no InnoDB buffer pool: the group should not be there")
		}
	}
	for _, want := range []string{"Cluster", "Storage"} {
		found := false
		for _, title := range titles {
			if title == want {
				found = true
			}
		}
		if !found {
			t.Errorf("groups = %v, want %q among them", titles, want)
		}
	}
}
