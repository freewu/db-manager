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

// fillChangeLog puts exactly n entries in the live log the quick way: writing
// the file an append would have produced says the same thing as appending n
// times, and the rotation that follows is what is being tested.
func fillChangeLog(t *testing.T, store *Store, n int) {
	t.Helper()
	var seeded strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&seeded, "{\"version\":1,\"kind\":\"alter\",\"statement\":\"ALTER TABLE t ADD c%d INT;\"}\n", i)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), changeLogFile), []byte(seeded.String()), fileMode); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// archives lists the archived logs, which is what a test asserts about.
func archives(t *testing.T, store *Store) []string {
	t.Helper()
	names, err := changeLogArchivesIn(store.Dir())
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	return names
}

func TestChangeLogRoundTrip(t *testing.T) {
	store := escapedStore(t)

	// A fresh install has no history, which is a state rather than an error.
	empty, err := store.ChangeLog("", 10)
	if err != nil || empty.Total != 0 || len(empty.Entries) != 0 {
		t.Fatalf("a fresh store listed %v (%d, %v), want nothing", empty.Entries, empty.Total, err)
	}
	if empty.File != changeLogFile {
		t.Fatalf("an empty name should read the live log, got %q", empty.File)
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

	log, err := store.ChangeLog("", 10)
	if err != nil || log.Total != 2 {
		t.Fatalf("change log = %v (total %d, %v), want two entries", log.Entries, log.Total, err)
	}
	if log.Entries[0].Statement != "DROP TABLE old_orders;" || log.Entries[1].Statement != "ALTER TABLE orders ADD note TEXT;" {
		t.Fatalf("the log is not newest first: %+v", log.Entries)
	}
	if log.Entries[0].Connection.Name != "shop" || log.Entries[0].Connection.Address != "db:3306" {
		t.Fatalf("the connection summary did not survive: %+v", log.Entries[0].Connection)
	}

	// A page is a page: the total still says how much is behind it.
	page, err := store.ChangeLog("", 1)
	if err != nil || len(page.Entries) != 1 || page.Total != 2 {
		t.Fatalf("a one-entry page returned %d entries of %d (%v)", len(page.Entries), page.Total, err)
	}

	// The listing names the file being written and how much is in it.
	if len(log.Files) != 1 || log.Files[0].Name != changeLogFile || log.Files[0].Archived || log.Files[0].Entries != 2 {
		t.Fatalf("unexpected file listing: %+v", log.Files)
	}
	if log.Files[0].Bytes == 0 {
		t.Fatalf("a file with entries in it has a size: %+v", log.Files[0])
	}
}

// A full log is moved aside whole, never trimmed: nothing a user has been shown
// disappears on its own, and the older file is there to be opened.
func TestChangeLogRotatesInsteadOfDroppingEntries(t *testing.T) {
	store := escapedStore(t)

	// The smallest log the setting allows, so filling one is cheap. The cap is
	// the user's, and rotation follows it rather than a number of its own.
	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: minChangeLogEntries}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	fillChangeLog(t, store, minChangeLogEntries)

	if err := store.AppendChangeLog(entry("ALTER TABLE t ADD cnew INT;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	// The old entries are all in one archive, and the live file holds only the
	// entry that did not fit.
	names := archives(t, store)
	if len(names) != 1 {
		t.Fatalf("expected one archive, got %v", names)
	}
	if !isChangeLogArchive(names[0]) {
		t.Fatalf("an archive is named for the day it was rotated out, got %q", names[0])
	}
	live, err := store.ChangeLog("", 10)
	if err != nil || live.Total != 1 || live.Entries[0].Statement != "ALTER TABLE t ADD cnew INT;" {
		t.Fatalf("the live log holds %+v (%d, %v), want the new entry alone", live.Entries, live.Total, err)
	}
	archived, err := store.ChangeLog(names[0], minChangeLogEntries)
	if err != nil || archived.Total != minChangeLogEntries {
		t.Fatalf("the archive holds %d entries (%v), want all %d", archived.Total, err, minChangeLogEntries)
	}
	old := archived.Entries
	// Oldest first on disk, newest first in the answer: the last statement that
	// went into the archive is the one on top of it.
	if old[0].Statement != fmt.Sprintf("ALTER TABLE t ADD c%d INT;", minChangeLogEntries-1) {
		t.Fatalf("unexpected newest entry in the archive: %q", old[0].Statement)
	}
	if last := old[len(old)-1].Statement; last != "ALTER TABLE t ADD c0 INT;" {
		t.Fatalf("unexpected oldest entry in the archive: %q", last)
	}

	// Both files are offered to the window, the live one first.
	files := archived.Files
	if len(files) != 2 || files[0].Name != changeLogFile || files[0].Entries != 1 {
		t.Fatalf("unexpected file listing: %+v", files)
	}
	if files[1].Name != names[0] || !files[1].Archived || files[1].Entries != minChangeLogEntries {
		t.Fatalf("the archive is not listed as one: %+v", files[1])
	}
	if files[1].At == 0 {
		t.Fatalf("an archive says when it was rotated out: %+v", files[1])
	}
}

// Two rotations on the same day do not overwrite each other: the name carries
// which rotation of that day it was.
func TestChangeLogArchivesAreNumberedWithinADay(t *testing.T) {
	store := escapedStore(t)
	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: minChangeLogEntries}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	fillChangeLog(t, store, minChangeLogEntries)
	if err := store.AppendChangeLog(entry("ALTER TABLE t ADD first INT;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	fillChangeLog(t, store, minChangeLogEntries)
	if err := store.AppendChangeLog(entry("ALTER TABLE t ADD second INT;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	names := archives(t, store)
	if len(names) != 2 {
		t.Fatalf("expected two archives, got %v", names)
	}
	if names[0] == names[1] {
		t.Fatalf("the second rotation reused the first name: %v", names)
	}
	// Oldest first, which is the order the names sort in, and the same day with
	// the next number along — not the same file written twice.
	if !strings.HasSuffix(names[0], "-1"+changeLogArchiveExt) {
		t.Fatalf("expected the first rotation of the day, got %q", names[0])
	}
	want := strings.TrimSuffix(names[0], "-1"+changeLogArchiveExt) + "-2" + changeLogArchiveExt
	if names[1] != want {
		t.Fatalf("expected %s, got %s", want, names[1])
	}

	// Both archives are readable, and the second one holds the second day's
	// worth of statements rather than being a copy of the first.
	second, err := store.ChangeLog(names[1], 1)
	if err != nil || second.Total != minChangeLogEntries {
		t.Fatalf("read the second archive: %d entries (%v)", second.Total, err)
	}
	if second.Entries[0].Statement != fmt.Sprintf("ALTER TABLE t ADD c%d INT;", minChangeLogEntries-1) {
		t.Fatalf("unexpected entry from the second archive: %q", second.Entries[0].Statement)
	}
}

// The name is the whole protection on the read path: a window asks for a file by
// name, so only the files this build writes are read.
func TestChangeLogOnlyReadsFilesItWrote(t *testing.T) {
	store := escapedStore(t)
	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	// A recognisable file in the same directory, to prove it is not reachable
	// through the log.
	if err := os.WriteFile(filepath.Join(store.Dir(), fileName), []byte("{}\n"), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, name := range []string{
		fileName,
		"../" + fileName,
		filepath.Join("..", fileName),
		"changelog.json",
		"20260214-1.json",
		"20260214-0.log",
		"2026-1.log",
		"20260214-x.log",
		"20261340-1.log", // a date that does not exist
		"notes.log",
	} {
		if _, err := store.ChangeLog(name, 5); err == nil {
			t.Errorf("%q was read as a change log", name)
		}
	}

	// And the two names a window really asks for are accepted: the live log by
	// an empty name, an archive by its own.
	if _, err := store.ChangeLog("", 5); err != nil {
		t.Fatalf("the live log: %v", err)
	}
	if _, err := store.ChangeLog("  "+changeLogFile+" ", 5); err != nil {
		t.Fatalf("a name with space around it: %v", err)
	}
	if missing, err := store.ChangeLog("20260214-1.log", 5); err != nil {
		// A name that is well formed but not there yet is an empty log rather
		// than a refusal: the alert is about the name, not the file.
		t.Fatalf("a well-formed archive name: %v", err)
	} else if missing.Total != 0 {
		t.Fatalf("a missing archive holds %d entries", missing.Total)
	}
}

func TestIsChangeLogArchive(t *testing.T) {
	yes := []string{"20260214-1.log", "19991231-42.log"}
	no := []string{
		"", "changelog.jsonl", "20260214-1.log.tmp", "20260214-1.LOG", "20260214-.log",
		"-1.log", "2026021-1.log", "202602141-1.log", "20260214-1.log.bak", "20261301-1.log",
		"20260214-0.log", "20260214-007.log", // numbering starts at one, unpadded
		"20260230-1.log", // February never has a 30th
	}
	for _, name := range yes {
		if !isChangeLogArchive(name) {
			t.Errorf("%q should be an archive", name)
		}
	}
	for _, name := range no {
		if isChangeLogArchive(name) {
			t.Errorf("%q should not be an archive", name)
		}
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

	page, err := store.ChangeLog("", 10)
	if err != nil {
		t.Fatalf("a damaged line must not fail the read: %v", err)
	}
	if page.Total != 2 || len(page.Entries) != 2 {
		t.Fatalf("read %d entries (total %d), want the two good ones", len(page.Entries), page.Total)
	}
	// The next append goes on the end: an append is what the log does, so the
	// damaged line stays where it is and stays skipped.
	if err := store.AppendChangeLog(entry("TRUNCATE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if after, err := store.ChangeLog("", 10); err != nil || after.Total != 3 {
		t.Fatalf("after the append the log holds %d entries (%v), want 3", after.Total, err)
	}
}

func TestChangeLogSettingsRoundTrip(t *testing.T) {
	store := escapedStore(t)

	// Nothing saved is the default policy, and the answer carries the bounds the
	// setting may take so the settings page does not have to restate them.
	settings, err := store.ChangeLogSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if settings.MaxEntries != defaultChangeLogEntries || settings.Default != defaultChangeLogEntries {
		t.Fatalf("a fresh store should rotate at the default: %+v", settings)
	}
	if settings.Min != minChangeLogEntries || settings.Max != maxChangeLogEntries {
		t.Fatalf("unexpected bounds: %+v", settings)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), changeLogSettingsFile)); !os.IsNotExist(err) {
		t.Fatal("nothing was chosen, so nothing should be written")
	}

	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: 500}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if settings, err = store.ChangeLogSettings(); err != nil || settings.MaxEntries != 500 {
		t.Fatalf("settings = %+v (%v), want 500", settings, err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), changeLogSettingsFile)); err != nil {
		t.Fatalf("the choice was not written: %v", err)
	}
	// The chosen size is what rotation follows.
	fillChangeLog(t, store, 500)
	if err := store.AppendChangeLog(entry("ALTER TABLE t ADD c INT;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if names := archives(t, store); len(names) != 1 {
		t.Fatalf("expected the 500-entry log to rotate, got %v", names)
	}

	// Going back to the default is the same as never having chosen: the file
	// goes away rather than saying what the default already says.
	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: defaultChangeLogEntries}); err != nil {
		t.Fatalf("save default: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), changeLogSettingsFile)); !os.IsNotExist(err) {
		t.Fatal("the settings file should be gone once the value is the default")
	}
	if settings, err = store.ChangeLogSettings(); err != nil || settings.MaxEntries != defaultChangeLogEntries {
		t.Fatalf("settings = %+v (%v), want the default back", settings, err)
	}
}

// A settings file this build cannot read is the default policy, for the same
// reason the log skips a line it cannot read: neither may be able to stop the
// application from recording what it runs.
func TestChangeLogSettingsFallBackToTheDefault(t *testing.T) {
	for _, raw := range []string{
		"{not json",
		"{}",
		`{"version":1,"maxEntries":1}`,
		`{"version":1,"maxEntries":-5}`,
		`{"version":1,"maxEntries":99999999}`,
	} {
		store := escapedStore(t)
		if err := os.WriteFile(filepath.Join(store.Dir(), changeLogSettingsFile), []byte(raw), fileMode); err != nil {
			t.Fatalf("write: %v", err)
		}
		settings, err := store.ChangeLogSettings()
		if err != nil {
			t.Fatalf("%s: reading settings must not fail: %v", raw, err)
		}
		if settings.MaxEntries != defaultChangeLogEntries {
			t.Fatalf("%s: expected the default, got %d", raw, settings.MaxEntries)
		}
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
	if moved, err := again.ChangeLog("", 10); err != nil || moved.Total != 1 {
		t.Fatalf("the moved log holds %d entries (%v), want 1", moved.Total, err)
	}
}

// An archived log is data like any other, so it is listed, moved and — this is
// the one that would be easy to get wrong — not reported as somebody else's file
// in the old directory.
func TestMoveDataCarriesTheArchivedLogs(t *testing.T) {
	def := withConfigHome(t)
	store, err := NewAt(def)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: minChangeLogEntries}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	fillChangeLog(t, store, minChangeLogEntries)
	if err := store.AppendChangeLog(entry("DROP TABLE audit;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	names := archives(t, store)
	if len(names) != 1 {
		t.Fatalf("expected one archive to move, got %v", names)
	}
	archive := names[0]

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var listed bool
	for _, file := range info.Files {
		if file.Name == archive && file.Bytes > 0 {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("the archive is not listed in the data directory: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved := strings.Join(result.Moved, ", "); !strings.Contains(moved, archive) {
		t.Fatalf("the report does not mention the archive: %v", result.Moved)
	}
	if len(result.LeftBehind) != 0 {
		t.Fatalf("an archive is ours, not something left behind: %v", result.LeftBehind)
	}
	if _, err := os.Stat(filepath.Join(dest, archive)); err != nil {
		t.Fatalf("the archive did not arrive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, archive)); !os.IsNotExist(err) {
		t.Fatal("the old archive is still there")
	}
	// The chosen rotation size travels with the log it governs.
	if _, err := os.Stat(filepath.Join(dest, changeLogSettingsFile)); err != nil {
		t.Fatalf("the log settings did not arrive: %v", err)
	}
	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open moved store: %v", err)
	}
	if settings, err := again.ChangeLogSettings(); err != nil || settings.MaxEntries != minChangeLogEntries {
		t.Fatalf("the moved settings say %+v (%v)", settings, err)
	}
	if moved, err := again.ChangeLog(archive, 10); err != nil || moved.Total != minChangeLogEntries {
		t.Fatalf("the moved archive holds %d entries (%v)", moved.Total, err)
	}
}

// A directory that already holds an archived log is not empty, and moving data
// into it would overwrite it.
func TestMoveDataRefusesADirectoryHoldingAnArchive(t *testing.T) {
	def := withConfigHome(t)
	dest := filepath.Join(t.TempDir(), "used")
	if err := os.MkdirAll(dest, dirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "20260214-1.log"), []byte("{\"version\":1}\n"), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := MoveData(dest); err == nil {
		t.Fatalf("moving into %s should have been refused", dest)
	} else if !strings.Contains(err.Error(), "20260214-1.log") {
		t.Fatalf("the refusal should name the file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, changeLogFile)); !os.IsNotExist(err) {
		t.Fatal("nothing should have been moved")
	}
}
