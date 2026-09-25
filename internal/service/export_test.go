package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// The database export writes a file rather than a result set, so what these
// tests look at is the file: the header, the definitions, the rows — plus the
// summary the window is handed and the progress it is told about on the way.

// exportPath is a destination inside the test's own directory.
func exportPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dump.sql")
}

func readExported(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	return string(data)
}

func orders() models.ExportTable { return models.ExportTable{Name: "orders"} }

// exportRequest is the shape the window builds: one run, one destination.
func exportRequest(
	path string,
	mode models.ExportMode,
	format models.ExportFormat,
	tables ...models.ExportTable,
) models.ExportRequest {
	return models.ExportRequest{
		ID:        "e1",
		SessionID: "s1",
		Database:  "main",
		Mode:      mode,
		Format:    format,
		Tables:    tables,
		Path:      path,
	}
}

// A structure export is the engine's own definition, with no rows in sight: it
// is the file that recreates empty tables, which is what makes it different from
// the other two modes.
func TestExportStructureWritesTheEngineOwnCreate(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	result, err := manager.ExportDatabase(
		exportRequest(path, models.ExportStructure, models.ExportSQL, orders()), nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Tables != 1 || result.Rows != 0 {
		t.Fatalf("unexpected summary: %+v", result)
	}
	if result.Cancelled || result.Path != path {
		t.Fatalf("unexpected summary: %+v", result)
	}

	// SQLite answers with the text its own catalog holds, so what the file must
	// hold is that text — not this application's reconstruction of it. (SQLite
	// keeps an index in its own catalog row, so its own CREATE does not mention
	// one; that is the engine's answer, and the file is the engine's answer.)
	structure, err := manager.Structure("s1", "main", "", "orders")
	if err != nil {
		t.Fatalf("structure: %v", err)
	}
	text := readExported(t, path)
	if ddl := strings.TrimSpace(structure.DDL) + ";"; !strings.Contains(text, ddl) {
		t.Fatalf("the definition is not the engine's own (%q):\n%s", ddl, text)
	}
	if strings.Contains(text, "INSERT") {
		t.Fatalf("a structure export wrote rows:\n%s", text)
	}
	// A dump nobody can date is a dump nobody can trust.
	for _, want := range []string{"-- db-manager export", "-- Database: main", "-- Driver: sqlite", "-- Table: orders"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the file does not say %q:\n%s", want, text)
		}
	}
	if result.Bytes != int64(len(text)) {
		t.Fatalf("bytes = %d, file is %d", result.Bytes, len(text))
	}
}

// The middle mode puts the definition and its rows in the same file, in that
// order: a script that inserts before it creates has nothing to insert into.
func TestExportStructureAndDataWritesBoth(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	result, err := manager.ExportDatabase(
		exportRequest(path, models.ExportStructureData, models.ExportSQL, orders()), nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Tables != 1 || result.Rows != 2 {
		t.Fatalf("unexpected summary: %+v", result)
	}

	text := readExported(t, path)
	if create, insert := strings.Index(text, "CREATE TABLE orders"), strings.Index(text, "INSERT INTO"); create < 0 || insert < 0 || create > insert {
		t.Fatalf("the definition does not come before the rows it creates:\n%s", text)
	}
	// The column list is spelled out on every row, so the file does not depend on
	// the table's column order to be reloaded.
	for _, want := range []string{
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (1, 7, 10);`,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (2, 8, 20);`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s:\n%s", want, text)
		}
	}
}

// A data export writes rows for tables that already exist, in the user's own
// field order: the column list of a statement and the values inside it are the
// same order, and that order is the one the window showed.
func TestExportDataWritesOnlyTheChosenFieldsInOrder(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	req := exportRequest(path, models.ExportData, models.ExportSQL, orders())
	req.Columns = []string{"total", "id"}
	result, err := manager.ExportDatabase(req, nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Rows != 2 {
		t.Fatalf("unexpected summary: %+v", result)
	}

	text := readExported(t, path)
	if !strings.Contains(text, `INSERT INTO "orders" ("total", "id") VALUES (10, 1);`) {
		t.Fatalf("the picked fields were not written in the picked order:\n%s", text)
	}
	if strings.Contains(text, "CREATE") || strings.Contains(text, "user_id") {
		t.Fatalf("a data export wrote more than the rows of the picked fields:\n%s", text)
	}
}

// A field name the table does not have is a request the window should not have
// been able to build, and it is refused before the file exists — a dump that
// silently lost a column because its name was misspelled is worse than no dump.
func TestExportDataRefusesAnUnknownField(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	req := exportRequest(path, models.ExportData, models.ExportSQL, orders())
	req.Columns = []string{"id", "nope"}
	if _, err := manager.ExportDatabase(req, nil); err == nil || !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("an unknown field was accepted: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a refused request still wrote a file")
	}
}

// Text has to be spelled the way the engine reads it: this is the whole reason
// the export renders its literals through the dialect instead of quoting them
// itself.
func TestExportDataEscapesTextForTheEngine(t *testing.T) {
	manager, _ := testManager(t)
	if _, err := manager.Execute(models.ExecRequest{
		SessionID: "s1",
		Database:  "main",
		SQL: `CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT);
		      INSERT INTO notes (id, body) VALUES (1, 'it''s a \ path');`,
	}); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	path := exportPath(t)
	if _, err := manager.ExportDatabase(
		exportRequest(path, models.ExportData, models.ExportSQL, models.ExportTable{Name: "notes"}), nil); err != nil {
		t.Fatalf("export: %v", err)
	}

	text := readExported(t, path)
	if !strings.Contains(text, `VALUES (1, 'it''s a \ path');`) {
		t.Fatalf("the text was not spelled for the engine:\n%s", text)
	}
}

// The row-oriented formats are one table's worth of data and nothing else: no
// comments (they would corrupt the file) and no second table (there is nowhere
// to say which table a row came from).
func TestExportDataWritesTheRowFormats(t *testing.T) {
	manager, _ := testManager(t)
	req := func(format models.ExportFormat) models.ExportRequest {
		path := filepath.Join(t.TempDir(), "dump."+string(format))
		return exportRequest(path, models.ExportData, format, orders())
	}

	t.Run("csv", func(t *testing.T) {
		request := req(models.ExportCSV)
		if _, err := manager.ExportDatabase(request, nil); err != nil {
			t.Fatalf("export: %v", err)
		}
		text := readExported(t, request.Path)
		want := "id,user_id,total\r\n1,7,10\r\n2,8,20\r\n"
		if text != want {
			t.Fatalf("csv = %q, want %q", text, want)
		}
	})

	t.Run("json", func(t *testing.T) {
		request := req(models.ExportJSON)
		if _, err := manager.ExportDatabase(request, nil); err != nil {
			t.Fatalf("export: %v", err)
		}
		text := readExported(t, request.Path)

		var rows []map[string]any
		if err := json.Unmarshal([]byte(text), &rows); err != nil {
			t.Fatalf("the file is not JSON (%v):\n%s", err, text)
		}
		if len(rows) != 2 || rows[1]["user_id"] != float64(8) {
			t.Fatalf("unexpected rows: %+v", rows)
		}
		// Numbers have to stay numbers: a dump whose counts come back quoted is
		// a file every reader has to clean up first.
		if _, ok := rows[0]["total"].(float64); !ok {
			t.Fatalf("the number came back as %T: %s", rows[0]["total"], text)
		}
	})

	t.Run("jsonl", func(t *testing.T) {
		request := req(models.ExportJSONL)
		if _, err := manager.ExportDatabase(request, nil); err != nil {
			t.Fatalf("export: %v", err)
		}
		text := readExported(t, request.Path)

		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("want one line per row, got %d:\n%s", len(lines), text)
		}
		for _, line := range lines {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("line is not JSON (%v): %s", err, line)
			}
			if row["total"] == nil {
				t.Fatalf("row lost a field: %s", line)
			}
		}
	})
}

// Progress is what a window has to show while a dump of a real database runs, so
// it has to be countable: tables finished, rows so far, and a final tick that
// says the run is over.
func TestExportReportsProgressTableByTable(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	var ticks []models.ExportProgress
	_, err := manager.ExportDatabase(
		exportRequest(path, models.ExportStructureData, models.ExportSQL, orders(), orders()),
		func(p models.ExportProgress) { ticks = append(ticks, p) },
	)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(ticks) < 3 {
		t.Fatalf("expected a tick per table plus one at the start, got %+v", ticks)
	}
	first, last := ticks[0], ticks[len(ticks)-1]
	if first.Done != 0 || first.Total != 2 {
		t.Fatalf("the run started at %+v", first)
	}
	if last.Done != 2 || last.Total != 2 || last.Rows != 4 {
		t.Fatalf("the last tick does not account for the whole run: %+v", last)
	}
	if last.Table != "orders" {
		t.Fatalf("the tick does not say which table it is about: %+v", last)
	}
	for i, tick := range ticks {
		if tick.ID != "e1" {
			t.Fatalf("tick %d lost the run id: %+v", i, tick)
		}
		if i > 0 && tick.Rows < ticks[i-1].Rows {
			t.Fatalf("the row count went backwards: %+v", ticks[:i+1])
		}
		if tick.Bytes <= 0 && tick.Done > 0 {
			t.Fatalf("a finished table wrote no bytes: %+v", tick)
		}
	}
}

// Stopping is a request the user made, not a failure: it is reported as such,
// and the file is left where they asked for it, holding what had been written.
func TestCancelExportReportsWhatItManagedToWrite(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	result, err := manager.ExportDatabase(
		exportRequest(path, models.ExportStructureData, models.ExportSQL, orders(), orders()),
		func(p models.ExportProgress) {
			// Stop once the first table has landed: the second is then never
			// started, and the file holds the first.
			if p.Done == 1 {
				if err := manager.CancelExport(p.ID); err != nil {
					t.Errorf("cancel: %v", err)
				}
			}
		},
	)
	if err != nil {
		t.Fatalf("a stopped export is not an error: %v", err)
	}
	if !result.Cancelled {
		t.Fatalf("the summary does not say the run was stopped: %+v", result)
	}
	if result.Tables != 1 || result.Rows != 2 {
		t.Fatalf("the summary does not account for what was written: %+v", result)
	}
	if text := readExported(t, path); strings.Count(text, "CREATE TABLE orders") != 1 {
		t.Fatalf("the file should hold the table that was written:\n%s", text)
	}
	// A stop that arrives for a run that is not going is what the window wants to
	// hear when it presses Stop a moment too late.
	if err := manager.CancelExport("e1"); err != nil {
		t.Fatalf("cancelling a finished run: %v", err)
	}
	if err := manager.CancelExport(""); err == nil {
		t.Fatalf("a cancel with no id was accepted")
	}
}

// A JSON file that was stopped mid-run is still JSON: a truncated array is a file
// no reader can open, which would make stopping worse than waiting.
func TestCancelledJSONIsStillClosed(t *testing.T) {
	manager, _ := testManager(t)
	path := filepath.Join(t.TempDir(), "dump.json")

	result, err := manager.ExportDatabase(
		exportRequest(path, models.ExportData, models.ExportJSON, orders()),
		func(p models.ExportProgress) {
			// The first tick comes before any row has been read.
			if p.Done == 0 {
				_ = manager.CancelExport(p.ID)
			}
		},
	)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !result.Cancelled {
		t.Fatalf("the summary does not say the run was stopped: %+v", result)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(readExported(t, path)), &rows); err != nil {
		t.Fatalf("the stopped file is not JSON: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("nothing should have been written: %+v", rows)
	}
}

// One table that cannot be read does not throw away the ones that can — but the
// summary has to say so, because a dump that quietly holds eleven of twelve
// tables is a dump somebody will restore and wonder about.
func TestExportWarnsAboutATableItCannotRead(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	result, err := manager.ExportDatabase(
		exportRequest(path, models.ExportStructureData, models.ExportSQL, orders(), models.ExportTable{Name: "ghost"}), nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Tables != 1 || result.Rows != 2 {
		t.Fatalf("the readable table was not written: %+v", result)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "ghost") {
		t.Fatalf("the missing table was not reported: %+v", result.Warnings)
	}
	if !strings.Contains(readExported(t, path), "CREATE TABLE orders") {
		t.Fatalf("the readable table is not in the file")
	}
}

// Everything the form cannot ask for is refused before a file is created: a
// half-written dump at a path the user chose is worse than a sentence saying why
// nothing happened.
func TestExportRefusesWhatTheFormCannotAsk(t *testing.T) {
	manager, _ := testManager(t)

	cases := []struct {
		name   string
		mutate func(*models.ExportRequest)
	}{
		{"no id", func(r *models.ExportRequest) { r.ID = "" }},
		{"no session", func(r *models.ExportRequest) { r.SessionID = "" }},
		{"no path", func(r *models.ExportRequest) { r.Path = "" }},
		{"no tables", func(r *models.ExportRequest) { r.Tables = nil }},
		{"empty table name", func(r *models.ExportRequest) { r.Tables = []models.ExportTable{{Name: " "}} }},
		{"unknown mode", func(r *models.ExportRequest) { r.Mode = "everything" }},
		{"unknown format", func(r *models.ExportRequest) { r.Format = "xml" }},
		{"csv with two tables", func(r *models.ExportRequest) {
			r.Format = models.ExportCSV
			r.Tables = []models.ExportTable{{Name: "orders"}, {Name: "orders"}}
		}},
		{"structure as csv", func(r *models.ExportRequest) {
			r.Mode = models.ExportStructure
			r.Format = models.ExportCSV
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dump.sql")
			req := exportRequest(path, models.ExportData, models.ExportSQL, orders())
			c.mutate(&req)

			if _, err := manager.ExportDatabase(req, nil); err == nil || !apperr.Is(err, apperr.CodeInvalidConfig) {
				t.Fatalf("accepted %+v: %v", req, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("a refused request still wrote a file")
			}
		})
	}
}

// The session has to be one that is open: an export naming a closed session is a
// window that is out of date, not a file.
func TestExportRefusesAnUnknownSession(t *testing.T) {
	manager, _ := testManager(t)
	path := exportPath(t)

	req := exportRequest(path, models.ExportStructure, models.ExportSQL, orders())
	req.SessionID = "closed"
	if _, err := manager.ExportDatabase(req, nil); err == nil || !apperr.Is(err, apperr.CodeNotFound) {
		t.Fatalf("an unknown session was accepted: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a refused request still wrote a file")
	}
}
