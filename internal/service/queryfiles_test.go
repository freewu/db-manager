package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// queryManager is a manager with a real store in a temp directory and one saved
// profile, which is what a query file has to belong to.
func queryManager(t *testing.T) (*Manager, models.ConnectionConfig) {
	t.Helper()
	manager, _ := managerOnDefaultDir(t)
	cfg, err := manager.SaveConnection(profileFor(t, "local"))
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	return manager, cfg
}

func TestQueryFilesNeedAConnectionAndADatabase(t *testing.T) {
	manager, cfg := queryManager(t)

	cases := []struct {
		name                 string
		connection, database string
		wantCode             string
	}{
		{"no connection", "", "shop", apperr.CodeInvalidConfig},
		{"no database", cfg.ID, "", apperr.CodeInvalidConfig},
		{"unknown connection", "nope", "shop", apperr.CodeNotFound},
	}
	for _, c := range cases {
		if _, err := manager.ListQueryFiles(c.connection, c.database); !apperr.Is(err, c.wantCode) {
			t.Errorf("%s: %v, want %s", c.name, err, c.wantCode)
		}
		if _, err := manager.SaveQueryFile(models.QueryFileSave{
			ConnectionID: c.connection, Database: c.database, Name: "q", SQL: "SELECT 1;",
		}); !apperr.Is(err, c.wantCode) {
			t.Errorf("%s (save): %v, want %s", c.name, err, c.wantCode)
		}
	}
}

func TestQueryNameIsRequiredAndBounded(t *testing.T) {
	manager, cfg := queryManager(t)

	for _, name := range []string{"", "   ", "...", strings.Repeat("x", maxQueryNameRunes+1)} {
		if _, err := manager.CreateQueryFile(cfg.ID, "shop", name, ""); !apperr.Is(err, apperr.CodeInvalidConfig) {
			t.Errorf("name %q: %v, want a refusal", name, err)
		}
	}
	if _, err := manager.CreateQueryFile(cfg.ID, "shop", strings.Repeat("x", maxQueryNameRunes), ""); err != nil {
		t.Fatalf("a name at the limit should be accepted: %v", err)
	}
}

func TestCreateQueryFileWritesTheTextItWasGiven(t *testing.T) {
	manager, cfg := queryManager(t)

	// Naming a scratchpad and writing it are one step: the file has to hold the
	// script the window was showing, not an empty one.
	created, err := manager.CreateQueryFile(cfg.ID, "shop", "orders", "SELECT 1;")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.SQL != "SELECT 1;" {
		t.Fatalf("created file holds %q, want the script it was given", created.SQL)
	}
	read, err := manager.ReadQueryFile(cfg.ID, "shop", "orders")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if read.SQL != "SELECT 1;" {
		t.Fatalf("on disk: %q", read.SQL)
	}
}

func TestCreateQueryFileRefusesAnExistingName(t *testing.T) {
	manager, cfg := queryManager(t)
	created, err := manager.CreateQueryFile(cfg.ID, "shop", "orders", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.SQL != "" {
		t.Fatalf("a new query file should be empty: %+v", created)
	}
	// A new window must not silently empty a script that is already there.
	if _, err := manager.CreateQueryFile(cfg.ID, "shop", "orders", ""); !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("creating over an existing query: %v", err)
	}
	if _, err := manager.CreateQueryFile(cfg.ID, "shop", "ORDERS", ""); !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("a name differing only in case is the same file: %v", err)
	}
}

func TestSaveAndReadQueryFileRoundTrip(t *testing.T) {
	manager, cfg := queryManager(t)

	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "daily", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, err := manager.ListQueryFiles(cfg.ID, "shop")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	read, err := manager.ReadQueryFile(cfg.ID, "shop", "daily")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.SQL != "SELECT 1;" {
		t.Fatalf("read back %q", read.SQL)
	}

	// The saved file lives under the data directory the app reports, so a user
	// can find it from the settings page.
	info, err := manager.DataDir()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if !strings.HasPrefix(read.Path, info.Path+string(filepath.Separator)) {
		t.Fatalf("%s is not inside the data directory %s", read.Path, info.Path)
	}
}

func TestSaveQueryFileRenamesThroughTheService(t *testing.T) {
	manager, cfg := queryManager(t)
	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "old", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := manager.RenameQueryFile(models.QueryFileRename{
		ConnectionID: cfg.ID, Database: "shop", From: "old", To: "new",
	}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	list, err := manager.ListQueryFiles(cfg.ID, "shop")
	if err != nil || len(list) != 1 || list[0].Name != "new" {
		t.Fatalf("after the rename: %+v, %v", list, err)
	}

	// Renaming onto a name that already holds a script is refused by name: the
	// other query would otherwise be deleted.
	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "new", SQL: "SELECT 3;",
	}); err != nil {
		t.Fatalf("saving over the open file should be fine: %v", err)
	}
	if _, err := manager.CreateQueryFile(cfg.ID, "shop", "other", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = manager.RenameQueryFile(models.QueryFileRename{
		ConnectionID: cfg.ID, Database: "shop", From: "new", To: "other",
	})
	if !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("renaming onto an existing query: %v", err)
	}
	if _, err := manager.ReadQueryFile(cfg.ID, "shop", "other"); err != nil {
		t.Fatalf("the refused rename damaged the other query: %v", err)
	}
}

func TestReadMissingQueryFileIsNotFound(t *testing.T) {
	manager, cfg := queryManager(t)

	if _, err := manager.ReadQueryFile(cfg.ID, "shop", "ghost"); !apperr.Is(err, apperr.CodeNotFound) {
		t.Fatalf("reading a missing file: %v", err)
	}
	if _, err := manager.ReadQueryFile(cfg.ID, "shop", ""); !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("reading with no name: %v", err)
	}
}

func TestDeleteQueryFileRemovesTheScript(t *testing.T) {
	manager, cfg := queryManager(t)
	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "temp", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := manager.DeleteQueryFile(cfg.ID, "shop", "temp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err := manager.ListQueryFiles(cfg.ID, "shop")
	if err != nil || len(list) != 0 {
		t.Fatalf("after the delete: %+v, %v", list, err)
	}
	if err := manager.DeleteQueryFile(cfg.ID, "shop", "temp"); err != nil {
		t.Fatalf("deleting twice should be a no-op: %v", err)
	}
}

// The scripts move with the rest of the data, and the store the running app uses
// is the one in the new directory.
func TestQueryFilesFollowAMove(t *testing.T) {
	manager, cfg := queryManager(t)
	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "orders", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "elsewhere")
	result, err := manager.MoveDataDir(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if result.Info.Path != dest {
		t.Fatalf("the app is on %s, want %s", result.Info.Path, dest)
	}
	if len(result.LeftBehind) != 0 {
		t.Fatalf("something was left behind: %v", result.LeftBehind)
	}

	// Read through the service: it has to look in the new directory, not the one
	// it was constructed with.
	if _, err := manager.ReadQueryFile(cfg.ID, "shop", "orders"); err != nil {
		t.Fatalf("read after the move: %v", err)
	}
	if _, err := manager.SaveQueryFile(models.QueryFileSave{
		ConnectionID: cfg.ID, Database: "shop", Name: "second", SQL: "SELECT 2;",
	}); err != nil {
		t.Fatalf("save after the move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".query", cfg.ID, "shop", "second.sql")); err != nil {
		t.Fatalf("the new script did not land in the new directory: %v", err)
	}
	if _, err := config.NewAt(dest); err != nil {
		t.Fatalf("the moved directory is not a usable store: %v", err)
	}
}
