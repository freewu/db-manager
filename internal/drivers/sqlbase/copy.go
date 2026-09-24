package sqlbase

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// PlanCopy renders the script that duplicates a table into a new one: the same
// fields, primary key and indexes, and — when the caller asks for the rows as
// well — one INSERT ... SELECT that moves the data.
//
// The structure half is PlanCreate's work, fed from the live catalog instead of
// from the designer. A copy *is* a CREATE TABLE of a table that does not exist
// yet, so there is no second renderer to keep in step with the first one: what a
// copy writes is what the designer would write for the same definition, engine
// limitations and warnings included. The data half is one statement rather than
// a read followed by writes, so the rows never leave the server, and it is part
// of the same plan for the same reason the CREATE is — the preview the user
// approves is the script that runs.
//
// Two things are deliberately not carried over verbatim, and both are said out
// loud in Warnings rather than left for the engine to refuse halfway through:
//
//   - PostgreSQL and SQLite keep index names in the schema (the database, for
//     SQLite), where the original's name is already used, so the copy's indexes
//     are named after the new table. MySQL keeps an index inside its table and
//     reuses the original name.
//   - a PostgreSQL serial column draws from a sequence of its own, so copying
//     the default verbatim would point the copy at the original's counter; the
//     copy gets an identity column instead.
func PlanCopy(
	d drivers.Dialect,
	current *models.TableStructure,
	target string,
	withData bool,
) (models.DesignPlan, error) {
	empty := models.DesignPlan{Statements: []string{}, Warnings: []string{}}
	if current == nil || len(current.Columns) == 0 {
		return empty, fmt.Errorf("there is no structure to copy")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return empty, fmt.Errorf("the copy needs a name")
	}
	if sameName(target, current.Object.Name) {
		return empty, fmt.Errorf("%s is the table being copied; the copy needs a name of its own",
			current.Object.Name)
	}

	want, warnings := designOfCopy(d, current, target)
	plan, err := PlanCreate(d, want)
	if err != nil {
		return empty, err
	}
	plan.Warnings = append(warnings, plan.Warnings...)

	if withData {
		plan.Statements = append(plan.Statements, copyRowsStatement(d, current, target))
		plan.Warnings = append(plan.Warnings, copyDataWarnings(d, current)...)
	}
	return plan, nil
}

// designOfCopy reads a live table as the definition of its copy, the way the
// designer's window prefills a draft from the catalog: every field with its
// type, default, comment and primary-key flag, and every index that is not the
// primary key (which belongs on the fields themselves).
//
// OriginalName stays empty throughout — this draft is for a table that does not
// exist yet, so no field or index of it came from anywhere.
func designOfCopy(
	d drivers.Dialect,
	current *models.TableStructure,
	target string,
) (models.TableDesign, []string) {
	want := models.TableDesign{
		Database: current.Object.Database,
		Schema:   current.Object.Schema,
		Object:   target,
		Columns:  make([]models.DesignColumn, 0, len(current.Columns)),
		Indexes:  make([]models.DesignIndex, 0, len(current.Indexes)),
	}
	warnings := []string{}

	for _, c := range current.Columns {
		column := models.DesignColumn{
			Name: c.Name,
			// The engine's own spelling wins, exactly as the designer prefills
			// it: varchar(255) is a different field from varchar.
			DataType:      columnTypeOf(c),
			Nullable:      c.Nullable && !c.PrimaryKey,
			DefaultValue:  c.DefaultValue,
			PrimaryKey:    c.PrimaryKey,
			AutoIncrement: c.AutoIncrement,
			Comment:       c.Comment,
		}
		if warning := postgresSerialIsCopiedAsIdentity(d, &column); warning != "" {
			warnings = append(warnings, warning)
		}
		want.Columns = append(want.Columns, column)
	}

	for _, ix := range current.Indexes {
		if ix.Primary {
			continue
		}
		name, warning := copyIndexName(d, target, ix.Name)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		want.Indexes = append(want.Indexes, models.DesignIndex{
			Name: name,
			// A copy of a slice: the catalog's own list is not ours to edit.
			Columns: append([]string{}, ix.Columns...),
			Unique:  ix.Unique,
		})
	}
	return want, warnings
}

// postgresSerialIsCopiedAsIdentity turns a serial column into an identity one,
// and returns the sentence that says so.
//
// A serial column is spelled `integer DEFAULT nextval('orders_id_seq')`: the
// default belongs to the *original's* sequence, so copying it verbatim would
// make the two tables hand out the same numbers from one counter. An identity
// column belongs to the table it is on, which is what a copy wants — and it is
// the same statement the designer writes for an AutoIncrement field.
//
// Only an integer can be an identity column. A sequence-backed default on any
// other type is left exactly as it is and reported instead of being replaced
// with something the engine would reject.
func postgresSerialIsCopiedAsIdentity(d drivers.Dialect, c *models.DesignColumn) string {
	if d.Name() != models.DriverPostgres || !c.AutoIncrement || c.DefaultValue == nil {
		return ""
	}
	if !strings.Contains(*c.DefaultValue, "nextval(") {
		return "" // an identity column already: it carries no default of its own
	}
	if !isIntegerColumn(c.DataType) {
		return fmt.Sprintf("field %s keeps its default of %s, which draws from a sequence of the table being copied; the two tables would share one counter",
			c.Name, strings.TrimSpace(*c.DefaultValue))
	}
	c.DefaultValue = nil
	return fmt.Sprintf("field %s is created as an identity column: its serial default draws from a sequence of its own, which a copy cannot have",
		c.Name)
}

// isIntegerColumn reports whether a type text is one of the integer types a
// PostgreSQL identity column may have.
func isIntegerColumn(dataType string) bool {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "smallint", "integer", "bigint", "int",
		"int2", "int4", "int8",
		"smallserial", "serial", "bigserial":
		return true
	}
	return false
}

// copyIndexName names an index of the copy and, when the name had to change,
// returns the sentence that explains why.
func copyIndexName(d drivers.Dialect, target, name string) (string, string) {
	var engine, scope string
	var limit int
	switch d.Name() {
	case models.DriverPostgres:
		engine, scope, limit = "PostgreSQL", "schema", 63 // NAMEDATALEN - 1
	case models.DriverSQLite:
		engine, scope = "SQLite", "database"
	default:
		// MySQL and TiDB keep an index inside its table, so the original's name
		// can be used again as it is.
		return name, ""
	}

	renamed := trimIdentifier(target+"_"+name, limit)
	if sameName(renamed, name) {
		return name, ""
	}
	return renamed, fmt.Sprintf(
		"index %s is created as %s: %s keeps index names in the %s, where the original's is already used",
		name, renamed, engine, scope)
}

// trimIdentifier cuts an identifier down to what an engine accepts. The limit is
// counted in characters rather than bytes, because a name may well be typed in
// something other than ASCII.
func trimIdentifier(name string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(name) <= limit {
		return name
	}
	return string([]rune(name)[:limit])
}

// copyRowsStatement moves the rows of one table into its new copy.
//
// One INSERT ... SELECT: the rows never travel through this program, and the
// column list is written out on both sides — the copy's fields are created in
// the original's order today, but naming them is what keeps the statement right
// if that ever stops being true, and it is the same text an engine would show.
func copyRowsStatement(d drivers.Dialect, current *models.TableStructure, target string) string {
	columns := make([]string, 0, len(current.Columns))
	for _, c := range current.Columns {
		columns = append(columns, d.Quote(c.Name))
	}
	list := strings.Join(columns, ", ")
	from := d.Qualify(current.Object.Database, current.Object.Schema, current.Object.Name)
	to := d.Qualify(current.Object.Database, current.Object.Schema, target)
	return "INSERT INTO " + to + " (" + list + ")\nSELECT " + list + "\nFROM " + from
}

// copyDataWarnings lists what copying the rows does beyond copying the rows.
//
// MySQL moves its AUTO_INCREMENT counter past the largest value it has just
// inserted and SQLite takes the largest rowid along with the rows, so on those
// two the copy is ready to be written to. A PostgreSQL identity column keeps its
// own counter, which the explicit values did not touch: the next insert without
// a key starts again at 1 and collides with a row that came with the copy.
func copyDataWarnings(d drivers.Dialect, current *models.TableStructure) []string {
	if d.Name() != models.DriverPostgres {
		return nil
	}
	var warnings []string
	for _, c := range current.Columns {
		if !c.AutoIncrement {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"the rows keep their own %s values, so the identity counter of the copy still starts at 1; the next insert without a key may collide with a copied row",
			c.Name))
	}
	return warnings
}
