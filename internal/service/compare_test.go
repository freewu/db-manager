package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlite"
	"dbmanager/internal/models"
)

// Comparing two databases is the one page that reads two sessions at once, so
// these tests run it against two real SQLite catalogs: what the window is shown,
// what script is generated from it, and what the right side looks like once the
// script has run. The statements themselves are pinned down by the driver tests;
// what matters here is the wiring and the two rules the page is built on — same
// engine, two different databases.

// addSession seeds a second SQLite file and registers it with the manager, so a
// test has a right-hand side to compare against the fixture's left one.
func addSession(t *testing.T, manager *Manager, id, name string, statements []string) {
	t.Helper()
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), id+".db")

	seed, err := sql.Open("sqlite", "file:"+file)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	for _, statement := range statements {
		if _, err := seed.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	cfg := models.ConnectionConfig{
		ID:       id,
		Name:     name,
		Driver:   models.DriverSQLite,
		FilePath: file,
		Database: "main",
	}
	driver := sqlite.Driver{}
	conn, err := driver.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open connection: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	manager.sessions[id] = &session{id: id, cfg: cfg, driver: driver, conn: conn, connectedAt: 2}
}

// runOn runs a statement on one session's own connection, bypassing the manager:
// a fixture is not a change the application made, so it does not belong in the
// change log.
func runOn(t *testing.T, manager *Manager, sessionID, statement string) {
	t.Helper()
	if _, err := manager.sessions[sessionID].conn.Execute(context.Background(),
		drivers.ExecRequest{SQL: statement}); err != nil {
		t.Fatalf("run %q: %v", statement, err)
	}
}

// compareFixture is the left side (the manager fixture: orders with a unique
// index and two rows) against a right side that differs in every way the
// comparison knows about: a field missing, a field extra, one field spelled
// differently, a different index, and a table of its own.
func compareFixture(t *testing.T) *Manager {
	t.Helper()
	manager, _ := testManager(t)
	addSession(t, manager, "s2", "other", []string{
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total NUMERIC, note TEXT)`,
		`CREATE INDEX idx_orders_total ON orders (total)`,
		`CREATE TABLE customers (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`,
	})
	return manager
}

// applyFixture is the same idea for the apply path, with differences SQLite can
// actually make: a column to drop, an index to add, a table to create and one to
// remove. Nothing here needs a change SQLite refuses.
func applyFixture(t *testing.T) *Manager {
	t.Helper()
	manager, _ := testManager(t)
	runOn(t, manager, "s1", `CREATE TABLE customers (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`)
	runOn(t, manager, "s1", `CREATE UNIQUE INDEX idx_customers_email ON customers (email)`)
	addSession(t, manager, "s2", "other", []string{
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, total NUMERIC, extra TEXT)`,
		`CREATE TABLE legacy (id INTEGER PRIMARY KEY, note TEXT)`,
	})
	return manager
}

func sides(left, right string) models.CompareRequest {
	return models.CompareRequest{
		Left:  models.CompareSide{SessionID: left, Database: "main"},
		Right: models.CompareSide{SessionID: right, Database: "main"},
	}
}

// tablesByName indexes a comparison the way a reader scans it.
func tablesByName(compare *models.SchemaCompare) map[string]models.TableDiff {
	out := map[string]models.TableDiff{}
	for _, table := range compare.Tables {
		out[table.Name] = table
	}
	return out
}

func tableField(t *testing.T, table models.TableDiff, name string) models.DiffItem {
	t.Helper()
	for _, item := range table.Columns {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("no field %s in %+v", name, table.Columns)
	return models.DiffItem{}
}

func tableIndex(t *testing.T, table models.TableDiff, name string) models.DiffItem {
	t.Helper()
	for _, item := range table.Indexes {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("no index %s in %+v", name, table.Indexes)
	return models.DiffItem{}
}

func TestCompareDatabasesNamesBothSidesAndEveryDifference(t *testing.T) {
	manager := compareFixture(t)

	compare, err := manager.CompareDatabases(sides("s1", "s2"))
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if compare.Driver != models.DriverSQLite {
		t.Fatalf("the comparison is run in the engine's terms: %q", compare.Driver)
	}
	if compare.LeftLabel != "fixture · main" || compare.RightLabel != "other · main" {
		t.Fatalf("each side should be named by connection and database: %q / %q",
			compare.LeftLabel, compare.RightLabel)
	}

	tables := tablesByName(compare)
	if len(tables) != 2 {
		t.Fatalf("expected one table per side plus the shared one: %+v", compare.Tables)
	}
	if customers := tables["customers"]; customers.Status != models.DiffRemoved ||
		customers.Summary != "only on the right" {
		t.Fatalf("a table only the right has is one to remove: %+v", customers)
	}

	orders := tables["orders"]
	if orders.Status != models.DiffChanged {
		t.Fatalf("orders differs on both sides, so it should read as changed: %+v", orders)
	}
	if item := tableField(t, orders, "user_id"); item.Status != models.DiffChanged ||
		item.Fields[0].Field != "nullability" {
		t.Fatalf("one side has user_id NOT NULL and the other does not: %+v", item)
	}
	if item := tableField(t, orders, "note"); item.Status != models.DiffRemoved {
		t.Fatalf("note is only on the right: %+v", item)
	}
	if item := tableIndex(t, orders, "idx_orders_user"); item.Status != models.DiffAdded {
		t.Fatalf("the unique index is only on the left: %+v", item)
	}
	if item := tableIndex(t, orders, "idx_orders_total"); item.Status != models.DiffRemoved {
		t.Fatalf("the total index is only on the right: %+v", item)
	}
}

// A comparison that skipped the views and said nothing would let "no
// differences" be read as "the two databases agree".
func TestCompareDatabasesSaysWhatItDidNotLookAt(t *testing.T) {
	manager, _ := testManager(t)
	addSession(t, manager, "s2", "other", []string{
		`CREATE TABLE orders (id INTEGER PRIMARY KEY)`,
		`CREATE VIEW recent AS SELECT id FROM orders`,
		`CREATE VIEW recent2 AS SELECT id FROM orders`,
	})

	compare, err := manager.CompareDatabases(sides("s1", "s2"))
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if len(compare.Warnings) != 1 || !strings.Contains(compare.Warnings[0], "2 views") {
		t.Fatalf("expected one warning naming the two views, got %v", compare.Warnings)
	}
}

func TestPlanSyncDatabasePlansCreatesChangesAndRemovals(t *testing.T) {
	manager := applyFixture(t)

	plan, err := manager.PlanSyncDatabase(models.SyncDatabaseRequest{
		Left:      models.CompareSide{SessionID: "s1", Database: "main"},
		Right:     models.CompareSide{SessionID: "s2", Database: "main"},
		DropExtra: true,
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Creations first, then changes, then removals: the script runs top to
	// bottom on an engine that cannot roll it back.
	want := []string{
		`CREATE TABLE "customers"`,
		`CREATE UNIQUE INDEX "idx_customers_email"`,
		`ALTER TABLE "orders" DROP COLUMN "extra"`,
		`CREATE UNIQUE INDEX "idx_orders_user" ON "orders" ("user_id")`,
		`DROP TABLE "legacy"`,
	}
	if len(plan.Statements) != len(want) {
		t.Fatalf("expected %d statements, got %v", len(want), plan.Statements)
	}
	for i, fragment := range want {
		if !strings.Contains(plan.Statements[i], fragment) {
			t.Fatalf("statement %d should contain %q, got %q", i, fragment, plan.Statements[i])
		}
	}
	if !plan.Destructive {
		t.Fatal("the script drops a column and a table, and the confirmation reads this flag")
	}
}

// Without DropExtra, a table on the right that the left does not have is left
// alone and the plan says so: "make B look like A" and "delete everything B has
// that A does not" are different promises.
func TestPlanSyncDatabaseLeavesExtraTablesAloneUnlessAsked(t *testing.T) {
	manager := applyFixture(t)

	plan, err := manager.PlanSyncDatabase(models.SyncDatabaseRequest{
		Left:  models.CompareSide{SessionID: "s1", Database: "main"},
		Right: models.CompareSide{SessionID: "s2", Database: "main"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, statement := range plan.Statements {
		if strings.Contains(statement, "legacy") {
			t.Fatalf("nothing should be dropped without asking: %v", plan.Statements)
		}
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "legacy") {
		t.Fatalf("the table that was left alone should be named: %v", plan.Warnings)
	}
}

func TestApplySyncDatabaseMakesTheRightSideMatchTheLeft(t *testing.T) {
	manager := applyFixture(t)
	req := models.SyncDatabaseRequest{
		Left:      models.CompareSide{SessionID: "s1", Database: "main"},
		Right:     models.CompareSide{SessionID: "s2", Database: "main"},
		DropExtra: true,
	}

	plan, err := manager.PlanSyncDatabase(req)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	result, err := manager.ApplySyncDatabase(req)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("apply failed at %d: %s", result.FailedIndex, result.Error)
	}
	if len(result.Executed) != len(plan.Statements) {
		t.Fatalf("executed %d statements, expected %d", len(result.Executed), len(plan.Statements))
	}

	after, err := manager.CompareDatabases(sides("s1", "s2"))
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	for _, table := range after.Tables {
		if table.Status != models.DiffSame {
			t.Fatalf("%s should match after the sync, got %+v", table.Name, table)
		}
	}
	if len(after.Tables) != 2 {
		t.Fatalf("the right side should now hold exactly the left's tables: %+v", after.Tables)
	}
}

// The right side is the one that changes, so it is the one that must not be
// read-only — the left may well be a production database being read as the
// reference, and nothing here writes to it.
func TestSyncRefusesAReadOnlyTarget(t *testing.T) {
	manager := applyFixture(t)
	manager.sessions["s2"].readOnly = true
	req := models.SyncDatabaseRequest{
		Left:  models.CompareSide{SessionID: "s1", Database: "main"},
		Right: models.CompareSide{SessionID: "s2", Database: "main"},
	}
	if _, err := manager.ApplySyncDatabase(req); err == nil {
		t.Fatal("expected a read-only target to be refused")
	}
	// Planning is a read, so the preview still works.
	if _, err := manager.PlanSyncDatabase(req); err != nil {
		t.Fatalf("planning a read-only target should still work: %v", err)
	}
}

func TestCompareRefusesTheSameDatabaseAndDifferentEngines(t *testing.T) {
	manager := compareFixture(t)

	if _, err := manager.CompareDatabases(sides("s1", "s1")); err == nil {
		t.Fatal("comparing a database with itself answers nothing")
	}

	// Two sessions of the same server, one of them an engine this build writes
	// no DDL for: the script at the end of the page is the point, so a pair the
	// page cannot script is refused before anything is read.
	manager.sessions["s2"].cfg.Driver = models.DriverMySQL
	if _, err := manager.CompareDatabases(sides("s1", "s2")); err == nil {
		t.Fatal("expected two different engines to be refused")
	}
	if _, err := manager.PlanSyncDatabase(models.SyncDatabaseRequest{
		Left:  models.CompareSide{SessionID: "s1", Database: "main"},
		Right: models.CompareSide{SessionID: "s2", Database: "main"},
	}); err == nil {
		t.Fatal("expected the sync to refuse two different engines")
	}

	if _, err := manager.CompareDatabases(sides("nope", "s2")); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
