package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// escapedStore is a store in a temp directory, with the OS-level config home
// moved out of the way: this file only exercises the query tree.
func escapedStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := NewAt(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestEscapeSegmentKeepsNamesReadableAndReversible(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"orders", "orders"},
		{"Order 2024-01", "Order 2024-01"},
		{"a/b", "a%2Fb"},
		{"a\\b", "a%5Cb"},
		{"100% sure", "100%25 sure"},
		{"q:1", "q%3A1"},
		{"a?b*c", "a%3Fb%2Ac"},
		{`say "hi"`, "say %22hi%22"},
		{"a<b>c|d", "a%3Cb%3Ec%7Cd"},
		{"tab\there", "tab%09here"},
		{"trailing.", "trailing%2E"},
		{"trailing ..", "trailing%20%2E%2E"}, // the trailing space would be stripped too
		{".hidden", "%2Ehidden"},
		{"   spaced   ", "spaced"}, // trimmed at the edges
		{"订单查询", "订单查询"},           // Unicode is fine, and stays readable
		{"con", "%63on"},           // a Windows device name
		{"CON", "%43ON"},
		{"nul.sql", "%6Eul.sql"}, // the part before the first dot is what counts
		{"console", "console"},   // ... but only an exact match
	}
	for _, c := range cases {
		got, err := escapeSegment(c.in)
		if err != nil {
			t.Fatalf("escape %q: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("escape %q = %q, want %q", c.in, got, c.want)
		}
		if back := unescapeSegment(got); back != strings.TrimSpace(c.in) {
			t.Errorf("unescape %q = %q, want %q", got, back, strings.TrimSpace(c.in))
		}
	}
}

func TestEscapeSegmentRefusesWhatCannotBeAFileName(t *testing.T) {
	if _, err := escapeSegment("   "); err == nil {
		t.Fatal("an empty name should be refused")
	}
	if _, err := escapeSegment(strings.Repeat("*", maxSegmentBytes)); err == nil {
		t.Fatal("a name that does not fit in a path segment should be refused")
	}
	// A long name that is *not* escaped still fits, so the limit is about the
	// escapes rather than about a low cap on names.
	if _, err := escapeSegment(strings.Repeat("a", maxSegmentBytes)); err != nil {
		t.Fatalf("a plain name of the limit length should be accepted: %v", err)
	}
}

// A name this build did not write (a file dropped in by hand) decodes to
// something that is not valid UTF-8: it is shown as it is on disk rather than
// being turned into replacement characters.
func TestUnescapeSegmentLeavesForeignNamesAlone(t *testing.T) {
	if got := unescapeSegment("caf%FF"); got != "caf%FF" {
		t.Fatalf("unescape of invalid UTF-8 = %q, want the name unchanged", got)
	}
	if got := unescapeSegment("100%"); got != "100%" {
		t.Fatalf("a trailing percent is not an escape: %q", got)
	}
}

func TestQueryFilesRoundTrip(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "2f1e0b6c-9d1a-4a1e-9f6b-0f4a2b7c1d55", "shop"

	if list, err := store.ListQueryFiles(conn, db); err != nil || len(list) != 0 {
		t.Fatalf("a database with no folder should list nothing: %v %v", list, err)
	}

	saved, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "daily orders", SQL: "SELECT 1;",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	want := filepath.Join(store.Dir(), queryDirName, conn, db, "daily orders.sql")
	if saved.Path != want {
		t.Fatalf("saved to %s, want %s", saved.Path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the file is not there: %v", err)
	}

	read, err := store.ReadQueryFile(conn, db, "daily orders")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.SQL != "SELECT 1;" || read.Name != "daily orders" || read.Database != db {
		t.Fatalf("read back %+v", read)
	}

	list, err := store.ListQueryFiles(conn, db)
	if err != nil || len(list) != 1 || list[0].Name != "daily orders" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	// The listing must not carry the scripts: the tree asks for names only.
	if list[0].SQL != "" {
		t.Fatal("the listing should not read the file contents")
	}
	if list[0].Size != int64(len("SELECT 1;")) || list[0].UpdatedAt == 0 {
		t.Fatalf("listing entry lacks size or time: %+v", list[0])
	}
}

// Two databases of the same connection are two folders, and the same name in
// each is two different scripts.
func TestQueryFilesAreScopedByConnectionAndDatabase(t *testing.T) {
	store := escapedStore(t)
	const conn = "conn-1"

	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: "a", Name: "count", SQL: "SELECT 'a';",
	}); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: "b", Name: "count", SQL: "SELECT 'b';",
	}); err != nil {
		t.Fatalf("save b: %v", err)
	}
	one, err := store.ReadQueryFile(conn, "a", "count")
	if err != nil {
		t.Fatalf("read a: %v", err)
	}
	two, err := store.ReadQueryFile(conn, "b", "count")
	if err != nil {
		t.Fatalf("read b: %v", err)
	}
	if one.SQL == two.SQL {
		t.Fatal("the two databases share one file")
	}
}

// A database name that looks like a path stays inside the data directory: the
// escaping is what keeps a name from choosing where its file goes.
func TestQueryFilesCannotEscapeTheDataDirectory(t *testing.T) {
	store := escapedStore(t)
	const conn = "conn-1"

	for _, db := range []string{"..", "../..", `..\..`, "/etc", "C:evil"} {
		saved, err := store.SaveQueryFile(models.QueryFileSave{
			ConnectionID: conn, Database: db, Name: "x", SQL: "SELECT 1;",
		})
		if err != nil {
			t.Fatalf("save into %q: %v", db, err)
		}
		rel, err := filepath.Rel(store.Dir(), saved.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("database %q wrote outside the data directory: %s", db, saved.Path)
		}
		if !strings.HasPrefix(rel, queryDirName+string(filepath.Separator)) {
			t.Fatalf("database %q did not land under %s: %s", db, queryDirName, rel)
		}
	}
}

func TestSaveQueryFileRenamesAndKeepsOneFile(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "conn-1", "shop"
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "old", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.RenameQueryFile(models.QueryFileRename{
		ConnectionID: conn, Database: db, From: "old", To: "new",
	}); err != nil {
		t.Fatalf("rename: %v", err)
	}

	list, err := store.ListQueryFiles(conn, db)
	if err != nil || len(list) != 1 || list[0].Name != "new" {
		t.Fatalf("after the rename the folder holds %+v, %v", list, err)
	}
	if _, err := store.ReadQueryFile(conn, db, "old"); err == nil {
		t.Fatal("the old file is still there")
	}

	// A rename keeps the contents: it moves the file instead of rewriting it.
	read, err := store.ReadQueryFile(conn, db, "new")
	if err != nil {
		t.Fatalf("read after rename: %v", err)
	}
	if read.SQL != "SELECT 1;" {
		t.Fatalf("the rename rewrote the script: %q", read.SQL)
	}

	// ... but it refuses a name that already holds a script.
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "keep", SQL: "SELECT 3;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.RenameQueryFile(models.QueryFileRename{
		ConnectionID: conn, Database: db, From: "new", To: "keep",
	}); err == nil {
		t.Fatal("renaming onto an existing query should be refused")
	}
	if read, err := store.ReadQueryFile(conn, db, "keep"); err != nil || read.SQL != "SELECT 3;" {
		t.Fatalf("the refused rename damaged the other query: %+v %v", read, err)
	}
}

// A rename that only changes the case of a name is a rename, not a second file.
func TestRenameQueryFileChangesOnlyTheCase(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "conn-1", "shop"
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "orders", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.RenameQueryFile(models.QueryFileRename{
		ConnectionID: conn, Database: db, From: "orders", To: "Orders",
	}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	list, err := store.ListQueryFiles(conn, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Orders" {
		t.Fatalf("folder holds %+v, want a single Orders", list)
	}
}

func TestRenameQueryFileRefusesAMissingSource(t *testing.T) {
	store := escapedStore(t)
	if _, err := store.RenameQueryFile(models.QueryFileRename{
		ConnectionID: "conn-1", Database: "shop", From: "ghost", To: "other",
	}); !os.IsNotExist(err) {
		t.Fatalf("renaming a script that is not there: %v", err)
	}
}

// Windows and macOS treat a name differing only in case as the same file; the
// save then rewrites it under the spelling the user typed instead of leaving two
// files (or one under a name nobody asked for).
func TestSaveQueryFileReplacesACaseOnlyDifference(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "conn-1", "shop"
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "orders", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "Orders", SQL: "SELECT 2;",
	}); err != nil {
		t.Fatalf("save again: %v", err)
	}
	list, err := store.ListQueryFiles(conn, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// On a case-insensitive filesystem there is one file and its name is the new
	// spelling; on a case-sensitive one the old file is removed as a stale
	// spelling, so either way the folder holds exactly `Orders`.
	if len(list) != 1 || list[0].Name != "Orders" {
		t.Fatalf("folder holds %+v, want a single Orders", list)
	}
}

func TestSaveQueryFileLeavesNoHalfWrittenFileBehind(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "conn-1", "shop"
	saved, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "q", SQL: strings.Repeat("SELECT 1;\n", 100),
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(saved.Path))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("a temp file was left behind: %s", entry.Name())
		}
	}
}

func TestDeleteQueryFileIsIdempotent(t *testing.T) {
	store := escapedStore(t)
	const conn, db = "conn-1", "shop"
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: conn, Database: db, Name: "q", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.DeleteQueryFile(conn, db, "q"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := store.DeleteQueryFile(conn, db, "q"); err != nil {
		t.Fatalf("deleting again should be a no-op: %v", err)
	}
	list, err := store.ListQueryFiles(conn, db)
	if err != nil || len(list) != 0 {
		t.Fatalf("after the delete the folder holds %+v, %v", list, err)
	}
}

// The query tree is part of the data directory, so a move takes it along and the
// settings listing shows it as a folder.
func TestMoveDataCarriesTheQueryFiles(t *testing.T) {
	def := withConfigHome(t)
	store, err := NewAt(def)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := store.SaveQueryFile(models.QueryFileSave{
		ConnectionID: "conn-1", Database: "shop", Name: "orders", SQL: "SELECT 1;",
	}); err != nil {
		t.Fatalf("save query: %v", err)
	}

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var found models.DataFileInfo
	for _, file := range info.Files {
		if file.Name == queryDirName {
			found = file
		}
	}
	if !found.Dir || found.Count != 1 {
		t.Fatalf("the query folder is not listed as one: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	moved := strings.Join(result.Moved, ", ")
	if !strings.Contains(moved, queryDirName+" (1 file)") {
		t.Fatalf("the report does not mention the query tree: %v", result.Moved)
	}
	if len(result.LeftBehind) != 0 {
		t.Fatalf("the query folder was reported as left behind: %v", result.LeftBehind)
	}
	if _, err := os.Stat(filepath.Join(dest, queryDirName, "conn-1", "shop", "orders.sql")); err != nil {
		t.Fatalf("the query did not arrive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, queryDirName)); !os.IsNotExist(err) {
		t.Fatal("the old query folder is still there")
	}

	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open moved store: %v", err)
	}
	if _, err := again.ReadQueryFile("conn-1", "shop", "orders"); err != nil {
		t.Fatalf("read after the move: %v", err)
	}

	// A directory that already holds a query tree is refused rather than merged:
	// two script files with the same name cannot both be the truth.
	occupied := t.TempDir()
	if _, err := NewAt(occupied); err != nil {
		t.Fatalf("open occupied store: %v", err)
	}
	third, err := NewAt(occupied)
	if err != nil {
		t.Fatalf("open occupied store: %v", err)
	}
	if _, err := third.SaveQueryFile(models.QueryFileSave{
		ConnectionID: "conn-1", Database: "shop", Name: "orders", SQL: "SELECT 'other';",
	}); err != nil {
		t.Fatalf("plant a query: %v", err)
	}
	if _, err := MoveData(occupied); err == nil {
		t.Fatal("moving into a directory that already holds queries should be refused")
	} else if !strings.Contains(err.Error(), queryDirName) {
		t.Fatalf("the refusal does not name the folder in the way: %v", err)
	}
}
