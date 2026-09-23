package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"dbmanager/internal/config"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// The change log is written by the layer that runs statements, so these tests
// drive it the way the application does — through the manager, with the real
// sqlite fixture — and then read the log back through the same call the window
// makes.

// loggedManager is the fixture manager with a data directory behind it: the log
// needs somewhere to live, and a manager built for the other tests has no store.
func loggedManager(t *testing.T) *Manager {
	t.Helper()
	manager, _ := testManager(t)
	store, err := config.NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	manager.store = store
	return manager
}

// entries reads the whole log back.
func entries(t *testing.T, manager *Manager) ([]models.ChangeLogEntry, int) {
	t.Helper()
	log, err := manager.ListChangeLog(0)
	if err != nil {
		t.Fatalf("list change log: %v", err)
	}
	return log.Entries, log.Total
}

func TestExecuteLogsTheStatementsThatChangedSomething(t *testing.T) {
	manager := loggedManager(t)

	// Order matters: only the two writes should be logged, and the create must
	// be recorded without a table while the alter below it names one — even
	// though the window passed the same object for both.
	_, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "main",
		Schema:   "",
		Object:   "orders",
		SQL: `SELECT count(*) FROM orders;
		      ALTER TABLE orders ADD note TEXT;
		      CREATE TABLE audit (id INTEGER PRIMARY KEY);
		      PRAGMA foreign_keys = ON;`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	log, total := entries(t, manager)
	if total != 2 || len(log) != 2 {
		t.Fatalf("logged %d entries (total %d), want the alter and the create: %+v", len(log), total, log)
	}
	// Newest first.
	if log[0].Kind != "create" || log[1].Kind != "alter" {
		t.Fatalf("the log is not newest first: %v then %v", log[0].Kind, log[1].Kind)
	}

	alter, create := log[1], log[0]
	if alter.Table != "orders" || alter.Database != "main" || alter.Schema != "" {
		t.Fatalf("the alter lost its object: %+v", alter)
	}
	if alter.Source != models.ChangeSourceScript || alter.Error != "" {
		t.Fatalf("unexpected source or error on a successful statement: %+v", alter)
	}
	if alter.Version != changeLogEntryVersion || alter.At == 0 {
		t.Fatalf("the entry is missing its stamp: %+v", alter)
	}
	if !strings.Contains(alter.Statement, "ADD note TEXT") {
		t.Fatalf("the statement was not kept: %q", alter.Statement)
	}

	// The table being created does not exist when the statement runs, and does
	// not get the window's object: the entry says what the statement was applied
	// to, and the name it introduces is in the statement itself.
	if create.Table != "" {
		t.Fatalf("a CREATE TABLE must not name an object, got %q", create.Table)
	}
	if create.Kind != "create" || create.Database != "main" {
		t.Fatalf("unexpected create entry: %+v", create)
	}

	// Where it ran is copied from the session, not looked up later.
	connection := alter.Connection
	if connection.Name != "fixture" || connection.Driver != models.DriverSQLite || connection.ID != "c1" {
		t.Fatalf("unexpected connection summary: %+v", connection)
	}
	if connection.Address == "" || !strings.HasSuffix(connection.Address, "shop.db") {
		t.Fatalf("a file-backed connection is addressed by its file, got %q", connection.Address)
	}
}

func TestExecuteLogsNothingForARead(t *testing.T) {
	manager := loggedManager(t)

	if _, err := manager.Execute(models.ExecRequest{SessionID: "s1", Database: "main", SQL: "SELECT * FROM orders;"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// A query window's DDL editor also runs scripts of its own; reads inside one
	// must not turn into entries either.
	if _, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "main",
		SQL:      "SELECT 1;\n-- a comment\nSELECT * FROM orders;",
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if log, total := entries(t, manager); total != 0 || len(log) != 0 {
		t.Fatalf("a read is not a change, but the log holds %+v", log)
	}
}

// A run that stops halfway really does leave part of the script applied, so the
// entries say what the run said rather than pretending each line was judged
// separately: the engine is handed the whole script in one call.
func TestExecuteRecordsTheMessageARunEndedWith(t *testing.T) {
	manager := loggedManager(t)

	_, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "main",
		SQL:      "INSERT INTO orders (id, user_id) VALUES (3, 9);\nINSERT INTO nope (id) VALUES (1);",
	})
	if err == nil {
		t.Fatal("expected the second statement to fail")
	}

	log, total := entries(t, manager)
	if total != 2 || len(log) != 2 {
		t.Fatalf("both statements ran, so both are logged: got %d (%d)", len(log), total)
	}
	if log[0].Error == "" || log[1].Error == "" {
		t.Fatalf("a failed run must be recorded on its statements: %+v", log)
	}
	if !strings.Contains(log[0].Error, "nope") {
		t.Fatalf("the entry should carry what the engine said: %q", log[0].Error)
	}
	// The first statement did land, and the fixture proves it: the row is there.
	count, err := manager.Execute(models.ExecRequest{SessionID: "s1", Database: "main", SQL: "SELECT count(*) FROM orders;"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if len(count.Rows) != 1 || fmt.Sprint(count.Rows[0][0]) != "3" {
		t.Fatalf("the row inserted before the failure is missing: %+v", count.Rows)
	}
}

// The structure page runs its plan statement by statement, so this is where an
// entry can honestly be attributed to the line that failed.
func TestApplyDesignLogsEveryStatementItRan(t *testing.T) {
	manager := loggedManager(t)

	design := designFor(t, manager)
	design.Columns = append(design.Columns, models.DesignColumn{Name: "note", DataType: "TEXT", Nullable: true})
	if _, err := manager.ApplyDesign(design); err != nil {
		t.Fatalf("apply: %v", err)
	}

	log, total := entries(t, manager)
	if total != 1 || len(log) != 1 {
		t.Fatalf("expected the one ADD COLUMN, got %+v", log)
	}
	if log[0].Source != models.ChangeSourceDesign || log[0].Kind != "alter" || log[0].Table != "orders" {
		t.Fatalf("unexpected design entry: %+v", log[0])
	}
	if !strings.Contains(log[0].Statement, "note") {
		t.Fatalf("the statement is not the one that ran: %q", log[0].Statement)
	}
}

func TestApplyCreateDesignRecordsTheNewTableWithoutAName(t *testing.T) {
	manager := loggedManager(t)

	design := newTableDesign()
	if _, err := manager.ApplyCreateDesign(design); err != nil {
		t.Fatalf("apply create: %v", err)
	}

	log, total := entries(t, manager)
	if total != 2 || len(log) != 2 {
		t.Fatalf("the plan has two statements, the log holds %+v", log)
	}
	// Newest first: the index is applied to the table the create introduced.
	index, create := log[0], log[1]
	if create.Kind != "create" || create.Table != "" {
		t.Fatalf("the CREATE TABLE must be logged without an object: %+v", create)
	}
	if index.Kind != "create" || index.Table != "customers" {
		t.Fatalf("the CREATE INDEX is applied to the new table: %+v", index)
	}
	if create.Source != models.ChangeSourceCreate || index.Source != models.ChangeSourceCreate {
		t.Fatalf("both statements came from the designer: %+v", log)
	}
}

// A manager with no store — which only a test builds — must not panic when it is
// asked to log, because the log is written after the statement has already run.
func TestLoggingWithoutAStoreIsSilent(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "main",
		SQL:      "INSERT INTO orders (id, user_id) VALUES (9, 9);",
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	log, err := manager.ListChangeLog(0)
	if err != nil {
		t.Fatalf("an empty log is not an error: %v", err)
	}
	if log.Entries == nil || len(log.Entries) != 0 || log.Total != 0 {
		t.Fatalf("expected an empty log, got %+v", log)
	}
}

func TestListChangeLogClampsWhatItIsAsked(t *testing.T) {
	manager := loggedManager(t)

	for i := 0; i < 5; i++ {
		if _, err := manager.Execute(models.ExecRequest{SessionID: "s1",
			Database: "main",
			SQL:      fmt.Sprintf("INSERT INTO orders (id, user_id) VALUES (%d, %d);", 100+i, 100+i),
		}); err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
	}

	// The default page is applied when no limit is given, and the total still
	// says how much is behind it.
	log, err := manager.ListChangeLog(0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if log.Total != 5 || len(log.Entries) != 5 {
		t.Fatalf("expected five entries, got %d of %d", len(log.Entries), log.Total)
	}
	log, err = manager.ListChangeLog(-10)
	if err != nil || len(log.Entries) != 5 {
		t.Fatalf("a negative limit must fall back to the default page: %+v (%v)", log, err)
	}
	log, err = manager.ListChangeLog(2)
	if err != nil || len(log.Entries) != 2 || log.Total != 5 {
		t.Fatalf("expected two of five, got %d of %d (%v)", len(log.Entries), log.Total, err)
	}
}

// shellConn stands in for a driver whose statements are not SQL. The log has to
// ask the driver what a script does — the same call the DDL editor's dry run
// makes — or a document store's writes would never be recorded at all.
type shellConn struct {
	drivers.Conn
	ran []string
}

func (c *shellConn) AnalyzeScript(script string, readOnly bool) models.ScriptAnalysis {
	analysis := models.ScriptAnalysis{
		Statements: []models.ScriptStatement{},
		Warnings:   []string{},
		ReadOnly:   readOnly,
	}
	for i, raw := range strings.Split(strings.TrimSpace(script), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		// `find` reads, `drop` is schema, everything else writes: this fake driver
		// only has to be right about the two answers the log cares about.
		kind := sqlutil.KindDML
		switch {
		case strings.Contains(raw, "find("):
			kind = sqlutil.KindQuery
		case strings.Contains(raw, "drop()"):
			kind = sqlutil.KindDDL
		}
		analysis.Statements = append(analysis.Statements, models.ScriptStatement{
			Index:   i,
			Kind:    kind,
			Preview: raw,
			SQL:     raw,
		})
	}
	return analysis
}

func (c *shellConn) Execute(_ context.Context, req drivers.ExecRequest) (*models.QueryResult, error) {
	c.ran = append(c.ran, req.SQL)
	return &models.QueryResult{AffectedRows: 1}, nil
}

// A driver that describes itself decides what its writes are: without asking it,
// `db.orders.insertOne(…)` would be classified by a SQL keyword table that has
// never heard of it, and the log would stay empty for a whole engine.
func TestExecuteLogsAScriptTheDriversOwnParserUnderstands(t *testing.T) {
	manager := loggedManager(t)

	session, err := manager.session("s1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	conn := &shellConn{Conn: session.conn}
	session.conn = conn

	if _, err := manager.Execute(models.ExecRequest{SessionID: "s1",
		Database: "shop",
		SQL:      "db.orders.insertOne({id: 1})\ndb.orders.find({})\ndb.reports.drop()",
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(conn.ran) != 1 {
		t.Fatalf("the script is handed to the engine in one call, got %d", len(conn.ran))
	}

	log, total := entries(t, manager)
	if total != 2 || len(log) != 2 {
		t.Fatalf("the read is not a change: got %+v", log)
	}
	// Newest first, and the statement is kept as written rather than as a
	// preview. A method call has no leading keyword, so the tag falls back to
	// the classification the driver itself produced.
	if log[0].Kind != "ddl" || log[1].Kind != "dml" {
		t.Fatalf("a shell statement is tagged by what the driver called it: %v then %v", log[0].Kind, log[1].Kind)
	}
	if log[0].Statement != "db.reports.drop()" || log[1].Statement != "db.orders.insertOne({id: 1})" {
		t.Fatalf("the log kept a preview rather than the statement: %+v", log)
	}
	if log[1].Database != "shop" || log[1].Table != "" {
		t.Fatalf("the window's database is the only context a shell has: %+v", log[1])
	}
}
