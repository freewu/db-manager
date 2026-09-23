// The data generation settings: the one number of that window's behaviour that
// belongs to the installation rather than to a table or a run.
package service

import (
	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// DataGenSettings reports how many rows one generation run may write, and what
// that setting may be set to.
func (m *Manager) DataGenSettings() (models.DataGenSettings, error) {
	store := m.storeRef()
	if store == nil {
		// A manager built without a store has no settings file to read; only a
		// test constructs one that way, and the defaults are the honest answer.
		return config.DefaultDataGenSettings(), nil
	}
	return store.DataGenSettings()
}

// SaveDataGenSettings stores that cap.
//
// The bounds are checked here rather than clamped quietly: the number is what
// the window offers to ask the engine for, and a value the user did not choose
// is a worse answer than being told what the limits are.
func (m *Manager) SaveDataGenSettings(settings models.DataGenSettings) (models.DataGenSettings, error) {
	current, err := m.DataGenSettings()
	if err != nil {
		return models.DataGenSettings{}, err
	}
	if settings.MaxRows < current.Min || settings.MaxRows > current.Max {
		return models.DataGenSettings{}, apperr.New(apperr.CodeInvalidConfig,
			"one generation run may write between %d and %d rows", current.Min, current.Max)
	}
	store := m.storeRef()
	if store == nil {
		return current, nil
	}
	if err := store.SaveDataGenSettings(models.DataGenSettings{MaxRows: settings.MaxRows}); err != nil {
		return models.DataGenSettings{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "save the data generation settings")
	}
	return m.DataGenSettings()
}
