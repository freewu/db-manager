package service

import (
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// A plan comes back through the session, wrapped in the engine's own form.
func TestExplainReadsThePlanThroughTheSession(t *testing.T) {
	manager, _ := testManager(t)

	plan, err := manager.Explain(models.ExplainRequest{
		SessionID: "s1",
		Database:  "main",
		SQL:       "SELECT * FROM orders WHERE id = 1",
	})
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if plan.Statement != "EXPLAIN QUERY PLAN SELECT * FROM orders WHERE id = 1" {
		t.Fatalf("unexpected statement: %q", plan.Statement)
	}
	if plan.SQL != "SELECT * FROM orders WHERE id = 1" {
		t.Fatalf("the plan must name the statement it is about: %q", plan.SQL)
	}
	if len(plan.Rows) == 0 {
		t.Fatal("SQLite should answer with at least one plan step")
	}
	if len(plan.Notes) == 0 {
		t.Fatal("the plan has to carry the note that nothing was run")
	}
}

// A plan describes one statement. Handing a script to an engine would either
// explain its first statement silently or return plans nobody asked for, so the
// window is told to narrow it down instead.
func TestExplainRefusesAScript(t *testing.T) {
	manager, _ := testManager(t)

	_, err := manager.Explain(models.ExplainRequest{
		SessionID: "s1",
		Database:  "main",
		SQL:       "SELECT 1;\nSELECT 2;",
	})
	if err == nil {
		t.Fatal("expected an error for two statements")
	}
	if !apperr.Is(err, apperr.CodeInvalidConfig) {
		t.Fatalf("expected %s, got %v", apperr.CodeInvalidConfig, err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("the message should say how many statements it found: %v", err)
	}

	// One statement with a trailing semicolon and a comment above it is still
	// one statement. The semicolon is dropped (the wrapper goes in front of the
	// text, so it has to be a statement, not a statement plus punctuation) and
	// the comment travels along — it is part of what the user wrote, and both
	// engines accept a comment between the keyword and the statement.
	plan, err := manager.Explain(models.ExplainRequest{
		SessionID: "s1",
		Database:  "main",
		SQL:       "-- how many?\nSELECT count(*) FROM orders;\n",
	})
	if err != nil {
		t.Fatalf("explain one statement: %v", err)
	}
	if !strings.HasSuffix(plan.SQL, "SELECT count(*) FROM orders") {
		t.Fatalf("the explainable statement was not cleaned up: %q", plan.SQL)
	}
	if strings.Contains(plan.SQL, ";") {
		t.Fatalf("the wrapper would have been put in front of a semicolon: %q", plan.SQL)
	}
}

func TestExplainRejectsEmptyAndUnknownSessions(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.Explain(models.ExplainRequest{SessionID: "s1", SQL: "  \n"}); err == nil {
		t.Fatal("expected an error for an empty statement")
	}
	if _, err := manager.Explain(models.ExplainRequest{SessionID: "nope", SQL: "SELECT 1"}); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}

// The plan view is hidden by a capability flag, but a driver that hides every
// other capability behind the drivers.Conn contract must still be answered
// honestly (this is what a document store does).
func TestExplainSaysSoWhenTheDriverHasNoPlan(t *testing.T) {
	manager, _ := testManager(t)

	manager.mu.Lock()
	manager.sessions["s1"].conn = plainConn{manager.sessions["s1"].conn}
	manager.mu.Unlock()

	_, err := manager.Explain(models.ExplainRequest{SessionID: "s1", SQL: "SELECT 1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("expected %s, got %v", apperr.CodeUnsupported, err)
	}
}

// The whole point of a plan is looking before you leap, so the statement being
// planned must not have happened afterwards — including on the session's own
// pooled connection.
func TestExplainNeverRunsWhatItPlans(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.Explain(models.ExplainRequest{
		SessionID: "s1",
		Database:  "main",
		SQL:       "DELETE FROM orders",
	}); err != nil {
		t.Fatalf("explain the delete: %v", err)
	}

	result, err := manager.Execute(models.ExecRequest{
		SessionID: "s1",
		Database:  "main",
		SQL:       "SELECT count(*) AS n FROM orders",
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n, _ := result.Rows[0][0].(int64); n != 2 {
		t.Fatalf("the explained DELETE ran: %d row(s) left", n)
	}
}
