package postgres

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// PostgreSQL's window is not MySQL's with different keywords: the character set
// is an encoding, the sort order is an operating-system locale, and naming
// either of them only works together with TEMPLATE template0. These tests pin
// down the statement and the choices that go with it.

func TestCreateDatabaseNamesTheTemplateOnlyWhenAChoiceWasMade(t *testing.T) {
	cases := []struct {
		name string
		req  models.CreateDatabaseRequest
		want string
	}{
		{
			name: "name only clones template1",
			req:  models.CreateDatabaseRequest{Name: "shop"},
			want: `CREATE DATABASE "shop"`,
		},
		{
			name: "encoding needs template0",
			req:  models.CreateDatabaseRequest{Name: "shop", Charset: "UTF8"},
			want: `CREATE DATABASE "shop" ENCODING 'UTF8' TEMPLATE template0`,
		},
		{
			name: "locale needs template0",
			req:  models.CreateDatabaseRequest{Name: "shop", Collation: "en_US.UTF-8"},
			want: `CREATE DATABASE "shop" LOCALE 'en_US.UTF-8' TEMPLATE template0`,
		},
		{
			name: "both",
			req:  models.CreateDatabaseRequest{Name: "shop", Charset: "UTF8", Collation: "en_US.UTF-8"},
			want: `CREATE DATABASE "shop" ENCODING 'UTF8' LOCALE 'en_US.UTF-8' TEMPLATE template0`,
		},
		{
			name: "a locale with the odd characters locales really have",
			req:  models.CreateDatabaseRequest{Name: "shop", Collation: "de_DE.UTF-8@euro"},
			want: `CREATE DATABASE "shop" LOCALE 'de_DE.UTF-8@euro' TEMPLATE template0`,
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
	plan, err := CreateDatabase(models.CreateDatabaseRequest{Name: `weird"name`})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if plan.Statement != `CREATE DATABASE "weird""name"` {
		t.Fatalf("the name must be quoted: %s", plan.Statement)
	}

	// The encoding and the locale are written as string literals, so a value
	// that would end the literal is refused instead of escaped: no encoding or
	// locale name contains a quote, and a statement this window renders should
	// not need one.
	for _, req := range []models.CreateDatabaseRequest{
		{Name: "shop", Charset: "UTF8'; DROP DATABASE shop; --"},
		{Name: "shop", Collation: "en_US.UTF-8'"},
		{Name: "shop", Collation: "en US"},
		{Name: "shop", Charset: "UTF8 ENCODING"},
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

func TestDatabaseOptionsPreselectTheClustersOwnEncodingAndLocale(t *testing.T) {
	options := databaseOptions("UTF8", "en_US.UTF-8", []string{"en_US.UTF-8", "C", "fr_FR.UTF-8", ""})

	if options.CharsetLabel != "Encoding" || options.CollationLabel != "Locale" {
		t.Fatalf("PostgreSQL calls the two choices encoding and locale: %+v", options)
	}
	if !options.CollationEditable {
		t.Fatal("locale names come from the operating system, so the window must accept a typed one")
	}

	marked := ""
	for _, charset := range options.Charsets {
		if charset.Default {
			marked = charset.Name
		}
	}
	if marked != "UTF8" {
		t.Fatalf("template1's encoding must be the preselected one, got %q", marked)
	}
	if !containsEncoding(pgEncodings, "UTF8") || !containsEncoding(pgEncodings, "utf8") {
		t.Fatal("encoding names are compared the way PostgreSQL compares them")
	}

	// C/POSIX are always available, a locale the cluster uses arrives once, and
	// the one template1 uses leads because it is what the window preselects.
	if options.Collations[0] != "en_US.UTF-8" || options.Collations[1] != "fr_FR.UTF-8" {
		t.Fatalf("the cluster's own locale leads, UTF-8 next: %+v", options.Collations)
	}
	count := map[string]int{}
	for _, locale := range options.Collations {
		count[locale]++
	}
	if count["C"] != 1 || count["POSIX"] != 1 || count["en_US.UTF-8"] != 1 || count[""] != 0 {
		t.Fatalf("locales must be deduplicated and never empty: %+v", options.Collations)
	}

	// A cluster whose template1 is not UTF-8 still gets its own locale first.
	plain := databaseOptions("LATIN1", "C", []string{"de_DE.UTF-8", "C"})
	if plain.Collations[0] != "C" {
		t.Fatalf("the cluster's own locale must lead whatever it is: %+v", plain.Collations)
	}
}

func TestDatabaseOptionsKeepAnEncodingTheCuratedListForgot(t *testing.T) {
	// A cluster whose encoding this table does not know still has to open a
	// window that agrees with the server.
	options := databaseOptions("MULE_INTERNAL_9000", "", nil)
	found := false
	for _, charset := range options.Charsets {
		if charset.Name == "MULE_INTERNAL_9000" && charset.Default {
			found = true
		}
	}
	if !found {
		t.Fatalf("the server's own encoding must be offered: %+v", options.Charsets)
	}
	// And a cluster we could not read answers with the documented list and the
	// always-available locales rather than nothing.
	blind := databaseOptions("", "", nil)
	if len(blind.Charsets) != len(pgEncodings) || len(blind.Collations) != 2 {
		t.Fatalf("a blind cluster still gets the documented choices: %d/%d",
			len(blind.Charsets), len(blind.Collations))
	}
}

// The plan the query window shows comes from an unmeasured EXPLAIN. The
// measured form (EXPLAIN ANALYZE) runs the statement, which the feature
// promises never to do, so the wrapper is asserted here rather than trusted.
func TestExplainCapabilityMatchesTheSpec(t *testing.T) {
	hasWrapper := spec().ExplainSQL != nil
	if info := (Driver{}).Info(); info.SupportsExplain != hasWrapper {
		t.Fatalf("SupportsExplain = %v but the spec's wrapper is %v", info.SupportsExplain, hasWrapper)
	}
	got := spec().ExplainSQL("SELECT 1")
	if got != "EXPLAIN SELECT 1" {
		t.Fatalf("ExplainSQL = %q", got)
	}
	if strings.Contains(strings.ToUpper(got), "ANALYZE") {
		t.Fatalf("the measured form runs the statement: %q", got)
	}
}
