// Data generation settings: how much one run may write.
//
//	<data directory>/datagen.json   {"version":1,"maxRows":1000000}
//
// The number is a ceiling on the Rows box of the data generation window, not a
// batch size and not a target: a run still sends small batches and stops when it
// is told to. It is a file in the data directory rather than a preference in the
// window state because the data directory is where this program's settings live
// — it moves with the rest of them, and it can be read and fixed by hand when a
// window will not start.
//
// A missing file is the default cap rather than an error, for the same reason a
// missing change log settings file is: the file is only written once the user
// has something to say, and most users never will.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"dbmanager/internal/models"
)

const (
	// datagenSettingsFile holds the cap on one generation run.
	datagenSettingsFile = "datagen.json"
	// defaultMaxGenRows is what a run may write when the user has never said
	// otherwise. A million rows is a table big enough to test a query against,
	// and it is a ceiling rather than a target: nothing generates a million rows
	// unless someone asks for them.
	defaultMaxGenRows = 1000000
	// minMaxGenRows keeps the setting honest: below this the cap stops bounding
	// the window and starts being the number the user wants in the Rows box,
	// which is what that box is for.
	minMaxGenRows = 100
	// maxMaxGenRows is the other end. Past this a single run is a mistake that
	// takes hours to walk back — the window reports the rows it wrote as it goes,
	// but they are in the table as soon as they are written — so the ceiling is
	// here to catch one keystroke too many, not because the engine could not take
	// more.
	maxMaxGenRows = 100000000
)

// datagenSettingsFormat is the settings file on disk.
type datagenSettingsFormat struct {
	Version int `json:"version"`
	MaxRows int `json:"maxRows"`
}

// DefaultDataGenSettings is the cap of a store that has never been asked: what a
// fresh install allows, and the bounds the setting may take. It is exported so
// the service layer answers the same numbers when it has no store to read them
// from, instead of restating them and drifting.
func DefaultDataGenSettings() models.DataGenSettings {
	return models.DataGenSettings{
		MaxRows: defaultMaxGenRows,
		Default: defaultMaxGenRows,
		Min:     minMaxGenRows,
		Max:     maxMaxGenRows,
	}
}

// DataGenSettings reports the cap in force.
func (s *Store) DataGenSettings() (models.DataGenSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dataGenSettingsLocked()
}

// SaveDataGenSettings writes the cap.
func (s *Store) SaveDataGenSettings(settings models.DataGenSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if settings.MaxRows == defaultMaxGenRows {
		// Back to the default is the same as never having chosen: dropping the
		// file keeps the data directory listing honest about what the user set.
		if err := os.Remove(filepath.Join(s.dir, datagenSettingsFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	raw, err := json.MarshalIndent(datagenSettingsFormat{
		Version: schemaVer,
		MaxRows: settings.MaxRows,
	}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, datagenSettingsFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) dataGenSettingsLocked() (models.DataGenSettings, error) {
	settings := DefaultDataGenSettings()
	raw, err := os.ReadFile(filepath.Join(s.dir, datagenSettingsFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}
	var parsed datagenSettingsFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// A settings file this build cannot read is the default cap, for the same
		// reason a corrupt state file is not fatal: it must not be able to keep the
		// data generation window from opening.
		return settings, nil
	}
	if parsed.MaxRows >= minMaxGenRows && parsed.MaxRows <= maxMaxGenRows {
		settings.MaxRows = parsed.MaxRows
	}
	return settings, nil
}
