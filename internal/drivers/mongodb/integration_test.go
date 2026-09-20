package mongodb

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

// The tests in this file talk to a real server. They are skipped unless
// DMB_TEST_MONGODB_HOST is set, so `go test ./...` stays green on a machine
// without MongoDB:
//
//	DMB_TEST_MONGODB_HOST=127.0.0.1 \
//	DMB_TEST_MONGODB_PORT=27017 \
//	go test ./internal/drivers/mongodb/ -run Integration -v
//
// The server needs nothing but an empty database to work in: every test
// creates its own and drops it again.

func testConfig(t *testing.T) models.ConnectionConfig {
	t.Helper()
	host := os.Getenv("DMB_TEST_MONGODB_HOST")
	if host == "" {
		t.Skip("set DMB_TEST_MONGODB_HOST to run the MongoDB integration tests")
	}
	port := defaultPort
	if raw := os.Getenv("DMB_TEST_MONGODB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("bad DMB_TEST_MONGODB_PORT %q: %v", raw, err)
		}
		port = parsed
	}
	return models.ConnectionConfig{
		Name:     "integration",
		Driver:   models.DriverMongoDB,
		Host:     host,
		Port:     port,
		Username: os.Getenv("DMB_TEST_MONGODB_USERNAME"),
		Password: os.Getenv("DMB_TEST_MONGODB_PASSWORD"),
		Database: os.Getenv("DMB_TEST_MONGODB_DATABASE"),
	}
}

// testConn opens a session and drops the scratch database afterwards, so a
// failed run does not leave anything behind.
func testConn(t *testing.T) (*Conn, models.ConnectionConfig) {
	t.Helper()
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opened, err := (Driver{}).Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	conn := opened.(*Conn)
	t.Cleanup(func() { _ = conn.Close() })
	return conn, cfg
}

// scratchDatabase creates a database with one collection and returns both
// names.
func scratchDatabase(t *testing.T, conn *Conn) (string, string) {
	t.Helper()
	database := fmt.Sprintf("dmb_test_%d", time.Now().UnixNano())
	collection := "orders"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: "db.dropDatabase()"}); err != nil {
			t.Logf("cleanup: drop %s: %v", database, err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script := fmt.Sprintf(`db.%s.insertMany([
		{"sku": "A-1", "qty": 2, "state": "paid", "address": {"city": "Shanghai"}, "tags": ["new"]},
		{"sku": "A-2", "qty": 5, "state": "open", "note": null},
		{"sku": "B-1", "qty": 1, "state": "paid"}
	])`, collection)
	if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: script}); err != nil {
		t.Fatalf("seed %s: %v", database, err)
	}
	return database, collection
}

func TestIntegrationServerInfo(t *testing.T) {
	conn, cfg := testConn(t)
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

	current, err := conn.CurrentDatabase(ctx)
	if err != nil {
		t.Fatalf("current database: %v", err)
	}
	if cfg.Database != "" && current != cfg.Database {
		t.Errorf("current database = %q, want the profile's %q", current, cfg.Database)
	}

	names, err := conn.Databases(ctx)
	if err != nil {
		t.Fatalf("databases: %v", err)
	}
	if len(names) == 0 {
		t.Error("no databases reported")
	}
	for _, name := range names {
		if name == "config" || name == "local" {
			t.Errorf("%s is an internal database and should be hidden", name)
		}
	}

	if schemas, err := conn.Schemas(ctx, "admin"); err != nil || len(schemas) != 0 {
		t.Errorf("schemas = %v / %v, want an empty list", schemas, err)
	}
}

func TestIntegrationCollectionsAndStructure(t *testing.T) {
	conn, _ := testConn(t)
	database, collection := scratchDatabase(t, conn)
	ctx := context.Background()

	objects, err := conn.Objects(ctx, database, "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != collection {
		t.Fatalf("objects = %+v, want one %s", objects, collection)
	}
	if objects[0].RowEstimate != 3 {
		t.Errorf("row estimate = %d, want 3", objects[0].RowEstimate)
	}

	structure, err := conn.Structure(ctx, database, "", collection)
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	if structure.Object.Name != collection {
		t.Errorf("object = %+v", structure.Object)
	}
	columns := map[string]models.ColumnInfo{}
	for _, column := range structure.Columns {
		columns[column.Name] = column
	}
	if columns[idField].DataType != "objectId" || !columns[idField].PrimaryKey {
		t.Errorf("_id = %+v", columns[idField])
	}
	if columns["qty"].DataType != "int32" {
		t.Errorf("qty = %+v", columns["qty"])
	}
	// Only two of the three documents have a note, and one of them is null.
	if !columns["note"].Nullable {
		t.Errorf("note = %+v, want nullable", columns["note"])
	}
	if columns["address"].DataType != "object" {
		t.Errorf("address = %+v", columns["address"])
	}

	if len(structure.Indexes) != 1 || !structure.Indexes[0].Primary {
		t.Errorf("indexes = %+v, want only _id_", structure.Indexes)
	}
	if len(structure.ForeignKeys) != 0 {
		t.Errorf("foreign keys = %+v, want none", structure.ForeignKeys)
	}
	if !strings.Contains(structure.DDL, "// Definition of "+database) {
		t.Errorf("ddl = %q", structure.DDL)
	}

	indexes, err := conn.Indexes(ctx, database, "")
	if err != nil {
		t.Fatalf("indexes: %v", err)
	}
	if len(indexes) != 1 || indexes[0].Table != collection {
		t.Errorf("indexes = %+v", indexes)
	}

	if _, err := conn.Structure(ctx, database, "", collection+"_missing"); err == nil {
		t.Error("a collection that does not exist must be an error")
	}
}

func TestIntegrationFetchAndMutate(t *testing.T) {
	conn, _ := testConn(t)
	database, collection := scratchDatabase(t, conn)
	ctx := context.Background()

	page, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database, Object: collection, Limit: 2, CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if page.RowCount != 2 || !page.HasTotal || page.Total != 3 {
		t.Fatalf("page = %+v", page)
	}
	if !page.Truncated {
		t.Error("a full page must be reported as truncated")
	}
	if !strings.Contains(page.SQL, "db.getCollection("+quoteText(collection)+")") {
		t.Errorf("the page must describe its query, got %q", page.SQL)
	}

	filtered, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database, Object: collection,
		Filters: []models.FilterSpec{
			{Column: "state", Operator: "eq", Value: "paid"},
			{Column: "qty", Operator: "gt", Value: "1"},
		},
	})
	if err != nil {
		t.Fatalf("filtered fetch: %v", err)
	}
	if filtered.RowCount != 1 {
		t.Fatalf("filtered rows = %d, want 1 (%+v)", filtered.RowCount, filtered.Rows)
	}

	regexed, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database, Object: collection,
		Filters: []models.FilterSpec{{Column: "sku", Operator: "startsWith", Value: "A-"}},
	})
	if err != nil {
		t.Fatalf("regex fetch: %v", err)
	}
	if regexed.RowCount != 2 {
		t.Errorf("rows for sku A-* = %d, want 2", regexed.RowCount)
	}

	// Update one cell, addressed the way the grid addresses a row.
	key := []models.KeyValue{{Column: idField, Value: idOf(t, page)}}
	modified, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: database, Object: collection, Key: key, Column: "state", Value: "shipped",
	})
	if err != nil {
		t.Fatalf("update cell: %v", err)
	}
	if modified != 1 {
		t.Fatalf("modified = %d, want 1", modified)
	}

	if _, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: database, Object: collection, Key: key, Column: idField, Value: "x",
	}); err == nil {
		t.Error("_id must not be editable")
	}
	if _, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: database, Object: collection, Column: "qty", Value: 1,
	}); err == nil {
		t.Error("a cell update without a row identity must be refused")
	}

	after, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database, Object: collection,
		Filters: []models.FilterSpec{{Column: "state", Operator: "eq", Value: "shipped"}},
	})
	if err != nil {
		t.Fatalf("fetch after update: %v", err)
	}
	if after.RowCount != 1 {
		t.Errorf("the update did not stick: %+v", after.Rows)
	}

	deleted, err := conn.DeleteRow(ctx, models.RowDelete{Database: database, Object: collection, Key: key})
	if err != nil {
		t.Fatalf("delete row: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
	if _, err := conn.DeleteRow(ctx, models.RowDelete{Database: database, Object: collection}); err == nil {
		t.Error("a delete without a row identity must be refused")
	}
}

// idOf reads the _id of the first row of a page, as the grid would send it
// back.
func idOf(t *testing.T, page *models.FetchResult) any {
	t.Helper()
	for i, column := range page.Columns {
		if column.Name == idField {
			return page.Rows[0][i]
		}
	}
	t.Fatalf("the page has no %s column: %+v", idField, page.Columns)
	return nil
}

func TestIntegrationExecute(t *testing.T) {
	conn, _ := testConn(t)
	database, collection := scratchDatabase(t, conn)
	ctx := context.Background()

	// A query returns rows and reports how long it took.
	result, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.find({"state": "paid"}).sort({"qty": -1})`, collection),
	})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !result.HasResultSet || result.RowCount != 2 {
		t.Fatalf("find result = %+v", result)
	}
	if result.StatementCount != 1 || result.StatementIndex != 0 {
		t.Errorf("statement bookkeeping = %+v", result)
	}
	if len(result.Messages) == 0 {
		t.Error("the run should report what it did")
	}

	// Several statements in one script: the last result set wins, and every
	// statement is reported.
	multi, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf("db.%s.countDocuments({});\ndb.%s.distinct(\"state\")", collection, collection),
	})
	if err != nil {
		t.Fatalf("multi statement: %v", err)
	}
	if multi.StatementCount != 2 || len(multi.Messages) != 2 {
		t.Errorf("multi statement result = %+v", multi)
	}
	if multi.RowCount != 2 {
		t.Errorf("distinct states = %d rows, want 2", multi.RowCount)
	}

	// Aggregation, count and the show shorthands.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL: fmt.Sprintf(`db.%s.aggregate([{"$group": {"_id": "$state", "n": {"$sum": 1}}}, {"$sort": {"_id": 1}}])`,
			collection),
	}); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: "show collections"}); err != nil {
		t.Fatalf("show collections: %v", err)
	}
	if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: "show dbs"}); err != nil {
		t.Fatalf("show dbs: %v", err)
	}

	// Writes report what they changed.
	write, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.updateMany({"state": "paid"}, {"$set": {"flagged": true}})`, collection),
	})
	if err != nil {
		t.Fatalf("updateMany: %v", err)
	}
	if write.AffectedRows != 2 || write.HasResultSet {
		t.Errorf("updateMany result = %+v", write)
	}

	insert, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.insertOne({"sku": "C-1"})`, collection),
	})
	if err != nil {
		t.Fatalf("insertOne: %v", err)
	}
	if insert.AffectedRows != 1 || !mentions(insert.Messages, "insertedId") {
		t.Errorf("insertOne result = %+v", insert)
	}

	// Index management round trips through createIndexes/dropIndexes.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.createIndex({"sku": 1}, {"name": "sku_1", "unique": true})`, collection),
	}); err != nil {
		t.Fatalf("createIndex: %v", err)
	}
	indexes, err := conn.Indexes(ctx, database, "")
	if err != nil {
		t.Fatalf("indexes: %v", err)
	}
	if len(indexes) != 2 {
		t.Fatalf("indexes = %+v, want _id_ and sku_1", indexes)
	}
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.dropIndex("sku_1")`, collection),
	}); err != nil {
		t.Fatalf("dropIndex: %v", err)
	}

	// An index created without a name gets the name the shell would give it.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.createIndex({"qty": -1})`, collection),
	}); err != nil {
		t.Fatalf("createIndex without a name: %v", err)
	}
	named, err := conn.Indexes(ctx, database, collection)
	if err != nil {
		t.Fatalf("indexes after createIndex: %v", err)
	}
	found := false
	for _, index := range named {
		if index.Name == "qty_-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("indexes = %+v, want the derived name qty_-1", named)
	}

	// The grid shows an id as hex and the shell spells it ObjectId(...); both
	// have to match an ObjectID on the server rather than a subdocument.
	const hexID = "64b7f1c2a4d3e5f60718293a"
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.insertOne({"_id": ObjectId(%q), "sku": "D-1"})`, collection, hexID),
	}); err != nil {
		t.Fatalf("insertOne with an ObjectId: %v", err)
	}
	byObjectID, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database,
		SQL:      fmt.Sprintf(`db.%s.find({"_id": ObjectId(%q)})`, collection, hexID),
	})
	if err != nil {
		t.Fatalf("find by ObjectId: %v", err)
	}
	if byObjectID.RowCount != 1 {
		t.Errorf("find by ObjectId matched %d rows, want 1", byObjectID.RowCount)
	}
	byHex, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: database, Object: collection,
		Filters: []models.FilterSpec{{Column: idField, Operator: "eq", Value: hexID}},
	})
	if err != nil {
		t.Fatalf("fetch by hex id: %v", err)
	}
	if byHex.RowCount != 1 {
		t.Errorf("the hex id in a filter matched %d rows, want 1", byHex.RowCount)
	}

	// A read-only session refuses anything that writes.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database, ReadOnly: true,
		SQL: fmt.Sprintf(`db.%s.deleteMany({})`, collection),
	}); err == nil {
		t.Error("a read-only session must refuse deleteMany")
	}
	if _, err := conn.Execute(ctx, drivers.ExecRequest{
		Database: database, ReadOnly: true,
		SQL: fmt.Sprintf(`db.%s.find({})`, collection),
	}); err != nil {
		t.Errorf("a read-only session must still read: %v", err)
	}

	// Bad input is reported, not silently ignored.
	if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: "select 1"}); err == nil {
		t.Error("SQL must not be accepted by a document store")
	}
	if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: database, SQL: "   "}); err == nil {
		t.Error("an empty script must be refused")
	}
}

// mentions reports whether any of the script's messages contains text.
func mentions(messages []string, text string) bool {
	for _, message := range messages {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

func TestIntegrationOverview(t *testing.T) {
	conn, _ := testConn(t)
	database, _ := scratchDatabase(t, conn)
	ctx := context.Background()

	overview, err := conn.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !overview.Supported || overview.Mongo == nil {
		t.Fatalf("overview = %+v", overview)
	}
	if overview.ServerVersion == "" {
		t.Error("the overview must report the server version")
	}

	titles := map[string]bool{}
	for _, group := range overview.Mongo.Groups {
		titles[group.Title] = true
		if len(group.Metrics) == 0 {
			t.Errorf("group %q has no metrics", group.Title)
		}
		for _, metric := range group.Metrics {
			if metric.Value == "" {
				t.Errorf("%s has an empty value", metric.Label)
			}
			if metric.Hint == "" {
				t.Errorf("%s has no hint explaining where the number comes from", metric.Label)
			}
		}
	}
	for _, want := range []string{"Server", "Connections", "Operations", "Memory"} {
		if !titles[want] {
			t.Errorf("the overview is missing the %q group (got %v)", want, titles)
		}
	}

	table := overview.Mongo.Databases
	if table == nil {
		t.Fatal("the overview must list the databases")
	}
	found := false
	for _, row := range table.Rows {
		if len(row) != len(table.Columns) {
			t.Fatalf("row %v does not match the %d columns", row, len(table.Columns))
		}
		if row[0] == database {
			found = true
		}
	}
	if !found {
		t.Errorf("the scratch database %s is missing from %v", database, table.Rows)
	}
}

func TestIntegrationReadOnlyServerIsRejected(t *testing.T) {
	cfg := testConfig(t)
	if cfg.Username != "" {
		t.Skip("only meaningful against a server without authentication")
	}
	// A wrong port must fail with a connection error rather than hang: the
	// driver sets a server selection timeout for exactly this case.
	cfg.Port = 1
	started := time.Now()
	_, err := (Driver{}).Open(context.Background(), cfg)
	if err == nil {
		t.Fatal("connecting to port 1 must fail")
	}
	if elapsed := time.Since(started); elapsed > 30*time.Second {
		t.Errorf("the failure took %s, want a bounded wait", elapsed)
	}
}
