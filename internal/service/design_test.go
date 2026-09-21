package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"dbmanager/internal/drivers/sqlite"
	"dbmanager/internal/models"
)

// The designer is driven through the manager exactly like the app drives it:
// plan a draft, apply it statement by statement, plan again. This test is about
// the wiring — the SQL itself is pinned down by the driver tests — so it also
// guards the JSON shape the two halves agree on.

func testManager(t *testing.T) (*Manager, string) {
	t.Helper()
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "shop.db")

	seed, err := sql.Open("sqlite", "file:"+file)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	statements := []string{
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, total NUMERIC)`,
		`CREATE UNIQUE INDEX idx_orders_user ON orders (user_id)`,
		`INSERT INTO orders (id, user_id, total) VALUES (1, 7, 10), (2, 8, 20)`,
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
		ID:       "c1",
		Name:     "fixture",
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

	manager := &Manager{
		baseCtx:  ctx,
		sessions: map[string]*session{"s1": {id: "s1", cfg: cfg, driver: driver, conn: conn}},
	}
	return manager, file
}

// designFor prefills a draft from the live catalog, the way the window does.
func designFor(t *testing.T, manager *Manager) models.TableDesign {
	t.Helper()
	structure, err := manager.Structure("s1", "main", "", "orders")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	design := models.TableDesign{
		SessionID: "s1",
		Database:  structure.Object.Database,
		Schema:    structure.Object.Schema,
		Object:    structure.Object.Name,
	}
	for _, column := range structure.Columns {
		field := models.DesignColumn{
			Name:          column.Name,
			OriginalName:  column.Name,
			DataType:      column.ColumnType,
			Nullable:      column.Nullable,
			PrimaryKey:    column.PrimaryKey,
			AutoIncrement: column.AutoIncrement,
		}
		if field.DataType == "" {
			field.DataType = column.DataType
		}
		design.Columns = append(design.Columns, field)
	}
	for _, index := range structure.Indexes {
		if index.Primary {
			continue
		}
		design.Indexes = append(design.Indexes, models.DesignIndex{
			Name:         index.Name,
			OriginalName: index.Name,
			Columns:      index.Columns,
			Unique:       index.Unique,
		})
	}
	return design
}

func TestDesignerRoundTripThroughTheManager(t *testing.T) {
	manager, _ := testManager(t)

	// An untouched draft must be a no-op: this is what the window shows the
	// moment it opens.
	design := designFor(t, manager)
	plan, err := manager.PlanDesign(design)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Statements) != 0 {
		t.Fatalf("a fresh draft should plan no statements, got %v", plan.Statements)
	}

	// Rename a field, drop another and add a nullable one.
	design.Columns[1].Name = "customer_id"
	design.Columns = design.Columns[:2]
	design.Columns = append(design.Columns, models.DesignColumn{
		Name:     "note",
		DataType: "TEXT",
		Nullable: true,
	})

	plan, err = manager.PlanDesign(design)
	if err != nil {
		t.Fatalf("plan changes: %v", err)
	}
	if len(plan.Statements) == 0 {
		t.Fatal("expected statements for the changed design")
	}

	result, err := manager.ApplyDesign(design)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("apply failed at %d: %s", result.FailedIndex, result.Error)
	}
	if len(result.Executed) != len(plan.Statements) {
		t.Fatalf("executed %d statements, expected %d", len(result.Executed), len(plan.Statements))
	}

	// The catalog really moved, and the same draft is now a no-op again.
	structure, err := manager.Structure("s1", "main", "", "orders")
	if err != nil {
		t.Fatalf("structure after apply: %v", err)
	}
	names := []string{}
	for _, column := range structure.Columns {
		names = append(names, column.Name)
	}
	if len(names) != 3 || names[0] != "id" || names[1] != "customer_id" || names[2] != "note" {
		t.Fatalf("unexpected columns after apply: %v", names)
	}

	again, err := manager.PlanDesign(designFor(t, manager))
	if err != nil {
		t.Fatalf("replan: %v", err)
	}
	if len(again.Statements) != 0 {
		t.Fatalf("the applied design should plan no statements, got %v", again.Statements)
	}
}

// TestDesignerIsIdempotent proves the designer compares against the live catalog
// instead of replaying a diff: applying the same draft twice plans nothing the
// second time, because the field is already there.
func TestDesignerIsIdempotent(t *testing.T) {
	manager, _ := testManager(t)

	design := designFor(t, manager)
	design.Columns = append(design.Columns, models.DesignColumn{
		Name:     "a_new",
		DataType: "TEXT",
		Nullable: true,
	})

	plan, err := manager.PlanDesign(design)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Statements) != 1 {
		t.Fatalf("expected a single ADD COLUMN, got %v", plan.Statements)
	}
	if _, err := manager.ApplyDesign(design); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Same draft, but the field now exists: nothing left to do. A draft that
	// arrived without OriginalName still matches by name.
	again, err := manager.PlanDesign(design)
	if err != nil {
		t.Fatalf("replan: %v", err)
	}
	if len(again.Statements) != 0 {
		t.Fatalf("re-applying a design must be a no-op, got %v", again.Statements)
	}
}

// TestDesignerReportsWhereItStopped drives the failure path. SQLite refuses to
// add a NOT NULL column without a default to a table that already holds rows,
// which the designer only warns about: the script really does stop in the
// middle, and the result has to say how far it got.
func TestDesignerReportsWhereItStopped(t *testing.T) {
	manager, _ := testManager(t)

	design := designFor(t, manager)
	design.Columns = append(design.Columns,
		models.DesignColumn{Name: "b_new", DataType: "TEXT", Nullable: true},
		models.DesignColumn{Name: "c_new", DataType: "TEXT"},
	)

	plan, err := manager.PlanDesign(design)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Statements) != 2 {
		t.Fatalf("expected two ADD COLUMN statements, got %v", plan.Statements)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("the designer should warn about the NOT NULL field it cannot add")
	}

	result, err := manager.ApplyDesign(design)
	if err != nil {
		t.Fatalf("apply must report, not throw: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected SQLite to refuse the NOT NULL column")
	}
	if result.FailedIndex != 1 || len(result.Executed) != 1 {
		t.Fatalf("expected the second statement to fail, got failedIndex=%d executed=%d",
			result.FailedIndex, len(result.Executed))
	}

	// What ran before the failure is really in the catalog.
	structure, err := manager.Structure("s1", "main", "", "orders")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	names := []string{}
	for _, column := range structure.Columns {
		names = append(names, column.Name)
	}
	if len(names) != 4 || names[3] != "b_new" {
		t.Fatalf("the first statement should have been applied: %v", names)
	}
}

func TestDesignerRefusesUnknownSessions(t *testing.T) {
	manager, _ := testManager(t)
	design := designFor(t, manager)
	design.SessionID = "nope"
	if _, err := manager.PlanDesign(design); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
	if _, err := manager.ApplyDesign(design); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}

// newTableDesign is the draft the window opens for a table that does not exist
// yet: a name to fill in and a key field to start from.
func newTableDesign() models.TableDesign {
	return models.TableDesign{
		SessionID: "s1",
		Database:  "main",
		Object:    "customers",
		Columns: []models.DesignColumn{
			{Name: "id", DataType: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "email", DataType: "TEXT"},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_customers_email", Columns: []string{"email"}},
		},
	}
}

// TestCreateDesignerRoundTripThroughTheManager drives the create path the way
// the window does: plan a draft for a table that is not there, run the script,
// then read the new table back from the catalog.
func TestCreateDesignerRoundTripThroughTheManager(t *testing.T) {
	manager, _ := testManager(t)
	design := newTableDesign()

	plan, err := manager.PlanCreateDesign(design)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Statements) != 2 || plan.Destructive {
		t.Fatalf("expected a CREATE TABLE and a CREATE INDEX, got %v", plan.Statements)
	}

	result, err := manager.ApplyCreateDesign(design)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("apply failed at %d: %s", result.FailedIndex, result.Error)
	}
	if len(result.Executed) != len(plan.Statements) {
		t.Fatalf("executed %d statements, expected %d", len(result.Executed), len(plan.Statements))
	}

	// The table is now an ordinary one: it reads back with the fields and the
	// index the draft asked for.
	structure, err := manager.Structure("s1", "main", "", "customers")
	if err != nil {
		t.Fatalf("structure after create: %v", err)
	}
	names := []string{}
	for _, column := range structure.Columns {
		names = append(names, column.Name)
	}
	if len(names) != 2 || names[0] != "id" || names[1] != "email" {
		t.Fatalf("unexpected columns after create: %v", names)
	}
	if len(structure.Indexes) != 1 || structure.Indexes[0].Name != "idx_customers_email" {
		t.Fatalf("unexpected indexes after create: %v", structure.Indexes)
	}

	// Creating it a second time is the engine's error to give, and the manager
	// reports where the script stopped instead of pretending it worked.
	again, err := manager.ApplyCreateDesign(design)
	if err != nil {
		t.Fatalf("apply again must report, not throw: %v", err)
	}
	if again.Error == "" || again.FailedIndex != 0 || len(again.Executed) != 0 {
		t.Fatalf("expected the second CREATE to fail on its first statement, got %+v", again)
	}
}

// A read-only connection must refuse before it writes anything: the window can
// never be allowed to open a CREATE TABLE on it.
func TestCreateDesignerRefusesReadOnlySessions(t *testing.T) {
	manager, _ := testManager(t)
	manager.sessions["s1"].readOnly = true

	// Planning is a read, so the preview still works and simply says what would
	// have been created.
	if _, err := manager.PlanCreateDesign(newTableDesign()); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := manager.ApplyCreateDesign(newTableDesign()); err == nil {
		t.Fatal("expected a read-only session to refuse CREATE TABLE")
	}
}

func TestCreateDesignerNeedsANameAndAKnownSession(t *testing.T) {
	manager, _ := testManager(t)

	nameless := newTableDesign()
	nameless.Object = "  "
	if _, err := manager.PlanCreateDesign(nameless); err == nil {
		t.Fatal("expected a table without a name to be refused")
	}
	if _, err := manager.ApplyCreateDesign(nameless); err == nil {
		t.Fatal("expected apply to refuse a table without a name")
	}

	unknown := newTableDesign()
	unknown.SessionID = "nope"
	if _, err := manager.PlanCreateDesign(unknown); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
	if _, err := manager.ApplyCreateDesign(unknown); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
