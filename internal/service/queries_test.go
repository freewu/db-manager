package service

import (
	"testing"

	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// The favourites feature has no database dependency: it is a small CRUD layer
// over the config store, so these tests only need a manager wired to a temp
// store (no sessions).

func favouritesManager(t *testing.T) *Manager {
	t.Helper()
	store, err := config.NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Manager{store: store}
}

func TestSaveSavedQueryValidatesInput(t *testing.T) {
	manager := favouritesManager(t)

	if _, err := manager.SaveSavedQuery(models.SavedQuery{SQL: "SELECT 1"}); err == nil {
		t.Fatal("expected an error for a missing name")
	}
	if _, err := manager.SaveSavedQuery(models.SavedQuery{Name: "   ", SQL: "SELECT 1"}); err == nil {
		t.Fatal("expected an error for a blank name")
	}
	if _, err := manager.SaveSavedQuery(models.SavedQuery{Name: "empty", SQL: "  \n "}); err == nil {
		t.Fatal("expected an error for empty SQL")
	}

	// The whitespace around a pasted statement must not be persisted.
	saved, err := manager.SaveSavedQuery(models.SavedQuery{Name: "  ping  ", SQL: "\n SELECT 1 \n"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Name != "ping" || saved.SQL != "SELECT 1" {
		t.Fatalf("expected trimmed values, got %+v", saved)
	}
}

func TestSavedQueryLifecycleThroughTheManager(t *testing.T) {
	manager := favouritesManager(t)

	saved, err := manager.SaveSavedQuery(models.SavedQuery{
		Name:     "customer orders",
		SQL:      "SELECT * FROM orders",
		Database: "shop",
		Driver:   models.DriverSQLite,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("the backend should assign an id")
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("timestamps not stamped: %+v", saved)
	}

	// A second one, to prove listing and ordering.
	if _, err := manager.SaveSavedQuery(models.SavedQuery{Name: "alpha report", SQL: "SELECT 2"}); err != nil {
		t.Fatalf("save second: %v", err)
	}

	list, err := manager.SavedQueries()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 saved queries, got %d", len(list))
	}
	// Sorted case-insensitively by name: "alpha report" first.
	if list[0].Name != "alpha report" || list[1].Name != "customer orders" {
		t.Fatalf("unexpected order: %q, %q", list[0].Name, list[1].Name)
	}
	if list[1].Database != "shop" || list[1].Driver != models.DriverSQLite {
		t.Fatalf("hints lost: %+v", list[1])
	}

	// Updating by id renames in place instead of adding a row.
	renamed := list[1]
	renamed.Name = "orders (all)"
	renamed.SQL = "SELECT id FROM orders"
	updated, err := manager.SaveSavedQuery(renamed)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("update must keep createdAt: %d != %d", updated.CreatedAt, saved.CreatedAt)
	}

	list, err = manager.SavedQueries()
	if err != nil {
		t.Fatalf("list after update: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("update duplicated the entry: %d", len(list))
	}
	if list[1].Name != "orders (all)" || list[1].SQL != "SELECT id FROM orders" {
		t.Fatalf("update did not stick: %+v", list[1])
	}

	if err := manager.DeleteSavedQuery(saved.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = manager.SavedQueries()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 1 || list[0].Name != "alpha report" {
		t.Fatalf("unexpected list after delete: %+v", list)
	}

	if err := manager.DeleteSavedQuery("  "); err == nil {
		t.Fatal("expected an error for a blank id")
	}
}
