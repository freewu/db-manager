// Package sqltest is a database/sql driver that answers queries from a canned
// list instead of a server.
//
// The engines on the MySQL wire protocol do their real work in SQL text: Doris
// reads its catalog through SHOW statements, and the status pages read SHOW
// FRONTENDS/BACKENDS. Those queries cannot be reviewed by reading them, and a
// real Doris is not something a test can assume, so this package stands in for
// one: it matches the statement, returns the rows, and records what was asked.
//
// It is deliberately hand-written against database/sql/driver rather than a
// mocking library: the project ships no test-only dependencies, and the surface
// a test needs here is small.
package sqltest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// Expectation is one canned answer. Match is a prefix of the statement after
// whitespace has been collapsed, so a multi-line constant can be matched by its
// first line.
type Expectation struct {
	Match   string
	Columns []string
	Rows    [][]any
	Err     error
}

// Handle is a database connection plus the statements it has been asked for.
// Handle.DB is a *sql.DB, which already satisfies sqlbase.Querier, so it can be
// handed straight to a driver's catalog methods.
type Handle struct {
	DB     *sql.DB
	driver *fakeDriver
}

// Calls returns the statements that were run, in order.
func (h *Handle) Calls() []string { return h.driver.calls() }

// New opens a fake database and closes it when the test ends.
func New(t *testing.T, expectations ...Expectation) *Handle {
	t.Helper()
	d := &fakeDriver{expectations: expectations}
	h := &Handle{DB: sql.OpenDB(connector{driver: d}), driver: d}
	t.Cleanup(func() { _ = h.DB.Close() })
	return h
}

// --- driver ----------------------------------------------------------------

type connector struct{ driver *fakeDriver }

func (c connector) Connect(context.Context) (driver.Conn, error) { return &conn{parent: c.driver}, nil }
func (c connector) Driver() driver.Driver                        { return c.driver }

type fakeDriver struct {
	mu           sync.Mutex
	expectations []Expectation
	called       []string
}

func (d *fakeDriver) Open(string) (driver.Conn, error) { return &conn{parent: d}, nil }

func (d *fakeDriver) record(query string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.called = append(d.called, collapse(query))
}

func (d *fakeDriver) calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.called...)
}

// lookup finds the expectation whose Match is a prefix of the statement.
func (d *fakeDriver) lookup(query string) (Expectation, bool) {
	d.record(query)
	flat := collapse(query)
	for _, e := range d.expectations {
		if strings.HasPrefix(flat, collapse(e.Match)) {
			return e, true
		}
	}
	return Expectation{}, false
}

type conn struct{ parent *fakeDriver }

func (c *conn) Prepare(string) (driver.Stmt, error) {
	// Every statement in the drivers goes through QueryContext/ExecContext, so
	// a prepared statement means the test is looking at a code path that was
	// never meant to reach a server.
	return nil, errors.New("sqltest: prepared statements are not supported")
}

func (c *conn) Close() error { return nil }
func (c *conn) Begin() (driver.Tx, error) {
	return nil, errors.New("sqltest: transactions are not supported")
}

func (c *conn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	expectation, ok := c.parent.lookup(query)
	if !ok {
		return nil, fmt.Errorf("sqltest: no expectation for %q", collapse(query))
	}
	if expectation.Err != nil {
		return nil, expectation.Err
	}
	return &rows{columns: expectation.Columns, rows: expectation.Rows}, nil
}

func (c *conn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("sqltest: ExecContext is not implemented")
}

// --- rows ------------------------------------------------------------------

type rows struct {
	columns []string
	rows    [][]any
	next    int
}

func (r *rows) Columns() []string { return r.columns }
func (r *rows) Close() error      { return nil }

func (r *rows) Next(dest []driver.Value) error {
	if r.next >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.next]
	r.next++
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	return nil
}

// collapse turns a statement into one line with single spaces so that a
// multi-line constant in the drivers can be matched by quoting its first line.
func collapse(query string) string {
	return strings.Join(strings.Fields(query), " ")
}
