package service

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// Emptying a table and removing one are the two writes the explorer's table menu
// offers without opening a window first, so these tests are about the two things
// the service adds around the renderer: what it reads the table as, and where
// the statement is recorded.

// opOn is the request the explorer's menu sends: the row it was opened on and
// the table's own name, with nothing else to decide.
func opOn(name string) models.TableOpRequest {
	return models.TableOpRequest{
		SessionID: "s1",
		Database:  "main",
		Object:    name,
	}
}

func TestPlanDropTableReadsTheTableBeforeAnythingRuns(t *testing.T) {
	manager, _ := testManager(t)

	plan, err := manager.PlanDropTable(opOn("orders"))
	if err != nil {
		t.Fatalf("plan drop: %v", err)
	}
	assertPlanStatements(t, plan.Statements, []string{`DROP TABLE "orders"`})
	if !plan.Destructive {
		t.Fatal("dropping a table is destructive, and the confirmation reads that flag")
	}

	// Planning runs nothing: the table is still the one the explorer lists.
	objects, err := manager.Objects("s1", "main", "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != "orders" {
		t.Fatalf("planning dropped something: %+v", objects)
	}
}

func TestDropTableRemovesTheTable(t *testing.T) {
	manager, _ := testManager(t)

	result, err := manager.DropTable(opOn("orders"))
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	if result.Error != "" || result.FailedIndex != -1 || len(result.Executed) != 1 {
		t.Fatalf("the drop should have gone through: %+v", result)
	}

	objects, err := manager.Objects("s1", "main", "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("the table is still there: %+v", objects)
	}
	// The rows went with it, which is the whole reason the confirmation is a
	// danger one.
	if _, err := manager.Structure("s1", "main", "", "orders"); err == nil {
		t.Fatal("expected the table to be gone")
	}
}

func TestTruncateTableEmptiesItAndKeepsIt(t *testing.T) {
	manager, _ := testManager(t)

	plan, err := manager.PlanTruncateTable(opOn("orders"))
	if err != nil {
		t.Fatalf("plan truncate: %v", err)
	}
	// SQLite has no TRUNCATE, so the script is a DELETE — and the plan says so
	// rather than leaving the difference for the user to notice afterwards.
	assertPlanStatements(t, plan.Statements, []string{`DELETE FROM "orders"`})
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "SQLite has no TRUNCATE") {
		t.Fatalf("the engine's substitution must be said out loud: %v", plan.Warnings)
	}

	result, err := manager.TruncateTable(opOn("orders"))
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if result.Error != "" || len(result.Executed) != 1 {
		t.Fatalf("the emptying should have gone through: %+v", result)
	}

	// The rows are gone; the table, its fields and its index are not. That is
	// the difference between these two menu entries, so it is what is checked.
	page, err := manager.Fetch(models.FetchRequest{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("the table should be empty, got %d rows", page.Total)
	}
	structure, err := manager.Structure("s1", "main", "", "orders")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	if len(structure.Columns) != 3 || len(structure.Indexes) != 1 {
		t.Fatalf("emptying a table must not touch its shape: %+v", structure)
	}
}

func TestTableOpsRefuseWhatTheyCannotDo(t *testing.T) {
	manager, _ := testManager(t)

	// The menu only ever sends an object; a request without one is a bug on the
	// other side of the bridge, not a table to act on.
	if _, err := manager.DropTable(opOn("   ")); err == nil {
		t.Fatal("expected a request without a table to be refused")
	}
	if _, err := manager.PlanTruncateTable(models.TableOpRequest{SessionID: "s1"}); err == nil {
		t.Fatal("expected a request without a table to be refused")
	}
	// An unknown session is a stale window, not an engine error.
	if _, err := manager.DropTable(models.TableOpRequest{SessionID: "gone", Object: "orders"}); err == nil {
		t.Fatal("expected an unknown session to be refused")
	}

	manager.sessions["s1"].readOnly = true
	if _, err := manager.DropTable(opOn("orders")); err == nil {
		t.Fatal("a read-only connection must not drop a table")
	}
	if _, err := manager.TruncateTable(opOn("orders")); err == nil {
		t.Fatal("a read-only connection must not empty a table")
	}
}

func TestTableOpsAreLoggedAgainstTheTableTheyChanged(t *testing.T) {
	manager := loggedManager(t)

	if _, err := manager.TruncateTable(opOn("orders")); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := manager.DropTable(opOn("orders")); err != nil {
		t.Fatalf("drop: %v", err)
	}

	log, total := entries(t, manager)
	if total != 2 {
		t.Fatalf("expected the emptying and the drop in the log, got %d", total)
	}
	// Newest first: the drop, then the DELETE SQLite spelled what was asked for.
	if log[0].Kind != "drop" || log[1].Kind != "delete" {
		t.Fatalf("unexpected kinds: %q %q", log[0].Kind, log[1].Kind)
	}
	for _, entry := range log {
		if entry.Table != "orders" {
			t.Fatalf("the entry should point at the table it changed, got %q", entry.Table)
		}
		if entry.Source != models.ChangeSourceExplorer {
			t.Fatalf("both come from the explorer's menu, got %q", entry.Source)
		}
		if entry.Error != "" {
			t.Fatalf("neither should have failed: %q", entry.Error)
		}
	}
}

// assertPlanStatements compares a plan's script, which is the one thing every
// caller of a plan cares about.
func assertPlanStatements(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d statement(s), got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d:\n got: %s\nwant: %s", i+1, got[i], want[i])
		}
	}
}
