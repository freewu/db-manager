// Package config persists connection profiles to the per-user config
// directory.
//
// Storage format notes:
//
//   - one JSON file holding every profile (connections.json), plus a second
//     one for the query favourites (queries.json) and a third for the explorer
//     arrangement (layout.json) so the three can evolve independently
//   - the files are created with 0600 permissions on platforms that honour it
//   - passwords are only written when the profile opts in via SavePassword;
//     otherwise the user is asked on every connect attempt. This keeps the
//     default posture "no secrets at rest" without forcing users to retype
//     credentials if they consciously opt in.
//   - a password that is written is sealed with AES-256-GCM (see the secret
//     package): the file holds a token, never the password itself, so a copy of
//     it that leaves this machine is not a copy of the secret
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"dbmanager/internal/models"
	"dbmanager/internal/secret"
)

const (
	appDirName  = "db-manager"
	fileName    = "connections.json"
	queriesFile = "queries.json"
	layoutFile  = "layout.json"
	schemaVer   = 1
	fileMode    = 0o600
	dirMode     = 0o700
	stateName   = "state.json"
	maxProfiles = 500
	maxQueries  = 500
	// maxPlacements caps the explorer arrangement. Every entry stands for one
	// profile or group, so it only has to leave room for the profile limit plus
	// the groups around them.
	maxPlacements = maxProfiles + 1000
)

type fileFormat struct {
	Version     int                       `json:"version"`
	Connections []models.ConnectionConfig `json:"connections"`
}

// queryFileFormat mirrors fileFormat for the query favourites file. It is a
// separate file so an older build can keep reading connections.json untouched.
type queryFileFormat struct {
	Version int                 `json:"version"`
	Queries []models.SavedQuery `json:"queries"`
}

// layoutFileFormat is the explorer arrangement on disk. It lives in its own
// file for the same reason the favourites do: saving a profile rewrites
// connections.json whole, and it must not be able to drop the arrangement along
// the way (nor an older build to drop it by not knowing the field).
type layoutFileFormat struct {
	Version int                          `json:"version"`
	Groups  []models.ConnectionGroup     `json:"groups"`
	Items   []models.ConnectionPlacement `json:"items"`
}

// Store is a small, mutex guarded JSON store.
type Store struct {
	mu     sync.Mutex
	dir    string
	path   string
	cipher *secret.Cipher

	// unreadable holds, by profile id, the sealed passwords this machine could
	// not open (a replaced or missing key). Keeping them means a later save does
	// not overwrite a secret that another machine — or a restored key file — can
	// still read. See loadLocked and sealLocked.
	unreadable map[string]string
}

// New returns a Store rooted at the application's data directory.
//
// Which directory that is comes from the location pointer (see location.go):
// the per-user config directory unless the user moved their data elsewhere from
// the settings page. `NewAt` stays for tests and portable installs, which keep
// the JSON files out of the user profile.
func New() (*Store, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}
	return NewAt(dir)
}

// NewAt returns a Store rooted at dir, creating the directory when needed.
func NewAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, err
	}
	return &Store{
		dir:        dir,
		path:       filepath.Join(dir, fileName),
		cipher:     secret.Open(dir),
		unreadable: map[string]string{},
	}, nil
}

// Dir exposes the data directory (used by the welcome screen, and by the
// settings page, which shows the path and offers to open or move it).
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
	// Passwords come back in the clear for the session that is running now; the
	// file only ever holds the sealed token (or a value an older build wrote).
	for i := range parsed.Connections {
		plain, err := s.cipher.Unseal(parsed.Connections[i].Password)
		if err != nil {
			// A token we cannot read (a replaced key, a hand-edited file) means
			// "no stored password" rather than a store that refuses to open: the
			// user is asked for the password again instead of being locked out of
			// their own profiles. The token itself is kept so that saving this
			// profile does not destroy it (see sealLocked).
			if parsed.Connections[i].ID != "" && secret.IsSealed(parsed.Connections[i].Password) {
				s.unreadable[parsed.Connections[i].ID] = parsed.Connections[i].Password
			}
			parsed.Connections[i].Password = ""
			continue
		}
		parsed.Connections[i].Password = plain
	}
	return parsed.Connections, nil
}

func (s *Store) saveLocked(list []models.ConnectionConfig) error {
	sealed, err := s.sealLocked(list)
	if err != nil {
		return err
	}
	out := fileFormat{Version: schemaVer, Connections: sealed}
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

// sealLocked returns the list as it goes to disk: every password replaced by a
// sealed token, and every password of a profile that did not opt in dropped.
// Callers keep passing plain values around, so no other layer has to know that
// the file on disk says something else.
func (s *Store) sealLocked(list []models.ConnectionConfig) ([]models.ConnectionConfig, error) {
	out := make([]models.ConnectionConfig, len(list))
	copy(out, list)
	for i := range out {
		if !out[i].SavePassword {
			out[i].Password = ""
			continue
		}
		if out[i].Password == "" || secret.IsSealed(out[i].Password) {
			// Put back a token this machine could not open instead of dropping
			// it: it is still the password on the machine that wrote it.
			if token, ok := s.unreadable[out[i].ID]; ok && out[i].Password == "" {
				out[i].Password = token
			}
			continue
		}
		token, err := s.cipher.Seal(out[i].Password)
		if err != nil {
			return nil, err
		}
		delete(s.unreadable, out[i].ID)
		out[i].Password = token
	}
	return out, nil
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

// --- query favourites ------------------------------------------------------

// LoadQueries returns every saved query.
func (s *Store) LoadQueries() ([]models.SavedQuery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadQueriesLocked()
}

func (s *Store) queriesPath() string { return filepath.Join(s.dir, queriesFile) }

func (s *Store) loadQueriesLocked() ([]models.SavedQuery, error) {
	raw, err := os.ReadFile(s.queriesPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []models.SavedQuery{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return []models.SavedQuery{}, nil
	}
	var parsed queryFileFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Queries == nil {
		parsed.Queries = []models.SavedQuery{}
	}
	return parsed.Queries, nil
}

func (s *Store) saveQueriesLocked(list []models.SavedQuery) error {
	raw, err := json.MarshalIndent(queryFileFormat{Version: schemaVer, Queries: list}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.queriesPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, s.queriesPath())
}

// UpsertQuery inserts or updates a favourite and returns the stored value.
func (s *Store) UpsertQuery(query models.SavedQuery) (models.SavedQuery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadQueriesLocked()
	if err != nil {
		return query, err
	}

	replaced := false
	for i := range list {
		if list[i].ID == query.ID {
			list[i] = query
			replaced = true
			break
		}
	}
	if !replaced {
		if len(list) >= maxQueries {
			return query, errors.New("saved query limit reached")
		}
		list = append(list, query)
	}
	if err := s.saveQueriesLocked(list); err != nil {
		return query, err
	}
	return query, nil
}

// DeleteQuery removes a favourite by id.
func (s *Store) DeleteQuery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadQueriesLocked()
	if err != nil {
		return err
	}
	kept := make([]models.SavedQuery, 0, len(list))
	for _, query := range list {
		if query.ID != id {
			kept = append(kept, query)
		}
	}
	return s.saveQueriesLocked(kept)
}

// --- connection explorer layout -------------------------------------------

// LoadLayout returns the arrangement the user left the explorer in.
//
// A missing or unreadable file is not an error: it means "no arrangement", and
// the service then falls back to listing the profiles in file order. A broken
// layout must never keep the tree from drawing.
func (s *Store) LoadLayout() (models.ConnectionLayout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLayoutLocked()
}

func (s *Store) layoutPath() string { return filepath.Join(s.dir, layoutFile) }

func (s *Store) loadLayoutLocked() (models.ConnectionLayout, error) {
	empty := models.ConnectionLayout{
		Groups: []models.ConnectionGroup{},
		Items:  []models.ConnectionPlacement{},
	}
	raw, err := os.ReadFile(s.layoutPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty, nil
		}
		return empty, err
	}
	if len(raw) == 0 {
		return empty, nil
	}
	var parsed layoutFileFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return empty, nil
	}
	layout := models.ConnectionLayout{Groups: parsed.Groups, Items: parsed.Items}
	if layout.Groups == nil {
		layout.Groups = []models.ConnectionGroup{}
	}
	if layout.Items == nil {
		layout.Items = []models.ConnectionPlacement{}
	}
	return layout, nil
}

// SaveLayout writes the arrangement, replacing the file atomically.
//
// The store keeps this dumb: what an arrangement may contain is decided in the
// service layer, where a mistake can carry an apperr code. Only the size limit
// is enforced here, so a runaway caller cannot fill the disk.
func (s *Store) SaveLayout(layout models.ConnectionLayout) error {
	if len(layout.Groups) > maxPlacements || len(layout.Items) > maxPlacements {
		return errors.New("connection layout is too large")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := layoutFileFormat{Version: schemaVer, Groups: layout.Groups, Items: layout.Items}
	if out.Groups == nil {
		out.Groups = []models.ConnectionGroup{}
	}
	if out.Items == nil {
		out.Items = []models.ConnectionPlacement{}
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.layoutPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, s.layoutPath())
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
