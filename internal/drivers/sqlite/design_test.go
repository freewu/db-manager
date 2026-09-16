package sqlite

import (
	"context"
	"strings"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// The designer unit tests pin down the SQL that is rendered; this test proves a
// real SQLite accepts that SQL and ends up with the structure the user asked
// for. It runs the script the same way the application does, one statement at a
// time.

// designFrom prefills a draft from a live structure exactly like the designer
// window does, so "no changes" is the starting point.
func designFrom(structure *models.TableStructure) models.TableDesign {
	draft := models.TableDesign{
		Database: structure.Object.Database,
		Schema:   structure.Object.Schema,
		Object:   structure.Object.Name,
	}
	for _, c := range structure.Columns {
		column := models.DesignColumn{
			Name:          c.Name,
			OriginalName:  c.Name,
			DataType:      c.ColumnType,
			Nullable:      c.Nullable,
			PrimaryKey:    c.PrimaryKey,
			AutoIncrement: c.AutoIncrement,
			Comment:       c.Comment,
		}
		if column.DataType == "" {
			column.DataType = c.DataType
		}
		if c.DefaultValue != nil {
			value := *c.DefaultValue
			column.DefaultValue = &value
		}
		draft.Columns = append(draft.Columns, column)
	}
	for _, ix := range structure.Indexes {
		if ix.Primary {
			continue
		}
		draft.Indexes = append(draft.Indexes, models.DesignIndex{
			Name:         ix.Name,
			OriginalName: ix.Name,
			Columns:      ix.Columns,
			Unique:       ix.Unique,
		})
	}
	return draft
}

// applyDesign renders and runs a design, statement by statement.
func applyDesign(t *testing.T, conn drivers.Conn, structure *models.TableStructure, want models.TableDesign) models.DesignPlan {
	t.Helper()

	plan, err := sqlbase.PlanAlter(conn.Dialect(), structure, want)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, statement := range plan.Statements {
		if _, err := conn.Execute(context.Background(), drivers.ExecRequest{
			Database: "main",
			SQL:      statement,
		}); err != nil {
			t.Fatalf("apply %q: %v", statement, err)
		}
	}
	return plan
}

func TestDesignerAltersARealTable(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)

	exec(t, conn, `
CREATE TABLE people (
	id      INTEGER PRIMARY KEY,
	name    TEXT NOT NULL,
	email   TEXT,
	age     INTEGER
);
CREATE UNIQUE INDEX idx_people_email ON people (email);
INSERT INTO people (id, name, email, age) VALUES
	(1, 'Alice', 'alice@example.com', 31),
	(2, 'Bob',   NULL,                42);`)

	structure, err := conn.Structure(ctx, "main", "main", "people")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}

	// A design that changes nothing must render nothing.
	if plan := applyDesign(t, conn, structure, designFrom(structure)); len(plan.Statements) != 0 {
		t.Fatalf("an untouched design should be empty, got %v", plan.Statements)
	}

	// Rename a column, drop a column, add a column with a default and rename
	// the index that follows the renamed column.
	draft := designFrom(structure)
	draft.Columns = []models.DesignColumn{
		draft.Columns[0],
		draft.Columns[1],
		{Name: "contact", OriginalName: "email", DataType: "TEXT", Nullable: true},
		{Name: "nickname", DataType: "TEXT", Nullable: false, DefaultValue: strptr("'anon'")},
	}
	draft.Indexes = []models.DesignIndex{
		{Name: "idx_people_contact", OriginalName: "idx_people_email", Columns: []string{"contact"}, Unique: true},
	}

	plan := applyDesign(t, conn, structure, draft)
	// SQLite cannot rename an index, so it says so and rebuilds it instead.
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "cannot rename an index") {
		t.Fatalf("expected only a warning about the index rename, got %v", plan.Warnings)
	}

	updated, err := conn.Structure(ctx, "main", "main", "people")
	if err != nil {
		t.Fatalf("structure after design: %v", err)
	}
	names := make([]string, 0, len(updated.Columns))
	for _, c := range updated.Columns {
		names = append(names, c.Name)
	}
	if got := len(names); got != 4 {
		t.Fatalf("expected 4 columns (id, name, contact, nickname), got %v", names)
	}
	for i, want := range []string{"id", "name", "contact", "nickname"} {
		if names[i] != want {
			t.Fatalf("expected %v, got %v", []string{"id", "name", "contact", "nickname"}, names)
		}
	}
	if !updated.Columns[0].PrimaryKey {
		t.Error("the primary key should still be id")
	}
	if updated.Columns[3].Nullable {
		t.Error("nickname was designed as NOT NULL")
	}

	indexNames := make([]string, 0, len(updated.Indexes))
	for _, ix := range updated.Indexes {
		indexNames = append(indexNames, ix.Name)
		if ix.Name == "idx_people_contact" {
			if !ix.Unique {
				t.Error("idx_people_contact should be unique")
			}
			if len(ix.Columns) != 1 || ix.Columns[0] != "contact" {
				t.Errorf("idx_people_contact should cover the renamed column, got %v", ix.Columns)
			}
		}
		if ix.Name == "idx_people_email" {
			t.Error("the old index name should be gone")
		}
	}
	if !contains(indexNames, "idx_people_contact") {
		t.Fatalf("expected the renamed index to exist, got %v", indexNames)
	}

	// The data survived the rename, and the new column took its default.
	rows, err := conn.Fetch(ctx, drivers.FetchRequest{Database: "main", Object: "people", Limit: 10})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if rows.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", rows.RowCount)
	}
	if rows.Rows[0][2] != "alice@example.com" || rows.Rows[1][2] != nil {
		t.Fatalf("contact should have kept the email values, got %v", rows.Rows)
	}
	if rows.Rows[0][3] != "anon" {
		t.Fatalf("nickname should default to anon, got %v", rows.Rows[0][3])
	}

	// And the result is stable: designing the new structure changes nothing.
	if again := applyDesign(t, conn, updated, designFrom(updated)); len(again.Statements) != 0 {
		t.Fatalf("the design should now be a no-op, got %v", again.Statements)
	}
}

func strptr(value string) *string { return &value }

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
