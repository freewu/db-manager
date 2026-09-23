// User-defined mock placeholders, one file each, under the data directory.
//
// The data generation window's mock column is a template in mock.js syntax, and
// the catalogue it offers is fixed by the build. A user who writes the same
// snippet over and over — an order number, a phone number with a country code, a
// product code — keeps it here instead and gets it back as `@name`:
//
//	<data directory>/.mock/<name>.json
//	{"version":1,"name":"orderNo","template":"SO@date(yyyy)@natural(1000, 9999)","description":"订单号"}
//
// One file per placeholder, for the reasons the query files are files: a
// placeholder is something a user edits, versions, and may want to read or write
// with another tool, and a folder of small JSON files is diffable where one big
// one is not. A placeholder is nothing but a name, a template and a line of
// prose, so its file is as small as the thing it describes.
//
// The name *is* the file name (through escapeSegment, the same escaper the query
// tree uses), so a name cannot escape its folder or hit a Windows device name,
// and renaming is a move of the file rather than an edit of its text.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dbmanager/internal/models"
)

const (
	// mockDirName is the folder inside the data directory. Leading dot, like
	// `.query`: it holds a tree rather than a file a user pokes at day to day,
	// while still being plainly visible in the settings listing.
	mockDirName = ".mock"
	// mockFileExt is the extension of a placeholder file. JSON because a
	// placeholder is a record with three fields, not a script.
	mockFileExt = ".json"
	// maxMockPlaceholders caps the folder. The picker draws them all, and the
	// engine resolves them by name, so a runaway folder is a picker nobody can
	// use rather than a disk problem.
	maxMockPlaceholders = 200
)

// mockFileFormat is one placeholder on disk. The version is written but not yet
// acted on, exactly as in the other stores: it is what a later build reads to
// know how to interpret a file this build wrote.
type mockFileFormat struct {
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description,omitempty"`
}

// mockDir returns the folder holding the custom placeholders, creating nothing.
//
// The result is guaranteed to sit inside the data directory; the check is
// repeated here for the same reason it is repeated in queryDir — this is the one
// place that turns user-supplied text into a path.
func (s *Store) mockDir() (string, error) {
	dir := filepath.Join(s.dir, mockDirName)
	if !within(s.dir, dir) {
		return "", fmt.Errorf("refusing to use %s: it is outside the data directory", dir)
	}
	return dir, nil
}

// mockPath returns the file one placeholder lives in.
func (s *Store) mockPath(name string) (string, error) {
	dir, err := s.mockDir()
	if err != nil {
		return "", err
	}
	segment, err := escapeSegment(name)
	if err != nil {
		return "", fmt.Errorf("placeholder name: %w", err)
	}
	path := filepath.Join(dir, segment+mockFileExt)
	if !within(s.dir, path) {
		return "", fmt.Errorf("refusing to use %s: it is outside the data directory", path)
	}
	return path, nil
}

// ListMockPlaceholders returns every custom placeholder, sorted by name.
//
// A folder that is not there yet is not an error: it means "none defined", which
// is what a fresh install has. A file that cannot be decoded still produces an
// entry — carrying Broken instead of a template — because a user who edited a
// file by hand into something unparseable needs to see it in the list (and be
// able to delete it) rather than wonder why a placeholder disappeared.
func (s *Store) ListMockPlaceholders() ([]models.MockPlaceholder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.mockDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []models.MockPlaceholder{}, nil
		}
		return nil, err
	}

	out := make([]models.MockPlaceholder, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), mockFileExt) {
			// The extension filter is also what keeps a half-written
			// `.json.tmp` from showing up as a placeholder.
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// A file that vanished between listing and stat'ing is not worth
			// failing the whole folder over.
			continue
		}
		name := unescapeSegment(strings.TrimSuffix(entry.Name(), mockFileExt))
		path := filepath.Join(dir, entry.Name())
		placeholder, err := readMockFile(path)
		if err != nil {
			out = append(out, models.MockPlaceholder{
				Name:      name,
				UpdatedAt: info.ModTime().UnixMilli(),
				Broken:    unreadableMock(err),
			})
			continue
		}
		// The file name is the placeholder's identity (a name is stored as a
		// file name), so it wins over whatever the file says about itself: a
		// hand-edited file then still saves back into the file it was read
		// from, instead of quietly creating a second one under the name it
		// claims.
		placeholder.Name = name
		placeholder.UpdatedAt = info.ModTime().UnixMilli()
		out = append(out, placeholder)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if left == right {
			return out[i].Name < out[j].Name
		}
		return left < right
	})
	return out, nil
}

// ReadMockPlaceholder returns one placeholder.
func (s *Store) ReadMockPlaceholder(name string) (models.MockPlaceholder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.mockPath(name)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	placeholder, err := readMockFile(path)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	placeholder.Name = name // the file name is the identity; see ListMockPlaceholders
	stat, err := os.Stat(path)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	placeholder.UpdatedAt = stat.ModTime().UnixMilli()
	return placeholder, nil
}

// SaveMockPlaceholder writes one placeholder, creating the folder when needed,
// and returns what is now on disk.
//
// A file whose name differs only in case from the one being written is the same
// file on Windows and macOS: it is replaced rather than left next to the new
// spelling, exactly as a saved query would be.
func (s *Store) SaveMockPlaceholder(placeholder models.MockPlaceholder) (models.MockPlaceholder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.mockPath(placeholder.Name)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	var existing []string
	if entries, err := os.ReadDir(filepath.Dir(path)); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), mockFileExt) &&
				!strings.EqualFold(entry.Name(), filepath.Base(path)) {
				existing = append(existing, entry.Name())
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return models.MockPlaceholder{}, err
	}
	if len(existing) >= maxMockPlaceholders {
		return models.MockPlaceholder{}, fmt.Errorf(
			"there are already %d placeholders, which is the limit — delete one first", maxMockPlaceholders)
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return models.MockPlaceholder{}, err
	}

	out := mockFileFormat{
		Version:     schemaVer,
		Name:        placeholder.Name,
		Template:    placeholder.Template,
		Description: placeholder.Description,
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	// Written next to the target and moved into place: a crash mid-write must
	// not leave a truncated placeholder where a working one was.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return models.MockPlaceholder{}, err
	}
	stale, err := caseVariant(path)
	if err != nil {
		_ = os.Remove(tmp)
		return models.MockPlaceholder{}, err
	}
	// `os.Rename` refuses to replace an existing file on Windows, so the target
	// is removed first — the content is already in tmp at this point.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return models.MockPlaceholder{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return models.MockPlaceholder{}, err
	}
	if err := dropStale(path, stale); err != nil {
		return models.MockPlaceholder{}, err
	}
	// What is on disk is exactly what was just written, so the caller gets it
	// back without a second read — and without taking the lock again, which would
	// deadlock on this non-reentrant mutex.
	stat, err := os.Stat(path)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	placeholder.UpdatedAt = stat.ModTime().UnixMilli()
	placeholder.Broken = ""
	return placeholder, nil
}

// DeleteMockPlaceholder removes one placeholder. An unknown name is not an
// error: the settings page may have been refreshed by another window a moment
// ago, and a delete that finds nothing has done its job.
func (s *Store) DeleteMockPlaceholder(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.mockPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// The folder is left in place: it is listed in the settings page as soon as
	// it holds something, and removing it would only make the next save create
	// it again.
	return nil
}

// readMockFile decodes one placeholder file.
func readMockFile(path string) (models.MockPlaceholder, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	var parsed mockFileFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return models.MockPlaceholder{}, err
	}
	return models.MockPlaceholder{
		Name:        parsed.Name,
		Template:    parsed.Template,
		Description: parsed.Description,
	}, nil
}

// unreadableMock turns a decode failure into a line for the settings page. The
// raw error is kept because it is the only thing that says *what* is wrong with
// a file a user has to fix (or delete) by hand.
func unreadableMock(err error) string {
	return fmt.Sprintf("this file could not be read (%v)", err)
}
