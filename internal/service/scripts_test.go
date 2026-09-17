package service

import (
	"strings"
	"testing"

	"dbmanager/internal/drivers"
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
