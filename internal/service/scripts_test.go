package service

import (
	"strings"
	"testing"
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
