// The change log: what this application ran against the databases it was
// pointed at.
//
//	<data directory>/changelog.jsonl
//	{"version":1,"at":1730000000000,"connection":{"name":"shop","driver":"mysql",…},"database":"shop","table":"orders","kind":"alter","source":"design","statement":"ALTER TABLE `orders` …"}
//
// One line of JSON per entry, rather than an array in one file, because of how
// this file is used: it is appended to far more often than it is read, always in
// the same direction, and it is the one file here that grows without a user
// asking for it. A line that cannot be decoded is one lost entry instead of a
// whole log that refuses to open, and the oldest lines are dropped once the log
// is at its limit — a log is read from the top.
//
// Nothing in this file is a gate in front of anything: an entry is written after
// the statement it describes has run, so a log that cannot be written must never
// turn a change that happened into an error the user sees.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dbmanager/internal/models"
)

const (
	// changeLogFile is the log inside the data directory.
	changeLogFile = "changelog.jsonl"
	// maxChangeLogEntries caps the file. It is a window into the recent past
	// rather than an archive: every entry holds a whole statement, and a log
	// nobody can scroll to the end of is a log nobody reads.
	maxChangeLogEntries = 2000
	// maxChangeLogBytes bounds the read. With the entry cap this is about ten
	// times what the file can hold in practice; it exists so that a log damaged
	// or padded by something other than this program cannot be pulled into
	// memory whole. Past it, the newest bytes are kept and the rest is left
	// where it is — the top of the log is what is read, and what gets rewritten.
	maxChangeLogBytes = 8 << 20
)

// AppendChangeLog adds one entry, dropping the oldest entries once the log is at
// its limit.
//
// The whole file is rewritten rather than appended to: trimming and writing are
// the same operation then, and the write stays atomic (temp file, then rename),
// which an append cannot be — a half-written line would sit in the log until the
// next build decided to skip it.
func (s *Store) AppendChangeLog(entry models.ChangeLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.changeLogLocked()
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	if len(entries) > maxChangeLogEntries {
		entries = entries[len(entries)-maxChangeLogEntries:]
	}
	return s.writeChangeLogLocked(entries)
}

// ChangeLog returns the newest entries first, plus how many the file holds.
//
// Oldest first on disk and newest first here, because that is the only way it is
// ever read: the appends go to the end, and a reader wants the last statement
// that ran, not the first.
func (s *Store) ChangeLog(limit int) ([]models.ChangeLogEntry, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.changeLogLocked()
	if err != nil {
		return nil, 0, err
	}
	total := len(entries)
	page := make([]models.ChangeLogEntry, 0, min(limit, total))
	for i := total - 1; i >= 0 && len(page) < limit; i-- {
		page = append(page, entries[i])
	}
	return page, total, nil
}

func (s *Store) changeLogPath() string { return filepath.Join(s.dir, changeLogFile) }

// changeLogLocked reads the log in file order.
func (s *Store) changeLogLocked() ([]models.ChangeLogEntry, error) {
	raw, err := readChangeLogTail(s.changeLogPath())
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

// writeChangeLogLocked replaces the log with exactly these entries.
func (s *Store) writeChangeLogLocked(entries []models.ChangeLogEntry) error {
	var buf bytes.Buffer
	for _, entry := range entries {
		raw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		buf.Write(raw)
		buf.WriteByte('\n')
	}
	path := s.changeLogPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readChangeLogTail returns the log's contents, or its last maxChangeLogBytes
// when it is somehow longer than this build would ever write.
//
// A missing file is an empty log: a fresh install has no history, which is a
// state to be in rather than an error to report.
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
