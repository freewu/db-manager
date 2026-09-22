// Named queries, one file each, under the data directory.
//
// A query window that is bound to a file (instead of to nothing at all) keeps
// its script in
//
//	<data directory>/.query/<connection id>/<database>/<name>.sql
//
// Why files and not the favourites JSON: a query is something a user edits over
// days, versions, and sometimes wants to read with another tool — that is a file
// on disk, not a row inside a file this app owns. The layout is per connection
// and per database because that is the scope a script is written in: the same
// name in two databases is two different scripts, and neither should overwrite
// the other when a window is saved.
//
// Every segment is percent-escaped (see escapeSegment), so a database called
// `a/b` or a query called `con` cannot escape its directory or hit a Windows
// device name. The escape is reversible, so the tree shows the name the user
// typed rather than the spelling the filesystem needed.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"dbmanager/internal/models"
)

const (
	// queryDirName is the folder inside the data directory. It is hidden-ish
	// (leading dot) because it holds a tree rather than a file a user pokes at,
	// while still being plainly visible in the settings listing.
	queryDirName = ".query"
	// queryFileExt is the extension of a query file. Scoped queries are always
	// SQL; a MongoDB shell snippet is still text and still named `.sql`, because
	// the extension says "this is a script", not "this is the SQL language".
	queryFileExt = ".sql"
	// maxSegmentBytes caps one escaped path segment. Windows refuses a whole
	// component over 255 bytes; staying well below keeps room for the `%XX`
	// escapes without ever tripping that limit.
	maxSegmentBytes = 200
)

// windowsReserved are the device names Windows resolves before it looks at the
// directory: a file called `con.sql` does not create a file, it writes to the
// console. The rule applies to the part before the first dot, and to every
// platform here — the same name has to mean the same file everywhere, or a data
// directory synced between two machines would hold two different things.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// escapeSegment turns one name into one path segment.
//
// Escaped: `%` itself (so the escaping stays reversible), the characters Windows
// forbids in a name, control characters, a leading or trailing dot (a leading one
// would hide the file on Unix, a trailing one is silently dropped by Windows),
// and — by escaping its first character — a name Windows would resolve as a
// device.
//
// A name that does not fit in maxSegmentBytes is refused rather than truncated:
// a truncated file name is a *different* query, and silently saving into it is
// worse than saying the name is too long.
func escapeSegment(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("empty name")
	}
	runes := []rune(name)
	// The trailing run of dots and spaces is what Windows strips, so those are
	// the ones that have to be escaped.
	end := len(runes)
	for end > 0 && (runes[end-1] == '.' || runes[end-1] == ' ') {
		end--
	}

	var b strings.Builder
	for i, r := range runes {
		var piece string
		switch {
		case r == '%':
			piece = "%25"
		case r < 0x20 || r == 0x7f:
			piece = fmt.Sprintf("%%%02X", r)
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' ||
			r == '"' || r == '<' || r == '>' || r == '|':
			piece = fmt.Sprintf("%%%02X", r)
		case (r == '.' || r == ' ') && (i == 0 || i >= end):
			piece = fmt.Sprintf("%%%02X", r)
		default:
			piece = string(r)
		}
		if b.Len()+len(piece) > maxSegmentBytes {
			return "", fmt.Errorf("the name is too long once it is escaped for the filesystem: %s", name)
		}
		b.WriteString(piece)
	}

	out := b.String()
	if base, _, _ := strings.Cut(strings.ToLower(out), "."); windowsReserved[base] {
		// Escape the first character instead of prefixing one: the escape stays
		// reversible, so the tree still shows `con`.
		out = fmt.Sprintf("%%%02X%s", out[0], out[1:])
	}
	return out, nil
}

// unescapeSegment reverses escapeSegment.
//
// A segment this build did not write (a file dropped in by hand) may not decode;
// it is then shown exactly as it is on disk, which is the honest answer — the
// alternative would be showing a name that names nothing.
func unescapeSegment(segment string) string {
	var b strings.Builder
	for i := 0; i < len(segment); {
		if segment[i] == '%' && i+2 < len(segment) {
			if raw, err := hex.DecodeString(segment[i+1 : i+3]); err == nil {
				b.Write(raw)
				i += 3
				continue
			}
		}
		b.WriteByte(segment[i])
		i++
	}
	out := b.String()
	if !utf8.ValidString(out) {
		return segment
	}
	return out
}

// queryDir returns the directory holding one connection's queries for one
// database, creating nothing.
//
// The result is guaranteed to sit inside the data directory: every segment is
// escaped, so no name can introduce a path separator or a `..`. The check is
// repeated anyway, because this is the one function that turns user-supplied
// text into a path and a future edit to escapeSegment must not be able to turn
// that into a write outside the data directory.
func (s *Store) queryDir(connectionID, database string) (string, error) {
	conn, err := escapeSegment(connectionID)
	if err != nil {
		return "", fmt.Errorf("connection id: %w", err)
	}
	db, err := escapeSegment(database)
	if err != nil {
		return "", fmt.Errorf("database name: %w", err)
	}
	dir := filepath.Join(s.dir, queryDirName, conn, db)
	if !within(s.dir, dir) {
		return "", fmt.Errorf("refusing to use %s: it is outside the data directory", dir)
	}
	return dir, nil
}

// queryPath returns the file one named query lives in.
func (s *Store) queryPath(connectionID, database, name string) (string, error) {
	dir, err := s.queryDir(connectionID, database)
	if err != nil {
		return "", err
	}
	segment, err := escapeSegment(name)
	if err != nil {
		return "", fmt.Errorf("query name: %w", err)
	}
	path := filepath.Join(dir, segment+queryFileExt)
	if !within(s.dir, path) {
		return "", fmt.Errorf("refusing to use %s: it is outside the data directory", path)
	}
	return path, nil
}

// within reports whether path is dir itself or sits below it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// sameFile reports whether two paths name the same file right now. The question
// only comes up when a save has to remove a stale spelling of the name it just
// wrote — and on a case-insensitive filesystem (`orders.sql` vs `Orders.sql`,
// which is Windows and the default on macOS) the answer decides between
// cleaning up an old file and deleting the script that was just saved. The
// string form cannot tell those apart; the file identity can.
func sameFile(a, b string) bool {
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(left, right)
}

// ListQueryFiles returns the saved queries of one connection and database,
// sorted by name, without their contents.
//
// A database that has no folder yet is not an error: it means "no queries here",
// which is exactly what the tree shows as an empty folder.
func (s *Store) ListQueryFiles(connectionID, database string) ([]models.QueryFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.queryDir(connectionID, database)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []models.QueryFile{}, nil
		}
		return nil, err
	}

	out := make([]models.QueryFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), queryFileExt) {
			// The extension filter is also what keeps a half-written `.sql.tmp`
			// from showing up as a query.
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// A file that vanished between listing and stat'ing is not worth
			// failing the whole folder over; the tree simply does not show it.
			continue
		}
		out = append(out, models.QueryFile{
			ConnectionID: connectionID,
			Database:     database,
			Name:         unescapeSegment(strings.TrimSuffix(entry.Name(), queryFileExt)),
			Path:         filepath.Join(dir, entry.Name()),
			Size:         info.Size(),
			UpdatedAt:    info.ModTime().UnixMilli(),
		})
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

// ReadQueryFile returns one query with its script.
func (s *Store) ReadQueryFile(connectionID, database, name string) (models.QueryFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.queryPath(connectionID, database, name)
	if err != nil {
		return models.QueryFile{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.QueryFile{}, err
	}
	file, err := s.queryFileAt(connectionID, database, name, path)
	if err != nil {
		return models.QueryFile{}, err
	}
	file.SQL = string(raw)
	return file, nil
}

// SaveQueryFile writes one query, creating its folders when needed, and returns
// what is now on disk.
//
// A file whose name differs only in case from the one being written is the same
// file on Windows and macOS: it is replaced rather than left next to the new
// spelling, so the folder never holds two entries a user cannot tell apart.
func (s *Store) SaveQueryFile(save models.QueryFileSave) (models.QueryFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.queryPath(save.ConnectionID, save.Database, save.Name)
	if err != nil {
		return models.QueryFile{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return models.QueryFile{}, err
	}

	// Written next to the target and moved into place: a crash mid-write must
	// not leave a truncated script where the old one was.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(save.SQL), fileMode); err != nil {
		return models.QueryFile{}, err
	}
	stale, err := caseVariant(path)
	if err != nil {
		_ = os.Remove(tmp)
		return models.QueryFile{}, err
	}

	// `os.Rename` refuses to replace an existing file on Windows, so the target
	// is removed first — the content is already in tmp at this point.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return models.QueryFile{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return models.QueryFile{}, err
	}
	if err := dropStale(path, stale); err != nil {
		return models.QueryFile{}, err
	}
	return s.queryFileAt(save.ConnectionID, save.Database, save.Name, path)
}

// RenameQueryFile moves a script to another name.
//
// The contents are not read or written: the file itself is renamed, which is
// both faster and the only version of this that cannot lose a script. A target
// name that is already taken is refused — the caller is a tree menu, and
// silently replacing another query with this one would be data loss the user
// never asked for.
func (s *Store) RenameQueryFile(rename models.QueryFileRename) (models.QueryFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	from, err := s.queryPath(rename.ConnectionID, rename.Database, rename.From)
	if err != nil {
		return models.QueryFile{}, err
	}
	to, err := s.queryPath(rename.ConnectionID, rename.Database, rename.To)
	if err != nil {
		return models.QueryFile{}, err
	}
	if _, err := os.Stat(from); err != nil {
		return models.QueryFile{}, err
	}
	if _, err := os.Stat(to); err == nil && !sameFile(from, to) {
		return models.QueryFile{}, fmt.Errorf("%s already exists", filepath.Base(to))
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return models.QueryFile{}, err
	}

	// A name that differs only in case is the same file on a case-insensitive
	// filesystem, and a *new* spelling of an old file elsewhere. Renaming covers
	// both: the entry ends up under the name the user typed, and no second file
	// is left behind under the old spelling.
	variant, err := caseVariant(to)
	if err != nil {
		return models.QueryFile{}, err
	}
	if err := os.Rename(from, to); err != nil {
		return models.QueryFile{}, err
	}
	if err := dropStale(to, variant); err != nil {
		return models.QueryFile{}, err
	}
	return s.queryFileAt(rename.ConnectionID, rename.Database, rename.To, to)
}

// caseVariant returns the other spellings of path that exist in the same folder
// — the same file on a case-insensitive filesystem, a leftover on a
// case-sensitive one.
func caseVariant(path string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	base := filepath.Base(path)
	var out []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == base {
			continue
		}
		if strings.EqualFold(entry.Name(), base) {
			out = append(out, filepath.Join(filepath.Dir(path), entry.Name()))
		}
	}
	return out, nil
}

// dropStale removes the stale spellings of a name that was just written or
// renamed.
func dropStale(path string, stale []string) error {
	for _, old := range stale {
		// The write may have landed *on* the stale path: on a case-insensitive
		// filesystem a differently-cased name is the same file, and removing it
		// would delete what was just saved.
		if sameFile(path, old) {
			continue
		}
		if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove the previous file %s: %w", old, err)
		}
	}
	return nil
}

// queryFileAt describes a script that is known to be on disk.
func (s *Store) queryFileAt(connectionID, database, name, path string) (models.QueryFile, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return models.QueryFile{}, err
	}
	return models.QueryFile{
		ConnectionID: connectionID,
		Database:     database,
		Name:         name,
		Path:         path,
		Size:         stat.Size(),
		UpdatedAt:    stat.ModTime().UnixMilli(),
	}, nil
}

// DeleteQueryFile removes one query. An unknown name is not an error: the tree
// may have been refreshed by another window a moment ago, and a delete that
// finds nothing has done its job.
func (s *Store) DeleteQueryFile(connectionID, database, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.queryPath(connectionID, database, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// The folder is left in place: an empty database folder is how the tree
	// remembers that this connection has been used, and removing it would make
	// the next expand re-create it anyway.
	return nil
}
