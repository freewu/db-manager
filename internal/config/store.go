// Package config persists connection profiles to the per-user config
// directory.
//
// Storage format notes:
//
//   - one JSON file holding every profile
//   - the file is created with 0600 permissions on platforms that honour it
//   - passwords are only written when the profile opts in via SavePassword;
//     otherwise the user is asked on every connect attempt. This keeps the
//     default posture "no secrets at rest" without forcing users to retype
//     credentials if they consciously opt in.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"dbmanager/internal/models"
)

const (
	appDirName  = "db-manager"
	fileName    = "connections.json"
	schemaVer   = 1
	fileMode    = 0o600
	dirMode     = 0o700
	stateName   = "state.json"
	maxProfiles = 500
)

type fileFormat struct {
	Version     int                       `json:"version"`
	Connections []models.ConnectionConfig `json:"connections"`
}

// Store is a small, mutex guarded JSON store.
type Store struct {
	mu   sync.Mutex
	dir  string
	path string
}

// New returns a Store rooted at the per-user config directory.
func New() (*Store, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, path: filepath.Join(dir, fileName)}, nil
}

// ConfigDir returns (and creates) the application config directory.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		// Fall back to the working directory rather than failing outright.
		base = "."
	}
	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", err
	}
	return dir, nil
}

// Dir exposes the config directory (used by the welcome screen).
func (s *Store) Dir() string { return s.dir }

// Path exposes the profile file path.
func (s *Store) Path() string { return s.path }

// Load returns every stored profile, with passwords populated internally.
func (s *Store) Load() ([]models.ConnectionConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() ([]models.ConnectionConfig, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []models.ConnectionConfig{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return []models.ConnectionConfig{}, nil
	}
	var parsed fileFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Connections == nil {
		parsed.Connections = []models.ConnectionConfig{}
	}
	return parsed.Connections, nil
}

func (s *Store) saveLocked(list []models.ConnectionConfig) error {
	out := fileFormat{Version: schemaVer, Connections: list}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	// Write to a temp file then rename so a crash cannot corrupt the store.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Upsert inserts or updates a profile and returns the stored value.
func (s *Store) Upsert(cfg models.ConnectionConfig) (models.ConnectionConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadLocked()
	if err != nil {
		return cfg, err
	}

	// A blank password on an existing profile means "keep the stored one".
	if cfg.Password == "" {
		for _, existing := range list {
			if existing.ID == cfg.ID && existing.Password != "" {
				cfg.Password = existing.Password
				break
			}
		}
	}
	if !cfg.SavePassword {
		cfg.Password = ""
	}

	replaced := false
	for i := range list {
		if list[i].ID == cfg.ID {
			list[i] = cfg
			replaced = true
			break
		}
	}
	if !replaced {
		if len(list) >= maxProfiles {
			return cfg, errors.New("connection profile limit reached")
		}
		list = append(list, cfg)
	}
	if err := s.saveLocked(list); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Delete removes a profile by id.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadLocked()
	if err != nil {
		return err
	}
	kept := make([]models.ConnectionConfig, 0, len(list))
	for _, c := range list {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	return s.saveLocked(kept)
}

// Find returns a single profile by id.
func (s *Store) Find(id string) (models.ConnectionConfig, bool, error) {
	list, err := s.Load()
	if err != nil {
		return models.ConnectionConfig{}, false, err
	}
	for _, c := range list {
		if c.ID == id {
			return c, true, nil
		}
	}
	return models.ConnectionConfig{}, false, nil
}

// --- lightweight UI state (last used theme, window prefs, ...) -------------

// LoadState reads an arbitrary JSON blob previously stored by the frontend.
func (s *Store) LoadState() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(filepath.Join(s.dir, stateName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	state := map[string]any{}
	if len(raw) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		// Corrupt state must never block startup.
		return map[string]any{}, nil
	}
	return state, nil
}

// SaveState persists the UI state blob.
func (s *Store) SaveState(state map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, stateName), raw, fileMode)
}
