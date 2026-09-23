// Where the application's data lives, and how it gets moved somewhere else.
//
// By default everything sits in the per-user config directory (see
// DefaultDir). A user who wants their profiles next to their other files, on a
// synced folder or on another drive picks a directory instead, and the choice
// has to survive a restart — so it is recorded in a small pointer file that
// stays in the default directory:
//
//	<default dir>/location.json   {"version":1,"dataDir":"E:\\db-manager-data"}
//
// The pointer is deliberately kept out of the data directory itself: it has to
// be readable *before* the data directory is known, and it must not travel with
// a move (or a copy of the data would claim to be the directory it was copied
// to). A missing, unreadable or empty pointer means "the default", which is also
// the fallback if the directory it names is gone — the app then starts with a
// fresh, empty store in the default place rather than refusing to open.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"dbmanager/internal/models"
	"dbmanager/internal/secret"
)

// locationFile is the pointer written in the default directory.
const locationFile = "location.json"

// dataFiles are the files this build keeps in the data directory: the profiles,
// the query favourites, the explorer arrangement, the UI state, the key the
// saved passwords are sealed with, the change log, and the change log's own
// settings. The folders beside them are in dataDirs.
//
// A feature that starts writing a new file (or a new folder) here has to add it
// to one of those lists, or a move will leave it behind — and now *say* it did,
// through DataDirMoveResult.LeftBehind, rather than quietly dropping it.
// Archived change logs are the one thing that cannot be listed here: they are
// named for the day they were rotated out, so they are found by scanning the
// directory instead (see dataFileNames).
var dataFiles = []string{
	fileName,
	queriesFile,
	layoutFile,
	stateName,
	secret.KeyFileName,
	changeLogFile,
	changeLogSettingsFile,
}

// dataDirs are the folders this build keeps in the data directory, next to the
// files above. They travel with a move exactly like the files do. The query tree
// is one folder per connection and database, and the custom mock placeholders
// are one file per placeholder, so both are copied by walking them — but only
// *these* folders are ever walked, never the whole data directory, which is what
// keeps "one directory may hold another" true (see MoveData).
var dataDirs = []string{queryDirName, mockDirName}

type locationFormat struct {
	Version int    `json:"version"`
	DataDir string `json:"dataDir"`
}

// DefaultDir returns the per-user config directory. It is where the pointer
// file lives, and the data directory until the user picks another one.
func DefaultDir() (string, error) {
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

// DataDir returns the directory the application is keeping its data in.
//
// The pointer file is read from the default directory; anything unexpected in
// it means the default. The returned directory is created when missing, so a
// user who deleted it gets an empty store instead of a start-up failure.
func DataDir() (string, error) {
	def, err := DefaultDir()
	if err != nil {
		return "", err
	}
	chosen, ok := readLocation(def)
	if !ok {
		return def, nil
	}
	if err := os.MkdirAll(chosen, dirMode); err != nil {
		return "", fmt.Errorf("data directory %s: %w", chosen, err)
	}
	return chosen, nil
}

// readLocation returns the directory the pointer file names, if there is one.
func readLocation(def string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(def, locationFile))
	if err != nil || len(raw) == 0 {
		return "", false
	}
	var parsed locationFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false
	}
	dir := strings.TrimSpace(parsed.DataDir)
	if dir == "" || !filepath.IsAbs(dir) {
		return "", false
	}
	return filepath.Clean(dir), true
}

// writeLocation points the application at dir, or back at the default when dir
// is empty. The write is atomic: a half-written pointer file would be read as
// "no pointer" and silently send the user back to the default directory.
func writeLocation(def, dir string) error {
	path := filepath.Join(def, locationFile)
	if dir == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	raw, err := json.MarshalIndent(locationFormat{Version: schemaVer, DataDir: dir}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// sameDir reports whether two paths name the same directory.
//
// Symlinks are resolved first because the user picks the directory through a
// file chooser, which happily returns a link to the directory the app is already
// using — and "move the data into itself" has to be recognised as a no-op
// instead of being attempted. Windows also compares paths case-insensitively,
// since `E:\Data` and `e:\data` are one directory there.
func sameDir(a, b string) bool {
	norm := func(p string) string {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
		p = filepath.Clean(p)
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	return norm(a) == norm(b)
}

// DescribeDataDir reports what dir holds, next to the default directory. An
// empty dir means the default.
//
// A file that cannot be stat'ed is skipped rather than failing the whole list:
// this feeds a settings page, and one unreadable file must not hide the rest.
func DescribeDataDir(dir string) (models.DataDirInfo, error) {
	def, err := DefaultDir()
	if err != nil {
		return models.DataDirInfo{}, err
	}
	if strings.TrimSpace(dir) == "" {
		dir = def
	}
	return describe(dir, def), nil
}

// describe builds the listing for one directory. Split out so the move can
// describe the *new* directory after switching to it.
func describe(dir, def string) models.DataDirInfo {
	info := models.DataDirInfo{
		Path:        dir,
		DefaultPath: def,
		IsDefault:   sameDir(dir, def),
		Files:       []models.DataFileInfo{},
	}
	// An unreadable directory is reported as one with nothing in it: this feeds a
	// settings page, and it must not fail to open because of the folder it is
	// describing.
	fileNames, _ := dataFileNames(dir)
	for _, name := range fileNames {
		stat, err := os.Stat(filepath.Join(dir, name))
		if err != nil || stat.IsDir() {
			continue
		}
		info.Files = append(info.Files, models.DataFileInfo{Name: name, Bytes: stat.Size()})
		info.TotalBytes += stat.Size()
	}
	for _, name := range dataDirs {
		files, bytes, err := describeTree(filepath.Join(dir, name))
		if err != nil || files == 0 {
			// A folder with nothing in it (or one that is not there yet) is not
			// worth a row: the settings page lists what the app has actually
			// written, and an empty tree is nothing.
			continue
		}
		info.Files = append(info.Files, models.DataFileInfo{Name: name, Bytes: bytes, Dir: true, Count: files})
		info.TotalBytes += bytes
	}
	return info
}

// describeTree counts the files in one tree and adds up their sizes.
func describeTree(dir string) (int, int64, error) {
	var files int
	var bytes int64
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files++
		bytes += info.Size()
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, nil
	}
	return files, bytes, err
}

// MoveData moves everything this build keeps in the active data directory into
// target and points the application at target. An empty target means the default
// directory, which is how the settings page offers "use the default again".
//
// The order is what makes it safe to interrupt:
//
//  1. every file is copied to target and read back to prove it arrived;
//  2. only then is the pointer rewritten, which is the moment the move happens;
//  3. only then are the source files deleted.
//
// A failure in step 1 or 2 leaves the source untouched, so the worst case is a
// second copy of the data in a directory nobody is using. Step 3 is best effort:
// a file that cannot be deleted is reported in Remaining rather than failing a
// move that has already succeeded.
//
// One directory may hold another (the default directory is a perfectly good
// place to keep a subfolder of data in): only the files above travel, nothing is
// copied recursively, so nesting cannot run away.
func MoveData(target string) (models.DataDirMoveResult, error) {
	source, err := DataDir()
	if err != nil {
		return models.DataDirMoveResult{}, err
	}
	def, err := DefaultDir()
	if err != nil {
		return models.DataDirMoveResult{}, err
	}

	dest := strings.TrimSpace(target)
	if dest == "" {
		dest = def
	}
	if !filepath.IsAbs(dest) {
		return models.DataDirMoveResult{}, fmt.Errorf("data directory must be an absolute path: %s", dest)
	}
	dest = filepath.Clean(dest)

	if sameDir(dest, source) {
		return models.DataDirMoveResult{}, fmt.Errorf("%s is already the data directory", dest)
	}

	// A directory that already holds data is refused instead of merged: two
	// connections.json files cannot both be the truth, and silently overwriting
	// one of them is the kind of data loss this whole path exists to avoid.
	if existing, err := existingData(dest); err != nil {
		return models.DataDirMoveResult{}, err
	} else if len(existing) > 0 {
		return models.DataDirMoveResult{}, fmt.Errorf(
			"%s already holds %s — pick an empty directory, or move those files away first",
			dest, strings.Join(existing, ", "))
	}

	if err := os.MkdirAll(dest, dirMode); err != nil {
		return models.DataDirMoveResult{}, fmt.Errorf("create %s: %w", dest, err)
	}

	result := models.DataDirMoveResult{Moved: []string{}, LeftBehind: []string{}, Remaining: []string{}}
	// What to delete once the pointer has moved, in the same order as Moved, so
	// the report can name an entry the way the user sees it. The two cannot be
	// derived from one another: a folder is copied as a tree but reported as one
	// line.
	type moved struct{ display, path string }
	sources := make([]moved, 0, len(dataFiles)+len(dataDirs))
	fileNames, err := dataFileNames(source)
	if err != nil {
		return models.DataDirMoveResult{}, err
	}
	for _, name := range fileNames {
		from := filepath.Join(source, name)
		if _, err := os.Stat(from); err != nil {
			// Not every install has every file (a store that never saved a
			// password has no key), so a missing source file is expected.
			continue
		}
		if err := copyVerified(from, filepath.Join(dest, name)); err != nil {
			return models.DataDirMoveResult{}, err
		}
		result.Moved = append(result.Moved, name)
		sources = append(sources, moved{display: name, path: from})
	}
	for _, name := range dataDirs {
		from := filepath.Join(source, name)
		count, _, err := describeTree(from)
		if err != nil {
			return models.DataDirMoveResult{}, err
		}
		if count == 0 {
			continue
		}
		if err := copyTree(from, filepath.Join(dest, name)); err != nil {
			return models.DataDirMoveResult{}, err
		}
		// One entry for the whole tree: a user with forty saved queries should
		// not have to read forty lines to learn that they all came along.
		label := fmt.Sprintf("%s (%s)", name, plural(count, "file"))
		result.Moved = append(result.Moved, label)
		sources = append(sources, moved{display: label, path: from})
	}

	// Everything in the old directory that is not ours stays where it is, and is
	// named in the report — the user is the only one who can decide about it. The
	// new directory is excluded: it may itself sit inside the old one, and
	// reporting it would read as "something was left behind" when in fact that is
	// where the data just went.
	left, err := otherEntries(source)
	if err != nil {
		return models.DataDirMoveResult{}, err
	}
	for _, name := range left {
		if !sameDir(filepath.Join(source, name), dest) {
			result.LeftBehind = append(result.LeftBehind, name)
		}
	}
	// Naming the default directory in the pointer is the same as having no
	// pointer at all, and the absent file is the honest spelling: a later build
	// that moves the default elsewhere is then followed instead of being pinned
	// to a path the user never chose.
	pointer := dest
	if sameDir(dest, def) {
		pointer = ""
	}
	if err := writeLocation(def, pointer); err != nil {
		return models.DataDirMoveResult{}, fmt.Errorf("record %s as the data directory: %w", dest, err)
	}

	for _, entry := range sources {
		if err := removePath(entry.path); err != nil {
			result.Remaining = append(result.Remaining, entry.display)
		}
	}

	result.Info = describe(dest, def)
	return result, nil
}

// dataFileNames lists every file this build keeps in dir: the ones it always
// writes, plus the archived change logs, whose names are decided when a log is
// rotated out and therefore cannot be listed ahead of time.
//
// Everything that has to know what is in the data directory — the listing, the
// move, the refusal to move into a directory already holding data, and the
// "what else is in here" report — goes through this one function, so a new file
// or a new kind of file is added in a single place.
func dataFileNames(dir string) ([]string, error) {
	names := append([]string{}, dataFiles...)
	archives, err := changeLogArchivesIn(dir)
	if err != nil {
		return nil, err
	}
	return append(names, archives...), nil
}

// existingData lists the recognised data files (and the folders this build owns)
// already present in dir, so a move can refuse a directory that is in use.
func existingData(dir string) ([]string, error) {
	found := make([]string, 0, len(dataFiles)+len(dataDirs))
	fileNames, err := dataFileNames(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	for _, name := range fileNames {
		stat, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("inspect %s: %w", filepath.Join(dir, name), err)
		}
		if stat.IsDir() || stat.Size() == 0 {
			// A directory we would have to replace, or an empty file (the app
			// writes a file only once it has something to say), is not data.
			continue
		}
		found = append(found, name)
	}
	for _, name := range dataDirs {
		count, _, err := describeTree(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", filepath.Join(dir, name), err)
		}
		if count > 0 {
			found = append(found, fmt.Sprintf("%s (%s)", name, plural(count, "file")))
		}
	}
	return found, nil
}

// otherEntries lists what a data directory holds besides this app's files and
// folders: the pointer file itself (it belongs to the default directory), temp
// files left by an interrupted write, and anything the user dropped there.
func otherEntries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	known := make(map[string]bool, len(dataFiles)+len(dataDirs)+1)
	for _, name := range dataFiles {
		known[name] = true
	}
	for _, name := range dataDirs {
		known[name] = true
	}
	known[locationFile] = true
	// Archived logs are ours too, and are found by the same rule that lists and
	// moves them.
	if archives, err := changeLogArchivesIn(dir); err == nil {
		for _, name := range archives {
			known[name] = true
		}
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if known[entry.Name()] {
			continue
		}
		// A temp file is a leftover of ours, not something to report on.
		if strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		out = append(out, entry.Name())
	}
	sort.Strings(out)
	return out, nil
}

// plural renders a count with its noun, because the move report is read by a
// person and "1 files" reads like a bug.
func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// copyTree copies a folder this build owns (the query files, the custom mock
// placeholders) and verifies every file on the way, the same way copyVerified
// proves a single file arrived.
//
// Nothing outside src is read: only the folders listed in dataDirs are ever
// walked, which is what makes a nested data directory safe.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, dirMode)
		}
		// A leftover temp file is our own rubbish; copying it would resurrect a
		// half-written query in the new directory.
		if strings.HasSuffix(entry.Name(), ".tmp") {
			return nil
		}
		return copyVerified(path, target)
	})
}

// removePath deletes a file or a folder this build wrote. It is best effort by
// contract — the caller reports what it could not remove rather than failing a
// move that has already happened.
func removePath(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	// os.RemoveAll answers nil for a path that is already gone, so reaching this
	// point means something is really in the way (a lock, a read-only folder).
	return err
}

// copyVerified copies src to dst and reads the copy back, so "the data is in the
// new directory" is something this build checked rather than assumed.
//
// The destination is known to hold no data — MoveData refuses a directory that
// does — so whatever is there is replaced. That has to be explicit: `os.Rename`
// onto an existing file is an error on Windows, and an empty leftover file is
// exactly the kind of thing that would otherwise fail a move for no reason.
func copyVerified(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	back, err := os.ReadFile(tmp)
	if err != nil || !bytes.Equal(back, raw) {
		_ = os.Remove(tmp)
		return fmt.Errorf("verify %s: the copy did not read back as it was written", dst)
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s: %w", dst, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("put %s in place: %w", dst, err)
	}
	return nil
}
