package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dbmanager/internal/secret"
)

// The data directory has a default that comes from the OS, so the tests move
// that default into a temp directory instead of writing to the real user
// profile. Windows reads %AppData%, everything else reads $XDG_CONFIG_HOME.
func withConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", home)
	} else {
		t.Setenv("XDG_CONFIG_HOME", home)
	}
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("default dir: %v", err)
	}
	if !strings.HasPrefix(dir, home) {
		t.Fatalf("default dir %s is not under the temp home %s", dir, home)
	}
	return dir
}

// writeData plants a data directory: every file this build owns, one stray file
// the user put there, and a temp file from an interrupted write.
func writeData(t *testing.T, dir string) map[string]string {
	t.Helper()
	want := map[string]string{
		fileName:           `{"version":1,"connections":[]}`,
		queriesFile:        `{"version":1,"queries":[]}`,
		layoutFile:         `{"version":1,"groups":[],"items":[]}`,
		stateName:          `{"theme":"dark"}`,
		secret.KeyFileName: "0123456789abcdef0123456789abcdef",
	}
	for name, body := range want {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), fileMode); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), fileMode); err != nil {
		t.Fatalf("write stray file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json.tmp"), []byte("half"), fileMode); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return want
}

func TestDataDirDefaultsToTheConfigDirectory(t *testing.T) {
	def := withConfigHome(t)

	got, err := DataDir()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if got != def {
		t.Fatalf("data dir = %s, want the default %s", got, def)
	}

	info, err := DescribeDataDir(got)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if !info.IsDefault || info.Path != def || info.DefaultPath != def {
		t.Fatalf("unexpected description: %+v", info)
	}
	// A fresh install has written nothing, and saying so is the honest answer.
	if len(info.Files) != 0 || info.TotalBytes != 0 {
		t.Fatalf("expected an empty listing, got %+v", info)
	}
}

func TestDescribeDataDirListsEveryFile(t *testing.T) {
	def := withConfigHome(t)
	want := writeData(t, def)

	info, err := DescribeDataDir(def)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	byName := map[string]int64{}
	for _, file := range info.Files {
		byName[file.Name] = file.Bytes
	}
	if len(byName) != len(want) {
		t.Fatalf("listed %d files, want %d: %+v", len(byName), len(want), info.Files)
	}
	var total int64
	for name, body := range want {
		if byName[name] != int64(len(body)) {
			t.Fatalf("%s: %d bytes, want %d", name, byName[name], len(body))
		}
		total += int64(len(body))
	}
	if info.TotalBytes != total {
		t.Fatalf("total %d, want %d", info.TotalBytes, total)
	}
}

func TestMoveDataCarriesTheDataAndReportsTheRest(t *testing.T) {
	def := withConfigHome(t)
	want := writeData(t, def)
	target := filepath.Join(t.TempDir(), "moved")

	result, err := MoveData(target)
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	if len(result.Moved) != len(want) {
		t.Fatalf("moved %v, want %d files", result.Moved, len(want))
	}
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("read moved %s: %v", name, err)
		}
		if string(got) != body {
			t.Fatalf("%s moved as %q, want %q", name, got, body)
		}
		// The whole point is that the new copy is the one to keep: a rewritten
		// file must not become world readable, and the key even less so.
		stat, err := os.Stat(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("stat moved %s: %v", name, err)
		}
		if runtime.GOOS != "windows" && stat.Mode().Perm() != fileMode {
			t.Fatalf("%s has mode %v, want %v", name, stat.Mode().Perm(), fileMode)
		}
		if _, err := os.Stat(filepath.Join(def, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be gone from the old directory (err=%v)", name, err)
		}
	}

	// The stray file is the user's business, not ours: left alone, and named in
	// the report so the settings page can say what happened to it. The temp file
	// is our own litter and the pointer file belongs to the default directory;
	// neither is worth reporting.
	if len(result.LeftBehind) != 1 || result.LeftBehind[0] != "notes.txt" {
		t.Fatalf("left behind %v, want [notes.txt]", result.LeftBehind)
	}
	if len(result.Remaining) != 0 {
		t.Fatalf("nothing should have been stuck: %v", result.Remaining)
	}
	if _, err := os.Stat(filepath.Join(def, "notes.txt")); err != nil {
		t.Fatalf("the stray file should still be there: %v", err)
	}

	if result.Info.Path != target || result.Info.IsDefault {
		t.Fatalf("unexpected info after the move: %+v", result.Info)
	}
	if result.Info.TotalBytes == 0 || len(result.Info.Files) != len(want) {
		t.Fatalf("the new directory should list the moved files: %+v", result.Info)
	}

	// And the choice has to survive a restart: this is what the app reads on the
	// next start.
	again, err := DataDir()
	if err != nil {
		t.Fatalf("data dir after move: %v", err)
	}
	if again != target {
		t.Fatalf("data dir after move = %s, want %s", again, target)
	}
	pointer, err := os.ReadFile(filepath.Join(def, locationFile))
	if err != nil {
		t.Fatalf("read pointer: %v", err)
	}
	var parsed locationFormat
	if err := json.Unmarshal(pointer, &parsed); err != nil {
		t.Fatalf("parse pointer %q: %v", pointer, err)
	}
	if parsed.DataDir != target || parsed.Version != schemaVer {
		t.Fatalf("pointer = %+v, want %s", parsed, target)
	}
}

func TestMoveDataBackToTheDefaultDirectory(t *testing.T) {
	def := withConfigHome(t)
	want := writeData(t, def)
	target := filepath.Join(t.TempDir(), "moved")

	if _, err := MoveData(target); err != nil {
		t.Fatalf("move out: %v", err)
	}
	result, err := MoveData("")
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if !result.Info.IsDefault || result.Info.Path != def {
		t.Fatalf("expected the default directory, got %+v", result.Info)
	}
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(def, name))
		if err != nil {
			t.Fatalf("read restored %s: %v", name, err)
		}
		if string(got) != body {
			t.Fatalf("%s came back as %q, want %q", name, got, body)
		}
	}
	// Back on the default means no pointer at all — the absence of the file is
	// the setting, so a later build that changes the default location follows it.
	if _, err := os.Stat(filepath.Join(def, locationFile)); !os.IsNotExist(err) {
		t.Fatalf("the pointer should be gone, err=%v", err)
	}
}

func TestMoveDataRefusesItsOwnDirectory(t *testing.T) {
	def := withConfigHome(t)
	writeData(t, def)

	// The same path spelled differently is still the same directory on Windows,
	// where the file chooser happily returns whatever case the user typed.
	same := def
	if runtime.GOOS == "windows" {
		same = strings.ToUpper(def)
	}
	if _, err := MoveData(same); err == nil || !strings.Contains(err.Error(), "already the data directory") {
		t.Fatalf("expected a refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, fileName)); err != nil {
		t.Fatalf("a refused move must not touch the data: %v", err)
	}
}

func TestMoveDataRefusesRelativeTarget(t *testing.T) {
	withConfigHome(t)
	if _, err := MoveData("somewhere/else"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

func TestMoveDataRefusesADirectoryThatHoldsData(t *testing.T) {
	def := withConfigHome(t)
	writeData(t, def)

	target := t.TempDir()
	// Another install's profiles are not ours to overwrite.
	if err := os.WriteFile(filepath.Join(target, fileName), []byte(`{"version":1}`), fileMode); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	_, err := MoveData(target)
	if err == nil || !strings.Contains(err.Error(), fileName) {
		t.Fatalf("expected a refusal naming %s, got %v", fileName, err)
	}
	if _, err := os.Stat(filepath.Join(def, fileName)); err != nil {
		t.Fatalf("a refused move must not touch the data: %v", err)
	}

	// An empty file of the same name is not data: the app writes a file only
	// once it has something to say, so a leftover empty one must not block a
	// move (and must be replaced by the real thing).
	if err := os.Truncate(filepath.Join(target, fileName), 0); err != nil {
		t.Fatalf("empty the target file: %v", err)
	}
	if _, err := MoveData(target); err != nil {
		t.Fatalf("an empty file should not block a move: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, fileName))
	if err != nil || len(got) == 0 {
		t.Fatalf("the moved file is empty: %q, %v", got, err)
	}
}

func TestMoveDataIntoASubdirectoryOfTheDataDirectory(t *testing.T) {
	def := withConfigHome(t)
	writeData(t, def)

	// Keeping the data in a subfolder is a normal thing to want, and only the
	// listed files travel, so nothing can recurse. The new directory must not be
	// reported as "left behind" either — that is where the data just went.
	nested := filepath.Join(def, "data")
	result, err := MoveData(nested)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(result.LeftBehind) != 1 || result.LeftBehind[0] != "notes.txt" {
		t.Fatalf("left behind %v, want [notes.txt]", result.LeftBehind)
	}
	if len(result.Moved) == 0 {
		t.Fatal("nothing was moved")
	}
	if _, err := os.Stat(filepath.Join(nested, fileName)); err != nil {
		t.Fatalf("the data should be in the nested directory: %v", err)
	}
}

func TestReadLocationIgnoresWhatItCannotUse(t *testing.T) {
	def := withConfigHome(t)

	cases := map[string]string{
		"empty":     "",
		"garbage":   "not json at all",
		"no field":  `{"version":1}`,
		"relative":  `{"version":1,"dataDir":"relative/path"}`,
		"blank":     `{"version":1,"dataDir":"   "}`,
		"json null": `{"version":1,"dataDir":null}`,
	}
	for name, body := range cases {
		if err := os.WriteFile(filepath.Join(def, locationFile), []byte(body), fileMode); err != nil {
			t.Fatalf("%s: write: %v", name, err)
		}
		got, err := DataDir()
		if err != nil {
			t.Fatalf("%s: data dir: %v", name, err)
		}
		if got != def {
			t.Fatalf("%s: data dir = %s, want the default %s", name, got, def)
		}
	}
}

func TestDataDirCreatesADirectoryThatWasDeleted(t *testing.T) {
	def := withConfigHome(t)
	target := filepath.Join(t.TempDir(), "elsewhere")
	if _, err := MoveData(target); err != nil {
		t.Fatalf("move: %v", err)
	}
	// The user removed the folder from their file manager: the app has to come
	// up with an empty store rather than refuse to start.
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, err := DataDir()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if got != target {
		t.Fatalf("data dir = %s, want %s", got, target)
	}
	if stat, err := os.Stat(got); err != nil || !stat.IsDir() {
		t.Fatalf("the directory should have been recreated: %v", err)
	}
	if def == target {
		t.Fatal("the test did not actually move anything")
	}
}
