package sqlutil

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSplitStatementsSeparatesOnSemicolons(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   []string
	}{
		{"two statements", "SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"tail without a semicolon", "SELECT 1;\nSELECT 2", []string{"SELECT 1", "SELECT 2"}},
		{"whitespace only fragments are dropped", " ; \n ;\t; ", nil},
		{"comment only fragments are dropped", "-- nothing\n; /* here */ ;", nil},
		{"empty script", "", nil},
		{"comment before a statement stays with it", "-- why\nSELECT 1;", []string{"-- why\nSELECT 1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitStatements(tc.script)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d statements %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("statement %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSplitStatementsRespectsLiteralsAndComments(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   int
	}{
		{"semicolon in a string", "INSERT INTO t VALUES ('a;b');", 1},
		{"doubled quote in a string", "SELECT 'it''s; fine';", 1},
		{"backslash escaped quote", "SELECT 'a\\';b';", 1},
		{"semicolon in a quoted identifier", "SELECT `a;b` FROM `t;u`;", 1},
		{"semicolon in a double quoted identifier", `SELECT "a;b" FROM t;`, 1},
		{"doubled backtick", "SELECT `a``b;c`;", 1},
		{"semicolon in a line comment", "SELECT 1 -- ;\n;", 1},
		{"semicolon in a block comment", "/* a; b */ SELECT 1;", 1},
		{"unterminated string swallows the rest", "SELECT 'a;b", 1},
		{"unterminated block comment swallows the rest", "/* a;b", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(SplitStatements(tc.script)); got != tc.want {
				t.Fatalf("got %d statements, want %d", got, tc.want)
			}
		})
	}
}

func TestSplitStatementsFollowsDelimiterDirective(t *testing.T) {
	// A mysqldump body: DELIMITER is a directive of the mysql command line
	// client, so it selects the separator instead of being a statement itself.
	script := "-- dump\n" +
		"DELIMITER ;;\n" +
		"CREATE TRIGGER `t` BEFORE INSERT ON `x` FOR EACH ROW BEGIN\n" +
		"  SET NEW.a = 1;\n" +
		"  SET NEW.b = 2;\n" +
		"END ;;\n" +
		"DELIMITER ;\n" +
		"INSERT INTO `x` VALUES (1);\n"

	got := SplitStatements(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements %q, want 2", len(got), got)
	}
	trigger := got[0]
	if !strings.Contains(trigger, "CREATE TRIGGER") || !strings.HasSuffix(trigger, "END") {
		t.Fatalf("trigger body was cut short: %q", trigger)
	}
	if !strings.Contains(trigger, "SET NEW.a = 1;\n  SET NEW.b = 2;") {
		t.Fatalf("trigger body lost its semicolons: %q", trigger)
	}
	if got[1] != "INSERT INTO `x` VALUES (1)" {
		t.Fatalf("the script continued with the standard separator: %q", got[1])
	}
}

func TestSplitStatementsDelimiterDoesNotEatTheRestOfTheLine(t *testing.T) {
	// The separator is the word after DELIMITER; whatever else stands on that
	// line is still part of the script.
	script := "DELIMITER //\n" +
		"CREATE PROCEDURE p() BEGIN SELECT 1; END//\n" +
		"DELIMITER ; SELECT 2;\n"

	got := SplitStatements(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements %q, want 2", len(got), got)
	}
	if got[0] != "CREATE PROCEDURE p() BEGIN SELECT 1; END" {
		t.Fatalf("procedure body was cut short: %q", got[0])
	}
	if got[1] != "SELECT 2" {
		t.Fatalf("the statement after the reset was lost: %q", got[1])
	}
}

func TestSplitStatementsKeepsDollarQuotedBodies(t *testing.T) {
	script := "CREATE FUNCTION f() RETURNS int AS $$\n" +
		"BEGIN\n" +
		"  RETURN 1;\n" +
		"END;\n" +
		"$$ LANGUAGE plpgsql;\n" +
		"SELECT $1;\n"

	got := SplitStatements(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements %q, want 2", len(got), got)
	}
	if !strings.HasSuffix(got[0], "$$ LANGUAGE plpgsql") {
		t.Fatalf("the function body was cut at its first semicolon: %q", got[0])
	}
	if got[1] != "SELECT $1" {
		t.Fatalf("a parameter was taken for a dollar quote: %q", got[1])
	}

	tagged := "CREATE FUNCTION g() RETURNS int AS $body$ SELECT 1; $body$ LANGUAGE sql;"
	if got := SplitStatements(tagged); len(got) != 1 {
		t.Fatalf("a tagged dollar quote was split: %q", got)
	}
}

func TestScanStatementsReportsTheBytesItRead(t *testing.T) {
	script := "SELECT 1;\nSELECT 2;\n"
	var (
		statements []string
		consumed   []int64
	)
	err := ScanStatements(strings.NewReader(script), func(statement string, at int64) error {
		statements = append(statements, statement)
		consumed = append(consumed, at)
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(statements))
	}
	// The offset is past the separator that ended the statement, and the last
	// statement ends at the end of the file.
	if consumed[0] != int64(len("SELECT 1;")) {
		t.Fatalf("first statement reported %d bytes, want %d", consumed[0], len("SELECT 1;"))
	}
	if want := int64(strings.LastIndex(script, ";") + 1); consumed[1] != want {
		t.Fatalf("last statement reported %d bytes, want %d", consumed[1], want)
	}
}

func TestScanStatementsAgreesWithSplitStatements(t *testing.T) {
	script := "-- header\n" +
		"DELIMITER ;;\n" +
		"CREATE TRIGGER t BEFORE INSERT ON x FOR EACH ROW BEGIN SET NEW.a = 1; END ;;\n" +
		"DELIMITER ;\n" +
		"INSERT INTO t VALUES ('a;b', $$c;d$$);\n" +
		"/* trailing */\n"

	var streamed []string
	if err := ScanStatements(strings.NewReader(script), func(statement string, _ int64) error {
		streamed = append(streamed, statement)
		return nil
	}); err != nil {
		t.Fatalf("scan: %v", err)
	}

	whole := SplitStatements(script)
	if len(streamed) != len(whole) {
		t.Fatalf("streaming gave %d statements, the string form gave %d", len(streamed), len(whole))
	}
	for i := range whole {
		if streamed[i] != whole[i] {
			t.Fatalf("statement %d differs:\nstream %q\nstring %q", i, streamed[i], whole[i])
		}
	}
}

func TestScanStatementsHandlesAStatementBiggerThanTheReadBuffer(t *testing.T) {
	body := strings.Repeat("a", 300<<10)
	script := "INSERT INTO t VALUES ('" + body + "');\nSELECT 1;\n"

	var statements []string
	if err := ScanStatements(strings.NewReader(script), func(statement string, _ int64) error {
		statements = append(statements, statement)
		return nil
	}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(statements))
	}
	if !strings.Contains(statements[0], body) {
		t.Fatalf("the long statement was truncated: %d bytes", len(statements[0]))
	}
}

func TestScanStatementsStopsAtTheFirstErrorFromTheCallback(t *testing.T) {
	stop := errors.New("stop")
	seen := 0
	err := ScanStatements(strings.NewReader("SELECT 1; SELECT 2; SELECT 3;"), func(string, int64) error {
		seen++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("got %v, want the callback error", err)
	}
	if seen != 1 {
		t.Fatalf("the scan continued after an error: %d statements", seen)
	}
}

func TestScanStatementsReportsAReadError(t *testing.T) {
	fail := errors.New("disk on fire")
	reader := io.MultiReader(strings.NewReader("SELECT 1; SELECT "), &failingReader{err: fail})

	var statements []string
	err := ScanStatements(reader, func(statement string, _ int64) error {
		statements = append(statements, statement)
		return nil
	})
	if !errors.Is(err, fail) {
		t.Fatalf("got %v, want the read error", err)
	}
	// The statements read before the failure are still handed over.
	if len(statements) != 1 || statements[0] != "SELECT 1" {
		t.Fatalf("got %q, want the statement that was read before the failure", statements)
	}
}

func TestInspectAgreesWithAnalyze(t *testing.T) {
	script := "DROP TABLE t;\nSELECT 1;\nUPDATE t SET a = 1;\n"

	analysis := Analyze(script, false)
	statements := SplitStatements(script)
	if len(analysis.Statements) != len(statements) {
		t.Fatalf("analysis has %d statements, the split has %d", len(analysis.Statements), len(statements))
	}
	for i, statement := range statements {
		if got, want := Inspect(statement, i), analysis.Statements[i]; got != want {
			t.Fatalf("statement %d: Inspect gave %+v, Analyze gave %+v", i, got, want)
		}
	}
}

func TestInspectClassifiesOneStatement(t *testing.T) {
	drop := Inspect("DROP TABLE t", 7)
	if drop.Index != 7 || drop.Kind != KindDDL || !drop.Destructive || drop.Reason == "" {
		t.Fatalf("DROP TABLE came back as %+v", drop)
	}
	select1 := Inspect("SELECT 1", 0)
	if select1.Kind != KindQuery || select1.Destructive {
		t.Fatalf("SELECT came back as %+v", select1)
	}
	if got := Inspect("  SELECT 1  ", 0).SQL; got != "SELECT 1" {
		t.Fatalf("Inspect kept the padding: %q", got)
	}
}

// failingReader is a reader that always fails, to prove a read error is
// reported rather than taken for the end of the script.
type failingReader struct{ err error }

func (r *failingReader) Read([]byte) (int, error) { return 0, r.err }
