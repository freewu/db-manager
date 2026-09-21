package mysqlcompat

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// The window's choices come from two SHOW statements, so the reading of those
// rows is what these tests pin down: MySQL, MariaDB, TiDB and Doris all answer
// with the same column names but never with the same values.

func charsetRows(pairs ...string) []ShowRow {
	rows := make([]ShowRow, 0, len(pairs)/3)
	for i := 0; i+2 < len(pairs); i += 3 {
		rows = append(rows, ShowRow{
			"charset":           pairs[i],
			"default collation": pairs[i+1],
			"maxlen":            pairs[i+2],
		})
	}
	return rows
}

func collationRows(triples ...string) []ShowRow {
	rows := make([]ShowRow, 0, len(triples)/3)
	for i := 0; i+2 < len(triples); i += 3 {
		rows = append(rows, ShowRow{
			"collation": triples[i],
			"charset":   triples[i+1],
			"default":   triples[i+2],
		})
	}
	return rows
}

func TestCharsetsFromShowAttachTheirCollationsAndLeadWithTheServerDefault(t *testing.T) {
	charsets := charsetsFromShow(
		charsetRows(
			"utf8mb4", "utf8mb4_0900_ai_ci", "4",
			"latin1", "latin1_swedish_ci", "1",
			"utf8mb3", "utf8mb3_general_ci", "3",
		),
		collationsFromShow(collationRows(
			"utf8mb4_unicode_ci", "utf8mb4", "",
			"utf8mb4_0900_ai_ci", "utf8mb4", "Yes",
			"latin1_swedish_ci", "latin1", "Yes",
		)),
		"latin1",
	)

	if len(charsets) != 3 {
		t.Fatalf("expected 3 charsets, got %+v", charsets)
	}
	// The server's own default leads, because that is what the window selects.
	if charsets[0].Name != "latin1" || !charsets[0].Default {
		t.Fatalf("the server's default must lead: %+v", charsets[0])
	}
	if charsets[0].Collation != "latin1_swedish_ci" {
		t.Fatalf("the default collation is read from its own column: %+v", charsets[0])
	}

	byName := map[string]models.DatabaseCharset{}
	for _, charset := range charsets {
		byName[charset.Name] = charset
	}
	// utf8mb4's collations are listed with the charset's own default first,
	// which is what SHOW COLLATION's Default column is for.
	if got := byName["utf8mb4"].Collations; len(got) != 2 || got[0] != "utf8mb4_0900_ai_ci" {
		t.Fatalf("collations of utf8mb4: %+v", got)
	}
	// A charset no collation row mentioned still arrives, with an empty list
	// rather than a nil one (the frontend renders the list directly).
	if got := byName["utf8mb3"].Collations; got == nil || len(got) != 0 {
		t.Fatalf("collations of utf8mb3: %#v", got)
	}
}

func TestCollationsFromShowSkipsRowsWithoutACharset(t *testing.T) {
	// A server that answers SHOW COLLATION with an unexpected column set must
	// not turn into a list of empty strings.
	if got := collationsFromShow([]ShowRow{{"collation": "utf8mb4_bin"}}); len(got) != 0 {
		t.Fatalf("a row without a charset adds nothing: %+v", got)
	}
}

func TestCreateDatabaseWritesOnlyTheClausesThatWereAskedFor(t *testing.T) {
	cases := []struct {
		name string
		req  models.CreateDatabaseRequest
		want string
	}{
		{
			name: "name only",
			req:  models.CreateDatabaseRequest{Name: "shop"},
			want: "CREATE DATABASE `shop`",
		},
		{
			name: "charset",
			req:  models.CreateDatabaseRequest{Name: "shop", Charset: "utf8mb4"},
			want: "CREATE DATABASE `shop` DEFAULT CHARACTER SET utf8mb4",
		},
		{
			name: "charset and collation",
			req:  models.CreateDatabaseRequest{Name: "shop", Charset: "utf8mb4", Collation: "utf8mb4_bin"},
			want: "CREATE DATABASE `shop` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin",
		},
		{
			name: "surrounding space is not part of the value",
			req:  models.CreateDatabaseRequest{Name: "shop", Charset: " latin1 ", Collation: " latin1_bin "},
			want: "CREATE DATABASE `shop` DEFAULT CHARACTER SET latin1 COLLATE latin1_bin",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := CreateDatabase(tc.req)
			if err != nil {
				t.Fatalf("create database: %v", err)
			}
			if plan.Statement != tc.want {
				t.Fatalf("statement:\n got %s\nwant %s", plan.Statement, tc.want)
			}
		})
	}
}

func TestCreateDatabaseQuotesTheNameAndRefusesBogusOptions(t *testing.T) {
	// The name is quoted rather than checked: `weird``name` is a legal (if
	// unwise) MySQL identifier, and backticks are what keeps it one.
	plan, err := CreateDatabase(models.CreateDatabaseRequest{Name: "weird`name"})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if plan.Statement != "CREATE DATABASE `weird``name`" {
		t.Fatalf("the name must be quoted: %s", plan.Statement)
	}

	// The two options are the only unquoted values in the statement, so they
	// are the ones that may not carry anything else.
	for _, req := range []models.CreateDatabaseRequest{
		{Name: "shop", Charset: "utf8mb4; DROP DATABASE shop"},
		{Name: "shop", Charset: "utf8mb4'"},
		{Name: "shop", Collation: "utf8mb4_bin COLLATE x"},
		{Name: "shop", Collation: "'"},
	} {
		got, err := CreateDatabase(req)
		if err == nil {
			t.Fatalf("expected %+v to be refused, got %q", req, got.Statement)
		}
		if !strings.Contains(err.Error(), "not a") {
			t.Fatalf("the error should say what is wrong: %v", err)
		}
	}
}
