package sqlbase

import (
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// The comparison between two tables is deliberately a small vocabulary: fields,
// their types and flags, and the indexes that are not the primary key. These
// tests pin that vocabulary down — what counts as a difference, what does not,
// and what a one-sided table looks like.

// columnItem pulls one field's entry out of a comparison, so a test can assert
// about that field instead of about the shape of the list it sits in.
func columnItem(t *testing.T, diff models.TableDiff, name string) models.DiffItem {
	t.Helper()
	for _, item := range diff.Columns {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("no field %s in %+v", name, diff.Columns)
	return models.DiffItem{}
}

func indexItem(t *testing.T, diff models.TableDiff, name string) models.DiffItem {
	t.Helper()
	for _, item := range diff.Indexes {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("no index %s in %+v", name, diff.Indexes)
	return models.DiffItem{}
}

// fieldNames lists the attributes an item says differ, in order.
func fieldNames(item models.DiffItem) []string {
	names := []string{}
	for _, field := range item.Fields {
		names = append(names, field.Field)
	}
	return names
}

func TestDiffTableFindsNothingInATableThatDidNotChange(t *testing.T) {
	left := serialStructure()
	right := serialStructure()

	diff := DiffTable(left, right)
	if diff.Status != models.DiffSame {
		t.Fatalf("two readings of the same table are the same table: %+v", diff)
	}
	for _, item := range append(append([]models.DiffItem{}, diff.Columns...), diff.Indexes...) {
		if item.Status != models.DiffSame {
			t.Fatalf("nothing differs, so %s should be the same: %+v", item.Name, item)
		}
	}
	if diff.Summary != "the same on both sides" {
		t.Fatalf("unexpected summary: %q", diff.Summary)
	}
}

func TestDiffTableNamesEachAttributeThatDiffers(t *testing.T) {
	left := serialStructure()
	right := serialStructure()

	// Same field, five different things about it.
	right.Columns[1].Nullable = false
	right.Columns[1].DataType = "bigint"
	right.Columns[1].ColumnType = "bigint"
	right.Columns[1].AutoIncrement = true
	defaultValue := "0"
	right.Columns[1].DefaultValue = &defaultValue
	right.Columns[2].Comment = ""
	right.Indexes[1].Columns = []string{"note"}
	right.Indexes[1].Unique = true

	diff := DiffTable(left, right)
	if diff.Status != models.DiffChanged {
		t.Fatalf("expected a changed table: %+v", diff)
	}

	userID := columnItem(t, diff, "user_id")
	if userID.Status != models.DiffChanged {
		t.Fatalf("expected user_id to be changed: %+v", userID)
	}
	// The order is the order the renderer checks them in, which is the order a
	// reader walks the definition.
	want := []string{"type", "nullability", "default", "auto increment"}
	if got := fieldNames(userID); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected fields: %v (want %v)", got, want)
	}
	if userID.Summary != "type, nullability, default, auto increment differ" {
		t.Fatalf("unexpected summary: %q", userID.Summary)
	}

	// A comment that goes away is a difference too, and an empty one reads as
	// "(none)" rather than as nothing at all.
	note := columnItem(t, diff, "note")
	if note.Status != models.DiffChanged || strings.Join(fieldNames(note), ",") != "comment" {
		t.Fatalf("expected the comment to be the only difference: %+v", note)
	}
	if note.Fields[0].Right != "(none)" {
		t.Fatalf("an empty comment should read as (none): %+v", note.Fields[0])
	}

	// An index is compared the same way: the fields it covers and whether it is
	// unique.
	userIndex := indexItem(t, diff, "idx_orders_user")
	if userIndex.Status != models.DiffChanged ||
		strings.Join(fieldNames(userIndex), ",") != "columns,unique" {
		t.Fatalf("expected the index to differ in its columns and uniqueness: %+v", userIndex)
	}
	if userIndex.Summary != "columns, unique differ" {
		t.Fatalf("unexpected summary: %q", userIndex.Summary)
	}
}

// A counter the engine owns is not a value to copy: PostgreSQL spells a serial
// column's default as the name of the sequence behind it, and two databases name
// their own. Comparing those texts would report every serial column as different
// in a database that is otherwise identical.
func TestDiffTableIgnoresTheEngineOwnedCounterDefault(t *testing.T) {
	left := serialStructure()
	right := serialStructure()
	leftDefault := "nextval('orders_id_seq'::regclass)"
	rightDefault := "nextval('orders_id_seq1'::regclass)"
	left.Columns[0].DefaultValue = &leftDefault
	right.Columns[0].DefaultValue = &rightDefault

	diff := DiffTable(left, right)
	if diff.Status != models.DiffSame {
		t.Fatalf("two serial columns are the same field however their sequences are named: %+v",
			columnItem(t, diff, "id"))
	}
}

func TestDiffTableDoesNotRepeatThePrimaryKeyAsAnIndex(t *testing.T) {
	left := serialStructure()
	right := serialStructure()
	// The primary key is carried by the fields; dropping it from the index list
	// is what keeps one change from looking like two.
	diff := DiffTable(left, right)
	for _, item := range diff.Indexes {
		if strings.EqualFold(item.Name, "PRIMARY") {
			t.Fatalf("the primary key belongs on the fields, not in the index list: %+v", item)
		}
	}

	right.Indexes[0].Primary = false // the key is now spelled on one side only
	diff = DiffTable(left, right)
	for _, item := range diff.Indexes {
		if strings.EqualFold(item.Name, "PRIMARY") {
			t.Fatalf("an index that is the key on either side stays off the list: %+v", item)
		}
	}
}

func TestDiffTableDescribesATableOnlyOneSideHas(t *testing.T) {
	left := serialStructure()

	created := DiffTable(left, nil)
	if created.Status != models.DiffAdded || created.Summary != "only on the left" {
		t.Fatalf("a table the left alone has is one to create: %+v", created)
	}
	if len(created.Columns) != len(left.Columns) {
		t.Fatalf("every field should be reported: %+v", created.Columns)
	}
	for _, item := range created.Columns {
		if item.Status != models.DiffAdded || item.Summary != "only on the left" {
			t.Fatalf("expected an added field: %+v", item)
		}
	}

	removed := DiffTable(nil, left)
	if removed.Status != models.DiffRemoved || removed.Summary != "only on the right" {
		t.Fatalf("a table the right alone has is one to remove: %+v", removed)
	}
	for _, item := range removed.Columns {
		if item.Status != models.DiffRemoved || item.Summary != "only on the right" {
			t.Fatalf("expected a removed field: %+v", item)
		}
	}

	// A table one side has and the other does not is a difference, and the
	// summary says which side it is on rather than "1 field differs".
	if created.Name != "orders" {
		t.Fatalf("the name comes from the side that has the table: %q", created.Name)
	}
}

func TestDiffTableMatchesNamesRegardlessOfCase(t *testing.T) {
	left := serialStructure()
	right := serialStructure()
	right.Columns[1].Name = "USER_ID"
	right.Indexes[1].Name = "IDX_ORDERS_USER"

	diff := DiffTable(left, right)
	if diff.Status != models.DiffSame {
		t.Fatalf("case is not a difference: %+v", diff)
	}
}

// A design for a script is the source table read as the definition to write
// somewhere else. The two things it must do are keep every attribute and leave
// the primary key on the fields where the create path expects it.
func TestDesignOfStructureKeepsEverythingButThePrimaryKeyIndex(t *testing.T) {
	structure := serialStructure()
	design, warnings := DesignOfStructure(PostgresDialect{}, structure, "orders")

	if len(warnings) != 0 {
		t.Fatalf("nothing about this table needs saying: %v", warnings)
	}
	if design.Object != "orders" {
		t.Fatalf("the script writes the object it was asked for: %+v", design)
	}
	if len(design.Columns) != 3 || design.Columns[0].Name != "id" || !design.Columns[0].PrimaryKey {
		t.Fatalf("the fields should carry the key: %+v", design.Columns)
	}
	if design.Columns[2].Comment != "free text" {
		t.Fatalf("the comment should travel: %+v", design.Columns[2])
	}
	if len(design.Indexes) != 1 || design.Indexes[0].Name != "idx_orders_user" {
		t.Fatalf("only the non-primary indexes belong here: %+v", design.Indexes)
	}
	if design.Columns[0].OriginalName != "" {
		t.Fatalf("a definition for a table that does not exist yet has no original: %+v", design.Columns[0])
	}
}

// A PostgreSQL serial default names the sequence of the table it was read from,
// so a definition that is about to create a table somewhere else cannot use it:
// the field becomes an identity column and the warning says why.
func TestDesignOfStructureTurnsASerialDefaultIntoAnIdentity(t *testing.T) {
	structure := serialStructure()
	serial := "nextval('orders_id_seq'::regclass)"
	structure.Columns[0].DefaultValue = &serial

	design, warnings := DesignOfStructure(PostgresDialect{}, structure, "orders")
	if design.Columns[0].DefaultValue != nil {
		t.Fatalf("the sequence of another table cannot be carried over: %v", *design.Columns[0].DefaultValue)
	}
	if !design.Columns[0].AutoIncrement {
		t.Fatal("the field is still the counter of its table, which is what identity says")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "identity") {
		t.Fatalf("expected one warning about the identity column, got %v", warnings)
	}
}

// KeepEngineCounters is what stops a sync from pointing the target table at the
// source's counter: the field stays a counter, but the default stays the one the
// target already owns.
func TestKeepEngineCountersLeavesTheTargetsOwnSequenceAlone(t *testing.T) {
	left := serialStructure()
	leftDefault := "nextval('orders_id_seq'::regclass)"
	left.Columns[0].DefaultValue = &leftDefault

	right := serialStructure()
	rightDefault := "nextval('orders_id_seq1'::regclass)"
	right.Columns[0].DefaultValue = &rightDefault

	design, _ := DesignOfStructure(PostgresDialect{}, left, "orders")
	design.Columns[0].DefaultValue = nil // DesignOfStructure already dropped it
	KeepEngineCounters(&design, right)

	if design.Columns[0].DefaultValue == nil || *design.Columns[0].DefaultValue != rightDefault {
		t.Fatalf("the target's own default should have been kept: %+v", design.Columns[0])
	}
	if !design.Columns[0].AutoIncrement {
		t.Fatal("keeping the default must not turn the field into an ordinary one")
	}

	// A field that is not a counter on the target keeps whatever the definition
	// says: there it really is a value to write.
	plain := serialStructure()
	KeepEngineCounters(&design, plain)
	if design.Columns[1].DefaultValue != nil {
		t.Fatalf("a field the target does not count has no counter to keep: %+v", design.Columns[1])
	}
}
