package service

import (
	"fmt"
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// The row detail layer edits several columns and sends them as one statement, and
// the change log is written by the layer that runs it. These tests drive both
// through the manager, against the real sqlite fixture, and then look at the row
// and at the log.

// rowUpdate is the request the row detail layer builds for row 1 of `orders`.
func rowUpdate(values ...models.KeyValue) models.RowUpdate {
	return models.RowUpdate{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Key:       []models.KeyValue{{Column: "id", Value: 1}},
		Values:    values,
	}
}

// cell reads one column of one row back through the fixture.
func cell(t *testing.T, manager *Manager, id int, column string) string {
	t.Helper()
	result, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "main",
		SQL:      fmt.Sprintf("SELECT %s FROM orders WHERE id = %d;", column, id),
	})
	if err != nil {
		t.Fatalf("read %s: %v", column, err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("row %d is gone: %+v", id, result.Rows)
	}
	return fmt.Sprint(result.Rows[0][0])
}

func TestUpdateRowChangesEveryColumnInOneStatement(t *testing.T) {
	manager := loggedManager(t)

	affected, err := manager.UpdateRow(rowUpdate(
		models.KeyValue{Column: "user_id", Value: "9"},
		models.KeyValue{Column: "total", Value: "42.5"},
	))
	if err != nil {
		t.Fatalf("update row: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if got := cell(t, manager, 1, "user_id"); got != "9" {
		t.Fatalf("user_id = %s, want 9", got)
	}
	if got := cell(t, manager, 1, "total"); got != "42.5" {
		t.Fatalf("total = %s, want 42.5", got)
	}
	// The other row is untouched: the WHERE identified one row.
	if got := cell(t, manager, 2, "user_id"); got != "8" {
		t.Fatalf("row 2 was caught by the update: user_id = %s", got)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("one statement was run, so one entry is expected: %+v", log)
	}
	entry := log[0]
	if entry.Source != models.ChangeSourceGrid || entry.Kind != "update" {
		t.Fatalf("a grid edit must be logged as one: %+v", entry)
	}
	if entry.Table != "orders" || entry.Database != "main" || entry.Error != "" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	// The logged text is the statement the driver rendered, with every column in
	// it — the same string the preview showed.
	for _, column := range []string{"user_id", "total"} {
		if !strings.Contains(entry.Statement, column) {
			t.Fatalf("the statement does not mention %s: %q", column, entry.Statement)
		}
	}
	if !strings.Contains(entry.Statement, "WHERE") {
		t.Fatalf("the statement lost its row identity: %q", entry.Statement)
	}
}

// A preview is a read of the plan, not a change: it must not touch the row and it
// must not leave a line in the log.
func TestPlanRowUpdateRendersWithoutRunningOrLogging(t *testing.T) {
	manager := loggedManager(t)

	req := rowUpdate(
		models.KeyValue{Column: "user_id", Value: "9"},
		models.KeyValue{Column: "total", Value: "42.5"},
	)
	statement, err := manager.PlanRowUpdate(req)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.HasPrefix(statement, "UPDATE ") {
		t.Fatalf("unexpected statement: %q", statement)
	}
	if !strings.Contains(statement, "user_id") || !strings.Contains(statement, "total") {
		t.Fatalf("the preview does not spell every changed column: %q", statement)
	}

	if got := cell(t, manager, 1, "user_id"); got != "7" {
		t.Fatalf("the preview changed the row: user_id = %s", got)
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a plan is not a change, but the log holds %+v", log)
	}
}

// The preview and the run must refuse the same requests with the same answer, or
// the box would show a statement for something that can never run.
func TestRowUpdateRefusesAnIdentityItCannotUse(t *testing.T) {
	manager := loggedManager(t)

	none := models.RowUpdate{SessionID: "s1", Database: "main", Object: "orders",
		Values: []models.KeyValue{{Column: "total", Value: "1"}}}
	empty := rowUpdate()

	for _, req := range []models.RowUpdate{none, empty} {
		if _, err := manager.PlanRowUpdate(req); err == nil {
			t.Fatalf("the preview accepted %+v", req)
		}
		if _, err := manager.UpdateRow(req); err == nil {
			t.Fatalf("the run accepted %+v", req)
		}
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a refused request did not run, so nothing may be logged: %+v", log)
	}
}

func TestUpdateRowRefusesAReadOnlyConnection(t *testing.T) {
	manager := loggedManager(t)
	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.readOnly = true

	if _, err := manager.UpdateRow(rowUpdate(models.KeyValue{Column: "total", Value: "1"})); err == nil {
		t.Fatal("a read-only connection must refuse the edit")
	} else if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cell(t, manager, 1, "total"); got != "10" {
		t.Fatalf("the row changed anyway: total = %s", got)
	}
}

// A statement that was sent and failed was still asked for, so it is logged with
// the engine's message on it: the log answers "what was this database asked to
// do", not "what succeeded".
func TestUpdateRowLogsAStatementTheEngineRefused(t *testing.T) {
	manager := loggedManager(t)

	// user_id is NOT NULL, and the editor's empty field means NULL.
	_, err := manager.UpdateRow(rowUpdate(models.KeyValue{Column: "user_id", Value: nil}))
	if err == nil {
		t.Fatal("expected the engine to refuse a NULL for a NOT NULL column")
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("the failed statement must be the one entry: %+v", log)
	}
	if log[0].Source != models.ChangeSourceGrid || log[0].Kind != "update" {
		t.Fatalf("unexpected entry: %+v", log[0])
	}
	if !strings.Contains(log[0].Error, "NOT NULL") {
		t.Fatalf("the entry does not carry what the engine said: %q", log[0].Error)
	}
	if got := cell(t, manager, 1, "user_id"); got != "7" {
		t.Fatalf("the row changed despite the failure: user_id = %s", got)
	}
}

// A single-cell edit is the same write, narrowed to one column: it goes through
// the same path, so it is logged the same way.
func TestUpdateCellIsLoggedAsAGridChange(t *testing.T) {
	manager := loggedManager(t)

	affected, err := manager.UpdateCell(models.CellUpdate{SessionID: "s1",
		Database: "main",
		Object:   "orders",
		Key:      []models.KeyValue{{Column: "id", Value: 1}},
		Column:   "total",
		Value:    "99",
	})
	if err != nil {
		t.Fatalf("update cell: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("expected one entry, got %+v", log)
	}
	if log[0].Source != models.ChangeSourceGrid || log[0].Kind != "update" {
		t.Fatalf("unexpected entry: %+v", log[0])
	}
	if !strings.Contains(log[0].Statement, "total") {
		t.Fatalf("the statement does not name the edited column: %q", log[0].Statement)
	}
}
