package service

import (
	"testing"

	"dbmanager/internal/models"
)

// The data generation cap is the user's, and the backend is where it is settled:
// an answer the frontend could have guessed is not a check.
func TestDataGenSettingsAreCheckedAndBounded(t *testing.T) {
	manager := loggedManager(t)

	settings, err := manager.DataGenSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if settings.MaxRows != settings.Default || settings.Min >= settings.Max {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	saved, err := manager.SaveDataGenSettings(models.DataGenSettings{MaxRows: settings.Min})
	if err != nil {
		t.Fatalf("save the smallest allowed cap: %v", err)
	}
	if saved.MaxRows != settings.Min {
		t.Fatalf("saving %d answered %+v", settings.Min, saved)
	}
	if again, err := manager.DataGenSettings(); err != nil || again.MaxRows != settings.Min {
		t.Fatalf("the choice did not stick: %+v (%v)", again, err)
	}
	// The bounds travel with the answer, so the window can show what it may ask
	// for without restating the numbers.
	if saved.Default != settings.Default || saved.Max != settings.Max {
		t.Fatalf("the bounds were lost on the way back: %+v", saved)
	}

	for _, max := range []int{0, -1, settings.Min - 1, settings.Max + 1} {
		if _, err := manager.SaveDataGenSettings(models.DataGenSettings{MaxRows: max}); err == nil {
			t.Errorf("%d was accepted as a cap", max)
		}
	}
	// A refused value changes nothing.
	if again, err := manager.DataGenSettings(); err != nil || again.MaxRows != settings.Min {
		t.Fatalf("a refused value must not be stored: %+v (%v)", again, err)
	}
}

// A manager with no store answers the settings page with the defaults rather
// than with zeroes: the page is the same page either way.
func TestDataGenSettingsWithoutAStore(t *testing.T) {
	manager, _ := testManager(t)

	settings, err := manager.DataGenSettings()
	if err != nil || settings.MaxRows == 0 || settings.Default == 0 {
		t.Fatalf("settings = %+v (%v)", settings, err)
	}
	if _, err := manager.SaveDataGenSettings(models.DataGenSettings{MaxRows: settings.Default}); err != nil {
		t.Fatalf("saving the default should be a no-op, not an error: %v", err)
	}
}
