package service

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// Deleting a row is the other half of the grid's editing, and it is the one that
// cannot be undone: what it ran, and how much of it, is what the grid shows
// before it does and what the log keeps afterwards.

// rowDelete is the request the grid builds for one row of `orders`.
func rowDelete(id int) models.RowDelete {
	return models.RowDelete{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Key:       []models.KeyValue{{Column: "id", Value: id}},
	}
}

// The preview has to be the statement that runs: it is what a selection is
// confirmed against, and a plan that differs from the run is worse than none.
func TestDeleteRowRunsTheStatementItPlanned(t *testing.T) {
	manager := loggedManager(t)

	statement, err := manager.PlanRowDelete(rowDelete(2))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.HasPrefix(statement, "DELETE FROM ") {
		t.Fatalf("unexpected statement: %q", statement)
	}
	if !strings.Contains(statement, "orders") || !strings.Contains(statement, "id") {
		t.Fatalf("the statement does not name the row it removes: %q", statement)
	}
	// A preview is a read: the row is still there, and nothing was logged.
	if got := cell(t, manager, 2, "id"); got != "2" {
		t.Fatalf("the preview removed the row: %+v", got)
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a plan is not a change, but the log holds %+v", log)
	}

	affected, err := manager.DeleteRow(rowDelete(2))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if got := cell(t, manager, 1, "id"); got != "1" {
		t.Fatalf("the other row went too: %+v", got)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("one statement was run, so one entry is expected: %+v", log)
	}
	entry := log[0]
	if entry.Source != models.ChangeSourceGrid || entry.Kind != "delete" || entry.Table != "orders" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	// What ran is the string that was previewed, and one row went.
	if entry.Statement != statement {
		t.Fatalf("the log kept %q, the preview showed %q", entry.Statement, statement)
	}
	if entry.Rows != 1 {
		t.Fatalf("the entry should say one row went, got %d: %+v", entry.Rows, entry)
	}
	if entry.Error != "" {
		t.Fatalf("unexpected error: %q", entry.Error)
	}
}

// A deletion that matched nothing is still a statement that was sent: the log
// says so, and the count that comes with it is the engine's own.
func TestDeleteRowRecordsAStatementThatMatchedNothing(t *testing.T) {
	manager := loggedManager(t)

	if _, err := manager.DeleteRow(rowDelete(9999)); err != nil {
		t.Fatalf("delete: %v", err)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("expected the statement to be logged: %+v", log)
	}
	if log[0].Rows != 0 {
		t.Fatalf("nothing matched, so no row count: %+v", log[0])
	}
}

// A row that cannot be identified again must not be deleted, and the preview and
// the run have to agree about it: the box cannot show a statement for something
// that would never be sent.
func TestDeleteRowRefusesAnIdentityItCannotUse(t *testing.T) {
	manager := loggedManager(t)
	none := models.RowDelete{SessionID: "s1", Database: "main", Object: "orders"}

	if _, err := manager.PlanRowDelete(none); err == nil {
		t.Fatalf("the preview accepted %+v", none)
	}
	if _, err := manager.DeleteRow(none); err == nil {
		t.Fatalf("the run accepted %+v", none)
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a refused request did not run, so nothing may be logged: %+v", log)
	}
}

func TestDeleteRowRefusesAReadOnlyConnection(t *testing.T) {
	manager := loggedManager(t)
	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.readOnly = true

	if _, err := manager.DeleteRow(rowDelete(1)); err == nil {
		t.Fatal("a read-only connection must refuse the deletion")
	}
	if got := cell(t, manager, 1, "id"); got != "1" {
		t.Fatalf("the row went anyway: %+v", got)
	}
}
