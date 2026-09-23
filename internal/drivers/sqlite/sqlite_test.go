package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// newConn creates an empty database file and opens it through the driver, which
// exercises the same code path the application uses.
func newConn(t *testing.T) drivers.Conn {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create database file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close database file: %v", err)
	}

	conn, err := Driver{}.Open(context.Background(), models.ConnectionConfig{
		Driver:   models.DriverSQLite,
		FilePath: path,
		Database: "main",
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func exec(t *testing.T, conn drivers.Conn, sqlText string) *models.QueryResult {
	t.Helper()
	res, err := conn.Execute(context.Background(), drivers.ExecRequest{
		Database: "main",
		SQL:      sqlText,
	})
	if err != nil {
		t.Fatalf("execute %q: %v", sqlText, err)
	}
	return res
}

// inserter is the data generation capability of this connection; a driver that
// could not write rows would not offer it (see drivers.Inserter).
func inserter(t *testing.T, conn drivers.Conn) drivers.Inserter {
	t.Helper()
	in, ok := conn.(drivers.Inserter)
	if !ok {
		t.Fatal("the SQLite connection must implement drivers.Inserter")
	}
	return in
}

func TestFetchStructureAndMutation(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)

	exec(t, conn, `
CREATE TABLE people (
	id      INTEGER PRIMARY KEY,
	name    TEXT NOT NULL,
	email   TEXT,
	age     INTEGER
);
INSERT INTO people (id, name, email, age) VALUES
	(1, 'Alice', 'alice@example.com', 31),
	(2, 'Bob',   NULL,                42),
	(3, 'Carol', 'carol@example.com', NULL);`)

	// --- catalog ----------------------------------------------------------

	objects, err := conn.Objects(ctx, "main", "main")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != "people" {
		t.Fatalf("unexpected objects: %+v", objects)
	}

	structure, err := conn.Structure(ctx, "main", "main", "people")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	if len(structure.Columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(structure.Columns))
	}
	if !structure.Columns[0].PrimaryKey || structure.Columns[0].Name != "id" {
		t.Fatalf("id should be the primary key: %+v", structure.Columns[0])
	}
	if structure.DDL == "" {
		t.Fatal("expected DDL for the table")
	}

	// --- paging -----------------------------------------------------------

	page, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database:   "main",
		Object:     "people",
		Limit:      2,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !page.HasTotal || page.Total != 3 {
		t.Fatalf("expected total 3, got %d (hasTotal=%v)", page.Total, page.HasTotal)
	}
	if page.RowCount != 2 {
		t.Fatalf("expected 2 rows on the first page, got %d", page.RowCount)
	}
	if !page.Columns[0].IsPrimaryKey {
		t.Fatalf("fetch should flag the primary key column: %+v", page.Columns)
	}
	// Without an explicit order the primary key is used, so paging is stable.
	if got := page.Rows[0][0]; got != int64(1) {
		t.Fatalf("expected the first row to be id=1, got %v", got)
	}

	// --- filtering --------------------------------------------------------

	filtered, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: "main",
		Object:   "people",
		Limit:    10,
		Filters: []models.FilterSpec{
			{Column: "name", Operator: sqlutil.OpContains, Value: "li"},
		},
	})
	if err != nil {
		t.Fatalf("filtered fetch: %v", err)
	}
	if filtered.RowCount != 1 || filtered.Rows[0][1] != "Alice" {
		t.Fatalf("expected only Alice, got %+v", filtered.Rows)
	}

	nulls, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database: "main",
		Object:   "people",
		Limit:    10,
		Filters:  []models.FilterSpec{{Column: "email", Operator: sqlutil.OpIsNull}},
	})
	if err != nil {
		t.Fatalf("null fetch: %v", err)
	}
	if nulls.RowCount != 1 || nulls.Rows[0][1] != "Bob" {
		t.Fatalf("expected only Bob, got %+v", nulls.Rows)
	}

	// --- inline edit ------------------------------------------------------

	affected, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: "main",
		Object:   "people",
		Key:      []models.KeyValue{{Column: "id", Value: "2"}},
		Column:   "email",
		Value:    "bob@example.com",
	})
	if err != nil {
		t.Fatalf("update cell: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 row updated, got %d", affected)
	}

	updated, err := conn.Fetch(ctx, drivers.FetchRequest{Database: "main", Object: "people", Limit: 10})
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if got := updated.Rows[1][2]; got != "bob@example.com" {
		t.Fatalf("expected the email to be updated, got %v", got)
	}

	// An emptied field must produce NULL, not an empty string.
	if _, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: "main",
		Object:   "people",
		Key:      []models.KeyValue{{Column: "id", Value: "2"}},
		Column:   "email",
		Value:    nil,
	}); err != nil {
		t.Fatalf("update to null: %v", err)
	}
	nulled, err := conn.Fetch(ctx, drivers.FetchRequest{Database: "main", Object: "people", Limit: 10})
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if nulled.Rows[1][2] != nil {
		t.Fatalf("expected NULL, got %v", nulled.Rows[1][2])
	}

	// A stale key must not silently update a different row.
	missed, err := conn.UpdateCell(ctx, models.CellUpdate{
		Database: "main",
		Object:   "people",
		Key:      []models.KeyValue{{Column: "id", Value: "999"}},
		Column:   "name",
		Value:    "Nobody",
	})
	if err != nil {
		t.Fatalf("update missing row: %v", err)
	}
	if missed != 0 {
		t.Fatalf("expected 0 rows updated, got %d", missed)
	}

	// --- delete -----------------------------------------------------------

	deleted, err := conn.DeleteRow(ctx, models.RowDelete{
		Database: "main",
		Object:   "people",
		Key:      []models.KeyValue{{Column: "id", Value: "3"}},
	})
	if err != nil {
		t.Fatalf("delete row: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 row deleted, got %d", deleted)
	}
	remaining, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database:   "main",
		Object:     "people",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if remaining.Total != 2 {
		t.Fatalf("expected 2 remaining rows, got %d", remaining.Total)
	}
}

func TestNormalizeRejectsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	err := Driver{}.Normalize(&models.ConnectionConfig{
		Driver:   models.DriverSQLite,
		FilePath: path,
	})
	if err == nil {
		t.Fatal("expected an error for a missing database file")
	}
}

// The data generation window hands whole batches of rows to this call; what the
// engine side owes it is that the values are bound (never spliced), that NULL
// stays NULL, and that a batch the engine turns down says which row it was.
func TestInsertRowsBindsEveryValueAndReportsTheRowItRefused(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)
	in := inserter(t, conn)

	exec(t, conn, `
CREATE TABLE people (
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
	name  TEXT NOT NULL,
	age   INTEGER,
	score REAL
);`)

	// One statement for the whole batch, every value a parameter.
	result, err := in.InsertRows(ctx, models.RowInsert{
		Database: "main",
		Object:   "people",
		Columns:  []string{"name", "age", "score"},
		Rows: [][]any{
			{"Alice", int64(31), 1.5},
			{"Bob", int64(42), nil},
			// A quote in a generated value must not become syntax.
			{"O'Brien); DROP TABLE people; --", nil, 9.25},
		},
	})
	if err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	if result.Inserted != 3 || result.Failed != 0 || result.Error != "" {
		t.Fatalf("expected all three rows to land, got %+v", result)
	}

	page, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database:   "main",
		Object:     "people",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("expected 3 rows, got %d", page.Total)
	}
	// `id` was left to the database: AUTOINCREMENT assigned it.
	if page.Rows[0][0] != int64(1) {
		t.Fatalf("expected the key to be assigned by the engine, got %v", page.Rows[0][0])
	}
	if page.Rows[1][3] != nil {
		t.Fatalf("an empty value must be written as NULL, got %v", page.Rows[1][3])
	}
	if page.Rows[2][1] != "O'Brien); DROP TABLE people; --" {
		t.Fatalf("the literal must have been stored as data, got %v", page.Rows[2][1])
	}

	// A row the engine refuses must not swallow the ones that fit: the batch is
	// replayed row by row so the caller learns which row stopped it.
	refused, err := in.InsertRows(ctx, models.RowInsert{
		Database: "main",
		Object:   "people",
		Columns:  []string{"name", "age"},
		Rows: [][]any{
			{"Dan", int64(1)},
			{"Eve", int64(2)},
			{nil, int64(3)}, // name is NOT NULL
		},
	})
	if err != nil {
		t.Fatalf("a refused row must be reported, not raised: %v", err)
	}
	if refused.Inserted != 2 || refused.Failed != 3 {
		t.Fatalf("expected 2 of 3 rows to land and row 3 to fail, got %+v", refused)
	}
	if refused.Error == "" {
		t.Fatal("the engine's own message has to travel back")
	}
	after, err := conn.Fetch(ctx, drivers.FetchRequest{
		Database:   "main",
		Object:     "people",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if after.Total != 5 {
		t.Fatalf("expected 5 rows after the partial batch, got %d", after.Total)
	}

	// A row whose width does not match the column list is refused before any
	// statement is built.
	if _, err := in.InsertRows(ctx, models.RowInsert{
		Database: "main",
		Object:   "people",
		Columns:  []string{"name", "age"},
		Rows:     [][]any{{"Frank"}},
	}); err == nil {
		t.Fatal("expected a width mismatch to be refused")
	}
	if _, err := in.InsertRows(ctx, models.RowInsert{
		Database: "main",
		Object:   "people",
		Columns:  []string{"name"},
		Rows:     [][]any{},
	}); err == nil {
		t.Fatal("expected an empty batch to be refused")
	}
}
