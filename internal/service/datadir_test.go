package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dbmanager/internal/config"
	"dbmanager/internal/models"
	// The store only needs *a* driver to validate a profile against; sqlite
	// needs no server, so this test never touches the network.
	_ "dbmanager/internal/drivers/sqlite"
)

// profileFor builds a profile the sqlite driver accepts: Normalize refuses a
// file that does not exist, since opening one would silently create an empty
// database.
func profileFor(t *testing.T, name string) models.ConnectionConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("create database file: %v", err)
	}
	return models.ConnectionConfig{Name: name, Driver: models.DriverSQLite, FilePath: path}
}

// The data directory comes from the OS, so the tests point that at a temp
// directory instead of the real user profile. Windows reads %AppData%,
// everything else reads $XDG_CONFIG_HOME.
func testConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", home)
	} else {
		t.Setenv("XDG_CONFIG_HOME", home)
	}
	dir, err := config.DefaultDir()
	if err != nil {
		t.Fatalf("default dir: %v", err)
	}
	return dir
}

// managerOnDefaultDir is a manager whose store sits exactly where the app's
// store sits on a fresh install, which is what makes a move meaningful: the
// source and the destination are then two different directories on disk.
func managerOnDefaultDir(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := testConfigHome(t)
	store, err := config.NewAt(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Manager{store: store}, dir
}

func TestDataDirDescribesTheStoreInUse(t *testing.T) {
	manager, dir := managerOnDefaultDir(t)

	info, err := manager.DataDir()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if info.Path != dir || !info.IsDefault {
		t.Fatalf("unexpected description: %+v", info)
	}

	if _, err := manager.SaveConnection(profileFor(t, "local")); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err = manager.DataDir()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if len(info.Files) != 1 || info.Files[0].Name != "connections.json" {
		t.Fatalf("expected the profile file to be listed: %+v", info.Files)
	}
	if info.TotalBytes == 0 {
		t.Fatal("the profile file is empty, so nothing was written")
	}
}

func TestMoveDataDirSwitchesTheRunningAppOver(t *testing.T) {
	manager, dir := managerOnDefaultDir(t)
	if _, err := manager.SaveConnection(profileFor(t, "local")); err != nil {
		t.Fatalf("save: %v", err)
	}

	target := filepath.Join(t.TempDir(), "data")
	result, err := manager.MoveDataDir(target)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if result.Info.Path != target || result.Info.IsDefault {
		t.Fatalf("unexpected result: %+v", result.Info)
	}

	// The running app has to be on the new directory already: the store is what
	// the next save goes through, and writing into the directory that was just
	// emptied would lose that save on the next start.
	if manager.ConfigDir() != target {
		t.Fatalf("ConfigDir() = %s, want %s", manager.ConfigDir(), target)
	}
	if _, err := os.Stat(filepath.Join(dir, "connections.json")); !os.IsNotExist(err) {
		t.Fatalf("the old directory should be empty of profiles, err=%v", err)
	}

	// And the profiles moved with it, passwords included — reading them back
	// through the store proves the key file travelled too.
	profiles, err := manager.Connections()
	if err != nil {
		t.Fatalf("list after move: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "local" {
		t.Fatalf("profiles after move: %+v", profiles)
	}
	if _, err := manager.SaveConnection(profileFor(t, "second")); err != nil {
		t.Fatalf("save after move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "connections.json")); err != nil {
		t.Fatalf("the new save should land in the new directory: %v", err)
	}

	// Moving back to the default is the same call with no target.
	back, err := manager.MoveDataDir("")
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if !back.Info.IsDefault || back.Info.Path != dir {
		t.Fatalf("unexpected result: %+v", back.Info)
	}
	if manager.ConfigDir() != dir {
		t.Fatalf("ConfigDir() = %s, want %s", manager.ConfigDir(), dir)
	}
	profiles, err = manager.Connections()
	if err != nil {
		t.Fatalf("list after moving back: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("profiles after moving back: %+v", profiles)
	}
}

func TestMoveDataDirRefusesItsOwnDirectory(t *testing.T) {
	manager, dir := managerOnDefaultDir(t)

	if _, err := manager.MoveDataDir(dir); err == nil {
		t.Fatal("expected a refusal")
	}
	if manager.ConfigDir() != dir {
		t.Fatalf("a refused move changed the directory: %s", manager.ConfigDir())
	}
}
