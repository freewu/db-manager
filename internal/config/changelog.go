// The change log: what this application ran against the databases it was
// pointed at.
//
//	<data directory>/changelog.jsonl
//	{"version":1,"at":1730000000000,"connection":{"name":"shop","driver":"mysql",…},"database":"shop","table":"orders","kind":"alter","source":"design","statement":"ALTER TABLE `orders` …"}
//	<data directory>/20260214-1.log       (an archived log, same format)
//	<data directory>/changelog.json       {"version":1,"maxEntries":2000}
//
// One line of JSON per entry, rather than an array in one file, because of how
// this file is used: it is appended to far more often than it is read, always in
// the same direction, and it is the one file here that grows without a user
// asking for it. A line that cannot be decoded is one lost entry instead of a
// whole log that refuses to open.
//
// Nothing is ever dropped. When the live file holds as many statements as the
// user allows it to, it is renamed to `<yyyymmdd>-<n>.log` — the calendar date
// it was rotated out, then which rotation of that day it was — and a fresh
// `changelog.jsonl` is started. Renaming a whole file is one atomic step, which
// is what makes rotating free: no copying, no rewriting, and a log that is
// being read at that moment is either entirely the old file or entirely the new
// one. The settings file next to it holds the one number that decides when that
// happens.
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
	"strings"
	"time"

	"dbmanager/internal/models"
)

const (
	// changeLogFile is the log inside the data directory.
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
	// changeLogArchiveExt is what an archived log is called.
	changeLogArchiveExt = ".log"
)

// changeLogSettingsFormat is the settings file on disk.
type changeLogSettingsFormat struct {
	Version    int `json:"version"`
	MaxEntries int `json:"maxEntries"`
}

// AppendChangeLog adds one entry, archiving the live log first when it is full.
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

	path := s.changeLogPath()
	// The rotated file is safe under its new name before the new entry is
	// written, so a failure from here on costs the entry, never the history.
	if _, err := s.rotateChangeLogLocked(path, settings.MaxEntries); err != nil {
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
// An empty file name means the live log. Oldest first on disk and newest first
// here, because that is the only way it is ever read: the appends go to the end,
// and a reader wants the last statement that ran, not the first.
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
	entries, err := s.readChangeLogLocked(filepath.Join(s.dir, name))
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

func (s *Store) changeLogPath() string { return filepath.Join(s.dir, changeLogFile) }

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

// rotateChangeLogLocked moves the live log aside when it is full, and answers
// the name it was given (empty when nothing was rotated).
//
// The file is renamed, never rewritten or truncated: whatever it holds — a
// complete log, a line a crash cut in half, something else entirely if the file
// was padded — moves aside as it is, under a name that says when it was taken
// out. A file that is somehow larger than this build would ever write is rotated
// too: it is not one this build is going to keep appending to forever.
func (s *Store) rotateChangeLogLocked(path string, maxEntries int) (string, error) {
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

	name := s.freeArchiveNameLocked(time.Now())
	if err := os.Rename(path, filepath.Join(s.dir, name)); err != nil {
		return "", err
	}
	return name, nil
}

// freeArchiveNameLocked picks the next archive name for that day: the date it
// was rotated out, then which rotation of that day it was.
func (s *Store) freeArchiveNameLocked(at time.Time) string {
	day := at.Format("20060102")
	for n := 1; ; n++ {
		name := fmt.Sprintf("%s-%d%s", day, n, changeLogArchiveExt)
		if _, err := os.Stat(filepath.Join(s.dir, name)); errors.Is(err, fs.ErrNotExist) {
			return name
		}
	}
}

// changeLogFilesLocked lists the files the log is spread over: the live one
// first, then the archives, newest first.
//
// The entries in each file are counted, which means reading them — bounded by
// maxChangeLogBytes each, and a window that shows the wrong number of entries is
// worse than one that took a moment to open.
func (s *Store) changeLogFilesLocked() ([]models.ChangeLogFile, error) {
	files := make([]models.ChangeLogFile, 0, 8)
	if stat, err := os.Stat(s.changeLogPath()); err == nil && !stat.IsDir() {
		raw, err := readChangeLogTail(s.changeLogPath())
		if err != nil {
			return nil, err
		}
		files = append(files, models.ChangeLogFile{
			Name:    changeLogFile,
			Bytes:   stat.Size(),
			Entries: countChangeLogEntries(raw),
		})
	}
	archives, err := s.changeLogArchivesLocked()
	if err != nil {
		return nil, err
	}
	for _, name := range archives {
		stat, err := os.Stat(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		raw, err := readChangeLogTail(filepath.Join(s.dir, name))
		if err != nil {
			return nil, err
		}
		info := models.ChangeLogFile{
			Name:     name,
			Archived: true,
			Bytes:    stat.Size(),
			Entries:  countChangeLogEntries(raw),
		}
		// The name carries the day it was rotated out; the file's own timestamp
		// is when the last statement in it ran, which is what a reader wants.
		info.At = stat.ModTime().UnixMilli()
		files = append(files, info)
	}
	return files, nil
}

// changeLogArchivesLocked lists the archived logs, oldest first (which is also
// the order their names are in, since a name starts with its date).
func (s *Store) changeLogArchivesLocked() ([]string, error) {
	return changeLogArchivesIn(s.dir)
}

// changeLogArchivesIn lists the archived logs in a directory, oldest first.
//
// It is a folder scan rather than a list of names held anywhere, because the
// names are decided when a log is rotated out: a build cannot know in advance
// which days it will be used on. Every place that has to know what is in the
// data directory goes through here (see dataFileNames), so an archive is listed,
// moved and recognised as ours by the same rule.
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
// read. An empty name is the live log.
//
// The check is a whitelist rather than a cleanup: the only files this build
// reads are the ones it writes, so anything else — a path, a name in another
// folder, `connections.json` — is refused instead of being sanitised into
// something that happens to be safe.
func ChangeLogName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return changeLogFile, nil
	}
	if name == changeLogFile || isChangeLogArchive(name) {
		return name, nil
	}
	return "", fmt.Errorf("%s is not a change log this program wrote", name)
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

// isChangeLogArchive reports whether a file name is one of the rotated logs:
// eight digits of date, a hyphen, then which rotation of that day it was.
func isChangeLogArchive(name string) bool {
	base, ok := strings.CutSuffix(name, changeLogArchiveExt)
	if !ok {
		return false
	}
	date, seq, ok := strings.Cut(base, "-")
	if !ok || len(date) != 8 || seq == "" {
		return false
	}
	// The number starts at one and is written without padding, so a leading zero
	// — `-0`, `-007` — is a name this build could not have produced.
	if seq[0] == '0' {
		return false
	}
	for _, part := range []string{date, seq} {
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	// The date has to be a real one: a file called 99999999-1.log is not a log
	// this build wrote, and the archive list should not offer it.
	_, err := time.Parse("20060102", date)
	return err == nil
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
