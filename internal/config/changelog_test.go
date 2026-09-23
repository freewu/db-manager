package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// entry is a small helper: an entry that is identifiable by its statement.
func entry(statement string) models.ChangeLogEntry {
	return models.ChangeLogEntry{
		Version:    schemaVer,
		At:         1730000000000,
		Connection: models.ChangeLogConnection{Name: "shop", Driver: models.DriverMySQL, Address: "db:3306"},
		Database:   "shop",
		Table:      "orders",
		Kind:       "alter",
		Source:     models.ChangeSourceDesign,
		Statement:  statement,
	}
}

func TestChangeLogRoundTrip(t *testing.T) {
	store := escapedStore(t)

	// A fresh install has no history, which is a state rather than an error.
	page, total, err := store.ChangeLog(10)
	if err != nil || total != 0 || len(page) != 0 {
		t.Fatalf("a fresh store listed %v (%d, %v), want nothing", page, total, err)
	}

	if err := store.AppendChangeLog(entry("ALTER TABLE orders ADD note TEXT;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	// The second entry is written after the first, and read before it: the log
	// is read from the top, where the newest statement is.
	if err := store.AppendChangeLog(entry("DROP TABLE old_orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	// One line per entry, in the file the settings page lists.
	raw, err := os.ReadFile(filepath.Join(store.Dir(), changeLogFile))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if lines := strings.Count(string(raw), "\n"); lines != 2 {
		t.Fatalf("log holds %d lines, want 2:\n%s", lines, raw)
	}

	page, total, err = store.ChangeLog(10)
	if err != nil || total != 2 {
		t.Fatalf("change log = %v (total %d, %v), want two entries", page, total, err)
	}
	if page[0].Statement != "DROP TABLE old_orders;" || page[1].Statement != "ALTER TABLE orders ADD note TEXT;" {
		t.Fatalf("the log is not newest first: %+v", page)
	}
	if page[0].Connection.Name != "shop" || page[0].Connection.Address != "db:3306" {
		t.Fatalf("the connection summary did not survive: %+v", page[0].Connection)
	}

	// A page is a page: the total still says how much is behind it.
	page, total, err = store.ChangeLog(1)
	if err != nil || len(page) != 1 || total != 2 {
		t.Fatalf("a one-entry page returned %d entries of %d (%v)", len(page), total, err)
	}
}

func TestChangeLogDropsTheOldestEntriesAtItsLimit(t *testing.T) {
	store := escapedStore(t)

	// The file is seeded rather than appended to 2000 times over: what is being
	// tested is the trim that follows an append, and one rewrite of a full log
	// says the same thing as two thousand rewrites of a growing one.
	var seeded strings.Builder
	for i := 0; i < maxChangeLogEntries; i++ {
		seeded.WriteString(fmt.Sprintf("{\"version\":1,\"kind\":\"alter\",\"statement\":\"ALTER TABLE t ADD c%d INT;\"}\n", i))
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), changeLogFile), []byte(seeded.String()), fileMode); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := store.AppendChangeLog(entry("ALTER TABLE t ADD cnew INT;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	page, total, err := store.ChangeLog(maxChangeLogEntries)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if total != maxChangeLogEntries || len(page) != maxChangeLogEntries {
		t.Fatalf("log holds %d entries (page %d), want the cap of %d", total, len(page), maxChangeLogEntries)
	}
	// The oldest statement made room for the new one, which is on top.
	if page[0].Statement != "ALTER TABLE t ADD cnew INT;" {
		t.Fatalf("unexpected newest entry: %q", page[0].Statement)
	}
	if last := page[len(page)-1].Statement; last != "ALTER TABLE t ADD c1 INT;" {
		t.Fatalf("unexpected oldest entry: %q", last)
	}
}

// A line this build cannot read costs the entry it held, not the log.
func TestChangeLogSkipsALineItCannotRead(t *testing.T) {
	store := escapedStore(t)

	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	path := filepath.Join(store.Dir(), changeLogFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// A half-written line, the way a crash would leave one, plus a blank line.
	patched := string(raw) + "{not json\n\n" + string(raw)
	if err := os.WriteFile(path, []byte(patched), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	page, total, err := store.ChangeLog(10)
	if err != nil {
		t.Fatalf("a damaged line must not fail the read: %v", err)
	}
	if total != 2 || len(page) != 2 {
		t.Fatalf("read %d entries (total %d), want the two good ones", len(page), total)
	}
	// And the next append rewrites the file without the damaged line.
	if err := store.AppendChangeLog(entry("TRUNCATE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, total, err := store.ChangeLog(10); err != nil || total != 3 {
		t.Fatalf("after the append the log holds %d entries (%v), want 3", total, err)
	}
}

// The log is part of the data directory, so a move takes it along and the
// settings listing shows it — the same contract the profile file is held to.
func TestMoveDataCarriesTheChangeLog(t *testing.T) {
	def := withConfigHome(t)
	store, err := NewAt(def)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.AppendChangeLog(entry("CREATE TABLE orders (id INT);")); err != nil {
		t.Fatalf("append: %v", err)
	}

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var listed bool
	for _, file := range info.Files {
		if file.Name == changeLogFile && file.Bytes > 0 {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("the log is not listed in the data directory: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved := strings.Join(result.Moved, ", "); !strings.Contains(moved, changeLogFile) {
		t.Fatalf("the report does not mention the log: %v", result.Moved)
	}
	if _, err := os.Stat(filepath.Join(dest, changeLogFile)); err != nil {
		t.Fatalf("the log did not arrive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, changeLogFile)); !os.IsNotExist(err) {
		t.Fatal("the old log is still there")
	}

	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open moved store: %v", err)
	}
	if _, total, err := again.ChangeLog(10); err != nil || total != 1 {
		t.Fatalf("the moved log holds %d entries (%v), want 1", total, err)
	}
}
