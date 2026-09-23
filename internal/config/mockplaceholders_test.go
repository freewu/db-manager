package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/models"
)

func TestMockPlaceholdersRoundTrip(t *testing.T) {
	store := escapedStore(t)

	if list, err := store.ListMockPlaceholders(); err != nil || len(list) != 0 {
		t.Fatalf("a fresh store listed %v (%v), want nothing", list, err)
	}

	saved, err := store.SaveMockPlaceholder(models.MockPlaceholder{
		Name:        "orderNo",
		Template:    "SO@date(yyyy)@natural(1000, 9999)",
		Description: "订单号",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Name != "orderNo" || saved.UpdatedAt == 0 {
		t.Fatalf("save returned %+v, want the stored name and a modification time", saved)
	}

	// The file is where the settings page says it is: one placeholder per file
	// under the `.mock` folder of the data directory.
	path := filepath.Join(store.Dir(), mockDirName, "orderNo"+mockFileExt)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("placeholder file: %v", err)
	}

	list, err := store.ListMockPlaceholders()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v (%v), want one entry", list, err)
	}
	if got := list[0]; got.Template != "SO@date(yyyy)@natural(1000, 9999)" || got.Description != "订单号" {
		t.Fatalf("listed %+v, want the template and description that were saved", got)
	}

	one, err := store.ReadMockPlaceholder("orderNo")
	if err != nil || one.Template != list[0].Template {
		t.Fatalf("read = %+v (%v), want the saved placeholder", one, err)
	}

	// Saving again is an update, not a second file.
	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{
		Name:     "orderNo",
		Template: "SO@natural(1, 9)",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err = store.ListMockPlaceholders()
	if err != nil || len(list) != 1 {
		t.Fatalf("after update: %v (%v), want still one entry", list, err)
	}
	if list[0].Template != "SO@natural(1, 9)" || list[0].Description != "" {
		t.Fatalf("after update: %+v, want the new template and no description", list[0])
	}

	if err := store.DeleteMockPlaceholder("orderNo"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, err := store.ListMockPlaceholders(); err != nil || len(list) != 0 {
		t.Fatalf("after delete: %v (%v), want an empty folder", list, err)
	}
	// Deleting what is not there is how a second window's stale list behaves.
	if err := store.DeleteMockPlaceholder("orderNo"); err != nil {
		t.Fatalf("delete again: %v", err)
	}
}

func TestMockPlaceholderNamesCannotLeaveTheirFolder(t *testing.T) {
	store := escapedStore(t)

	cases := []struct {
		name string
		file string
	}{
		{"../../escape", "%2E.%2F..%2Fescape" + mockFileExt},
		{"a/b", "a%2Fb" + mockFileExt},
		{"con", "%63on" + mockFileExt},
		{"订单号", "订单号" + mockFileExt},
	}
	for _, c := range cases {
		if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: c.name, Template: "@word"}); err != nil {
			t.Fatalf("save %q: %v", c.name, err)
		}
		path := filepath.Join(store.Dir(), mockDirName, c.file)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%q did not land at %s: %v", c.name, path, err)
		}
		// The name survives the trip through the file system.
		one, err := store.ReadMockPlaceholder(c.name)
		if err != nil {
			t.Fatalf("read %q: %v", c.name, err)
		}
		if one.Name != c.name {
			t.Fatalf("read %q back as %q", c.name, one.Name)
		}
	}

	// Nothing was written outside the folder it belongs to.
	entries, err := os.ReadDir(filepath.Join(store.Dir(), mockDirName))
	if err != nil {
		t.Fatalf("read folder: %v", err)
	}
	if len(entries) != len(cases) {
		t.Fatalf("folder holds %d entries, want %d", len(entries), len(cases))
	}
}

func TestMockPlaceholderCaseVariantsAreOneFile(t *testing.T) {
	store := escapedStore(t)

	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: "orderNo", Template: "@word"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: "OrderNo", Template: "@cword(2, 3)"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, err := store.ListMockPlaceholders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %+v, want one file — a name that differs only in case is the same file", list)
	}
	if list[0].Template != "@cword(2, 3)" {
		t.Fatalf("list = %+v, want the second spelling to have replaced the first", list)
	}
}

func TestMockPlaceholderThatCannotBeReadIsStillListed(t *testing.T) {
	store := escapedStore(t)

	dir := filepath.Join(store.Dir(), mockDirName)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		t.Fatalf("prepare folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "halfway"+mockFileExt), []byte("{not json"), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	// A temp file left by an interrupted write is not a placeholder.
	if err := os.WriteFile(filepath.Join(dir, "draft"+mockFileExt+".tmp"), []byte("{}"), fileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	list, err := store.ListMockPlaceholders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "halfway" {
		t.Fatalf("list = %+v, want the unreadable file listed by its name", list)
	}
	if !strings.Contains(list[0].Broken, "could not be read") {
		t.Fatalf("Broken = %q, want it to say the file could not be read", list[0].Broken)
	}
	// It is listed precisely so it can be removed from the settings page.
	if err := store.DeleteMockPlaceholder("halfway"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMockPlaceholdersRefuseToGrowWithoutBound(t *testing.T) {
	store := escapedStore(t)

	for i := 0; i < maxMockPlaceholders; i++ {
		name := fmt.Sprintf("p%03d", i)
		if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: name, Template: "@word"}); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: "oneTooMany", Template: "@word"}); err == nil {
		t.Fatal("saving past the limit was accepted")
	}
	// Updating one of the existing placeholders still works: the limit is on how
	// many there are, not on how many times one may be written.
	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{Name: "p000", Template: "@cword(2, 2)"}); err != nil {
		t.Fatalf("update at the limit: %v", err)
	}
}

// The placeholder folder is part of the data directory, so a move takes it along
// and the settings listing shows it as a folder — the same contract the query
// tree is held to.
func TestMoveDataCarriesTheMockPlaceholders(t *testing.T) {
	def := withConfigHome(t)
	store, err := NewAt(def)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := store.SaveMockPlaceholder(models.MockPlaceholder{
		Name: "orderNo", Template: "SO@date(yyyy)@natural(1000, 9999)",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var found models.DataFileInfo
	for _, file := range info.Files {
		if file.Name == mockDirName {
			found = file
		}
	}
	if !found.Dir || found.Count != 1 {
		t.Fatalf("the placeholder folder is not listed as one: %+v", info.Files)
	}

	dest := filepath.Join(t.TempDir(), "moved")
	result, err := MoveData(dest)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved := strings.Join(result.Moved, ", "); !strings.Contains(moved, mockDirName+" (1 file)") {
		t.Fatalf("the report does not mention the placeholders: %v", result.Moved)
	}
	if _, err := os.Stat(filepath.Join(dest, mockDirName, "orderNo"+mockFileExt)); err != nil {
		t.Fatalf("the placeholder did not arrive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, mockDirName)); !os.IsNotExist(err) {
		t.Fatal("the old placeholder folder is still there")
	}

	again, err := NewAt(dest)
	if err != nil {
		t.Fatalf("open moved store: %v", err)
	}
	if _, err := again.ReadMockPlaceholder("orderNo"); err != nil {
		t.Fatalf("read after the move: %v", err)
	}
}
