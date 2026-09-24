package service

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// A copy is the design/create path with the structure read from the catalog
// instead of from a window, so these tests are about the two things the service
// adds: what it reads the source as, and where the run is recorded.

// copyOf is the request the explorer's menu sends: the table it was opened on,
// and the name typed into the window.
func copyOf(target string, withData bool) models.CopyTableRequest {
	return models.CopyTableRequest{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Target:    target,
		WithData:  withData,
	}
}

func TestPlanCopyTableReadsTheTableBeingCopied(t *testing.T) {
	manager, _ := testManager(t)

	plan, err := manager.PlanCopyTable(copyOf("orders_copy", false))
	if err != nil {
		t.Fatalf("plan copy: %v", err)
	}
	if len(plan.Statements) != 2 {
		t.Fatalf("expected the create and the index, got %v", plan.Statements)
	}
	if !strings.Contains(plan.Statements[0], `CREATE TABLE "orders_copy"`) {
		t.Fatalf("the create does not name the copy: %s", plan.Statements[0])
	}
	// The unique index of the fixture travels with the table, under the copy's
	// name: SQLite keeps index names in the database, where the original's is
	// already used.
	if !strings.Contains(plan.Statements[1], `"orders_copy_idx_orders_user"`) {
		t.Fatalf("the index was not renamed: %s", plan.Statements[1])
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "idx_orders_user is created as") {
		t.Fatalf("the rename must be said out loud: %v", plan.Warnings)
	}

	// Planning runs nothing: the source is still the only table there.
	objects, err := manager.Objects("s1", "main", "")
	if err != nil {
		t.Fatalf("objects: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != "orders" {
		t.Fatalf("planning created something: %+v", objects)
	}
}

func TestCopyTableCarriesTheStructureAndTheRows(t *testing.T) {
	manager, _ := testManager(t)

	result, err := manager.CopyTable(copyOf("orders_copy", true))
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if result.Error != "" || result.FailedIndex != -1 {
		t.Fatalf("the copy should have gone through: %+v", result)
	}
	if len(result.Executed) != 3 {
		t.Fatalf("expected the create, the index and the insert, got %v", result.Executed)
	}
	if !strings.Contains(result.Executed[2], "INSERT INTO") {
		t.Fatalf("the rows are moved by the last statement: %s", result.Executed[2])
	}

	// The rows never travelled through this program: the engine moved them.
	page, err := manager.Fetch(models.FetchRequest{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders_copy",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch the copy: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("expected both rows to be copied, got %d", page.Total)
	}

	structure, err := manager.Structure("s1", "main", "", "orders_copy")
	if err != nil {
		t.Fatalf("structure of the copy: %v", err)
	}
	if len(structure.Columns) != 3 || structure.Columns[0].Name != "id" {
		t.Fatalf("the fields did not travel: %+v", structure.Columns)
	}
	if !structure.Columns[0].PrimaryKey {
		t.Fatal("the primary key did not travel")
	}
	names := make([]string, 0, len(structure.Indexes))
	for _, index := range structure.Indexes {
		names = append(names, index.Name)
	}
	if len(names) != 1 || names[0] != "orders_copy_idx_orders_user" {
		t.Fatalf("the index did not travel: %v", names)
	}
}

func TestCopyTableWithoutDataLeavesTheCopyEmpty(t *testing.T) {
	manager, _ := testManager(t)

	result, err := manager.CopyTable(copyOf("orders_copy", false))
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if len(result.Executed) != 2 || result.Error != "" {
		t.Fatalf("only the structure should have been written: %+v", result)
	}

	page, err := manager.Fetch(models.FetchRequest{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders_copy",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch the copy: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("structure only means structure only, got %d rows", page.Total)
	}
}

func TestCopyTableRefusesWhatItCannotDo(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.CopyTable(copyOf("orders", true)); err == nil {
		t.Fatal("a table cannot be copied onto itself")
	}
	if _, err := manager.CopyTable(copyOf("   ", true)); err == nil {
		t.Fatal("a copy needs a name")
	}
	// The window only ever sends an object; a request without one is a bug on
	// the other side of the bridge, not a table to read.
	if _, err := manager.CopyTable(models.CopyTableRequest{SessionID: "s1", Target: "orders_copy"}); err == nil {
		t.Fatal("expected a request without a source to be refused")
	}

	manager.sessions["s1"].readOnly = true
	if _, err := manager.CopyTable(copyOf("orders_copy", true)); err == nil {
		t.Fatal("a read-only connection must not create a table")
	}
}

func TestCopyTableReportsWhereItStopped(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.CopyTable(copyOf("orders_copy", true)); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	// The same copy again: the name is taken, so the very first statement is
	// the one that fails and nothing else has run.
	result, err := manager.CopyTable(copyOf("orders_copy", true))
	if err != nil {
		t.Fatalf("second copy: %v", err)
	}
	if result.FailedIndex != 0 || result.Error == "" {
		t.Fatalf("the failure must be reported against the create: %+v", result)
	}
	if len(result.Executed) != 0 {
		t.Fatalf("nothing after a failed create may run: %v", result.Executed)
	}
}

func TestCopyTableIsLoggedAsACopy(t *testing.T) {
	manager := loggedManager(t)

	if _, err := manager.CopyTable(copyOf("orders_copy", true)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	log, total := entries(t, manager)
	if total != 3 {
		t.Fatalf("expected the create, the index and the insert in the log, got %d", total)
	}
	// Newest first: the insert, the index, then the create.
	if log[0].Kind != "insert" || log[1].Kind != "create" || log[2].Kind != "create" {
		t.Fatalf("unexpected kinds: %q %q %q", log[0].Kind, log[1].Kind, log[2].Kind)
	}
	if !strings.HasPrefix(log[0].Statement, "INSERT INTO") {
		t.Fatalf("the newest entry should be the row copy: %q", log[0].Statement)
	}
	// Every statement of a copy is about the table being created, so the log
	// points at it — and the create itself has no table yet, exactly as the
	// other windows record a CREATE.
	for i, entry := range log {
		want := "orders_copy"
		if i == 2 {
			want = ""
		}
		if entry.Table != want {
			t.Fatalf("entry %d names %q, expected %q", i, entry.Table, want)
		}
	}
}
