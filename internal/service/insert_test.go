package service

import (
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// The data generation window sends one batch at a time over the bridge, as the
// frontend serialises it: every number arrives as a float64. What the service
// owns is the part right before the driver — who may write at all, and what a
// JSON number is allowed to become once it is bound to a column.

// A batch of generated rows is one statement and one entry: the window runs it
// from a page away from the grid, so the log is the only place the table's own
// history can say that data was made up into it rather than typed.
func TestInsertRowsLogsTheBatchOnceWithItsCount(t *testing.T) {
	manager := loggedManager(t)

	result, err := manager.InsertRows(batch(float64(11), float64(12), float64(13)))
	if err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	if result.Inserted != 3 {
		t.Fatalf("inserted = %d, want 3", result.Inserted)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("one batch is one statement, so one entry: %+v", log)
	}
	entry := log[0]
	if entry.Source != models.ChangeSourceDataGen || entry.Kind != "insert" || entry.Table != "orders" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Rows != 3 {
		t.Fatalf("the entry should say three rows went in, got %d: %+v", entry.Rows, entry)
	}
	// The statement the engine was handed is the one kept — the shape of the
	// batch, with its values bound rather than spelled out, because five hundred
	// generated values are neither readable nor small.
	if !strings.HasPrefix(entry.Statement, "INSERT INTO ") || !strings.Contains(entry.Statement, "user_id") {
		t.Fatalf("unexpected statement: %q", entry.Statement)
	}
	if strings.Contains(entry.Statement, "11") {
		t.Fatalf("a generated value was written into the log: %q", entry.Statement)
	}
}

// A batch the engine refused is logged with the rows that did land: the rest of
// the run carried on, or stopped, but those rows are in the table.
func TestInsertRowsLogsWhatLandedFromAPartialBatch(t *testing.T) {
	manager := loggedManager(t)

	// id is the table's integer primary key, so a text value is refused: the
	// first row lands, the second does not, and the run stops there.
	result, err := manager.InsertRows(models.RowInsert{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Columns:   []string{"id", "user_id"},
		Rows:      [][]any{{float64(41), float64(1)}, {"not a number", float64(2)}},
	})
	if err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	if result.Inserted != 1 || result.Failed != 2 {
		t.Fatalf("expected the second row to be refused: %+v", result)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("expected one entry for the batch: %+v", log)
	}
	if log[0].Rows != 1 {
		t.Fatalf("the entry should count the row that landed, got %d", log[0].Rows)
	}
	// The refusal is not an error the entry carries: the engine answered the
	// batch, and the count beside it is what it answered with.
	if log[0].Error != "" {
		t.Fatalf("a partial batch ran to its own end: %+v", log[0])
	}
}

// The window shows the statement a batch will go in as before it starts making
// values up, so the preview is rendered from the same renderer the run uses —
// and, being a read, it runs nothing and logs nothing.
func TestPlanInsertRowsRendersTheStatementTheBatchWillRun(t *testing.T) {
	manager := loggedManager(t)

	statement, err := manager.PlanInsertRows(batch())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.HasPrefix(statement, "INSERT INTO ") {
		t.Fatalf("unexpected statement: %q", statement)
	}
	if !strings.Contains(statement, "orders") || !strings.Contains(statement, "user_id") {
		t.Fatalf("the preview does not name the table and its columns: %q", statement)
	}
	// The values do not exist yet, and the preview says so in the shape of the
	// statement rather than in a note beside it.
	if !strings.HasSuffix(statement, "VALUES (?)") {
		t.Fatalf("the preview should show a bind placeholder: %q", statement)
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a plan is not a change, but the log holds %+v", log)
	}
}

// The preview is refused exactly where the run would be, so a window that could
// never write anything does not get a statement that looks like it could.
func TestPlanInsertRowsRefusesWhatTheRunRefuses(t *testing.T) {
	manager := loggedManager(t)

	empty := models.RowInsert{SessionID: "s1", Database: "main", Object: "orders"}
	noColumns := batch()
	noColumns.Columns = nil
	noObject := batch()
	noObject.Object = ""

	for _, req := range []models.RowInsert{empty, noColumns, noObject} {
		if _, err := manager.PlanInsertRows(req); err == nil {
			t.Fatalf("the preview accepted %+v", req)
		}
		if _, err := manager.InsertRows(req); err == nil {
			t.Fatalf("the run accepted %+v", req)
		}
	}

	// And a connection that cannot take generated rows at all — there is no
	// preview to show for it, which is what the window reads as "not offered".
	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.readOnly = true
	if _, err := manager.PlanInsertRows(batch()); err == nil {
		t.Fatal("a read-only connection must not get a preview")
	}
	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("nothing ran, so nothing may be logged: %+v", log)
	}
}

// batch is a small helper for the happy path: one column, three rows.
func batch(rows ...any) models.RowInsert {
	values := make([][]any, 0, len(rows))
	for _, row := range rows {
		values = append(values, []any{row})
	}
	return models.RowInsert{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Columns:   []string{"user_id"},
		Rows:      values,
	}
}

func TestInsertRowsFoldsWholeNumbersBackIntoIntegers(t *testing.T) {
	manager, _ := testManager(t)

	// 11 and 12 are whole; 4.5 is not. SQLite would happily store the first two
	// as REAL, which is exactly what a `NUMERIC` column must not become.
	result, err := manager.InsertRows(models.RowInsert{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Columns:   []string{"id", "user_id", "total"},
		Rows: [][]any{
			{float64(3), float64(11), float64(30)},
			{float64(4), float64(12), 4.5},
		},
	})
	if err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	if result.Inserted != 2 || result.Failed != 0 || result.Error != "" {
		t.Fatalf("expected both rows to land, got %+v", result)
	}

	page, err := manager.Fetch(models.FetchRequest{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if page.Total != 4 {
		t.Fatalf("expected 4 rows in total, got %d", page.Total)
	}

	// The fixture's first two rows are (1,7,10) and (2,8,20); the inserted ones
	// follow. Comparing the dynamic type is the point: an int64 here means the
	// value never reached the engine as a double.
	last := page.Rows[2]
	if got, ok := last[0].(int64); !ok || got != 3 {
		t.Fatalf("id should have been bound as an integer, got %#v", last[0])
	}
	if got, ok := last[1].(int64); !ok || got != 11 {
		t.Fatalf("user_id should have been bound as an integer, got %#v", last[1])
	}
	// The fractional column is the other half of the rule: a number that was
	// never whole is left for the column to judge.
	if got, ok := last[2].(int64); !ok || got != 30 {
		t.Fatalf("a whole NUMERIC should have been stored as one, got %#v", last[2])
	}
	if got, ok := page.Rows[3][2].(float64); !ok || got != 4.5 {
		t.Fatalf("a fractional NUMERIC must keep its value, got %#v", page.Rows[3][2])
	}
}

func TestInsertRowsRefusesAReadOnlySession(t *testing.T) {
	manager, _ := testManager(t)
	manager.sessions["s1"].readOnly = true

	if _, err := manager.InsertRows(batch(float64(1))); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("a read-only connection must refuse generated rows, got %v", err)
	}
}

// A batch that cannot even be attempted is an error; a batch the engine refuses
// is a result. This test pins the first half of that rule.
func TestInsertRowsRefusesAnImpossibleBatch(t *testing.T) {
	manager, _ := testManager(t)

	rows := [][]any{{float64(1)}}
	cases := []struct {
		name string
		req  models.RowInsert
	}{
		{"no session", models.RowInsert{SessionID: "gone", Database: "main", Object: "orders",
			Columns: []string{"user_id"}, Rows: rows}},
		{"no table", models.RowInsert{SessionID: "s1", Database: "main", Object: " ",
			Columns: []string{"user_id"}, Rows: rows}},
		{"no column", models.RowInsert{SessionID: "s1", Database: "main", Object: "orders",
			Rows: rows}},
		{"no row", models.RowInsert{SessionID: "s1", Database: "main", Object: "orders",
			Columns: []string{"user_id"}}},
		{"a short row", models.RowInsert{SessionID: "s1", Database: "main", Object: "orders",
			Columns: []string{"user_id", "total"}, Rows: rows}},
	}
	for _, tc := range cases {
		if _, err := manager.InsertRows(tc.req); err == nil {
			t.Errorf("%s: expected the batch to be refused before it was sent", tc.name)
		}
	}

	// One batch may not be unbounded; the window sends small ones and asks for
	// progress in between.
	tooMany := batch()
	tooMany.Rows = make([][]any, maxInsertRows+1)
	for i := range tooMany.Rows {
		tooMany.Rows[i] = []any{float64(i)}
	}
	if _, err := manager.InsertRows(tooMany); err == nil {
		t.Fatal("expected an oversized batch to be refused")
	}
}

// The window reports the row that stopped it, so a refused row has to come back
// as a count and a message rather than as a failed call.
func TestInsertRowsReportsTheRowTheEngineRefused(t *testing.T) {
	manager, _ := testManager(t)

	// user_id is NOT NULL in the fixture, so the third row is the one the
	// engine will turn down.
	result, err := manager.InsertRows(models.RowInsert{
		SessionID: "s1",
		Database:  "main",
		Object:    "orders",
		Columns:   []string{"id", "user_id"},
		Rows: [][]any{
			{float64(3), float64(11)},
			{float64(4), float64(12)},
			{float64(5), nil},
		},
	})
	if err != nil {
		t.Fatalf("a refused row must be reported, not returned as an error: %v", err)
	}
	if result.Failed != 3 {
		t.Fatalf("expected row 3 to be the one refused, got %+v", result)
	}
	if result.Inserted != 2 {
		t.Fatalf("expected the two rows before it to land, got %+v", result)
	}
	if result.Error == "" {
		t.Fatal("the engine's own message has to travel back to the window")
	}
}

// The window's checkbox decides whether a refused row ends the run. It travels in
// the request, and what comes back has to line up with it: counting rows over is
// not the same thing as naming the row that stopped the batch.
func TestInsertRowsCountsOverRefusedRowsWhenAsked(t *testing.T) {
	manager, _ := testManager(t)

	// user_id is NOT NULL in the fixture, so rows 2 and 4 are the ones the engine
	// will turn down.
	result, err := manager.InsertRows(models.RowInsert{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders",
		Columns:    []string{"id", "user_id"},
		SkipErrors: true,
		Rows: [][]any{
			{float64(3), float64(11)},
			{float64(4), nil},
			{float64(5), float64(12)},
			{float64(6), nil},
			{float64(7), float64(13)},
		},
	})
	if err != nil {
		t.Fatalf("a skipped row must be reported, not returned as an error: %v", err)
	}
	if result.Inserted != 3 || result.Skipped != 2 {
		t.Fatalf("expected three rows in and two skipped, got %+v", result)
	}
	if result.Failed != 0 {
		t.Fatalf("nothing stopped the batch, so no row may be named as having: %+v", result)
	}
	if result.Error == "" {
		t.Fatal("the window has to be able to say why rows were left out")
	}

	page, err := manager.Fetch(models.FetchRequest{
		SessionID:  "s1",
		Database:   "main",
		Object:     "orders",
		Limit:      10,
		CountTotal: true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// The fixture starts with two rows; three more were added.
	if page.Total != 5 {
		t.Fatalf("expected 5 rows in total, got %d", page.Total)
	}
}
