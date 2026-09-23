package config

import (
	"os"
	"path/filepath"
	"testing"

	"dbmanager/internal/models"
)

// A fresh store allows the default cap and says what the cap may be set to, so
// the settings page can show the bounds instead of restating them.
func TestDataGenSettingsRoundTrip(t *testing.T) {
	store := escapedStore(t)

	settings, err := store.DataGenSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if settings.MaxRows != defaultMaxGenRows || settings.Default != defaultMaxGenRows {
		t.Fatalf("a fresh store should allow the default: %+v", settings)
	}
	if settings.Min != minMaxGenRows || settings.Max != maxMaxGenRows {
		t.Fatalf("unexpected bounds: %+v", settings)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), datagenSettingsFile)); !os.IsNotExist(err) {
		t.Fatal("nothing was chosen, so nothing should be written")
	}

	if err := store.SaveDataGenSettings(models.DataGenSettings{MaxRows: 5000}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if settings, err = store.DataGenSettings(); err != nil || settings.MaxRows != 5000 {
		t.Fatalf("settings = %+v (%v), want 5000", settings, err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), datagenSettingsFile)); err != nil {
		t.Fatalf("the choice was not written: %v", err)
	}

	// Going back to the default is the same as never having chosen: the file
	// goes away rather than saying what the default already says.
	if err := store.SaveDataGenSettings(models.DataGenSettings{MaxRows: defaultMaxGenRows}); err != nil {
		t.Fatalf("save default: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), datagenSettingsFile)); !os.IsNotExist(err) {
		t.Fatal("the settings file should be gone once the value is the default")
	}
	if settings, err = store.DataGenSettings(); err != nil || settings.MaxRows != defaultMaxGenRows {
		t.Fatalf("settings = %+v (%v), want the default back", settings, err)
	}
}

// A settings file this build cannot read is the default cap, for the same reason
// a corrupt state file is not fatal: it must not be able to keep the data
// generation window from opening.
func TestDataGenSettingsFallBackToTheDefault(t *testing.T) {
	for _, raw := range []string{
		"{not json",
		"{}",
		`{"version":1,"maxRows":1}`,
		`{"version":1,"maxRows":-5}`,
		`{"version":1,"maxRows":99999999999}`,
	} {
		store := escapedStore(t)
		if err := os.WriteFile(filepath.Join(store.Dir(), datagenSettingsFile), []byte(raw), fileMode); err != nil {
			t.Fatalf("write: %v", err)
		}
		settings, err := store.DataGenSettings()
		if err != nil {
			t.Fatalf("%s: reading settings must not fail: %v", raw, err)
		}
		if settings.MaxRows != defaultMaxGenRows {
			t.Fatalf("%s: expected the default, got %d", raw, settings.MaxRows)
		}
	}
}

// The cap is part of the data directory, so a move takes it along and the
// settings listing shows it - the same contract the change log settings are held
// to.
func TestMoveDataCarriesTheDataGenSettings(t *testing.T) {
	def := withConfigHome(t)
	store, err := NewAt(def)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SaveDataGenSettings(models.DataGenSettings{MaxRows: minMaxGenRows}); err != nil {
		t.Fatalf("save: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	if _, err := MoveData(dest); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, datagenSettingsFile)); err != nil {
		t.Fatalf("the cap did not travel with the rest of the data: %v", err)
	}
	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open the moved store: %v", err)
	}
	if settings, err := again.DataGenSettings(); err != nil || settings.MaxRows != minMaxGenRows {
		t.Fatalf("settings = %+v (%v), want %d", settings, err, minMaxGenRows)
	}
}
