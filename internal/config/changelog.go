// The change log: what this application ran against the databases it was
// pointed at.
//
//	<data directory>/log/20260214.log
//	{"version":1,"at":1730000000000,"connection":{"name":"shop","driver":"mysql",…},"database":"shop","table":"orders","kind":"alter","source":"design","rows":3,"statement":"ALTER TABLE `orders` …"}
//	<data directory>/log/20260214-1.log   (same day, the earlier file)
//	<data directory>/log/20260213.log     (the day before)
//	<data directory>/changelog.json       {"version":1,"maxEntries":2000}
//
// One line of JSON per entry, rather than an array in one file, because of how
// this file is used: it is appended to far more often than it is read, always in
// the same direction, and it is the one file here that grows without a user
// asking for it. A line that cannot be decoded is one lost entry instead of a
// whole log that refuses to open.
//
// A file a day, named for the day: "what ran on the 14th" is one file to open,
// not a slice of a roll to search for. Nothing is ever dropped. When a day's
// file holds as many statements as the user allows it to, it is renamed to
// `<yyyymmdd>-<n>.log` — the day, then which rotation of it this was — and a
// fresh `<yyyymmdd>.log` is started under the day's own name. Renaming a whole
// file is one atomic step, which is what makes rotating free: no copying, no
// rewriting, and a log that is being read at that moment is either entirely the
// old file or entirely the new one. The settings file holds the one number that
// decides when that happens.
//
// The log folder is a folder and not a fence: logs written by older builds sat
// in the data directory itself (`changelog.jsonl`, and archives named the same
// way), and those are still listed and still read. They sort after the folder's
// files, because a file with no day in its name is the oldest thing here.
//
// Nothing in this file is a gate in front of anything: an entry is written after
// the statement it describes has run, so a log that cannot be written must never
// turn a change that happened into an error the user sees.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dbmanager/internal/models"
)

const (
	// changeLogDirName is the folder inside the data directory the logs live in.
	changeLogDirName = "log"
	// changeLogFile is the live log of builds before the folder existed. It is
	// still read, and still moved with the rest of the data, but nothing writes
	// to it any more.
	changeLogFile = "changelog.jsonl"
	// changeLogSettingsFile holds the rotation policy. It is separate from the
	// log itself because it has to be readable without reading the log, and it
	// is not a preference the frontend owns: the rotation happens inside an
	// append, in the backend.
	changeLogSettingsFile = "changelog.json"
	// defaultChangeLogEntries is how many statements one log file holds before
	// it is archived. It is a window onto the recent past that a person can
	// still scroll, not a limit on how much history is kept — nothing is
	// deleted, it is moved to the next file.
	defaultChangeLogEntries = 2000
	// minChangeLogEntries keeps the setting meaningful: a file that rotates
	// every few statements would fill the data directory with archives, and one
	// statement per file is not a log.
	minChangeLogEntries = 100
	// maxChangeLogEntries is the other end: past this the live file is bigger
	// than anything a reader wants to load in one go.
	maxChangeLogEntries = 100000
	// maxChangeLogBytes bounds one read. With the entry limit this is about ten
	// times what a file can hold in practice; it exists so that a log damaged or
	// padded by something other than this program cannot be pulled into memory
	// whole. Past it, the newest bytes are kept and the rest is left where it is
	// — that is also a sign the file is not one this build wrote, which is one
	// of the reasons it is rotated away rather than read forever.
	maxChangeLogBytes = 8 << 20
	// changeLogArchiveExt is what an archived log is called. The file being
	// written today carries the same extension: it is a log either way, and the
	// day in its name is what tells a reader which one it is looking at.
	changeLogArchiveExt = ".log"
	// changeLogDayFormat is the day at the front of every log file name, and the
	// layout a day is parsed back with.
	changeLogDayFormat = "20060102"
)

// changeLogSettingsFormat is the settings file on disk.
type changeLogSettingsFormat struct {
	Version    int `json:"version"`
	MaxEntries int `json:"maxEntries"`
}

// AppendChangeLog adds one entry, rotating that day's file first when it is
// full.
//
// The entry is appended rather than written as part of a rewrite: trimming is
// gone now that a full log is rotated instead, so the append is one small write
// and the rotation is one rename, and neither can leave a half-written line
// behind (which a rewrite of the whole file could, if it were interrupted).
func (s *Store) AppendChangeLog(entry models.ChangeLogEntry) error {
	settings, err := s.ChangeLogSettings()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// The entry's own timestamp picks the day's file, rather than the clock
	// read here: what a reader sees next to the statement is when it ran, and an
	// entry that is a minute stale at midnight should not land in the new day's
	// file. A caller that left the timestamp out gets today, which is the only
	// thing left to say.
	day := time.Now()
	if entry.At > 0 {
		day = time.UnixMilli(entry.At)
	}

	dir := s.changeLogDir()
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	path := filepath.Join(dir, changeLogDayName(day))
	// The rotated file is safe under its new name before the new entry is
	// written, so a failure from here on costs the entry, never the history.
	if _, err := s.rotateChangeLogLocked(path, day, settings.MaxEntries); err != nil {
		return err
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

// ChangeLog reads one file, newest entry first, along with how many entries it
// holds and the files there are to choose between.
//
// An empty file name means the file being written right now — today's. Oldest
// first on disk and newest first here, because that is the only way it is ever
// read: the appends go to the end, and a reader wants the last statement that
// ran, not the first.
func (s *Store) ChangeLog(file string, limit int) (models.ChangeLog, error) {
	name, err := ChangeLogName(file)
	if err != nil {
		return models.ChangeLog{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := s.changeLogFilesLocked()
	if err != nil {
		return models.ChangeLog{}, err
	}
	entries, err := s.readChangeLogLocked(s.changeLogReadPath(name))
	if err != nil {
		return models.ChangeLog{}, err
	}
	total := len(entries)
	page := make([]models.ChangeLogEntry, 0, min(limit, total))
	for i := total - 1; i >= 0 && len(page) < limit; i-- {
		page = append(page, entries[i])
	}
	return models.ChangeLog{File: name, Entries: page, Total: total, Files: files}, nil
}

// ChangeLogSettings reports the rotation policy in force.
//
// A missing file is the default policy rather than an error: the file is only
// written once the user has something to say, and most users never will.
func (s *Store) ChangeLogSettings() (models.ChangeLogSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changeLogSettingsLocked()
}

// SaveChangeLogSettings writes the rotation policy.
func (s *Store) SaveChangeLogSettings(settings models.ChangeLogSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if settings.MaxEntries == defaultChangeLogEntries {
		// Back to the default is the same as never having chosen: dropping the
		// file keeps the data directory listing honest about what the user set.
		if err := os.Remove(filepath.Join(s.dir, changeLogSettingsFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	raw, err := json.MarshalIndent(changeLogSettingsFormat{
		Version:    schemaVer,
		MaxEntries: settings.MaxEntries,
	}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, changeLogSettingsFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// changeLogDir is the folder the logs live in.
func (s *Store) changeLogDir() string { return filepath.Join(s.dir, changeLogDirName) }

// changeLogReadPath is the file to read for a name: the folder first, then the
// data directory itself, where logs written by an older build are. A name that
// is nowhere yet answers the folder, which reads as an empty log.
func (s *Store) changeLogReadPath(name string) string {
	path := filepath.Join(s.changeLogDir(), name)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	legacy := filepath.Join(s.dir, name)
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return path
}

func (s *Store) changeLogSettingsLocked() (models.ChangeLogSettings, error) {
	settings := DefaultChangeLogSettings()
	raw, err := os.ReadFile(filepath.Join(s.dir, changeLogSettingsFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}
	var parsed changeLogSettingsFormat
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// A settings file this build cannot read is the default policy, for the
		// same reason a corrupt state file is not fatal: it must not be able to
		// stop the application from logging what it runs.
		return settings, nil
	}
	if parsed.MaxEntries >= minChangeLogEntries && parsed.MaxEntries <= maxChangeLogEntries {
		settings.MaxEntries = parsed.MaxEntries
	}
	return settings, nil
}

// rotateChangeLogLocked moves the file being written aside when it is full, and
// answers the name it was given (empty when nothing was rotated).
//
// The file is renamed, never rewritten or truncated: whatever it holds — a
// complete log, a line a crash cut in half, something else entirely if the file
// was padded — moves aside as it is, under a name that says when it was taken
// out. A file that is somehow larger than this build would ever write is rotated
// too: it is not one this build is going to keep appending to forever.
func (s *Store) rotateChangeLogLocked(path string, day time.Time, maxEntries int) (string, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if stat.Size() <= maxChangeLogBytes {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if countChangeLogEntries(raw) < maxEntries {
			return "", nil
		}
	}

	name := s.freeArchiveNameLocked(day)
	if err := os.Rename(path, filepath.Join(s.changeLogDir(), name)); err != nil {
		return "", err
	}
	return name, nil
}

// freeArchiveNameLocked picks the next archive name for that day: the day it was
// rotated out, then which rotation of that day it was.
//
// Both places are checked. A name that is already taken in the data directory by
// a log an older build wrote must not be handed out again: the reader prefers
// the folder's copy of a name, so reusing it would hide that file.
func (s *Store) freeArchiveNameLocked(day time.Time) string {
	date := day.Format(changeLogDayFormat)
	for n := 1; ; n++ {
		name := fmt.Sprintf("%s-%d%s", date, n, changeLogArchiveExt)
		if _, err := os.Stat(filepath.Join(s.changeLogDir(), name)); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.dir, name)); errors.Is(err, fs.ErrNotExist) {
			return name
		}
	}
}

// changeLogFilesLocked lists the files the log is spread over, newest first,
// with the one being written right now at the top.
//
// The entries in each file are counted, which means reading them — bounded by
// maxChangeLogBytes each, and a window that shows the wrong number of entries is
// worse than one that took a moment to open.
func (s *Store) changeLogFilesLocked() ([]models.ChangeLogFile, error) {
	names, err := s.changeLogNamesLocked()
	if err != nil {
		return nil, err
	}
	live := changeLogDayName(time.Now())
	files := make([]models.ChangeLogFile, 0, len(names))
	for _, name := range names {
		path := s.changeLogReadPath(name)
		stat, err := os.Stat(path)
		if err != nil || stat.IsDir() {
			// Gone between the listing and here: a rotation in flight, which is a
			// file that will be there next time under its other name.
			continue
		}
		raw, err := readChangeLogTail(path)
		if err != nil {
			return nil, err
		}
		info := models.ChangeLogFile{
			Name:    name,
			Bytes:   stat.Size(),
			Entries: countChangeLogEntries(raw),
		}
		if name != live {
			// Not the file being written right now, so it is finished: the only
			// one that is not is today's own name. Its timestamp is when the last
			// statement in it ran, which is what a reader wants to see.
			info.Archived = true
			info.At = stat.ModTime().UnixMilli()
		}
		files = append(files, info)
	}
	return files, nil
}

// changeLogNamesLocked lists every log file there is to choose between, newest
// first: today's file (when it exists), the days before it, and — last, because
// a name with no day in it is the oldest thing here — the live log of a build
// from before the folder existed.
func (s *Store) changeLogNamesLocked() ([]string, error) {
	seen := make(map[string]bool, 8)
	names := make([]string, 0, 8)

	entries, err := os.ReadDir(s.changeLogDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !isChangeLogFile(entry.Name()) {
			continue
		}
		seen[entry.Name()] = true
		names = append(names, entry.Name())
	}

	// The older layout: the log and its archives sat in the data directory
	// itself. A name the folder also holds is the folder's — that is the one
	// being written to, and the copy beside it is what a user left behind by
	// copying files around.
	legacy := []string{changeLogFile}
	archives, err := changeLogArchivesIn(s.dir)
	if err != nil {
		return nil, err
	}
	legacy = append(legacy, archives...)
	for _, name := range legacy {
		if seen[name] {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.dir, name)); err != nil {
			continue
		}
		names = append(names, name)
	}

	sort.Slice(names, func(i, j int) bool { return changeLogNewer(names[i], names[j]) })
	return names, nil
}

// changeLogNewer orders two names the way a reader wants them: the newest day
// first, and within a day the file being written — the day's own name, which is
// still growing — ahead of every rotation of it, then the highest rotation
// number. A name with no day in it (the log of an older build) is older than
// everything here and sorts last.
func changeLogNewer(a, b string) bool {
	dayA, seqA, datedA := changeLogStamp(a)
	dayB, seqB, datedB := changeLogStamp(b)
	if datedA != datedB {
		return datedA
	}
	if !datedA {
		return a < b
	}
	if dayA != dayB {
		return dayA > dayB
	}
	// Within a day, the file still being written is the newest one there is,
	// whatever number a rotation of it happens to carry: zero is not a
	// rotation, it is the day itself.
	if (seqA == 0) != (seqB == 0) {
		return seqA == 0
	}
	return seqA > seqB
}

// changeLogArchivesIn lists the archived logs in a directory, oldest first.
//
// It is a folder scan rather than a list of names held anywhere, because the
// names are decided when a log is rotated out: a build cannot know in advance
// which days it will be used on. Every place that has to know what is in the
// data directory goes through here (see dataFileNames), so an archive is listed,
// moved and recognised as ours by the same rule. This is the data directory
// itself, where logs written by older builds are; the folder the app writes in
// now is a data folder, and is listed and moved as a whole.
func changeLogArchivesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isChangeLogArchive(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// ChangeLogName checks a file name a window asked for and answers the file to
// read. An empty name is the file being written right now.
//
// The check is a whitelist rather than a cleanup: the only files this build
// reads are the ones it writes, so anything else — a path, a name in another
// folder, `connections.json` — is refused instead of being sanitised into
// something that happens to be safe.
func ChangeLogName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return changeLogDayName(time.Now()), nil
	}
	if name == changeLogFile || isChangeLogFile(name) {
		return name, nil
	}
	return "", fmt.Errorf("%s is not a change log this program wrote", name)
}

// changeLogDayName is what a day's log file is called: the day itself.
func changeLogDayName(at time.Time) string {
	return at.Format(changeLogDayFormat) + changeLogArchiveExt
}

// DefaultChangeLogSettings is the rotation policy of a store that has never been
// asked: what a fresh install rotates at, and the bounds the setting may take.
// It is exported so the service layer answers the same numbers when it has no
// store to read them from, instead of restating them and drifting.
func DefaultChangeLogSettings() models.ChangeLogSettings {
	return models.ChangeLogSettings{
		MaxEntries: defaultChangeLogEntries,
		Default:    defaultChangeLogEntries,
		Min:        minChangeLogEntries,
		Max:        maxChangeLogEntries,
	}
}

// isChangeLogFile reports whether a file name is one of the dated logs this
// build writes: a day, and — for a file that was rotated out — which rotation of
// that day it was.
func isChangeLogFile(name string) bool {
	_, _, ok := changeLogStamp(name)
	return ok
}

// isChangeLogArchive reports whether a file name is one of the rotated logs:
// eight digits of date, a hyphen, then which rotation of that day it was.
func isChangeLogArchive(name string) bool {
	_, seq, ok := changeLogStamp(name)
	return ok && seq > 0
}

// changeLogStamp reads a dated log file name: the day it belongs to, and which
// rotation of that day it was — zero for the day's own file, which is the one
// still being written and therefore the newest of that day.
func changeLogStamp(name string) (day string, seq int, ok bool) {
	base, found := strings.CutSuffix(name, changeLogArchiveExt)
	if !found {
		return "", 0, false
	}
	day, digits, numbered := strings.Cut(base, "-")
	if numbered {
		// The number starts at one and is written without padding, so a leading
		// zero — `-0`, `-007` — is a name this build could not have produced.
		if digits == "" || digits[0] == '0' {
			return "", 0, false
		}
		n, err := strconv.Atoi(digits)
		if err != nil || n <= 0 {
			return "", 0, false
		}
		seq = n
	}
	if len(day) != 8 || !isDigits(day) {
		return "", 0, false
	}
	// The day has to be a real one: a file called 99999999.log is not a log this
	// build wrote, and the listing should not offer it.
	if _, err := time.Parse(changeLogDayFormat, day); err != nil {
		return "", 0, false
	}
	return day, seq, true
}

// isDigits reports whether a string is nothing but ASCII digits.
func isDigits(text string) bool {
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// readChangeLogLocked reads one log file in file order.
func (s *Store) readChangeLogLocked(path string) ([]models.ChangeLogEntry, error) {
	raw, err := readChangeLogTail(path)
	if err != nil {
		return nil, err
	}
	entries := make([]models.ChangeLogEntry, 0, 64)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry models.ChangeLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// A line this build cannot read is skipped rather than failing the
			// whole file: it costs exactly the entries it holds, and a log that
			// refuses to open because of one bad line is worse than a short one.
			// It can be a line a crash cut in half, or a line someone typed.
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// countChangeLogEntries counts a file's entries without decoding them.
//
// What is being counted is whether the file is full, and a line that cannot be
// decoded still takes up room in it, so an unreadable line counts here even
// though the reader skips it.
func countChangeLogEntries(raw []byte) int {
	count := 0
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) > 0 {
			count++
		}
	}
	return count
}

// readChangeLogTail returns a log's contents, or its last maxChangeLogBytes when
// it is somehow longer than this build would ever write.
//
// A missing file is an empty log: a fresh install has no history, which is a
// state to be in rather than an error to report. A file that is being rotated at
// this very moment is the same thing — it is gone from the live name and not yet
// in the listing.
func readChangeLogTail(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() <= maxChangeLogBytes {
		return io.ReadAll(file)
	}
	if _, err := file.Seek(stat.Size()-maxChangeLogBytes, io.SeekStart); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	// The first line is almost certainly cut in half, and half a JSON object is
	// exactly the sort of line the reader would otherwise have to skip.
	if i := bytes.IndexByte(raw, '\n'); i >= 0 {
		raw = raw[i+1:]
	}
	return raw, nil
}
