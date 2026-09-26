package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Running a SQL file is the one place where this application runs text it did
// not write, from a file, without a transaction around it. What these tests look
// at is exactly that: that every statement reaches the engine in order, that a
// failure stops the run where it happened rather than at the end, that what did
// run is kept, and that the change log can say afterwards which file each
// statement came from and how much it changed.

// writeScript puts a script somewhere in the test's own directory and answers
// with its path, the way the window hands one over.
func writeScript(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func sqlFileRequest(path string) models.SQLFileRequest {
	return models.SQLFileRequest{
		ID:          "r1",
		SessionID:   "s1",
		Database:    "main",
		Path:        path,
		StopOnError: true,
	}
}

// countRows reads the fixture file directly rather than through the manager, so
// what the run did to the database is checked by something the run cannot have
// influenced.
func countRows(t *testing.T, file, query string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+file)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("count with %q: %v", query, err)
	}
	return n
}

// fileSize is what progress is a fraction of, measured by the test rather than
// taken from the run that reports it.
func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat script: %v", err)
	}
	return info.Size()
}

// The analysis is the dry run: what the file holds, before anything is sent
// anywhere. It is a heuristic reading of the text, so what it has to get right is
// the count, the classification and the statements worth a second look.
func TestAnalyzeSQLFileDescribesWhatTheFileHolds(t *testing.T) {
	manager, _ := testManager(t)
	path := writeScript(t, `
-- a dump of the shop database
CREATE TABLE audit (id INTEGER PRIMARY KEY);
INSERT INTO audit (id) VALUES (1);
INSERT INTO audit (id) VALUES (2);
/* the table was a mistake */
DROP TABLE audit;
BEGIN;
USE `+"`other`"+`;
`)

	analysis, err := manager.AnalyzeSQLFile(sqlFileRequest(path))
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.Statements != 6 {
		t.Fatalf("expected six statements, got %d: %+v", analysis.Statements, analysis.Shown)
	}
	if len(analysis.Shown) != 6 {
		t.Fatalf("every statement should be listed for a file this size: %+v", analysis.Shown)
	}
	if got := analysis.Shown[0].Preview; !strings.HasPrefix(got, "CREATE TABLE audit") {
		t.Fatalf("the first statement should be the create, got %q", got)
	}
	if got := analysis.Shown[3].Preview; got != "DROP TABLE audit" {
		t.Fatalf("comments should be stripped from the preview, got %q", got)
	}
	if analysis.Destructive != 1 {
		t.Fatalf("only the drop can lose anything, got %d", analysis.Destructive)
	}
	if len(analysis.DestructiveIndexes) != 1 || analysis.DestructiveIndexes[0] != 4 {
		t.Fatalf("the drop is the fourth statement: %+v", analysis.DestructiveIndexes)
	}
	if analysis.Size == 0 {
		t.Fatal("the analysis should carry the size of the file")
	}
	if analysis.Refused != 0 {
		t.Fatalf("nothing is refused on a writable connection, got %d", analysis.Refused)
	}

	// The warnings are the file telling on itself: the drop, the statement this
	// build cannot read, and the USE that sends the rest of the file elsewhere.
	joined := strings.Join(analysis.Warnings, "\n")
	for _, want := range []string{"statement 4", "statement 5", "statement 6", "other", "unrecognised"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings should mention %q: %s", want, joined)
		}
	}
	if len(analysis.Warnings) != 3 {
		t.Fatalf("expected three warnings, got %d: %s", len(analysis.Warnings), joined)
	}
}

// A read-only connection is refused no statement by the analysis — it only says
// how many of them will be: the run is where they are actually refused, one at a
// time, with the engine's own answer.
func TestAnalyzeSQLFileCountsWhatAReadOnlyConnectionWouldRefuse(t *testing.T) {
	manager, _ := testManager(t)
	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.readOnly = true

	path := writeScript(t, "SELECT 1;\nCREATE TABLE audit (id INTEGER PRIMARY KEY);\nINSERT INTO audit (id) VALUES (1);")
	analysis, err := manager.AnalyzeSQLFile(sqlFileRequest(path))
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.Refused != 2 {
		t.Fatalf("the two writes would be refused, the read would not: %+v", analysis)
	}
}

func TestRunSQLFileRunsEveryStatementInOrder(t *testing.T) {
	manager, file := testManager(t)
	path := writeScript(t, `
CREATE TABLE import_test (id INTEGER PRIMARY KEY, note TEXT);
INSERT INTO import_test (id, note) VALUES (1, 'first');
INSERT INTO import_test (id, note) VALUES (2, 'second');
`)

	var ticks []models.SQLFileProgress
	result, err := manager.RunSQLFile(sqlFileRequest(path), func(p models.SQLFileProgress) {
		ticks = append(ticks, p)
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Statements != 3 || result.Failed != 0 {
		t.Fatalf("three statements, none failed: %+v", result)
	}
	if result.Rows != 2 {
		t.Fatalf("the two inserts changed two rows, got %d: %+v", result.Rows, result)
	}
	if result.Cancelled || result.StoppedOnError {
		t.Fatalf("nothing stopped this run: %+v", result)
	}
	if n := countRows(t, file, "SELECT count(*) FROM import_test"); n != 2 {
		t.Fatalf("both rows should be in the table, got %d", n)
	}

	// Progress is drawn from the file, so it both starts at nothing read and
	// ends with the whole file read — and it never goes backwards.
	if len(ticks) < 2 {
		t.Fatalf("expected progress before and after, got %d ticks", len(ticks))
	}
	if ticks[0].Bytes != 0 || ticks[0].Done != 0 {
		t.Fatalf("the first tick should be at the start: %+v", ticks[0])
	}
	last := ticks[len(ticks)-1]
	if last.Bytes != fileSize(t, path) {
		t.Fatalf("the last tick should be the whole file: %+v", last)
	}
	if last.Done != 3 || last.Failed != 0 || last.Rows != 2 {
		t.Fatalf("the last tick should carry the totals: %+v", last)
	}
	for i, tick := range ticks {
		if i > 0 && tick.Bytes < ticks[i-1].Bytes {
			t.Fatalf("progress went backwards at %d: %+v", i, tick)
		}
	}
	if last.Statement == "" {
		t.Fatal("the last tick should name the statement it just ran")
	}
}

// A failure stops the run at the statement that failed, and what already ran is
// left in place: there is no transaction around the file, so this is the truth
// about what an interrupted file did, not a bug to be papered over.
func TestRunSQLFileStopsAtTheFailureItWasToldToStopAt(t *testing.T) {
	manager, file := testManager(t)
	path := writeScript(t, `
INSERT INTO orders (id, user_id) VALUES (41, 1);
INSERT INTO missing_table (id) VALUES (1);
INSERT INTO orders (id, user_id) VALUES (43, 3);
`)

	result, err := manager.RunSQLFile(sqlFileRequest(path), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Statements != 2 {
		t.Fatalf("the run should end at the failing statement, got %d: %+v", result.Statements, result)
	}
	if result.Failed != 1 || !result.StoppedOnError || result.Cancelled {
		t.Fatalf("one failure stopped the run: %+v", result)
	}
	if result.Rows != 1 {
		t.Fatalf("the statement before the failure changed a row, got %d: %+v", result.Rows, result)
	}
	if len(result.Errors) != 1 || result.Errors[0].Index != 2 {
		t.Fatalf("the failure should be the second statement: %+v", result.Errors)
	}
	if !strings.Contains(result.Errors[0].Statement, "missing_table") {
		t.Fatalf("the failure should quote the statement: %+v", result.Errors[0])
	}
	if result.ErrorsTruncated {
		t.Fatalf("one failure is not a truncated list: %+v", result)
	}

	if n := countRows(t, file, "SELECT count(*) FROM orders WHERE id = 41"); n != 1 {
		t.Fatal("the statement that ran before the failure should still be there")
	}
	if n := countRows(t, file, "SELECT count(*) FROM orders WHERE id = 43"); n != 0 {
		t.Fatal("the statements after a failure should not have run")
	}
}

// With stop-on-error off, a file that half-applies is carried to the end and the
// failures are collected: that is what an import of a dump with a duplicate key
// in the middle of it wants.
func TestRunSQLFileCarriesOnPastFailuresWhenToldTo(t *testing.T) {
	manager, file := testManager(t)
	path := writeScript(t, `
INSERT INTO orders (id, user_id) VALUES (41, 1);
INSERT INTO missing_table (id) VALUES (1);
INSERT INTO orders (id, user_id) VALUES (43, 3);
`)

	req := sqlFileRequest(path)
	req.StopOnError = false
	result, err := manager.RunSQLFile(req, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Statements != 3 || result.Failed != 1 {
		t.Fatalf("all three statements should be tried: %+v", result)
	}
	if result.StoppedOnError || result.Cancelled {
		t.Fatalf("this run was carried to the end: %+v", result)
	}
	if n := countRows(t, file, "SELECT count(*) FROM orders WHERE id = 43"); n != 1 {
		t.Fatal("the statement after the failure should have run")
	}
}

// A file where everything fails must not answer with everything: the summary
// lists the first few failures and says that it stopped listing, because a
// window that has to paint ten thousand identical errors has told the reader
// nothing the first twenty did not.
func TestRunSQLFileCapsTheFailuresItReports(t *testing.T) {
	manager, _ := testManager(t)
	var script strings.Builder
	for i := 0; i < 30; i++ {
		script.WriteString("INSERT INTO missing_table (id) VALUES (" + strconv.Itoa(i) + ");\n")
	}
	path := writeScript(t, script.String())

	req := sqlFileRequest(path)
	req.StopOnError = false
	result, err := manager.RunSQLFile(req, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Statements != 30 || result.Failed != 30 {
		t.Fatalf("every statement failed: %+v", result)
	}
	if len(result.Errors) != sqlFileErrorLimit || !result.ErrorsTruncated {
		t.Fatalf("expected %d listed failures and the truncation flag: %d listed",
			sqlFileErrorLimit, len(result.Errors))
	}
	if result.Errors[0].Index != 1 || result.Errors[sqlFileErrorLimit-1].Index != sqlFileErrorLimit {
		t.Fatalf("the failures listed should be the first ones: %+v", result.Errors)
	}
}

// The change log is where the run has to be legible afterwards, and running one
// statement at a time is what makes it so: every write gets its own entry, with
// the row count that belongs to it, tagged as coming from a file.
func TestRunSQLFileLogsEveryWriteWithItsOwnCount(t *testing.T) {
	manager := loggedManager(t)
	path := writeScript(t, `
INSERT INTO orders (id, user_id) VALUES (51, 1);
INSERT INTO orders (id, user_id) VALUES (52, 2), (53, 3);
SELECT count(*) FROM orders;
`)

	if _, err := manager.RunSQLFile(sqlFileRequest(path), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	log, total := entries(t, manager)
	if total != 2 || len(log) != 2 {
		t.Fatalf("the two inserts should be logged and the read should not: %+v", log)
	}
	// Newest first: the two-row insert, then the one-row insert.
	if log[0].Rows != 2 || log[1].Rows != 1 {
		t.Fatalf("each entry should carry its own count: %+v", log)
	}
	for _, entry := range log {
		if entry.Source != models.ChangeSourceSQLFile {
			t.Fatalf("entries from a file should say so: %+v", entry)
		}
		if entry.Database != "main" {
			t.Fatalf("the entry should name the database it ran against: %+v", entry)
		}
		if entry.Error != "" {
			t.Fatalf("nothing failed in this run: %+v", entry)
		}
	}
}

// A file written by another client says which database it is about with a USE.
// The statement is not sent — every statement this window sends names its
// database itself — it changes the name the following ones are sent with, which
// the log then shows.
func TestRunSQLFileFollowsUseInsteadOfSendingIt(t *testing.T) {
	manager := loggedManager(t)
	path := writeScript(t, "USE other;\nINSERT INTO orders (id, user_id) VALUES (61, 1);")

	result, err := manager.RunSQLFile(sqlFileRequest(path), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Statements != 2 || result.Failed != 0 {
		t.Fatalf("the USE is a statement of the file even though it is not sent: %+v", result)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("only the insert is a change: %+v", log)
	}
	if log[0].Database != "other" {
		t.Fatalf("the statements after a USE go to the database it named: %+v", log[0])
	}
	if strings.Contains(log[0].Statement, "USE") {
		t.Fatalf("the USE itself is not a change: %+v", log[0])
	}
}

// Stopping a run ends it at the next statement, and the statement that was
// interrupted is not reported as a failure: the engine never got to refuse it,
// the window took it away. What already ran stays — there is no transaction
// around the file — so the summary says how far the run got instead.
func TestRunSQLFileStopsAtTheNextStatementWhenAsked(t *testing.T) {
	manager, file := testManager(t)
	var script strings.Builder
	script.WriteString("CREATE TABLE stop_test (id INTEGER PRIMARY KEY);\n")
	for i := 1; i <= 200; i++ {
		script.WriteString("INSERT INTO stop_test (id) VALUES (" + strconv.Itoa(i) + ");\n")
	}
	path := writeScript(t, script.String())

	stopped := false
	result, err := manager.RunSQLFile(sqlFileRequest(path), func(p models.SQLFileProgress) {
		if stopped {
			return
		}
		stopped = true
		if err := manager.CancelSQLFile("r1"); err != nil {
			t.Errorf("cancel: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Cancelled {
		t.Fatalf("the run should report that it was stopped: %+v", result)
	}
	if result.Statements != 0 {
		t.Fatalf("the first tick is sent before the first statement, so the run should stop before it: %+v", result)
	}
	if result.Failed != 0 || len(result.Errors) != 0 {
		t.Fatalf("a stopped run has no failures to report: %+v", result)
	}
	if n := countRows(t, file, "SELECT count(*) FROM sqlite_master WHERE name = 'stop_test'"); n != 0 {
		t.Fatalf("the file should not have got as far as creating the table, got %d", n)
	}
}

// The window cannot ask to stop in the instant between two statements, so this
// drives the run object with a context that is already cancelled: a statement
// interrupted by the stop is not a failed statement, whatever the engine says
// about it.
func TestAStatementInterruptedByTheStopIsNotAFailure(t *testing.T) {
	manager, file := testManager(t)
	path := writeScript(t, "SELECT 1;")
	run := &sqlFileRun{manager: manager, req: sqlFileRequest(path)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if run.step(ctx, "INSERT INTO orders (id, user_id) VALUES (99, 1);", nil, 0) {
		t.Fatal("a stopped run should not carry on")
	}
	if !run.cancelled || run.stopped {
		t.Fatalf("the run was stopped, not failed: %+v", run)
	}
	if run.failed != 0 || len(run.errors) != 0 {
		t.Fatalf("an interrupted statement did not get to fail: %+v", run.errors)
	}
	if n := countRows(t, file, "SELECT count(*) FROM orders WHERE id = 99"); n != 0 {
		t.Fatal("the interrupted statement should not have run")
	}
}

// Cancelling a run that has already finished, or one that never existed, is the
// answer the window wanted: nothing is going.
func TestCancelSQLFileIsQuietAboutRunsThatAreNotGoing(t *testing.T) {
	manager, _ := testManager(t)
	if err := manager.CancelSQLFile("nobody"); err != nil {
		t.Fatalf("cancelling an unknown run: %v", err)
	}
	if err := manager.CancelSQLFile("  "); !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("a run with no id is a request the window should not have sent: %v", err)
	}
}

func TestRunSQLFileRefusesRequestsTheWindowShouldNotSend(t *testing.T) {
	manager, _ := testManager(t)
	path := writeScript(t, "SELECT 1;")

	cases := []struct {
		name string
		req  models.SQLFileRequest
		code string
		// only the run needs the id the window answers for, so the analysis is
		// only asked about the requests it could have been given.
		analysis bool
	}{
		{"no run id", models.SQLFileRequest{SessionID: "s1", Path: path}, apperr.CodeInvalidConfig, false},
		{"no session", models.SQLFileRequest{ID: "r1", Path: path}, apperr.CodeInvalidConfig, true},
		{"unknown session", models.SQLFileRequest{ID: "r1", SessionID: "s9", Path: path}, apperr.CodeNotFound, true},
		{"no file", models.SQLFileRequest{ID: "r1", SessionID: "s1"}, apperr.CodeInvalidConfig, true},
		{"no such file", models.SQLFileRequest{ID: "r1", SessionID: "s1", Path: filepath.Join(t.TempDir(), "gone.sql")}, apperr.CodeInvalidConfig, true},
		{"a folder", models.SQLFileRequest{ID: "r1", SessionID: "s1", Path: t.TempDir()}, apperr.CodeInvalidConfig, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manager.RunSQLFile(tc.req, nil); !apperr.Is(err, tc.code) {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			if !tc.analysis {
				return
			}
			if _, err := manager.AnalyzeSQLFile(tc.req); !apperr.Is(err, tc.code) {
				t.Fatalf("the analysis should refuse it too: %v", err)
			}
		})
	}
}

// The gate on this feature is a capability rather than an engine name: a driver
// whose statements are not SQL has no file to run, and says so before reading it.
func TestSQLFileNeedsADriverWhoseStatementsAreSQL(t *testing.T) {
	manager, _ := testManager(t)
	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session.driver = shellDriver{conn: &shellConn{}}
	path := writeScript(t, "db.orders.find({})")

	req := sqlFileRequest(path)
	if _, err := manager.AnalyzeSQLFile(req); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("expected the analysis to refuse a non-SQL engine, got %v", err)
	}
	if _, err := manager.RunSQLFile(req, nil); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("expected the run to refuse a non-SQL engine, got %v", err)
	}
}

// shellDriver is a driver whose statements are not SQL, standing in for an engine
// that would only be reachable with a server in these tests. Only Info and the
// refusal matter here: nothing opens it.
type shellDriver struct{ conn *shellConn }

func (shellDriver) Info() models.DriverInfo {
	return models.DriverInfo{Type: "shell", DisplayName: "Shell", Relational: false}
}

func (shellDriver) Normalize(*models.ConnectionConfig) error { return nil }

func (d shellDriver) Open(context.Context, models.ConnectionConfig) (drivers.Conn, error) {
	return d.conn, nil
}
