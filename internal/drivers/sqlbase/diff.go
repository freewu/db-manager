package sqlbase

import (
	"strconv"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Comparing two namespaces is reading, not writing: this file turns two live
// table structures into the list of ways they disagree. It never touches a
// connection, so the same function answers both the window's diff and the
// generated script — what the script changes is exactly what the diff named.
//
// The comparison is deliberately in the designer's terms: fields, their types
// and flags, and the indexes that are not the primary key. Those are the things
// this application can write a statement for, and a difference it cannot act on
// would be a difference the generated script quietly ignored. Anything outside
// that vocabulary — foreign keys, triggers, views — is reported in the caller's
// warnings rather than folded into a comparison that cannot honour it.

// DiffTable compares one table as the left and right namespaces describe it.
//
// Either side may be absent, which is how a table that exists on only one of
// them is described: every field and index of the side that has it comes back
// as added (left only) or removed (right only). That keeps one code path for
// "this table is missing" and "this table is different", so the window does not
// have to draw two kinds of answer.
func DiffTable(left, right *models.TableStructure) models.TableDiff {
	diff := models.TableDiff{
		Name:    structureName(left, right),
		Kind:    models.KindTable,
		Columns: []models.DiffItem{},
		Indexes: []models.DiffItem{},
	}

	diff.Columns = diffColumns(left, right)
	diff.Indexes = diffIndexes(left, right)
	switch {
	case left == nil:
		// The right has it and the left does not: a table to remove.
		diff.Status = models.DiffRemoved
	case right == nil:
		// The left has it and the right does not: a table to create.
		diff.Status = models.DiffAdded
	default:
		diff.Status = tableStatus(diff.Columns, diff.Indexes)
	}
	diff.Summary = tableSummary(diff)
	return diff
}

// diffColumns compares the fields of two tables: those only one side has, and
// those both have but spell differently.
func diffColumns(left, right *models.TableStructure) []models.DiffItem {
	items := []models.DiffItem{}
	rightByName := map[string]models.ColumnInfo{}
	if right != nil {
		for _, c := range right.Columns {
			rightByName[identifierKey(c.Name)] = c
		}
	}
	seen := map[string]bool{}
	if left != nil {
		for _, lc := range left.Columns {
			key := identifierKey(lc.Name)
			seen[key] = true
			rc, ok := rightByName[key]
			if !ok {
				items = append(items, oneSidedItem(lc.Name, "column", models.DiffAdded))
				continue
			}
			items = append(items, diffColumn(lc, rc))
		}
	}
	if right != nil {
		for _, rc := range right.Columns {
			key := identifierKey(rc.Name)
			if seen[key] {
				continue
			}
			items = append(items, oneSidedItem(rc.Name, "column", models.DiffRemoved))
		}
	}
	return items
}

// diffColumn compares two fields that carry the same name.
func diffColumn(left, right models.ColumnInfo) models.DiffItem {
	item := models.DiffItem{Name: left.Name, Kind: "column", Status: models.DiffSame}
	fields := []models.DiffField{}

	if !sameText(columnTypeOf(left), columnTypeOf(right)) {
		fields = append(fields, models.DiffField{
			Field: "type", Left: columnTypeOf(left), Right: columnTypeOf(right),
		})
	}
	if currentNotNull(left) != currentNotNull(right) {
		fields = append(fields, models.DiffField{
			Field: "nullability", Left: nullabilityText(left), Right: nullabilityText(right),
		})
	}
	// A counter the engine owns has no default the comparison can hold anyone
	// to: PostgreSQL spells a serial column's default as the name of the
	// sequence behind it, and two databases name their own sequences, which is
	// not a difference a script should try to remove.
	if !(left.AutoIncrement && right.AutoIncrement) && !sameText(ptrText(left.DefaultValue), ptrText(right.DefaultValue)) {
		fields = append(fields, models.DiffField{
			Field: "default", Left: defaultText(left.DefaultValue), Right: defaultText(right.DefaultValue),
		})
	}
	if left.PrimaryKey != right.PrimaryKey {
		fields = append(fields, models.DiffField{
			Field: "primary key", Left: yesNo(left.PrimaryKey), Right: yesNo(right.PrimaryKey),
		})
	}
	if left.AutoIncrement != right.AutoIncrement {
		fields = append(fields, models.DiffField{
			Field: "auto increment", Left: yesNo(left.AutoIncrement), Right: yesNo(right.AutoIncrement),
		})
	}
	if !sameText(left.Comment, right.Comment) {
		fields = append(fields, models.DiffField{
			Field: "comment", Left: orNone(left.Comment), Right: orNone(right.Comment),
		})
	}

	if len(fields) > 0 {
		item.Status = models.DiffChanged
		item.Fields = fields
		item.Summary = fieldSummary(fields)
	}
	return item
}

// diffIndexes compares the indexes of two tables, leaving the primary key out:
// it is already carried by the fields, and reporting it twice would make one
// change look like two. An index that is the key on either side is left out, so
// a primary key that is spelled differently on the two sides is still the
// fields' business and not a second difference.
func diffIndexes(left, right *models.TableStructure) []models.DiffItem {
	items := []models.DiffItem{}
	rightByName := map[string]models.IndexInfo{}
	if right != nil {
		for _, ix := range right.Indexes {
			rightByName[identifierKey(ix.Name)] = ix
		}
	}
	seen := map[string]bool{}
	if left != nil {
		for _, lix := range left.Indexes {
			key := identifierKey(lix.Name)
			seen[key] = true
			rix, hasRight := rightByName[key]
			if lix.Primary || (hasRight && rix.Primary) {
				continue
			}
			if !hasRight {
				items = append(items, oneSidedItem(lix.Name, "index", models.DiffAdded))
				continue
			}
			items = append(items, diffIndex(lix, rix))
		}
	}
	if right != nil {
		for _, rix := range right.Indexes {
			if seen[identifierKey(rix.Name)] || rix.Primary {
				continue
			}
			items = append(items, oneSidedItem(rix.Name, "index", models.DiffRemoved))
		}
	}
	return items
}

// identifierKey is how two sides' identifiers are matched: case is not a
// difference, and neither is the space an engine reported around a name.
func identifierKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// diffIndex compares two indexes that carry the same name.
func diffIndex(left, right models.IndexInfo) models.DiffItem {
	item := models.DiffItem{Name: left.Name, Kind: "index", Status: models.DiffSame}
	fields := []models.DiffField{}

	if !sameColumns(left.Columns, right.Columns) {
		fields = append(fields, models.DiffField{
			Field: "columns",
			Left:  strings.Join(left.Columns, ", "),
			Right: strings.Join(right.Columns, ", "),
		})
	}
	if left.Unique != right.Unique {
		fields = append(fields, models.DiffField{
			Field: "unique", Left: yesNo(left.Unique), Right: yesNo(right.Unique),
		})
	}

	if len(fields) > 0 {
		item.Status = models.DiffChanged
		item.Fields = fields
		item.Summary = fieldSummary(fields)
	}
	return item
}

// DesignOfStructure reads a live table as the definition a script would write:
// every field with its type, default, comment and primary-key flag, and every
// index that is not the primary key.
//
// It is the copy's column reading without the copy's renaming: an object created
// in another namespace does not have to dodge the names the source is using,
// because the two are not the same namespace. The one thing that is still said
// out loud is a PostgreSQL serial default, which names the sequence of the table
// it was read from — a new table cannot have that sequence, so it gets an
// identity column instead and the warning says so.
func DesignOfStructure(
	d drivers.Dialect,
	current *models.TableStructure,
	object string,
) (models.TableDesign, []string) {
	want := models.TableDesign{
		Database: current.Object.Database,
		Schema:   current.Object.Schema,
		Object:   object,
		Indexes:  make([]models.DesignIndex, 0, len(current.Indexes)),
	}
	columns, warnings := designColumns(d, current)
	want.Columns = columns

	for _, ix := range current.Indexes {
		if ix.Primary {
			continue
		}
		want.Indexes = append(want.Indexes, models.DesignIndex{
			Name:    ix.Name,
			Columns: append([]string{}, ix.Columns...),
			Unique:  ix.Unique,
		})
	}
	return want, warnings
}

// KeepEngineCounters copies the target's own counter defaults into a design that
// is about to alter it.
//
// The comparison reads its design from the left table, and a PostgreSQL serial
// column's default there names the left table's sequence. Writing that into an
// ALTER would point the right table at a counter the left owns — so for a field
// whose counter the target already has, the target's own default is kept and the
// ALTER leaves it alone. A field that is not a counter on the target is left as
// the design has it, because then the default really is a value to set.
func KeepEngineCounters(want *models.TableDesign, target *models.TableStructure) {
	if target == nil {
		return
	}
	own := map[string]*string{}
	for _, c := range target.Columns {
		if c.AutoIncrement {
			own[strings.ToLower(strings.TrimSpace(c.Name))] = c.DefaultValue
		}
	}
	for i := range want.Columns {
		if !want.Columns[i].AutoIncrement {
			continue
		}
		if value, ok := own[strings.ToLower(strings.TrimSpace(want.Columns[i].Name))]; ok {
			want.Columns[i].DefaultValue = value
		}
	}
}

// --- small pieces of the answer --------------------------------------------

// oneSidedItem describes something only one side has. There is nothing to
// compare it with, so it carries no fields, only the fact itself.
func oneSidedItem(name, kind string, status models.DiffStatus) models.DiffItem {
	summary := "only on the left"
	if status == models.DiffRemoved {
		summary = "only on the right"
	}
	return models.DiffItem{Name: name, Kind: kind, Status: status, Summary: summary}
}

// structureName is the table's name, taken from whichever side has it.
func structureName(left, right *models.TableStructure) string {
	switch {
	case left != nil:
		return left.Object.Name
	case right != nil:
		return right.Object.Name
	}
	return ""
}

// tableStatus reads the whole table's standing off its parts: it is changed when
// any part of it is, and the same when none is. A table that is only on one side
// never reaches here — that is the table's own fact, not its parts'.
func tableStatus(columns, indexes []models.DiffItem) models.DiffStatus {
	status := models.DiffSame
	for _, item := range append(append([]models.DiffItem{}, columns...), indexes...) {
		if item.Status != models.DiffSame {
			return models.DiffChanged
		}
	}
	return status
}

// tableSummary is the one line a list row shows.
func tableSummary(diff models.TableDiff) string {
	switch diff.Status {
	case models.DiffSame:
		return "the same on both sides"
	case models.DiffChanged:
		parts := []string{}
		if n := differing(diff.Columns); n > 0 {
			parts = append(parts, plural(n, "field"))
		}
		if n := differing(diff.Indexes); n > 0 {
			parts = append(parts, plural(n, "index"))
		}
		if len(parts) == 0 {
			// A table marked changed by a difference that is not in its parts
			// (there is none today, but the sentence should not be empty if one
			// is ever added).
			return "different"
		}
		return strings.Join(parts, " and ") + " differ"
	}
	// Added or removed: the same sentence the parts carry, about the table.
	if diff.Status == models.DiffAdded {
		return "only on the left"
	}
	return "only on the right"
}

// differing counts the items that are not the same.
func differing(items []models.DiffItem) int {
	n := 0
	for _, item := range items {
		if item.Status != models.DiffSame {
			n++
		}
	}
	return n
}

// fieldSummary names the attributes that differ, for the sentence beside the
// field: "type, nullability differ".
func fieldSummary(fields []models.DiffField) string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Field)
	}
	if len(names) == 1 {
		return names[0] + " differs"
	}
	return strings.Join(names, ", ") + " differ"
}

// nullabilityText spells the NOT NULL flag the way a column definition does.
func nullabilityText(c models.ColumnInfo) string {
	if currentNotNull(c) {
		return "NOT NULL"
	}
	return "NULL"
}

// defaultText spells a default, with "(none)" for a column that has none.
func defaultText(value *string) string {
	text := ptrText(value)
	if text == "" {
		return "(none)"
	}
	return text
}

// orNone renders an optional piece of text, so an empty comment reads as one.
func orNone(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "(none)"
	}
	return text
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// plural is "1 field" / "2 fields".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
