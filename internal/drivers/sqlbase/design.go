package sqlbase

import (
	"fmt"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// PlanAlter renders the statements that bring a live table in line with the
// table designer's draft.
//
// The designer always sends the complete desired definition instead of a diff:
// the difference is computed here, against the live catalog structure, so the
// script the user previews and the script that is applied come out of exactly
// the same code.
//
// Statements are ordered so that the script works on a real server:
//
//	1. drop the indexes that are going away      (they may block a column drop)
//	2. drop columns
//	3. drop the primary key                      (it may cover a changing column)
//	4. add columns
//	5. change columns (rename / type / nullability / default / comment)
//	6. add the primary key
//	7. create or rename indexes
//
// Anything an engine cannot express is reported in Plan.Warnings instead of
// being silently skipped: the designer never pretends to have changed something
// it did not change.
func PlanAlter(d drivers.Dialect, current *models.TableStructure, want models.TableDesign) (models.DesignPlan, error) {
	empty := models.DesignPlan{Statements: []string{}, Warnings: []string{}}
	if current == nil || len(current.Columns) == 0 {
		return empty, fmt.Errorf("there is no structure to compare the design against")
	}
	if strings.TrimSpace(want.Object) == "" {
		want.Object = current.Object.Name
	}
	if !strings.EqualFold(strings.TrimSpace(want.Object), current.Object.Name) {
		return empty, fmt.Errorf("the designer cannot rename a table yet")
	}
	if err := validateDesign(want); err != nil {
		return empty, err
	}

	diff := newAlterDiff(d, current, want)
	switch d.Name() {
	case models.DriverMySQL, models.DriverTiDB:
		// TiDB is MySQL-compatible down to the ALTER TABLE forms below, so
		// the two share one plan; only the dialect's name differs.
		return diff.mysqlPlan()
	case models.DriverPostgres:
		return diff.postgresPlan()
	case models.DriverSQLite:
		return diff.sqlitePlan()
	default:
		return empty, fmt.Errorf("the table designer cannot alter %s tables yet", d.Name())
	}
}

// --- diff ------------------------------------------------------------------

// columnChange is one existing column together with the draft that replaces it.
type columnChange struct {
	from models.ColumnInfo
	to   models.DesignColumn
}

// indexChange is one existing index together with the draft that replaces it.
type indexChange struct {
	from models.IndexInfo
	to   models.DesignIndex
}

// alterDiff is the computed difference between the live structure and a draft.
type alterDiff struct {
	d        drivers.Dialect
	table    string
	database string
	schema   string
	want     models.TableDesign

	added   []models.DesignColumn
	changed []columnChange
	dropped []models.ColumnInfo

	// pkBefore/pkAfter are the primary key columns on both sides, pkName is the
	// name of the current primary key (PostgreSQL drops it as a constraint).
	pkBefore []string
	pkAfter  []string
	pkName   string

	indexAdded   []models.DesignIndex
	indexChanged []indexChange
	indexDropped []models.IndexInfo

	plan models.DesignPlan
}

func newAlterDiff(d drivers.Dialect, current *models.TableStructure, want models.TableDesign) *alterDiff {
	a := &alterDiff{
		d:        d,
		table:    d.Qualify(current.Object.Database, current.Object.Schema, current.Object.Name),
		database: current.Object.Database,
		schema:   current.Object.Schema,
		want:     want,
		plan:     models.DesignPlan{Statements: []string{}, Warnings: []string{}},
	}

	// --- columns -----------------------------------------------------------
	used := make([]bool, len(current.Columns))
	for _, w := range want.Columns {
		if i := matchColumn(current.Columns, used, w); i >= 0 {
			used[i] = true
			a.changed = append(a.changed, columnChange{from: current.Columns[i], to: w})
			continue
		}
		a.added = append(a.added, w)
	}
	for i, c := range current.Columns {
		if !used[i] {
			a.dropped = append(a.dropped, c)
		}
	}

	// --- primary key -------------------------------------------------------
	gone := map[string]bool{}
	for _, c := range a.dropped {
		gone[strings.ToLower(c.Name)] = true
	}
	renamed := map[string]string{}
	for _, ch := range a.changed {
		if !sameName(ch.from.Name, ch.to.Name) {
			renamed[strings.ToLower(ch.from.Name)] = strings.TrimSpace(ch.to.Name)
		}
	}
	// A draft may still name a renamed field by its old name in an index it did
	// not touch: the index follows the rename either way.
	if len(renamed) > 0 {
		indexes := make([]models.DesignIndex, len(a.want.Indexes))
		copy(indexes, a.want.Indexes)
		for i := range indexes {
			columns := make([]string, 0, len(indexes[i].Columns))
			for _, column := range indexes[i].Columns {
				if to := renamed[strings.ToLower(strings.TrimSpace(column))]; to != "" {
					columns = append(columns, to)
					continue
				}
				columns = append(columns, column)
			}
			indexes[i].Columns = columns
		}
		a.want.Indexes = indexes
	}
	for _, ix := range current.Indexes {
		if !ix.Primary {
			continue
		}
		a.pkName = ix.Name
		if len(a.pkBefore) == 0 {
			a.pkBefore = append(a.pkBefore, ix.Columns...)
		}
		break
	}
	if len(a.pkBefore) == 0 {
		for _, c := range current.Columns {
			if c.PrimaryKey {
				a.pkBefore = append(a.pkBefore, c.Name)
			}
		}
	}
	for _, c := range want.Columns {
		if c.PrimaryKey {
			a.pkAfter = append(a.pkAfter, strings.TrimSpace(c.Name))
		}
	}

	// --- indexes -----------------------------------------------------------
	// Indexes follow their columns through renames, and disappear with them.
	effective := make([]models.IndexInfo, 0, len(current.Indexes))
	for _, ix := range current.Indexes {
		if ix.Primary {
			continue // the primary key is handled above
		}
		columns := make([]string, 0, len(ix.Columns))
		lost := ""
		for _, column := range ix.Columns {
			switch {
			case gone[strings.ToLower(column)]:
				lost = column
			case renamed[strings.ToLower(column)] != "":
				columns = append(columns, renamed[strings.ToLower(column)])
			default:
				columns = append(columns, column)
			}
		}
		if lost != "" {
			a.warn("index %s is dropped together with field %s", ix.Name, lost)
			a.indexDropped = append(a.indexDropped, ix)
			continue
		}
		effective = append(effective, models.IndexInfo{Name: ix.Name, Columns: columns, Unique: ix.Unique})
	}

	fkNames := map[string]bool{}
	for _, fk := range current.ForeignKeys {
		fkNames[strings.ToLower(fk.Name)] = true
		for _, column := range fk.Columns {
			for _, dropped := range a.dropped {
				if sameName(column, dropped.Name) {
					a.warn("field %s is used by foreign key %s; the engine may refuse to drop it", dropped.Name, fk.Name)
					break
				}
			}
		}
	}

	usedIndex := make([]bool, len(effective))
	for _, w := range a.want.Indexes {
		i := matchIndex(effective, usedIndex, w)
		if i < 0 {
			a.indexAdded = append(a.indexAdded, w)
			continue
		}
		usedIndex[i] = true
		from := effective[i]
		if sameName(from.Name, w.Name) && sameColumns(from.Columns, w.Columns) && from.Unique == w.Unique {
			continue
		}
		a.indexChanged = append(a.indexChanged, indexChange{from: from, to: w})
	}
	for i, ix := range effective {
		if !usedIndex[i] {
			a.indexDropped = append(a.indexDropped, ix)
		}
	}
	for _, ix := range a.indexDropped {
		if fkNames[strings.ToLower(ix.Name)] {
			a.warn("index %s backs the foreign key of the same name; MySQL refuses to drop it while that constraint exists", ix.Name)
		}
	}

	return a
}

// --- MySQL -----------------------------------------------------------------

func (a *alterDiff) mysqlPlan() (models.DesignPlan, error) {
	for _, c := range a.want.Columns {
		if c.AutoIncrement && !a.autoIncrementIsKeyed(c.Name) {
			a.warn("AUTO_INCREMENT needs an index starting with %s; MySQL rejects the statement otherwise", c.Name)
		}
	}

	for _, ix := range a.indexDropped {
		a.add("ALTER TABLE " + a.table + " DROP INDEX " + a.quote(ix.Name))
	}
	for _, ch := range a.indexChanged {
		if a.indexRenamedInPlace(ch) {
			continue // renamed below instead of rebuilt
		}
		a.add("ALTER TABLE " + a.table + " DROP INDEX " + a.quote(ch.from.Name))
	}
	for _, c := range a.dropped {
		a.add("ALTER TABLE " + a.table + " DROP COLUMN " + a.quote(c.Name))
		a.plan.Destructive = true
	}
	if a.primaryKeyChanged() && len(a.pkBefore) > 0 {
		a.add("ALTER TABLE " + a.table + " DROP PRIMARY KEY")
		a.plan.Destructive = true
	}
	for _, c := range a.added {
		a.add("ALTER TABLE " + a.table + " ADD COLUMN " + mysqlColumnDef(a.quote, c))
	}
	for _, ch := range a.changed {
		if !a.mysqlColumnDiffers(ch) {
			continue
		}
		// CHANGE COLUMN carries the whole definition, which is also how a
		// rename is spelled on every MySQL and MariaDB version.
		a.add("ALTER TABLE " + a.table + " CHANGE COLUMN " + a.quote(ch.from.Name) + " " + mysqlColumnDef(a.quote, ch.to))
	}
	if a.primaryKeyChanged() && len(a.pkAfter) > 0 {
		a.add("ALTER TABLE " + a.table + " ADD PRIMARY KEY (" + strings.Join(a.quoteAll(a.pkAfter), ", ") + ")")
	}
	for _, ix := range a.indexAdded {
		a.add(a.createIndexStatement(ix))
	}
	for _, ch := range a.indexChanged {
		// MySQL can rename an index in place; anything else is a drop and a
		// create, because CHANGE/ADD INDEX has no partial form.
		if sameColumns(ch.from.Columns, ch.to.Columns) && ch.from.Unique == ch.to.Unique {
			a.add("ALTER TABLE " + a.table + " RENAME INDEX " + a.quote(ch.from.Name) + " TO " + a.quote(strings.TrimSpace(ch.to.Name)))
			continue
		}
		a.add(a.createIndexStatement(ch.to))
	}

	return a.plan, nil
}

// mysqlColumnDef renders one column the way MySQL spells it, including the
// comment, which MySQL keeps on the column itself. It takes the quoting
// function instead of a diff so the create and alter paths share it.
func mysqlColumnDef(quote func(string) string, c models.DesignColumn) string {
	name := strings.TrimSpace(c.Name)
	var b strings.Builder
	b.WriteString(quote(name))
	b.WriteString(" ")
	b.WriteString(strings.TrimSpace(c.DataType))
	if c.Nullable && !c.PrimaryKey {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}
	if c.AutoIncrement {
		b.WriteString(" AUTO_INCREMENT")
	}
	if c.DefaultValue != nil {
		b.WriteString(" DEFAULT " + strings.TrimSpace(*c.DefaultValue))
	}
	if comment := strings.TrimSpace(c.Comment); comment != "" {
		b.WriteString(" COMMENT '" + escapeString(comment) + "'")
	}
	return b.String()
}

func (a *alterDiff) mysqlColumnDiffers(ch columnChange) bool {
	to := ch.to
	return !sameName(ch.from.Name, to.Name) ||
		!sameText(columnTypeOf(ch.from), to.DataType) ||
		currentNotNull(ch.from) != designNotNull(to) ||
		!sameText(ptrText(ch.from.DefaultValue), ptrText(to.DefaultValue)) ||
		ch.from.AutoIncrement != to.AutoIncrement ||
		!sameText(ch.from.Comment, to.Comment)
}

func (a *alterDiff) autoIncrementIsKeyed(name string) bool {
	if len(a.pkAfter) > 0 && sameName(a.pkAfter[0], name) {
		return true
	}
	for _, ix := range a.want.Indexes {
		if ix.Unique && len(ix.Columns) > 0 && sameName(ix.Columns[0], name) {
			return true
		}
	}
	return false
}

// autoIncrementIsKeyed answers the same question for a table that is about to
// be created: MySQL only accepts AUTO_INCREMENT on a column that starts a key,
// and there it may be the primary key written at the end of the statement.
func autoIncrementIsKeyed(want models.TableDesign, name string) bool {
	if pk := primaryKeyFields(want); len(pk) > 0 && sameName(pk[0], name) {
		return true
	}
	for _, ix := range want.Indexes {
		if ix.Unique && len(ix.Columns) > 0 && sameName(strings.TrimSpace(ix.Columns[0]), name) {
			return true
		}
	}
	return false
}

// --- PostgreSQL ------------------------------------------------------------

func (a *alterDiff) postgresPlan() (models.DesignPlan, error) {
	for _, ix := range a.indexDropped {
		a.add("DROP INDEX " + a.qualifiedIndex(ix.Name))
	}
	for _, ch := range a.indexChanged {
		if a.indexRenamedInPlace(ch) {
			continue // renamed below instead of rebuilt
		}
		a.add("DROP INDEX " + a.qualifiedIndex(ch.from.Name))
	}
	for _, c := range a.dropped {
		a.add("ALTER TABLE " + a.table + " DROP COLUMN " + a.quote(c.Name))
		a.plan.Destructive = true
	}
	if a.primaryKeyChanged() && len(a.pkBefore) > 0 && a.pkName != "" {
		a.add("ALTER TABLE " + a.table + " DROP CONSTRAINT " + a.quote(a.pkName))
		a.plan.Destructive = true
	}
	for _, c := range a.added {
		a.add("ALTER TABLE " + a.table + " ADD COLUMN " + postgresColumnDef(a.quote, c))
	}
	for _, ch := range a.changed {
		a.postgresColumnChanges(ch)
	}
	if a.primaryKeyChanged() && len(a.pkAfter) > 0 {
		a.add("ALTER TABLE " + a.table + " ADD PRIMARY KEY (" + strings.Join(a.quoteAll(a.pkAfter), ", ") + ")")
	}
	for _, ix := range a.indexAdded {
		a.add(a.createIndexStatement(ix))
	}
	for _, ch := range a.indexChanged {
		if sameColumns(ch.from.Columns, ch.to.Columns) && ch.from.Unique == ch.to.Unique {
			a.add("ALTER INDEX " + a.qualifiedIndex(ch.from.Name) + " RENAME TO " + a.quote(strings.TrimSpace(ch.to.Name)))
			continue
		}
		a.add(a.createIndexStatement(ch.to))
	}

	return a.plan, nil
}

// postgresColumnDef renders one column the way PostgreSQL spells it. Comments
// are not part of the column here; the caller emits a COMMENT ON statement.
func postgresColumnDef(quote func(string) string, c models.DesignColumn) string {
	var b strings.Builder
	b.WriteString(quote(strings.TrimSpace(c.Name)))
	b.WriteString(" ")
	b.WriteString(strings.TrimSpace(c.DataType))
	if c.AutoIncrement {
		b.WriteString(" GENERATED BY DEFAULT AS IDENTITY")
	}
	if c.DefaultValue != nil {
		b.WriteString(" DEFAULT " + strings.TrimSpace(*c.DefaultValue))
	}
	if !c.Nullable || c.PrimaryKey {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

func (a *alterDiff) postgresColumnChanges(ch columnChange) {
	from, to := ch.from, ch.to
	name := from.Name
	comment := strings.TrimSpace(to.Comment)

	if !sameName(from.Name, to.Name) {
		a.add("ALTER TABLE " + a.table + " RENAME COLUMN " + a.quote(from.Name) + " TO " + a.quote(strings.TrimSpace(to.Name)))
		name = strings.TrimSpace(to.Name)
	}
	target := a.quote(name)
	if !sameText(columnTypeOf(from), to.DataType) {
		// PostgreSQL can cast most columns on its own; anything it cannot cast
		// needs a USING clause, which only the user can write.
		a.add("ALTER TABLE " + a.table + " ALTER COLUMN " + target + " TYPE " + strings.TrimSpace(to.DataType))
		a.warn("changing the type of field %s needs a USING clause when the values cannot be cast automatically", name)
	}
	wantNotNull := designNotNull(to)
	if currentNotNull(from) != wantNotNull {
		if wantNotNull {
			a.add("ALTER TABLE " + a.table + " ALTER COLUMN " + target + " SET NOT NULL")
		} else {
			a.add("ALTER TABLE " + a.table + " ALTER COLUMN " + target + " DROP NOT NULL")
		}
	}
	if !sameText(ptrText(from.DefaultValue), ptrText(to.DefaultValue)) {
		if to.DefaultValue == nil || strings.TrimSpace(*to.DefaultValue) == "" {
			a.add("ALTER TABLE " + a.table + " ALTER COLUMN " + target + " DROP DEFAULT")
		} else {
			a.add("ALTER TABLE " + a.table + " ALTER COLUMN " + target + " SET DEFAULT " + strings.TrimSpace(*to.DefaultValue))
		}
	}
	if from.AutoIncrement != to.AutoIncrement {
		a.warn("PostgreSQL identity is not changed by the designer (field %s); use ALTER TABLE ... ADD GENERATED ... AS IDENTITY", name)
	}
	if !sameText(from.Comment, to.Comment) {
		if comment == "" {
			a.add("COMMENT ON COLUMN " + a.table + "." + target + " IS NULL")
		} else {
			a.add("COMMENT ON COLUMN " + a.table + "." + target + " IS '" + escapeString(comment) + "'")
		}
	}
}

// --- SQLite ----------------------------------------------------------------

// SQLite only grew ALTER TABLE ... DROP COLUMN in 3.35 and RENAME COLUMN in
// 3.25; everything else about an existing column is frozen. The designer says so
// out loud instead of writing a script that cannot work.
func (a *alterDiff) sqlitePlan() (models.DesignPlan, error) {
	for _, c := range a.added {
		if c.AutoIncrement {
			a.warn("SQLite only auto-increments INTEGER PRIMARY KEY columns; field %s is created without AUTOINCREMENT", c.Name)
		}
		if c.PrimaryKey {
			a.warn("SQLite cannot add a primary key column to an existing table; field %s is created as an ordinary one", c.Name)
		}
		if !c.Nullable && (c.DefaultValue == nil || isNullLiteral(*c.DefaultValue)) {
			a.warn("SQLite cannot add the NOT NULL field %s without a default value", c.Name)
		}
	}
	if a.primaryKeyChanged() {
		a.warn("SQLite cannot change a primary key; recreate the table if you need this")
	}

	for _, ix := range a.indexDropped {
		a.add("DROP INDEX " + a.qualifiedIndex(ix.Name))
	}
	for _, ch := range a.indexChanged {
		a.add("DROP INDEX " + a.qualifiedIndex(ch.from.Name))
	}
	for _, c := range a.dropped {
		a.add("ALTER TABLE " + a.table + " DROP COLUMN " + a.quote(c.Name))
		a.plan.Destructive = true
	}
	for _, c := range a.added {
		a.add("ALTER TABLE " + a.table + " ADD COLUMN " + sqliteColumnDef(a.quote, c))
	}
	for _, ch := range a.changed {
		if !sameName(ch.from.Name, ch.to.Name) {
			a.add("ALTER TABLE " + a.table + " RENAME COLUMN " + a.quote(ch.from.Name) + " TO " + a.quote(strings.TrimSpace(ch.to.Name)))
		}
		a.sqliteUnsupportedChange(ch)
	}
	for _, ix := range a.indexAdded {
		a.add(a.createIndexStatement(ix))
	}
	for _, ch := range a.indexChanged {
		if a.indexRenamedInPlace(ch) {
			a.warn("SQLite cannot rename an index; %s is recreated as %s", ch.from.Name, strings.TrimSpace(ch.to.Name))
		}
		a.add(a.createIndexStatement(ch.to))
	}

	return a.plan, nil
}

// sqliteColumnDef renders one column the way SQLite spells it. SQLite has no
// column comments and only auto-increments INTEGER PRIMARY KEY columns, so the
// create path reports both rather than writing something the engine ignores.
func sqliteColumnDef(quote func(string) string, c models.DesignColumn) string {
	var b strings.Builder
	b.WriteString(quote(strings.TrimSpace(c.Name)))
	b.WriteString(" ")
	b.WriteString(strings.TrimSpace(c.DataType))
	if !c.Nullable || c.PrimaryKey {
		b.WriteString(" NOT NULL")
	}
	if c.DefaultValue != nil {
		b.WriteString(" DEFAULT " + strings.TrimSpace(*c.DefaultValue))
	}
	return b.String()
}

func (a *alterDiff) sqliteUnsupportedChange(ch columnChange) {
	from, to := ch.from, ch.to
	var changes []string
	if !sameText(columnTypeOf(from), to.DataType) {
		changes = append(changes, "the type")
	}
	if currentNotNull(from) != designNotNull(to) {
		changes = append(changes, "the NOT NULL flag")
	}
	if !sameText(ptrText(from.DefaultValue), ptrText(to.DefaultValue)) {
		changes = append(changes, "the default value")
	}
	if from.AutoIncrement != to.AutoIncrement {
		changes = append(changes, "the AUTOINCREMENT flag")
	}
	if !sameText(from.Comment, to.Comment) && strings.TrimSpace(to.Comment) != "" {
		changes = append(changes, "the comment")
	}
	if len(changes) > 0 {
		a.warn("SQLite cannot change %s of field %s; recreate the table and copy the data if you really need it",
			strings.Join(changes, " and "), to.Name)
	}
}

// --- create ----------------------------------------------------------------

// PlanCreate renders the statements that create a table from a design.
//
// It is the alter path's mirror image: there is no catalog structure to compare
// against, because the table does not exist yet, so every field is written out
// and the engine's own CREATE TABLE form decides how. The preview the user
// approves and the script that runs come out of this one function, exactly like
// PlanAlter.
func PlanCreate(d drivers.Dialect, want models.TableDesign) (models.DesignPlan, error) {
	empty := models.DesignPlan{Statements: []string{}, Warnings: []string{}}
	if strings.TrimSpace(want.Object) == "" {
		return empty, fmt.Errorf("the new table needs a name")
	}
	if err := validateDesign(want); err != nil {
		return empty, err
	}

	switch d.Name() {
	case models.DriverMySQL, models.DriverTiDB:
		return createPlan(d, mysqlColumnDef, want), nil
	case models.DriverPostgres:
		return createPlan(d, postgresColumnDef, want), nil
	case models.DriverSQLite:
		return createPlan(d, sqliteColumnDef, want), nil
	default:
		return empty, fmt.Errorf("the table designer cannot create %s tables yet", d.Name())
	}
}

// createPlan renders the portable half of a CREATE: the columns, the primary
// key and then one CREATE INDEX per index.
//
// Indexes are separate statements rather than inline clauses because that is
// the only form PostgreSQL and SQLite share with MySQL; the table is therefore
// momentarily unique-constraint-less between the two statements, which is why
// the whole script must be run in order.
func createPlan(
	d drivers.Dialect,
	columnDef func(quote func(string) string, c models.DesignColumn) string,
	want models.TableDesign,
) models.DesignPlan {
	plan := models.DesignPlan{Statements: []string{}, Warnings: []string{}}
	table := d.Qualify(want.Database, want.Schema, strings.TrimSpace(want.Object))

	for _, c := range want.Columns {
		name := strings.TrimSpace(c.Name)
		if !c.AutoIncrement {
			continue
		}
		switch d.Name() {
		case models.DriverMySQL, models.DriverTiDB:
			if !autoIncrementIsKeyed(want, name) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf(
					"AUTO_INCREMENT needs an index starting with %s; MySQL rejects the statement otherwise", name))
			}
		case models.DriverSQLite:
			// SQLite only auto-increments an INTEGER PRIMARY KEY, which the
			// definition above writes as a plain column: it is the rowid alias
			// that does the work, so the flag is honoured without the keyword.
			if !c.PrimaryKey {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf(
					"SQLite only auto-increments INTEGER PRIMARY KEY columns; field %s is created without AUTOINCREMENT", name))
			}
		}
	}

	lines := make([]string, 0, len(want.Columns)+1)
	for _, c := range want.Columns {
		lines = append(lines, "  "+columnDef(d.Quote, c))
	}
	if pk := quoteAll(d, primaryKeyFields(want)); len(pk) > 0 {
		lines = append(lines, "  PRIMARY KEY ("+strings.Join(pk, ", ")+")")
	}
	plan.Statements = append(plan.Statements, "CREATE TABLE "+table+" (\n"+strings.Join(lines, ",\n")+"\n)")

	for _, c := range want.Columns {
		comment := strings.TrimSpace(c.Comment)
		if comment == "" {
			continue
		}
		if d.Name() == models.DriverPostgres {
			plan.Statements = append(plan.Statements, "COMMENT ON COLUMN "+table+"."+d.Quote(strings.TrimSpace(c.Name))+" IS '"+escapeString(comment)+"'")
		} else if d.Name() == models.DriverSQLite {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("SQLite has no column comments; the comment on field %s is not written", strings.TrimSpace(c.Name)))
		}
	}

	for _, ix := range want.Indexes {
		name := strings.TrimSpace(ix.Name)
		if name == "" || len(ix.Columns) == 0 {
			continue
		}
		kind := "INDEX"
		if ix.Unique {
			kind = "UNIQUE INDEX"
		}
		plan.Statements = append(plan.Statements,
			"CREATE "+kind+" "+qualifiedIndexName(d, want.Database, want.Schema, name)+
				" ON "+table+" ("+strings.Join(quoteAll(d, ix.Columns), ", ")+")")
	}

	return plan
}

// primaryKeyFields lists the fields a draft marks as its primary key.
func primaryKeyFields(want models.TableDesign) []string {
	var out []string
	for _, c := range want.Columns {
		if c.PrimaryKey {
			out = append(out, strings.TrimSpace(c.Name))
		}
	}
	return out
}

// --- shared helpers --------------------------------------------------------

func (a *alterDiff) add(statement string) {
	a.plan.Statements = append(a.plan.Statements, statement)
}

func (a *alterDiff) warn(format string, args ...any) {
	a.plan.Warnings = append(a.plan.Warnings, fmt.Sprintf(format, args...))
}

func (a *alterDiff) quote(ident string) string { return a.d.Quote(ident) }

func (a *alterDiff) quoteAll(idents []string) []string {
	out := make([]string, 0, len(idents))
	for _, ident := range idents {
		out = append(out, a.d.Quote(strings.TrimSpace(ident)))
	}
	return out
}

func (a *alterDiff) primaryKeyChanged() bool {
	return !sameColumns(a.pkBefore, a.pkAfter)
}

// mysqlFamily reports whether a dialect writes MySQL-flavoured DDL. TiDB speaks
// the MySQL wire protocol and the same identifier and index syntax, so it is
// planned by the MySQL code paths instead of growing a copy of them.
func mysqlFamily(t models.DriverType) bool {
	return t == models.DriverMySQL || t == models.DriverTiDB
}

// indexRenamedInPlace reports whether an index only changes its name, which
// MySQL and PostgreSQL can do in place. SQLite cannot, so it drops and
// recreates the index instead.
func (a *alterDiff) indexRenamedInPlace(ch indexChange) bool {
	return sameColumns(ch.from.Columns, ch.to.Columns) && ch.from.Unique == ch.to.Unique
}

// qualifiedIndex names an index the way the engine scopes it: MySQL indexes
// belong to their table, PostgreSQL indexes live in a schema and SQLite indexes
// live in a database.
func (a *alterDiff) qualifiedIndex(name string) string {
	return qualifiedIndexName(a.d, a.database, a.schema, name)
}

// qualifiedIndexName is the same rule without a diff behind it, so creating a
// table scopes its indexes exactly like altering one does.
func qualifiedIndexName(d drivers.Dialect, database, schema, name string) string {
	switch d.Name() {
	case models.DriverPostgres:
		return d.Qualify(database, schema, name)
	case models.DriverSQLite:
		return d.Qualify(database, "", name)
	default:
		return d.Quote(name)
	}
}

func (a *alterDiff) createIndexStatement(ix models.DesignIndex) string {
	kind := "INDEX"
	if ix.Unique {
		kind = "UNIQUE INDEX"
	}
	columns := strings.Join(a.quoteAll(ix.Columns), ", ")
	if mysqlFamily(a.d.Name()) {
		return "ALTER TABLE " + a.table + " ADD " + kind + " " + a.quote(strings.TrimSpace(ix.Name)) + " (" + columns + ")"
	}
	return "CREATE " + kind + " " + a.qualifiedIndex(strings.TrimSpace(ix.Name)) + " ON " + a.table + " (" + columns + ")"
}

// validateDesign rejects drafts that no engine would accept, before any
// statement is rendered.
func validateDesign(want models.TableDesign) error {
	if len(want.Columns) == 0 {
		return fmt.Errorf("a table needs at least one field")
	}
	columns := map[string]bool{}
	// Indexes may still name a renamed field by its old name; the rename map
	// below rewrites them, so both spellings are accepted here.
	originals := map[string]bool{}
	for _, c := range want.Columns {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return fmt.Errorf("every field needs a name")
		}
		if strings.TrimSpace(c.DataType) == "" {
			return fmt.Errorf("field %s needs a type", name)
		}
		key := strings.ToLower(name)
		if columns[key] {
			return fmt.Errorf("two fields are called %s", name)
		}
		columns[key] = true
		if original := strings.TrimSpace(c.OriginalName); original != "" {
			originals[strings.ToLower(original)] = true
		}
	}
	indexes := map[string]bool{}
	for _, ix := range want.Indexes {
		name := strings.TrimSpace(ix.Name)
		if name == "" {
			return fmt.Errorf("every index needs a name")
		}
		if indexes[strings.ToLower(name)] {
			return fmt.Errorf("two indexes are called %s", name)
		}
		indexes[strings.ToLower(name)] = true
		if len(ix.Columns) == 0 {
			return fmt.Errorf("index %s does not name a field", name)
		}
		for _, column := range ix.Columns {
			key := strings.ToLower(strings.TrimSpace(column))
			if !columns[key] && !originals[key] {
				return fmt.Errorf("index %s refers to the unknown field %s", name, column)
			}
		}
	}
	return nil
}

// matchColumn pairs a draft field with the catalog column it came from. The
// designer sets OriginalName for fields that already existed, but a draft that
// only carries the (unchanged) name still matches, so a hand written draft
// works too.
func matchColumn(columns []models.ColumnInfo, used []bool, want models.DesignColumn) int {
	if strings.TrimSpace(want.OriginalName) != "" {
		for i, c := range columns {
			if !used[i] && sameName(c.Name, want.OriginalName) {
				return i
			}
		}
	}
	for i, c := range columns {
		if !used[i] && sameName(c.Name, want.Name) {
			return i
		}
	}
	return -1
}

func matchIndex(indexes []models.IndexInfo, used []bool, want models.DesignIndex) int {
	if strings.TrimSpace(want.OriginalName) != "" {
		for i, ix := range indexes {
			if !used[i] && sameName(ix.Name, want.OriginalName) {
				return i
			}
		}
	}
	for i, ix := range indexes {
		if !used[i] && sameName(ix.Name, want.Name) {
			return i
		}
	}
	return -1
}

// columnTypeOf prefers the full type text (varchar(255)) over the bare type
// name, so a draft that keeps the pre-filled value compares equal.
func columnTypeOf(c models.ColumnInfo) string {
	if strings.TrimSpace(c.ColumnType) != "" {
		return c.ColumnType
	}
	return c.DataType
}

func ptrText(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// currentNotNull and designNotNull answer the same question on both sides of the
// diff: does the engine enforce NOT NULL on this column?
//
// A primary key column always does, even when the catalog says otherwise —
// SQLite reports `id INTEGER PRIMARY KEY` as nullable, because the primary key
// itself is what rejects NULLs there.
func currentNotNull(c models.ColumnInfo) bool { return !c.Nullable || c.PrimaryKey }

func designNotNull(c models.DesignColumn) bool { return !c.Nullable || c.PrimaryKey }

func isNullLiteral(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, "null")
}

func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func sameText(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameName(a[i], b[i]) {
			return false
		}
	}
	return true
}

func escapeString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
