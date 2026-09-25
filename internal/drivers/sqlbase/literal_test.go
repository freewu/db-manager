package sqlbase

import (
	"testing"

	"dbmanager/internal/models"
)

// The preview is read by a person and copied by hand, so what it spells has to be
// what the engine would read: a string is quoted, a quote inside it is doubled,
// and NULL is a word rather than an empty pair of quotes.

func TestSQLLiteralSpellsEachKindOfValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"null", nil, "NULL"},
		{"true", true, "TRUE"},
		{"false", false, "FALSE"},
		{"int", 42, "42"},
		{"negative", int64(-7), "-7"},
		{"float", 1.5, "1.5"},
		{"text", "paid", "'paid'"},
		{"quote inside", "O'Brien", "'O''Brien'"},
		{"empty text", "", "''"},
	}
	for _, c := range cases {
		if got := sqlLiteral(PostgresDialect{}, c.value); got != c.want {
			t.Errorf("%s: sqlLiteral(%#v) = %s, want %s", c.name, c.value, got, c.want)
		}
	}
}

// A backslash is an escape character in MySQL's string literals and an ordinary
// character in PostgreSQL's, so the same value has two honest spellings. The
// dialect answers this optionally, because a document store has no SQL at all.
func TestSQLLiteralDoublesABackslashOnlyWhereItIsAnEscape(t *testing.T) {
	const path = `C:\tmp\a`
	if got, want := sqlLiteral(MySQLDialect{}, path), `'C:\\tmp\\a'`; got != want {
		t.Errorf("MySQL: %s, want %s", got, want)
	}
	if got, want := sqlLiteral(PostgresDialect{}, path), `'C:\tmp\a'`; got != want {
		t.Errorf("Postgres: %s, want %s", got, want)
	}
}

// Literal is what the export window writes into a file, so it has to be the same
// spelling the preview uses — dialect and all — rather than the simpler,
// dialect-unaware QuoteLiteral in scan.go.
func TestLiteralIsTheDialectAwareSpelling(t *testing.T) {
	const path = `C:\tmp\a`
	if got, want := Literal(MySQLDialect{}, path), sqlLiteral(MySQLDialect{}, path); got != want {
		t.Errorf("Literal = %s, sqlLiteral = %s", got, want)
	}
	if got, want := Literal(PostgresDialect{}, nil), "NULL"; got != want {
		t.Errorf("Literal(nil) = %s, want %s", got, want)
	}
}

func TestLiteralWhereRendersARowIdentity(t *testing.T) {
	key := []models.KeyValue{
		{Column: "id", Value: 1},
		{Column: "tenant", Value: nil},
		{Column: "code", Value: "a'b"},
	}
	got, err := literalWhere(SQLiteDialect{}, key)
	if err != nil {
		t.Fatalf("literalWhere: %v", err)
	}
	want := ` WHERE "id" = 1 AND "tenant" IS NULL AND "code" = 'a''b'`
	if got != want {
		t.Fatalf("literalWhere = %q, want %q", got, want)
	}
}

// The preview refuses exactly what the run refuses, so a request that could never
// be applied does not get a statement that looks as if it could.
func TestLiteralWhereRefusesAnIdentityItCannotWrite(t *testing.T) {
	if _, err := literalWhere(SQLiteDialect{}, nil); err != errNoKey {
		t.Fatalf("an empty key must be refused with errNoKey, got %v", err)
	}
	blank := []models.KeyValue{{Column: "  ", Value: 1}}
	if _, err := literalWhere(SQLiteDialect{}, blank); err != errEmptyKeyColumn {
		t.Fatalf("a blank key column must be refused with errEmptyKeyColumn, got %v", err)
	}
}
