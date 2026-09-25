package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dbmanager/internal/models"
)

// entry is a small helper: an entry that is identifiable by its statement, and
// by the size of the change it reports.
//
// The timestamp comes from the clock rather than from a constant, because which
// file an entry lands in is one of the things being tested: the app writes into
// the file named for today, and a test that pinned the timestamp would be
// asserting about a day the reader is not looking at. Every entry in a test
// carries the clock's answer, so the rotation tests stay inside one file.
func entry(statement string) models.ChangeLogEntry {
	return entryAt(statement, time.Now().UnixMilli())
}

// entryAt is entry with the timestamp spelled out, for the few places where the
// day an entry belongs to is the point.
func entryAt(statement string, at int64) models.ChangeLogEntry {
	return models.ChangeLogEntry{
		Version:    schemaVer,
		At:         at,
		Connection: models.ChangeLogConnection{Name: "shop", Driver: models.DriverMySQL, Address: "db:3306"},
		Database:   "shop",
		Table:      "orders",
		Kind:       "alter",
		Source:     models.ChangeSourceDesign,
		Statement:  statement,
		Rows:       3,
	}
}

// dayName is the file an append writes to right now: the day's own name.
func dayName() string { return changeLogDayName(time.Now()) }

// logDir is the folder the logs live in.
func logDir(store *Store) string { return filepath.Join(store.Dir(), changeLogDirName) }

// fillChangeLog puts exactly n entries in the day's live log the quick way:
// writing the file an append would have produced says the same thing as
// appending n times, and the rotation that follows is what is being tested.
func fillChangeLog(t *testing.T, store *Store, n int) {
	t.Helper()
	var seeded strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&seeded, "{\"version\":1,\"kind\":\"alter\",\"statement\":\"ALTER TABLE t ADD c%d INT;\"}\n", i)
	}
	if err := os.MkdirAll(logDir(store), dirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir(store), dayName()), []byte(seeded.String()), fileMode); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// archives lists the rotated logs in the folder, which is what a test asserts
// about.
func archives(t *testing.T, store *Store) []string {
	t.Helper()
	names, err := changeLogArchivesIn(logDir(store))
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	return names
}

// names lists every log there is to choose between, in the order the window
// offers them.
func names(t *testing.T, store *Store) []string {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	listed, err := store.changeLogNamesLocked()
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	return listed
}

func TestChangeLogRoundTrip(t *testing.T) {
	store := escapedStore(t)

	// A fresh install has no history, which is a state rather than an error.
	empty, err := store.ChangeLog("", 10)
	if err != nil || empty.Total != 0 || len(empty.Entries) != 0 {
		t.Fatalf("a fresh store listed %v (%d, %v), want nothing", empty.Entries, empty.Total, err)
	}
	// An empty name means the file being written right now, which is today's:
	// there is no single log file any more, so "the log" has to be a day.
	if empty.File != dayName() {
		t.Fatalf("an empty name should read today's log, got %q", empty.File)
	}

	if err := store.AppendChangeLog(entry("ALTER TABLE orders ADD note TEXT;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	// The second entry is written after the first, and read before it: the log
	// is read from the top, where the newest statement is.
	if err := store.AppendChangeLog(entry("DROP TABLE old_orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	// One line per entry, in the day's file in the log folder.
	raw, err := os.ReadFile(filepath.Join(logDir(store), dayName()))
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
	// The size of the change is part of the entry, and survives a round trip
	// through the file like everything else in it.
	if log.Entries[0].Rows != 3 {
		t.Fatalf("the row count did not survive: %+v", log.Entries[0])
	}

	// A page is a page: the total still says how much is behind it.
	page, err := store.ChangeLog("", 1)
	if err != nil || len(page.Entries) != 1 || page.Total != 2 {
		t.Fatalf("a one-entry page returned %d entries of %d (%v)", len(page.Entries), page.Total, err)
	}

	// The listing names the file being written and how much is in it. It is not
	// an archive: it is the one an append goes to next.
	if len(log.Files) != 1 || log.Files[0].Name != dayName() || log.Files[0].Archived || log.Files[0].Entries != 2 {
		t.Fatalf("unexpected file listing: %+v", log.Files)
	}
	if log.Files[0].Bytes == 0 {
		t.Fatalf("a file with entries in it has a size: %+v", log.Files[0])
	}
}

// An entry that was a minute stale at midnight lands in the file of the day it
// ran on, not in the one named after the clock at the moment it was written.
func TestChangeLogFilesAnEntryUnderItsOwnDay(t *testing.T) {
	store := escapedStore(t)
	now := time.Now()
	// Yesterday at the same time of day, which is a different file whatever the
	// clock says: the test is about the stamp being read, not about a midnight
	// race.
	yesterday := now.AddDate(0, 0, -1)
	if err := store.AppendChangeLog(entryAt("ALTER TABLE t ADD c INT;", yesterday.UnixMilli())); err != nil {
		t.Fatalf("append: %v", err)
	}

	stale := changeLogDayName(yesterday)
	if _, err := os.Stat(filepath.Join(logDir(store), stale)); err != nil {
		t.Fatalf("the entry did not go to its own day's file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir(store), dayName())); !os.IsNotExist(err) {
		t.Fatal("nothing should have been written to today's file")
	}
	// Yesterday is not the file being written, so it is offered as a finished
	// one — which is also how a day that has ended reads.
	log, err := store.ChangeLog(stale, 10)
	if err != nil || log.Total != 1 {
		t.Fatalf("read yesterday: %d entries (%v)", log.Total, err)
	}
	if !log.Files[0].Archived || log.Files[0].At == 0 {
		t.Fatalf("a finished day is listed as one: %+v", log.Files[0])
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
	rotated := archives(t, store)
	if len(rotated) != 1 {
		t.Fatalf("expected one archive, got %v", rotated)
	}
	// The archive is the day's own file with the rotation number on it, and the
	// day is the day the entries carry.
	wantArchive := strings.TrimSuffix(dayName(), changeLogArchiveExt) + "-1" + changeLogArchiveExt
	if rotated[0] != wantArchive {
		t.Fatalf("expected %s, got %s", wantArchive, rotated[0])
	}
	if !isChangeLogArchive(rotated[0]) {
		t.Fatalf("an archive is named for the day it was rotated out, got %q", rotated[0])
	}
	live, err := store.ChangeLog("", 10)
	if err != nil || live.Total != 1 || live.Entries[0].Statement != "ALTER TABLE t ADD cnew INT;" {
		t.Fatalf("the live log holds %+v (%d, %v), want the new entry alone", live.Entries, live.Total, err)
	}
	archived, err := store.ChangeLog(rotated[0], minChangeLogEntries)
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

	// Both files are offered to the window, the live one first — that is the one
	// an append is about to go into.
	files := archived.Files
	if len(files) != 2 || files[0].Name != dayName() || files[0].Entries != 1 {
		t.Fatalf("unexpected file listing: %+v", files)
	}
	if files[1].Name != rotated[0] || !files[1].Archived || files[1].Entries != minChangeLogEntries {
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

	rotated := archives(t, store)
	if len(rotated) != 2 {
		t.Fatalf("expected two archives, got %v", rotated)
	}
	if rotated[0] == rotated[1] {
		t.Fatalf("the second rotation reused the first name: %v", rotated)
	}
	// Oldest first, which is the order the names sort in, and the same day with
	// the next number along — not the same file written twice.
	if !strings.HasSuffix(rotated[0], "-1"+changeLogArchiveExt) {
		t.Fatalf("expected the first rotation of the day, got %q", rotated[0])
	}
	want := strings.TrimSuffix(rotated[0], "-1"+changeLogArchiveExt) + "-2" + changeLogArchiveExt
	if rotated[1] != want {
		t.Fatalf("expected %s, got %s", want, rotated[1])
	}

	// Both archives are readable, and the second one holds the second day's
	// worth of statements rather than being a copy of the first.
	second, err := store.ChangeLog(rotated[1], 1)
	if err != nil || second.Total != minChangeLogEntries {
		t.Fatalf("read the second archive: %d entries (%v)", second.Total, err)
	}
	if second.Entries[0].Statement != fmt.Sprintf("ALTER TABLE t ADD c%d INT;", minChangeLogEntries-1) {
		t.Fatalf("unexpected entry from the second archive: %q", second.Entries[0].Statement)
	}

	// The day's file and both of its rotations are offered, newest first: the
	// file being written, then the rotation that has just been taken out, then
	// the one before it.
	listed := names(t, store)
	wantNames := []string{dayName(), rotated[1], rotated[0]}
	if len(listed) != len(wantNames) {
		t.Fatalf("expected %v, got %v", wantNames, listed)
	}
	for i := range wantNames {
		if listed[i] != wantNames[i] {
			t.Fatalf("expected %v, got %v", wantNames, listed)
		}
	}
}

// The logs of a build from before the folder existed are still listed and still
// read. Upgrading must not look like losing history, and nothing is migrated:
// the files stay where the user's other copies of them are.
func TestChangeLogStillReadsTheLogsOfOlderBuilds(t *testing.T) {
	store := escapedStore(t)
	legacy := "{\"version\":1,\"kind\":\"alter\",\"statement\":\"ALTER TABLE t ADD old INT;\"}\n"
	if err := os.WriteFile(filepath.Join(store.Dir(), changeLogFile), []byte(legacy), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	// An archive of that layout, in the data directory itself.
	oldArchive := "20260214-1" + changeLogArchiveExt
	if err := os.WriteFile(filepath.Join(store.Dir(), oldArchive), []byte(legacy), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	// And a new entry, which goes to the folder as the app writes them now.
	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Everything is offered, the folder's file first and the log with no day in
	// its name last, because that is the oldest thing here.
	listed := names(t, store)
	if len(listed) != 3 || listed[0] != dayName() || listed[2] != changeLogFile {
		t.Fatalf("unexpected listing: %v", listed)
	}
	if listed[1] != oldArchive {
		t.Fatalf("the older archive should sort between them: %v", listed)
	}

	// Both older files read as they always did.
	if old, err := store.ChangeLog(changeLogFile, 10); err != nil || old.Total != 1 {
		t.Fatalf("the log of the older build holds %d entries (%v)", old.Total, err)
	}
	if old, err := store.ChangeLog(oldArchive, 10); err != nil || old.Total != 1 {
		t.Fatalf("the archive of the older build holds %d entries (%v)", old.Total, err)
	}
}

// A name the folder also holds is the folder's. That is the file being written;
// the copy beside it is what a user leaves behind by copying files around, and
// reading it instead would show them a log that stopped a while ago.
func TestChangeLogPrefersTheFolderOverTheOldLayout(t *testing.T) {
	store := escapedStore(t)
	if err := store.SaveChangeLogSettings(models.ChangeLogSettings{MaxEntries: minChangeLogEntries}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	// A log that rotates once, so the name in question is a real archive.
	fillChangeLog(t, store, minChangeLogEntries)
	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	rotated := archives(t, store)
	if len(rotated) != 1 {
		t.Fatalf("expected one archive, got %v", rotated)
	}
	// The same name, in the old place, holding one entry instead of many.
	if err := os.WriteFile(
		filepath.Join(store.Dir(), rotated[0]),
		[]byte("{\"version\":1,\"kind\":\"alter\",\"statement\":\"ALTER TABLE t ADD leftover INT;\"}\n"),
		fileMode,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Offered once, and it is the folder's copy.
	listed := names(t, store)
	count := 0
	for _, name := range listed {
		if name == rotated[0] {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the same name was offered %d times: %v", count, listed)
	}
	log, err := store.ChangeLog(rotated[0], minChangeLogEntries)
	if err != nil || log.Total != minChangeLogEntries {
		t.Fatalf("read the archive: %d entries (%v), want the folder's %d", log.Total, err, minChangeLogEntries)
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
		"2026021-1.log",
		"202602141-1.log",
		"20260214.log.tmp",
		"20260229-1.log",   // 2026 is not a leap year
		"log/20260214.log", // a path, however it is spelled
		filepath.Join(changeLogDirName, "20260214.log"),
		".." + string(filepath.Separator) + "log" + string(filepath.Separator) + "20260214.log",
	} {
		if _, err := store.ChangeLog(name, 5); err == nil {
			t.Errorf("%q was read as a change log", name)
		}
	}

	// And the names a window really asks for are accepted: today's file by an
	// empty name, a day's file by its own name, an archive by its own.
	if _, err := store.ChangeLog("", 5); err != nil {
		t.Fatalf("the live log: %v", err)
	}
	if _, err := store.ChangeLog("  "+changeLogFile+" ", 5); err != nil {
		t.Fatalf("a name with space around it: %v", err)
	}
	if _, err := store.ChangeLog(dayName(), 5); err != nil {
		t.Fatalf("today's file by name: %v", err)
	}
	if missing, err := store.ChangeLog("20260214-1.log", 5); err != nil {
		// A name that is well formed but not there yet is an empty log rather
		// than a refusal: the alert is about the name, not the file.
		t.Fatalf("a well-formed archive name: %v", err)
	} else if missing.Total != 0 {
		t.Fatalf("a missing archive holds %d entries", missing.Total)
	}
	if missing, err := store.ChangeLog("20260214.log", 5); err != nil {
		t.Fatalf("a well-formed day name: %v", err)
	} else if missing.Total != 0 {
		t.Fatalf("a missing day holds %d entries", missing.Total)
	}
}

// A day's file and an archive of that day are told apart by the number, and
// every name this build could not have written is refused.
func TestChangeLogNameForms(t *testing.T) {
	days := []string{"20260214.log", "19991231.log"}
	for _, name := range days {
		if !isChangeLogFile(name) {
			t.Errorf("%q is a day's log", name)
		}
		if isChangeLogArchive(name) {
			t.Errorf("%q is not an archive", name)
		}
	}
	archived := []string{"20260214-1.log", "19991231-42.log"}
	for _, name := range archived {
		if !isChangeLogFile(name) || !isChangeLogArchive(name) {
			t.Errorf("%q is an archived log", name)
		}
	}
	no := []string{
		"", "changelog.jsonl", "20260214.log.tmp", "20260214.LOG", "20260214-.log",
		"-1.log", "2026021-1.log", "202602141-1.log", "20260214-1.log.bak", "20261301-1.log",
		"20260214-0.log", "20260214-007.log", // numbering starts at one, unpadded
		"20260230-1.log", // February never has a 30th
		"20260229.log",   // 2026 is not a leap year
		"log/20260214.log", "log/20260214-1.log", "../20260214.log",
	}
	for _, name := range no {
		if isChangeLogFile(name) {
			t.Errorf("%q is not a log this build writes", name)
		}
	}
}

// A line this build cannot read costs the entry it held, not the log.
func TestChangeLogSkipsALineItCannotRead(t *testing.T) {
	store := escapedStore(t)

	if err := store.AppendChangeLog(entry("DROP TABLE orders;")); err != nil {
		t.Fatalf("append: %v", err)
	}
	path := filepath.Join(logDir(store), dayName())
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
	if rotated := archives(t, store); len(rotated) != 1 {
		t.Fatalf("expected the 500-entry log to rotate, got %v", rotated)
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

// The log folder is part of the data directory, so a move takes it along and the
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
	// The folder is one line in the listing: a user with a log per day should
	// not have to read a line per day.
	var listed *models.DataFileInfo
	for i, file := range info.Files {
		if file.Name == changeLogDirName {
			listed = &info.Files[i]
		}
	}
	if listed == nil || !listed.Dir || listed.Count != 1 || listed.Bytes == 0 {
		t.Fatalf("the log folder is not listed as a folder holding the log: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved := strings.Join(result.Moved, ", "); !strings.Contains(moved, changeLogDirName) {
		t.Fatalf("the report does not mention the log folder: %v", result.Moved)
	}
	if _, err := os.Stat(filepath.Join(dest, changeLogDirName, dayName())); err != nil {
		t.Fatalf("the log did not arrive: %v", err)
	}
	if _, err := os.Stat(logDir(store)); !os.IsNotExist(err) {
		t.Fatal("the old log folder is still there")
	}

	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open moved store: %v", err)
	}
	if moved, err := again.ChangeLog(dayName(), 10); err != nil || moved.Total != 1 {
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
	rotated := archives(t, store)
	if len(rotated) != 1 {
		t.Fatalf("expected one archive to move, got %v", rotated)
	}
	archive := rotated[0]

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var listed bool
	for _, file := range info.Files {
		if file.Name == changeLogDirName && file.Count == 2 && file.Bytes > 0 {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("the archive is not counted in the data directory: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved := strings.Join(result.Moved, ", "); !strings.Contains(moved, changeLogDirName) {
		t.Fatalf("the report does not mention the log folder: %v", result.Moved)
	}
	if len(result.LeftBehind) != 0 {
		t.Fatalf("a log folder is ours, not something left behind: %v", result.LeftBehind)
	}
	if _, err := os.Stat(filepath.Join(dest, changeLogDirName, archive)); err != nil {
		t.Fatalf("the archive did not arrive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, changeLogDirName, archive)); !os.IsNotExist(err) {
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
	// And the day's own file came with it, so appends carry on where they were.
	if moved, err := again.ChangeLog(dayName(), 10); err != nil || moved.Total != 1 {
		t.Fatalf("the moved live log holds %d entries (%v)", moved.Total, err)
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
