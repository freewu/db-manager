package service

import (
	"strings"
	"testing"
	"time"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// AnalyzeScript is the DDL editor's dry run: it must reflect the session's
// read-only flag without touching the database.

func TestAnalyzeScriptCarriesSessionReadOnly(t *testing.T) {
	manager, _ := testManager(t)

	analysis, err := manager.AnalyzeScript("s1", "DROP TABLE orders;")
	if err != nil {
		t.Fatalf("analyse: %v", err)
	}
	if analysis.ReadOnly {
		t.Fatal("the fixture session is not read-only")
	}
	if !analysis.Destructive || analysis.Refused != 0 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}

	// Flip the session read-only and the same script is reported as refused.
	manager.mu.Lock()
	manager.sessions["s1"].readOnly = true
	manager.mu.Unlock()

	analysis, err = manager.AnalyzeScript("s1", "SELECT 1; DROP TABLE orders;")
	if err != nil {
		t.Fatalf("analyse read-only: %v", err)
	}
	if !analysis.ReadOnly || analysis.Refused != 1 {
		t.Fatalf("expected one refused statement, got %+v", analysis)
	}
	if len(analysis.Warnings) == 0 || !strings.Contains(analysis.Warnings[0], "read-only") {
		t.Fatalf("expected a read-only warning, got %v", analysis.Warnings)
	}
}

// A document store cannot be described by SQL keywords, so it answers the dry
// run itself (drivers.Analyzer). The manager's job is only to prefer that
// answer over the keyword table, and to hand it the session's read-only flag.
func TestAnalyzeScriptPrefersTheDriverAnalyzer(t *testing.T) {
	manager, _ := testManager(t)

	// `db.orders.drop()` is a method call: nothing in the keyword table can
	// recognise it, which is exactly why a real driver implements the interface.
	plain, err := manager.AnalyzeScript("s1", "db.orders.drop()")
	if err != nil {
		t.Fatalf("analyse: %v", err)
	}
	if plain.Statements[0].Kind != sqlutil.KindUnknown {
		t.Fatalf("expected the SQL heuristic to give up, got %+v", plain.Statements[0])
	}

	manager.mu.Lock()
	manager.sessions["s1"].conn = docConn{Conn: manager.sessions["s1"].conn}
	manager.sessions["s1"].readOnly = true
	manager.mu.Unlock()

	described, err := manager.AnalyzeScript("s1", "db.orders.drop()")
	if err != nil {
		t.Fatalf("analyse through the driver: %v", err)
	}
	if !described.ReadOnly || described.Refused != 1 {
		t.Fatalf("the driver's answer must carry the session's read-only flag: %+v", described)
	}
	if len(described.Statements) != 1 || described.Statements[0].Kind != sqlutil.KindDDL {
		t.Fatalf("the manager used the keyword table instead of the driver: %+v", described.Statements)
	}
}

// docConn answers the dry run the way a document store would, and hides every
// other capability behind the drivers.Conn contract — the SQL fixture below it
// would otherwise classify the script itself.
type docConn struct {
	drivers.Conn
}

func (docConn) AnalyzeScript(script string, readOnly bool) models.ScriptAnalysis {
	refused := 0
	if readOnly {
		refused = 1
	}
	return models.ScriptAnalysis{
		ReadOnly: readOnly,
		Refused:  refused,
		Statements: []models.ScriptStatement{
			{Index: 0, Kind: sqlutil.KindDDL, Preview: script, Destructive: true, Reason: "drops the collection"},
		},
		Warnings: []string{"statement 1: drops the collection"},
	}
}

func TestAnalyzeScriptRejectsEmptyAndUnknownSessions(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.AnalyzeScript("s1", "   \n"); err == nil {
		t.Fatal("expected an error for an empty script")
	}
	if _, err := manager.AnalyzeScript("nope", "SELECT 1"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}

// The ER diagram goes through drivers.Grapher when the driver has it, and falls
// back to one Structure call per object when it does not. The fallback is what
// keeps every engine diagrammable, so it is tested through a wrapper that
// deliberately hides the Grapher capability.
func TestGraphFallsBackToStructure(t *testing.T) {
	manager, _ := testManager(t)

	graph, err := manager.Graph("s1", "main", "")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if len(graph.Nodes) != 1 || graph.Nodes[0].Name != "orders" {
		t.Fatalf("unexpected nodes: %+v", graph.Nodes)
	}
	if len(graph.Nodes[0].Columns) == 0 {
		t.Fatal("the fallback must still carry the columns")
	}

	// Swap in a connection that only implements drivers.Conn: same result.
	manager.mu.Lock()
	manager.sessions["s1"].conn = plainConn{manager.sessions["s1"].conn}
	manager.mu.Unlock()

	fallback, err := manager.Graph("s1", "main", "")
	if err != nil {
		t.Fatalf("fallback graph: %v", err)
	}
	if len(fallback.Nodes) != 1 || fallback.Nodes[0].Name != "orders" {
		t.Fatalf("unexpected fallback nodes: %+v", fallback.Nodes)
	}
	if len(fallback.Nodes[0].Columns) != len(graph.Nodes[0].Columns) {
		t.Fatalf("the fallback describes %d columns, the driver %d",
			len(fallback.Nodes[0].Columns), len(graph.Nodes[0].Columns))
	}
}

// plainConn hides everything except the drivers.Conn contract.
type plainConn struct {
	drivers.Conn
}

// The runtime overview is engine specific, but the session-level frame around it
// (who, which version, how long the snapshot took) is the service's job — that
// frame is what the UI shows even when the engine has nothing to add.
func TestOverviewFrameComesFromTheSession(t *testing.T) {
	manager, _ := testManager(t)

	// The fixture builds its session by hand, so give it the two facts a real
	// connect records: the version the server reported and when it did so.
	manager.mu.Lock()
	manager.sessions["s1"].version = "3.50.0"
	manager.sessions["s1"].connectedAt = time.Now().UnixMilli()
	manager.mu.Unlock()

	page, err := manager.Overview("s1")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if page.SessionID != "s1" || page.Name == "" || page.Driver == "" {
		t.Fatalf("the page must identify its session: %+v", page)
	}
	if page.ServerVersion != "3.50.0" || page.ConnectedAt == 0 || page.CollectedAt == 0 {
		t.Fatalf("the page must carry the session's own facts: %+v", page)
	}
	if !page.Supported || page.SQLite == nil {
		t.Fatalf("the fixture is SQLite, so its section must be filled in: %+v", page)
	}
	if page.ElapsedMS < 0 {
		t.Fatalf("elapsed time cannot be negative: %d", page.ElapsedMS)
	}

	// An engine without runtime reporting keeps the same frame and says so
	// instead of failing: the session is alive, it just has nothing to report.
	manager.mu.Lock()
	manager.sessions["s1"].conn = plainConn{manager.sessions["s1"].conn}
	manager.mu.Unlock()

	quiet, err := manager.Overview("s1")
	if err != nil {
		t.Fatalf("overview without a reporter: %v", err)
	}
	if quiet.Supported || quiet.SQLite != nil {
		t.Fatalf("an engine without a reporter must not invent numbers: %+v", quiet)
	}
	if len(quiet.Warnings) == 0 {
		t.Fatal("the page must explain why it is empty")
	}
	if _, err := manager.Overview("nope"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
