package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/models"
	"dbmanager/internal/secret"
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

// --- passwords at rest -----------------------------------------------------

func testProfile(name string, savePassword bool) models.ConnectionConfig {
	return models.ConnectionConfig{
		ID:           name,
		Name:         name,
		Driver:       models.DriverMySQL,
		Host:         "localhost",
		Port:         3306,
		Username:     "root",
		Password:     "s3cret",
		SavePassword: savePassword,
	}
}

func rawProfiles(t *testing.T, store *Store) string {
	t.Helper()
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read profile file: %v", err)
	}
	return string(raw)
}

func TestSavedPasswordIsSealedOnDisk(t *testing.T) {
	store := testStore(t)
	if _, err := store.Upsert(testProfile("keeps", true)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	file := rawProfiles(t, store)
	if strings.Contains(file, "s3cret") {
		t.Fatalf("the password is on disk in the clear:\n%s", file)
	}
	if !strings.Contains(file, secret.TokenPrefix) {
		t.Fatalf("expected a sealed token in the file:\n%s", file)
	}

	// The key lives next to the profiles, and the store must be able to read
	// its own token back — including through a fresh instance, which is what
	// happens on the next start.
	reopened, err := NewAt(store.Dir())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list, err := reopened.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(list) != 1 || list[0].Password != "s3cret" {
		t.Fatalf("the password did not come back: %+v", list)
	}
}

func TestPasswordWithoutOptInIsNotWritten(t *testing.T) {
	store := testStore(t)
	if _, err := store.Upsert(testProfile("forgets", false)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if file := rawProfiles(t, store); strings.Contains(file, "s3cret") || strings.Contains(file, "password") {
		t.Fatalf("a profile that did not opt in must carry no password:\n%s", file)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), "secret.key")); !os.IsNotExist(err) {
		t.Fatalf("no key should be created for a profile without a secret (err=%v)", err)
	}
}

func TestBlankPasswordKeepsTheStoredOne(t *testing.T) {
	store := testStore(t)
	stored, err := store.Upsert(testProfile("keeps", true))
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Editing anything else in the dialog sends no password, which means "keep
	// what is stored" rather than "forget it".
	stored.Password = ""
	stored.Host = "db.internal"
	updated, err := store.Upsert(stored)
	if err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	if updated.Password != "s3cret" {
		t.Fatalf("the stored password was dropped: %+v", updated)
	}
	if file := rawProfiles(t, store); strings.Contains(file, "s3cret") {
		t.Fatalf("the password came back to disk in the clear:\n%s", file)
	}
}

func TestPlainTextPasswordFromAnOlderBuildIsMigrated(t *testing.T) {
	store := testStore(t)
	// What an older build wrote: version 1 file, password in the clear.
	seed := `{"version":1,"connections":[{"id":"legacy","name":"legacy","driver":"mysql",` +
		`"host":"localhost","port":3306,"password":"s3cret","savePassword":true}]}`
	if err := os.WriteFile(store.Path(), []byte(seed), fileMode); err != nil {
		t.Fatalf("seed legacy file: %v", err)
	}

	list, err := store.Load()
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	if len(list) != 1 || list[0].Password != "s3cret" {
		t.Fatalf("a legacy password must keep working: %+v", list)
	}

	// Saving any other change seals it, so the plain-text window closes on the
	// first write after the upgrade.
	list[0].Name = "legacy (renamed)"
	if _, err := store.Upsert(list[0]); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if file := rawProfiles(t, store); strings.Contains(file, "s3cret") {
		t.Fatalf("the upgrade did not seal the legacy password:\n%s", file)
	}
}

// storedToken pulls the sealed password back out of the profile file.
func storedToken(t *testing.T, store *Store) string {
	t.Helper()
	var parsed struct {
		Connections []struct {
			Password string `json:"password"`
		} `json:"connections"`
	}
	if err := json.Unmarshal([]byte(rawProfiles(t, store)), &parsed); err != nil {
		t.Fatalf("decode profile file: %v", err)
	}
	if len(parsed.Connections) != 1 {
		t.Fatalf("expected one profile, got %d", len(parsed.Connections))
	}
	return parsed.Connections[0].Password
}

func TestUnreadableTokenIsKeptForAnotherMachine(t *testing.T) {
	store := testStore(t)
	if _, err := store.Upsert(testProfile("keeps", true)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	before := storedToken(t, store)
	if !secret.IsSealed(before) {
		t.Fatalf("expected a sealed password, got %q", before)
	}

	// A different key: the token cannot be opened here, but it is still the
	// password on the machine that wrote it, so a save must not destroy it.
	if err := os.Remove(filepath.Join(store.Dir(), "secret.key")); err != nil {
		t.Fatalf("remove key: %v", err)
	}
	elsewhere, err := NewAt(store.Dir())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list, err := elsewhere.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(list) != 1 || list[0].Password != "" {
		t.Fatalf("an unreadable token must not surface as a password: %+v", list)
	}

	list[0].Name = "renamed elsewhere"
	if _, err := elsewhere.Upsert(list[0]); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if after := storedToken(t, store); after != before {
		t.Fatalf("the token changed while it was unreadable:\nbefore: %s\nafter:  %s", before, after)
	}
}
