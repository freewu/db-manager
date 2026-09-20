package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// The SQLite status page is the only one that describes a file, so it is the
// only one that can be asserted on without a server: create a database, read the
// page back, and check both the pragmas and the object list.
func TestOverviewDescribesTheFile(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)

	exec(t, conn, `
CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
CREATE INDEX people_name ON people (name);
CREATE VIEW named_people AS SELECT name FROM people;`)

	reporter, ok := conn.(drivers.Overviewer)
	if !ok {
		t.Fatal("the sqlite connection must implement drivers.Overviewer")
	}
	page, err := reporter.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !page.Supported || page.SQLite == nil {
		t.Fatalf("expected a supported sqlite page, got %+v", page)
	}
	if page.MySQL != nil || page.Postgres != nil {
		t.Fatal("an engine must fill in its own section only")
	}
	if page.SQLite.Path == "" || page.SQLite.FileSize < 0 {
		t.Fatalf("expected a real file, got %+v", page.SQLite)
	}
	if len(page.SQLite.Groups) != 2 {
		t.Fatalf("expected the file and format groups, got %d", len(page.SQLite.Groups))
	}

	metrics := map[string]models.OverviewMetric{}
	for _, group := range page.SQLite.Groups {
		for _, metric := range group.Metrics {
			metrics[metric.Label] = metric
		}
	}
	// SQLite's default page size has been 4096 since 3.12; the point is that the
	// number was read at all rather than that it is a specific value.
	if metrics["Page size"].Value == "—" || metrics["Page count"].Value == "—" {
		t.Fatalf("page size and page count must be read, got %+v", metrics)
	}
	if metrics["Journal mode"].Value == "—" {
		t.Fatalf("journal mode must be read, got %+v", metrics["Journal mode"])
	}
	if metrics["Foreign keys"].Value != "no" {
		t.Fatalf("foreign keys are off by default, got %q", metrics["Foreign keys"].Value)
	}
	if metrics["File"].Value != page.SQLite.Path {
		t.Fatalf("the file metric must name the database, got %q", metrics["File"].Value)
	}

	objects := page.SQLite.Objects
	if objects == nil {
		t.Fatal("the schema object list must not be empty")
	}
	byName := map[string]string{}
	for _, row := range objects.Rows {
		byName[row[1]] = row[0]
	}
	if byName["people"] != "table" || byName["people_name"] != "index" || byName["named_people"] != "view" {
		t.Fatalf("unexpected objects: %+v", objects.Rows)
	}
	// The listed index has to point back at its table.
	for _, row := range objects.Rows {
		if row[1] == "people_name" && row[2] != "people" {
			t.Fatalf("the index should name its table, got %+v", row)
		}
	}

	attached := page.SQLite.Attached
	if attached == nil || len(attached.Rows) != 1 || attached.Rows[0][1] != "main" {
		t.Fatalf("expected the main database to be attached, got %+v", attached)
	}
}

// An in-memory database, or a file the process cannot see any more, has no size
// to report; saying so is better than claiming zero bytes.
func TestOverviewHandlesAMissingFile(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)
	pool, err := conn.(*sqlbase.Conn).DB(ctx, "")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	// The open connection still works, but the path it reports no longer exists
	// — the read-only share / unmounted volume case.
	cfg := models.ConnectionConfig{
		Driver:   models.DriverSQLite,
		FilePath: filepath.Join(t.TempDir(), "gone.db"),
		Database: "main",
	}
	page, err := overview(ctx, pool, cfg)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if page.SQLite.FileSize != -1 {
		t.Fatalf("a missing file must report -1, got %d", page.SQLite.FileSize)
	}
	for _, group := range page.SQLite.Groups {
		for _, metric := range group.Metrics {
			if metric.Label == "Size on disk" && metric.Value != "—" {
				t.Fatalf("a missing file must render as a dash, got %q", metric.Value)
			}
			if metric.Label == "File" && metric.Value != cfg.FilePath {
				t.Fatalf("the page must name the configured path, got %q", metric.Value)
			}
		}
	}
}
