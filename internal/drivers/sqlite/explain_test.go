package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// fixture opens a real SQLite file through the driver and seeds one table.
func fixture(t *testing.T) *sqlbase.Conn {
	t.Helper()
	cfg := models.ConnectionConfig{
		Driver:   models.DriverSQLite,
		FilePath: filepath.Join(t.TempDir(), "explain.sqlite"),
		Database: "main",
		// The file does not exist yet, which the driver only allows when the
		// caller says so (see Normalize).
		Params: map[string]string{"mode": "rwc"},
	}
	conn, err := Driver{}.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	base, ok := conn.(*sqlbase.Conn)
	if !ok {
		t.Fatalf("expected the shared connection, got %T", conn)
	}
	if _, err := base.Execute(context.Background(), drivers.ExecRequest{
		Database: "main",
		SQL:      `CREATE TABLE orders (id INTEGER PRIMARY KEY, total NUMERIC)`,
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return base
}

// The plan is the engine's own, read through the wrapper the spec declares.
func TestExplainReadsThePlan(t *testing.T) {
	conn := fixture(t)

	plan, err := conn.Explain(context.Background(), drivers.ExplainRequest{
		Database: "main",
		SQL:      "SELECT * FROM orders WHERE id = 1",
	})
	if err != nil {
		t.Fatalf("explain: %v", err)
	}

	if plan.Statement != "EXPLAIN QUERY PLAN SELECT * FROM orders WHERE id = 1" {
		t.Fatalf("unexpected statement: %q", plan.Statement)
	}
	if plan.SQL != "SELECT * FROM orders WHERE id = 1" {
		t.Fatalf("the plan must echo the statement it is about: %q", plan.SQL)
	}
	// SQLite answers one row per step, with a "detail" column carrying the text.
	var detail = -1
	for i, column := range plan.Columns {
		if strings.EqualFold(column.Name, "detail") {
			detail = i
		}
	}
	if detail < 0 {
		t.Fatalf("no detail column in %+v", plan.Columns)
	}
	if len(plan.Rows) == 0 {
		t.Fatal("the plan came back empty")
	}
	if text, _ := plan.Rows[0][detail].(string); !strings.Contains(text, "orders") {
		t.Fatalf("the first step does not mention the table: %v", plan.Rows[0][detail])
	}
	if len(plan.Notes) == 0 {
		t.Fatal("a plan has to say that nothing was run")
	}
}

// The promise in the name of the feature: explaining is *reading* a plan, so
// the statement it is about must not have happened afterwards. Each of the
// three writing statements is explained, and nothing moves.
func TestExplainDoesNotRunTheStatement(t *testing.T) {
	conn := fixture(t)
	ctx := context.Background()

	run := func(sql string) {
		t.Helper()
		if _, err := conn.Execute(ctx, drivers.ExecRequest{Database: "main", SQL: sql}); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	count := func() int {
		t.Helper()
		res := run1(t, conn, "SELECT count(*) FROM orders")
		n, _ := res.(int64)
		return int(n)
	}
	total := func() any {
		t.Helper()
		return run1(t, conn, "SELECT total FROM orders WHERE id = 1")
	}

	run(`INSERT INTO orders (id, total) VALUES (1, 10)`)
	if got := count(); got != 1 {
		t.Fatalf("fixture is wrong: %d row(s)", got)
	}
	before := total()

	for _, statement := range []string{
		`INSERT INTO orders (id, total) VALUES (2, 20)`,
		`UPDATE orders SET total = 999`,
		`DELETE FROM orders`,
	} {
		if _, err := conn.Explain(ctx, drivers.ExplainRequest{Database: "main", SQL: statement}); err != nil {
			t.Fatalf("explain %q: %v", statement, err)
		}
		if got := count(); got != 1 {
			t.Fatalf("explaining %q changed the row count to %d", statement, got)
		}
		if got := total(); got != before {
			t.Fatalf("explaining %q changed total from %v to %v", statement, before, got)
		}
	}
}

// run1 reads a single value from a statement.
func run1(t *testing.T, conn *sqlbase.Conn, sql string) any {
	t.Helper()
	res, err := conn.Execute(context.Background(), drivers.ExecRequest{Database: "main", SQL: sql})
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		t.Fatalf("%s returned no value", sql)
	}
	return res.Rows[0][0]
}

func TestExplainReportsABadStatementAsAQueryError(t *testing.T) {
	conn := fixture(t)

	_, err := conn.Explain(context.Background(), drivers.ExplainRequest{
		Database: "main",
		SQL:      "SELECT * FROM no_such_table",
	})
	if err == nil {
		t.Fatal("expected an error for a statement SQLite cannot plan")
	}
	if !apperr.Is(err, apperr.CodeQueryFailed) {
		t.Fatalf("expected %s, got %v", apperr.CodeQueryFailed, err)
	}
}

// A spec that declares no wrapper has no plan to give, and says so before it
// tries to reach a server with a statement it cannot form.
func TestExplainWithoutAWrapperIsUnsupported(t *testing.T) {
	conn := sqlbase.NewConn(sqlbase.Spec{
		Info:      Driver{}.Info(),
		SQLDriver: "sqlite",
		DSN: func(models.ConnectionConfig, string) (string, error) {
			t.Error("a driver that cannot explain must not open a connection to find out")
			return "", nil
		},
	}, models.ConnectionConfig{})

	_, err := conn.Explain(context.Background(), drivers.ExplainRequest{SQL: "SELECT 1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("expected %s, got %v", apperr.CodeUnsupported, err)
	}
}

// The flag the UI hides the button with has to agree with the spec that would
// carry the statement, otherwise the button is drawn for an engine that answers
// CodeUnsupported (or hidden for one that would work).
func TestExplainCapabilityMatchesTheSpec(t *testing.T) {
	hasWrapper := spec().ExplainSQL != nil
	if info := (Driver{}).Info(); info.SupportsExplain != hasWrapper {
		t.Fatalf("SupportsExplain = %v but the spec's wrapper is %v", info.SupportsExplain, hasWrapper)
	}
}
