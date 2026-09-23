package sqlutil

import (
	"strings"
	"testing"
)

func TestAnalyzeClassifiesStatements(t *testing.T) {
	script := `
		-- a healthy script
		SELECT * FROM orders;
		CREATE INDEX idx_orders_total ON orders (total);
		UPDATE orders SET total = 0 WHERE id = 1;
		INSERT INTO orders (id) VALUES (1);
		EXPLAIN SELECT 1;
	`
	analysis := Analyze(script, false)

	if len(analysis.Statements) != 5 {
		t.Fatalf("expected 5 statements, got %d", len(analysis.Statements))
	}
	want := []string{KindQuery, KindDDL, KindDML, KindDML, KindQuery}
	for i, kind := range want {
		if analysis.Statements[i].Kind != kind {
			t.Fatalf("statement %d: expected %s, got %s", i+1, kind, analysis.Statements[i].Kind)
		}
		if analysis.Statements[i].Index != i {
			t.Fatalf("statement %d: index is %d", i+1, analysis.Statements[i].Index)
		}
	}
	if analysis.Destructive {
		t.Fatalf("nothing here is destructive, got warnings %v", analysis.Warnings)
	}
	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", analysis.Warnings)
	}
	if analysis.Refused != 0 {
		t.Fatalf("read-write session must not refuse anything, got %d", analysis.Refused)
	}
}

func TestAnalyzeFlagsDestructiveStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want bool
	}{
		{"drop table", "DROP TABLE orders;", true},
		{"drop column", "ALTER TABLE orders DROP COLUMN note;", true},
		{"truncate", "TRUNCATE TABLE orders;", true},
		{"delete with where", "DELETE FROM orders WHERE id = 1;", false},
		{"delete without where", "DELETE FROM orders;", true},
		{"update with where", "UPDATE orders SET a = 1 WHERE id = 2;", false},
		{"update without where", "UPDATE orders SET a = 1", true},
		{"create table", "CREATE TABLE t (id int);", false},
		{"add column", "ALTER TABLE t ADD COLUMN c int;", false},
		// A comment must not be mistaken for a statement's own keywords.
		{"comment mentions drop", "/* drop table t */ SELECT 1;", false},
		// "where" inside a string literal still counts as a WHERE for a human
		// reader, so this one is intentionally not flagged.
		{"crlf delete without where", "DELETE FROM t;\r\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			analysis := Analyze(tc.sql, false)
			if analysis.Destructive != tc.want {
				t.Fatalf("destructive=%v (wanted %v), warnings=%v", analysis.Destructive, tc.want, analysis.Warnings)
			}
			if tc.want && len(analysis.Warnings) == 0 {
				t.Fatal("a destructive statement must come with an explanation")
			}
		})
	}
}

func TestAnalyzeSeparatesDangerousDropNamespace(t *testing.T) {
	analysis := Analyze("DROP DATABASE shop;", false)
	if !analysis.Destructive {
		t.Fatal("DROP DATABASE must be destructive")
	}
	if len(analysis.Warnings) == 0 || !strings.Contains(analysis.Warnings[0], "DROP DATABASE") {
		t.Fatalf("expected a namespace warning, got %v", analysis.Warnings)
	}
}

func TestAnalyzeCountsWhatAReadOnlySessionRefuses(t *testing.T) {
	analysis := Analyze("SELECT 1; INSERT INTO t VALUES (1); UPDATE t SET a = 1 WHERE id = 1;", true)

	if !analysis.ReadOnly {
		t.Fatal("the analysis must carry the session's read-only flag")
	}
	if analysis.Refused != 2 {
		t.Fatalf("expected 2 refused statements, got %d", analysis.Refused)
	}
	if len(analysis.Warnings) == 0 || !strings.Contains(analysis.Warnings[0], "read-only") {
		t.Fatalf("expected a read-only warning first, got %v", analysis.Warnings)
	}
}

func TestAnalyzeWarnsAboutUnknownStatements(t *testing.T) {
	analysis := Analyze("WIBBLE wobble;", false)
	if len(analysis.Statements) != 1 || analysis.Statements[0].Kind != KindUnknown {
		t.Fatalf("expected one unknown statement, got %+v", analysis.Statements)
	}
	if len(analysis.Warnings) != 1 || !strings.Contains(analysis.Warnings[0], "unrecognised") {
		t.Fatalf("expected an unrecognised warning, got %v", analysis.Warnings)
	}
}

func TestAnalyzePreviewIsSingleLineAndCommentFree(t *testing.T) {
	analysis := Analyze("SELECT a,\n  -- explain\n  b FROM t;", false)
	if len(analysis.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(analysis.Statements))
	}
	preview := analysis.Statements[0].Preview
	if strings.Contains(preview, "\n") || strings.Contains(preview, "--") {
		t.Fatalf("preview must be one comment-free line, got %q", preview)
	}
	if !strings.HasPrefix(preview, "SELECT a, b") {
		t.Fatalf("unexpected preview %q", preview)
	}
}

// The change log names a statement by its own first keyword, and decides
// whether it created a table from the same reading of the text. Both have to
// survive comments, casing and the words some engines put in between.
func TestCreatesTableReadsTheStatementRatherThanGuessing(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want bool
	}{
		{"plain", "CREATE TABLE orders (id INT);", true},
		{"already there", "CREATE TABLE IF NOT EXISTS orders (id INT);", true},
		{"temporary", "CREATE TEMPORARY TABLE draft (id INT);", true},
		{"unlogged", "CREATE UNLOGGED TABLE fast (id INT);", true},
		{"lowercase", "create table t (id int)", true},
		{"after a comment", "-- build it\nCREATE TABLE t (id INT);", true},
		{"index", "CREATE INDEX idx ON orders (id);", false},
		{"unique index", "CREATE UNIQUE INDEX idx ON orders (id);", false},
		{"view", "CREATE OR REPLACE VIEW v AS SELECT 1;", false},
		{"database", "CREATE DATABASE shop;", false},
		{"alter", "ALTER TABLE orders ADD note TEXT;", false},
		{"a comment about a table", "/* create table t */ SELECT 1;", false},
		{"a column named table", "INSERT INTO t (table) VALUES ('x');", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CreatesTable(tc.sql); got != tc.want {
				t.Fatalf("CreatesTable(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

func TestLeadingWordAndKindOfAgreeWithAnalyze(t *testing.T) {
	kindCases := map[string]string{
		"SELECT 1":                          KindQuery,
		"DROP TABLE t;":                     KindDDL,
		"update t set a = 1;":               KindDML,
		"WIBBLE wobble;":                    KindUnknown,
		"-- only a comment\n":               KindUnknown,
		"/* c */ INSERT INTO t VALUES (1);": KindDML,
	}
	for sql, want := range kindCases {
		if got := KindOf(sql); got != want {
			t.Errorf("KindOf(%q) = %s, want %s", sql, got, want)
		}
	}

	wordCases := map[string]string{
		"CREATE TABLE t (id INT);": "create",
		"  Alter Table t;":         "alter",
		"-- why\nTRUNCATE t;":      "truncate",
		"SELECT 1":                 "select",
	}
	for sql, want := range wordCases {
		if got := LeadingWord(sql); got != want {
			t.Errorf("LeadingWord(%q) = %q, want %q", sql, got, want)
		}
	}
}

func TestAnalyzeIgnoresEmptyScript(t *testing.T) {
	analysis := Analyze("-- nothing but a comment\n", false)
	if len(analysis.Statements) != 0 {
		t.Fatalf("expected no statements, got %+v", analysis.Statements)
	}
	if analysis.Destructive {
		t.Fatal("an empty script cannot be destructive")
	}
	// The slices must be non-nil so the JSON shape is stable for the UI.
	if analysis.Statements == nil || analysis.Warnings == nil {
		t.Fatal("statements and warnings must serialise as arrays, not null")
	}
}
