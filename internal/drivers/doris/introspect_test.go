package doris

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqltest"
	"dbmanager/internal/models"
)

// Doris keeps its catalog behind SHOW statements, so these tests stand a fake
// server behind the connection and check what the introspector makes of the
// rows — including the cases where Doris answers differently than MySQL would.

var objectSelectMatch = "SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_COMMENT, '')"

func showDatabases(rows ...[]any) sqltest.Expectation {
	return sqltest.Expectation{Match: "SHOW DATABASES", Columns: []string{"Database"}, Rows: rows}
}

func TestDatabasesHidesTheSchemasDorisShipsWith(t *testing.T) {
	handle := sqltest.New(t, showDatabases(
		[]any{"information_schema"},
		[]any{"demo"},
		[]any{"__internal_schema"},
		[]any{"mysql"},
	))
	got, err := newIntrospector().Databases(context.Background(), handle.DB)
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if len(got) != 1 || got[0] != "demo" {
		t.Fatalf("Databases() = %v, want [demo]", got)
	}
}

// Doris appends columns to SHOW DATABASES on some versions; the name stays
// first.
func TestDatabasesReadsTheFirstColumn(t *testing.T) {
	handle := sqltest.New(t, sqltest.Expectation{
		Match:   "SHOW DATABASES",
		Columns: []string{"Database", "CreateTime"},
		Rows:    [][]any{{"demo", "2026-01-01"}, {"  ", "2026-01-02"}},
	})
	got, err := newIntrospector().Databases(context.Background(), handle.DB)
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if len(got) != 1 || got[0] != "demo" {
		t.Fatalf("Databases() = %v, want [demo]", got)
	}
}

func TestObjectsListsTablesAndViewsAndEnrichesThem(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW FULL TABLES FROM `demo`",
			Columns: []string{"Tables_in_demo", "Table_type"},
			Rows: [][]any{
				{"orders", "BASE TABLE"},
				{"revenue", "VIEW"},
			},
		},
		sqltest.Expectation{
			Match:   objectSelectMatch,
			Columns: []string{"TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT", "TABLE_ROWS", "SIZE", "ENGINE"},
			Rows:    [][]any{{"orders", "BASE TABLE", "one row per order", 42, 8192, "OLAP"}},
		},
	)

	got, err := newIntrospector().Objects(context.Background(), handle.DB, "demo", "")
	if err != nil {
		t.Fatalf("Objects() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Objects() returned %d objects, want 2: %+v", len(got), got)
	}
	if got[0].Kind != models.KindTable || got[1].Kind != models.KindView {
		t.Fatalf("Objects() kinds = %q, %q", got[0].Kind, got[1].Kind)
	}
	if got[0].Comment != "one row per order" || got[0].RowEstimate != 42 || got[0].SizeBytes != 8192 {
		t.Errorf("Objects() did not enrich the table: %+v", got[0])
	}
	if got[1].Comment != "" || got[1].RowEstimate != 0 {
		t.Errorf("Objects() invented enrichment for the view: %+v", got[1])
	}
}

// Losing information_schema must not lose the listing: Doris versions disagree
// about which columns it has, so enrichment is best-effort.
func TestObjectsSurviveAMissingInformationSchema(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW FULL TABLES FROM `demo`",
			Columns: []string{"Tables_in_demo", "Table_type"},
			Rows:    [][]any{{"orders", "BASE TABLE"}},
		},
		sqltest.Expectation{Match: objectSelectMatch, Err: errors.New("Unknown table 'TABLES'")},
	)

	got, err := newIntrospector().Objects(context.Background(), handle.DB, "demo", "")
	if err != nil {
		t.Fatalf("Objects() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "orders" {
		t.Fatalf("Objects() = %+v, want the table without its details", got)
	}
}

func TestObjectFallsBackToShowFullTables(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{Match: objectSelectMatch, Err: errors.New("Unknown table 'TABLES'")},
		sqltest.Expectation{
			Match:   "SHOW FULL TABLES FROM `demo` LIKE ?",
			Columns: []string{"Tables_in_demo", "Table_type"},
			Rows:    [][]any{{"orders", "BASE TABLE"}},
		},
	)

	got, err := newIntrospector().Object(context.Background(), handle.DB, "demo", "", "orders")
	if err != nil {
		t.Fatalf("Object() error = %v", err)
	}
	if got.Name != "orders" || got.Kind != models.KindTable || got.Database != "demo" {
		t.Fatalf("Object() = %+v", got)
	}
}

func TestObjectReportsAMissingTable(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   objectSelectMatch,
			Columns: []string{"TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT", "TABLE_ROWS", "SIZE", "ENGINE"},
			Rows:    [][]any{},
		},
		sqltest.Expectation{
			Match:   "SHOW FULL TABLES FROM `demo` LIKE ?",
			Columns: []string{"Tables_in_demo", "Table_type"},
			Rows:    [][]any{},
		},
	)

	_, err := newIntrospector().Object(context.Background(), handle.DB, "demo", "", "nope")
	if err == nil {
		t.Fatal("Object() found a table that is not there")
	}
	if !apperr.Is(err, apperr.CodeNotFound) {
		t.Fatalf("Object() error = %v, want a %q error", err, apperr.CodeNotFound)
	}
}

func TestColumnsReadsShowFullColumns(t *testing.T) {
	handle := sqltest.New(t, sqltest.Expectation{
		Match: "SHOW FULL COLUMNS FROM `demo`.`orders`",
		Columns: []string{
			"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment",
		},
		Rows: [][]any{
			{"id", "bigint(20)", nil, "No", "true", nil, "", "", "primary key"},
			{"title", "varchar(120)", "utf8mb4_general_ci", "Yes", "false", nil, "", "", ""},
			{"amount", "decimal(10,2)", nil, "Yes", "false", "0.00", "", "", "in yuan"},
			{"seq", "int(11)", nil, "No", "false", nil, "auto_increment", "", ""},
		},
	})

	got, err := newIntrospector().Columns(context.Background(), handle.DB, "demo", "", "orders")
	if err != nil {
		t.Fatalf("Columns() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Columns() returned %d columns, want 4", len(got))
	}

	id := got[0]
	if id.Ordinal != 1 || id.DataType != "bigint" || id.ColumnType != "bigint(20)" {
		t.Errorf("id = %+v, want bigint with the raw type kept", id)
	}
	if !id.PrimaryKey || id.Nullable || id.DefaultValue != nil || id.Comment != "primary key" {
		t.Errorf("id = %+v, want a non-null primary key without a default", id)
	}

	title := got[1]
	if !title.Nullable || title.CharMaxLength == nil || *title.CharMaxLength != 120 {
		t.Errorf("title = %+v, want a nullable varchar(120)", title)
	}
	if title.NumericPrecision != nil {
		t.Errorf("title = %+v, want no numeric precision for a varchar", title)
	}

	amount := got[2]
	if amount.DefaultValue == nil || *amount.DefaultValue != "0.00" {
		t.Errorf("amount = %+v, want the default Doris reported", amount)
	}
	if amount.NumericPrecision == nil || *amount.NumericPrecision != 10 || amount.NumericScale == nil || *amount.NumericScale != 2 {
		t.Errorf("amount = %+v, want decimal(10,2)", amount)
	}

	if !got[3].AutoIncrement {
		t.Errorf("seq = %+v, want AUTO_INCREMENT detected", got[3])
	}
}

func TestIndexesGroupsShowIndexRows(t *testing.T) {
	handle := sqltest.New(t, sqltest.Expectation{
		Match: "SHOW INDEX FROM `demo`.`orders`",
		Columns: []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
			"Collation", "Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment",
		},
		// SHOW INDEX is not guaranteed to be ordered; the second part of the
		// primary key arrives first here on purpose.
		Rows: [][]any{
			{"orders", "0", "PRIMARY", "2", "created_at", "A", "0", nil, nil, "", "BTREE", ""},
			{"orders", "0", "PRIMARY", "1", "id", "A", "0", nil, nil, "", "BTREE", ""},
			{"orders", "1", "idx_title", "1", "title", "A", "0", nil, nil, "YES", "BTREE", "lookups"},
		},
	})

	got, err := newIntrospector().Indexes(context.Background(), handle.DB, "demo", "", "orders")
	if err != nil {
		t.Fatalf("Indexes() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Indexes() returned %d indexes, want 2: %+v", len(got), got)
	}
	if got[0].Name != "PRIMARY" || !got[0].Primary || !got[0].Unique {
		t.Fatalf("got[0] = %+v, want the unique primary key", got[0])
	}
	if strings.Join(got[0].Columns, ",") != "id,created_at" {
		t.Errorf("primary key columns = %v, want them in sequence order", got[0].Columns)
	}
	if got[1].Unique || got[1].Primary || got[1].Comment != "lookups" {
		t.Errorf("got[1] = %+v, want a non-unique secondary index", got[1])
	}
}

// The index page walks the tables, and internal tables reject SHOW INDEX. One
// of them must not empty the page.
func TestNamespaceIndexesSkipsTablesThatRefuse(t *testing.T) {
	handle := sqltest.New(t,
		sqltest.Expectation{
			Match:   "SHOW FULL TABLES FROM `demo`",
			Columns: []string{"Tables_in_demo", "Table_type"},
			Rows: [][]any{
				{"__internal", "BASE TABLE"},
				{"orders", "BASE TABLE"},
			},
		},
		sqltest.Expectation{Match: objectSelectMatch, Err: errors.New("Unknown table 'TABLES'")},
		sqltest.Expectation{Match: "SHOW INDEX FROM `demo`.`__internal`", Err: errors.New("not supported")},
		sqltest.Expectation{
			Match: "SHOW INDEX FROM `demo`.`orders`",
			Columns: []string{
				"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name",
				"Collation", "Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment",
			},
			Rows: [][]any{{"orders", "0", "PRIMARY", "1", "id", "A", "0", nil, nil, "", "BTREE", ""}},
		},
	)

	got, err := newIntrospector().NamespaceIndexes(context.Background(), handle.DB, "demo", "")
	if err != nil {
		t.Fatalf("NamespaceIndexes() error = %v", err)
	}
	if len(got) != 1 || got[0].Table != "orders" || got[0].Database != "demo" {
		t.Fatalf("NamespaceIndexes() = %+v, want the index of orders", got)
	}
}

func TestForeignKeysIsEmptyRatherThanFailing(t *testing.T) {
	handle := sqltest.New(t)
	got, err := newIntrospector().ForeignKeys(context.Background(), handle.DB, "demo", "", "orders")
	if err != nil {
		t.Fatalf("ForeignKeys() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ForeignKeys() = %+v, want none — Doris has no foreign keys", got)
	}
	if calls := handle.Calls(); len(calls) != 0 {
		t.Errorf("ForeignKeys() ran %v, want no query at all", calls)
	}
}
