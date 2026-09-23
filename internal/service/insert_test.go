package service

import (
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// The data generation window sends one batch at a time over the bridge, as the
// frontend serialises it: every number arrives as a float64. What the service
// owns is the part right before the driver — who may write at all, and what a
// JSON number is allowed to become once it is bound to a column.

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
