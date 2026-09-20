package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// The connect dialog promises that a password typed into it is kept when the
// profile asked for that ("remember password") — which is what stops the dialog
// from showing up again on every connect. The service owns that rule, not the
// drivers, so these tests drive Manager.Open against a real (temp) store.

func connectManager(t *testing.T) (*Manager, *config.Store) {
	t.Helper()
	store, err := config.NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Manager{
		baseCtx:  context.Background(),
		store:    store,
		sessions: map[string]*session{},
	}, store
}

// sqliteProfile saves a profile against a real (empty) file so Open can succeed
// without any server. SQLite ignores passwords; what is under test here is the
// bookkeeping around them.
func sqliteProfile(t *testing.T, manager *Manager, name string, savePassword bool) models.ConnectionConfig {
	t.Helper()
	file := filepath.Join(t.TempDir(), "fixture.db")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	profile, err := manager.SaveConnection(models.ConnectionConfig{
		Name:         name,
		Driver:       models.DriverSQLite,
		FilePath:     file,
		Database:     "main",
		SavePassword: savePassword,
	})
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	return profile
}

func TestTypedPasswordIsRememberedWhenTheProfileOptsIn(t *testing.T) {
	manager, store := connectManager(t)
	profile := sqliteProfile(t, manager, "remembers", true)

	session, err := manager.Open(models.OpenRequest{ConnectionID: profile.ID, Password: "s3cret"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close(session.ID) })

	stored, found, err := store.Find(profile.ID)
	if err != nil || !found {
		t.Fatalf("read back the profile: found=%v err=%v", found, err)
	}
	if stored.Password != "s3cret" {
		t.Fatalf("the typed password must be kept, got %q", stored.Password)
	}

	// And the list the UI reads has to say so, because `hasPassword` is what
	// keeps the next connect from asking at all.
	list, err := manager.Connections()
	if err != nil {
		t.Fatalf("connections: %v", err)
	}
	if len(list) != 1 || !list[0].HasPassword {
		t.Fatalf("the UI must be told a password is stored: %+v", list)
	}
	if list[0].Password != "" {
		t.Fatal("the stored password must never travel back to the UI")
	}
}

func TestTypedPasswordStaysInMemoryWithoutTheOptIn(t *testing.T) {
	manager, store := connectManager(t)
	profile := sqliteProfile(t, manager, "forgets", false)

	session, err := manager.Open(models.OpenRequest{ConnectionID: profile.ID, Password: "hunter2"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close(session.ID) })

	stored, found, err := store.Find(profile.ID)
	if err != nil || !found {
		t.Fatalf("read back the profile: found=%v err=%v", found, err)
	}
	if stored.Password != "" {
		t.Fatalf("without the opt-in the secret must not be written, got %q", stored.Password)
	}

	list, err := manager.Connections()
	if err != nil {
		t.Fatalf("connections: %v", err)
	}
	if list[0].HasPassword {
		t.Fatal("the profile does not opt in, so there is nothing to report")
	}
}
