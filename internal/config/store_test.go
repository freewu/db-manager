package config

import (
	"os"
	"path/filepath"
	"testing"

	"dbmanager/internal/models"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewAt(filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestQueryFavouritesRoundTrip(t *testing.T) {
	store := testStore(t)

	// An untouched install has no file at all and must read back cleanly.
	list, err := store.LoadQueries()
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no favourites, got %d", len(list))
	}

	first := models.SavedQuery{ID: "q1", Name: "slow queries", SQL: "SELECT 1", Database: "shop"}
	if _, err := store.UpsertQuery(first); err != nil {
		t.Fatalf("upsert first: %v", err)
	}
	second := models.SavedQuery{ID: "q2", Name: "locks", SQL: "SELECT 2"}
	if _, err := store.UpsertQuery(second); err != nil {
		t.Fatalf("upsert second: %v", err)
	}

	// Upserting an existing id must replace in place, not append.
	first.Name = "slow queries (v2)"
	first.SQL = "SELECT 3"
	if _, err := store.UpsertQuery(first); err != nil {
		t.Fatalf("update first: %v", err)
	}

	// A second store instance sees what the first one wrote: this is the whole
	// point of persisting to disk.
	reopened, err := NewAt(store.Dir())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list, err = reopened.LoadQueries()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 favourites, got %d", len(list))
	}
	if list[0].ID != "q1" || list[0].Name != "slow queries (v2)" || list[0].SQL != "SELECT 3" {
		t.Fatalf("update did not stick: %+v", list[0])
	}
	if list[0].Database != "shop" {
		t.Fatalf("database hint lost: %+v", list[0])
	}

	if err := store.DeleteQuery("q1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = store.LoadQueries()
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if len(list) != 1 || list[0].ID != "q2" {
		t.Fatalf("unexpected list after delete: %+v", list)
	}

	// Deleting something that is not there is not an error.
	if err := store.DeleteQuery("nope"); err != nil {
		t.Fatalf("delete unknown: %v", err)
	}
}

func TestQueryFavouritesAreSeparateFromProfiles(t *testing.T) {
	store := testStore(t)

	if _, err := store.UpsertQuery(models.SavedQuery{ID: "q1", Name: "n", SQL: "SELECT 1"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Saving a favourite must not create (or touch) connections.json, and the
	// profile loader must not see the query file.
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("saving a favourite created %s (err=%v)", store.Path(), err)
	}
	profiles, err := store.Load()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(profiles))
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), queriesFile)); err != nil {
		t.Fatalf("queries file missing: %v", err)
	}
}

func TestQueryFavouritesSurviveCorruptFile(t *testing.T) {
	store := testStore(t)
	path := filepath.Join(store.Dir(), queriesFile)
	if err := os.WriteFile(path, []byte("{not json"), fileMode); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	if _, err := store.LoadQueries(); err == nil {
		t.Fatal("a corrupt file should surface as an error, not as an empty list")
	}
}
