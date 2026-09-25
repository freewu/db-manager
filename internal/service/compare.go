// Comparing two databases, and turning the difference into a script.
//
// This is the one feature that reads two sessions at once: both sides are read
// live at the moment the comparison runs, because a schema is exactly the thing
// that must not be a snapshot taken when a window was opened. What comes back is
// the difference in the designer's vocabulary (fields, their types and flags,
// and the indexes that are not the primary key), which is also the vocabulary
// the generated script is written in — so the script changes exactly what the
// comparison named, and nothing else.
//
// Nothing here writes. ApplySyncDatabase plans the same work again and hands it
// to applyPlan, so the statements that run are rendered from the catalog that is
// current when they run, not from the preview a user read a minute ago.
package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// compareTimeout bounds reading both sides. A comparison reads every table's
// structure on both sides, so it is a good deal more work than opening one
// window: the limit is generous, and it is one limit for the whole comparison
// rather than one per table, so a very large schema fails as a whole instead of
// trickling in.
const compareTimeout = 3 * time.Minute

// CompareDatabases reads two namespaces and answers, table by table, how they
// differ. Nothing is executed.
func (m *Manager) CompareDatabases(req models.CompareRequest) (*models.SchemaCompare, error) {
	left, right, err := m.comparableSides(req.Left, req.Right)
	if err != nil {
		return nil, err
	}

	ctx, cancel := m.ctx(compareTimeout)
	defer cancel()

	leftTables, leftSkipped, err := sideTables(ctx, left, req.Left)
	if err != nil {
		return nil, err
	}
	rightTables, rightSkipped, err := sideTables(ctx, right, req.Right)
	if err != nil {
		return nil, err
	}

	compare := &models.SchemaCompare{
		Left:       req.Left,
		Right:      req.Right,
		LeftLabel:  sideLabel(left, req.Left),
		RightLabel: sideLabel(right, req.Right),
		Driver:     left.info().Driver,
		Tables:     []models.TableDiff{},
		Warnings:   []string{},
	}
	compare.Warnings = append(compare.Warnings, leftSkipped...)
	compare.Warnings = append(compare.Warnings, rightSkipped...)

	for _, name := range comparedNames(leftTables, rightTables) {
		leftObject, hasLeft := leftTables[name]
		rightObject, hasRight := rightTables[name]
		diff, err := m.diffTable(ctx, left, req.Left, right, req.Right,
			objectOrNil(leftObject, hasLeft), objectOrNil(rightObject, hasRight))
		if err != nil {
			return nil, err
		}
		compare.Tables = append(compare.Tables, diff)
	}
	return compare, nil
}

// PlanSyncDatabase renders the script that would bring the right side in line
// with the left. Nothing is executed.
func (m *Manager) PlanSyncDatabase(req models.SyncDatabaseRequest) (*models.DesignPlan, error) {
	left, right, err := m.comparableSides(req.Left, req.Right)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(compareTimeout)
	defer cancel()
	return m.planSync(ctx, left, right, req)
}

// ApplySyncDatabase plans the script again and runs it against the right side.
//
// The right side is the one that changes, so it is the one that must not be
// read-only — the left may well be a production database being used as the
// reference, and reading it is all this ever does to it.
func (m *Manager) ApplySyncDatabase(req models.SyncDatabaseRequest) (*models.DesignResult, error) {
	left, right, err := m.comparableSides(req.Left, req.Right)
	if err != nil {
		return nil, err
	}
	if right.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}

	ctx, cancel := m.ctx(compareTimeout)
	plan, err := m.planSync(ctx, left, right, req)
	cancel()
	if err != nil {
		return nil, err
	}

	// The entry points at the database, not at a table: one run can touch many
	// tables, and naming one of them would be a lie about the others.
	return m.applyPlan(right, statementPlace{
		database: req.Right.Database,
		schema:   req.Right.Schema,
		source:   models.ChangeSourceCompare,
	}, plan), nil
}

// --- planning --------------------------------------------------------------

// planSync walks the two object lists and renders one plan per table, in the
// order the statements have to run: creations, then changes, then removals.
//
// The order matters because the script runs top to bottom on an engine that
// cannot roll it back. A table must exist before it can be altered, and removals
// go last so that a run which fails halfway has created and fixed more than it
// has destroyed.
func (m *Manager) planSync(
	ctx context.Context,
	left, right *session,
	req models.SyncDatabaseRequest,
) (*models.DesignPlan, error) {
	plan := &models.DesignPlan{Statements: []string{}, Warnings: []string{}}

	leftTables, leftSkipped, err := sideTables(ctx, left, req.Left)
	if err != nil {
		return nil, err
	}
	rightTables, rightSkipped, err := sideTables(ctx, right, req.Right)
	if err != nil {
		return nil, err
	}
	plan.Warnings = append(plan.Warnings, leftSkipped...)
	plan.Warnings = append(plan.Warnings, rightSkipped...)

	d := right.conn.Dialect()
	var creates, alters, drops []models.DesignPlan

	for _, name := range comparedNames(leftTables, rightTables) {
		leftObject, hasLeft := leftTables[name]
		rightObject, hasRight := rightTables[name]
		switch {
		case !hasRight:
			part, err := m.createPart(ctx, d, left, req.Left, leftObject)
			if err != nil {
				return nil, err
			}
			creates = append(creates, part)
		case !hasLeft:
			if !req.DropExtra {
				plan.Warnings = append(plan.Warnings,
					fmt.Sprintf("table %s exists only on the right and was left alone", rightObject.Name))
				continue
			}
			part, err := m.dropPart(ctx, d, right, req.Right, rightObject)
			if err != nil {
				return nil, err
			}
			drops = append(drops, part)
		default:
			part, err := m.alterPart(ctx, d, left, req.Left, leftObject, right, req.Right, rightObject)
			if err != nil {
				return nil, err
			}
			alters = append(alters, part)
		}
	}

	for _, group := range [][]models.DesignPlan{creates, alters, drops} {
		for _, part := range group {
			plan.Statements = append(plan.Statements, part.Statements...)
			plan.Warnings = append(plan.Warnings, part.Warnings...)
			plan.Destructive = plan.Destructive || part.Destructive
		}
	}
	return plan, nil
}

// createPart plans the table the left has and the right does not.
func (m *Manager) createPart(
	ctx context.Context,
	d drivers.Dialect,
	left *session,
	leftSide models.CompareSide,
	object models.ObjectInfo,
) (models.DesignPlan, error) {
	structure, err := left.conn.Structure(ctx, leftSide.Database, leftSide.Schema, object.Name)
	if err != nil {
		return models.DesignPlan{}, err
	}
	design, warnings := sqlbase.DesignOfStructure(d, structure, object.Name)
	part, err := sqlbase.PlanCreate(d, design)
	if err != nil {
		return models.DesignPlan{}, apperr.Wrap(apperr.CodeInvalidConfig, err,
			"cannot plan the creation of table %s", object.Name)
	}
	part.Warnings = tableWarnings(object.Name, append(warnings, part.Warnings...))
	return part, nil
}

// dropPart plans the removal of a table the right has and the left does not.
func (m *Manager) dropPart(
	ctx context.Context,
	d drivers.Dialect,
	right *session,
	rightSide models.CompareSide,
	object models.ObjectInfo,
) (models.DesignPlan, error) {
	structure, err := right.conn.Structure(ctx, rightSide.Database, rightSide.Schema, object.Name)
	if err != nil {
		return models.DesignPlan{}, err
	}
	part, err := sqlbase.PlanDrop(d, structure)
	if err != nil {
		return models.DesignPlan{}, apperr.Wrap(apperr.CodeInvalidConfig, err,
			"cannot plan the removal of table %s", object.Name)
	}
	part.Warnings = tableWarnings(object.Name, part.Warnings)
	return part, nil
}

// alterPart plans the changes to a table both sides have.
//
// The design comes from the left, but the target's own counters are kept: a
// PostgreSQL serial column's default names the sequence of the table it was read
// from, and an ALTER that wrote the left's sequence name into the right table
// would point it at a counter it does not own.
func (m *Manager) alterPart(
	ctx context.Context,
	d drivers.Dialect,
	left *session,
	leftSide models.CompareSide,
	leftObject models.ObjectInfo,
	right *session,
	rightSide models.CompareSide,
	rightObject models.ObjectInfo,
) (models.DesignPlan, error) {
	leftStructure, err := left.conn.Structure(ctx, leftSide.Database, leftSide.Schema, leftObject.Name)
	if err != nil {
		return models.DesignPlan{}, err
	}
	rightStructure, err := right.conn.Structure(ctx, rightSide.Database, rightSide.Schema, rightObject.Name)
	if err != nil {
		return models.DesignPlan{}, err
	}

	design, warnings := sqlbase.DesignOfStructure(d, leftStructure, leftObject.Name)
	sqlbase.KeepEngineCounters(&design, rightStructure)

	part, err := sqlbase.PlanAlter(d, rightStructure, design)
	if err != nil {
		return models.DesignPlan{}, apperr.Wrap(apperr.CodeInvalidConfig, err,
			"cannot plan the changes to table %s", leftObject.Name)
	}
	part.Warnings = tableWarnings(leftObject.Name, append(warnings, part.Warnings...))
	return part, nil
}

// tableWarnings marks a plan's warnings with the table they are about.
//
// One script can span the whole database, and a sentence like "SQLite cannot
// change a primary key" is not much use without knowing which table. The
// design's own warnings and the renderer's are marked the same way, because to
// the reader they are the same list.
func tableWarnings(name string, warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, name+": "+warning)
	}
	return out
}

// --- reading the two sides -------------------------------------------------

// comparableSides resolves both sides and refuses the pairs that could not be
// compared honestly.
//
// Same engine is the rule the page is built on: comparing a MySQL table with a
// PostgreSQL one would need a translation table rather than a diff, and a script
// generated from it would be guesswork. A driver this build ships no DDL for is
// refused too, because the comparison's whole point is the script at the end.
func (m *Manager) comparableSides(left, right models.CompareSide) (*session, *session, error) {
	l, err := m.session(left.SessionID)
	if err != nil {
		return nil, nil, err
	}
	r, err := m.session(right.SessionID)
	if err != nil {
		return nil, nil, err
	}

	if sameScope(left, right) {
		return nil, nil, apperr.New(apperr.CodeInvalidConfig,
			"both sides are the same database; pick two different ones")
	}
	if l.info().Driver != r.info().Driver {
		return nil, nil, apperr.New(apperr.CodeInvalidConfig,
			"the two sides must be the same engine: %s and %s", l.info().Driver, r.info().Driver)
	}
	if !comparableDriver(r.conn.Dialect().Name()) {
		return nil, nil, apperr.New(apperr.CodeUnsupported,
			"%s databases cannot be compared yet", r.info().Driver)
	}
	return l, r, nil
}

// comparableDriver reports whether an engine's tables can be compared and
// scripted. It is the designer's list: the same four engines whose tables the
// structure page can write DDL for.
func comparableDriver(t models.DriverType) bool {
	switch t {
	case models.DriverMySQL, models.DriverTiDB, models.DriverPostgres, models.DriverSQLite:
		return true
	}
	return false
}

// sameScope reports whether two sides name the same database.
//
// Both halves have to match: the same session pointed at two databases is a real
// comparison, and two sessions of the same server pointed at the same database
// through the same login is not — it would answer "identical" by construction.
// Two sessions of the same server pointed at the same database are left alone,
// because a different login can see a different subset of it.
func sameScope(left, right models.CompareSide) bool {
	return left.SessionID == right.SessionID &&
		strings.EqualFold(strings.TrimSpace(left.Database), strings.TrimSpace(right.Database)) &&
		strings.EqualFold(strings.TrimSpace(left.Schema), strings.TrimSpace(right.Schema))
}

// sideTables reads one side's object list and keeps the tables from it.
//
// The kinds that are dropped are reported rather than ignored: a comparison that
// silently skipped three views would let "no differences" be read as "the two
// databases agree", which they do not.
func sideTables(
	ctx context.Context,
	s *session,
	side models.CompareSide,
) (map[string]models.ObjectInfo, []string, error) {
	objects, err := s.conn.Objects(ctx, side.Database, side.Schema)
	if err != nil {
		return nil, nil, err
	}

	tables := map[string]models.ObjectInfo{}
	counts := map[models.ObjectKind]int{}
	for _, object := range objects {
		if object.Kind != models.KindTable {
			counts[object.Kind]++
			continue
		}
		tables[strings.ToLower(strings.TrimSpace(object.Name))] = object
	}

	warnings := []string{}
	if len(counts) > 0 {
		kinds := make([]string, 0, len(counts))
		for kind, n := range counts {
			kinds = append(kinds, countOf(n, kind))
		}
		sort.Strings(kinds)
		warnings = append(warnings, fmt.Sprintf("only tables are compared; this side also holds %s",
			strings.Join(kinds, ", ")))
	}
	return tables, warnings, nil
}

// diffTable compares one table across the two sides, reading each side's
// structure. A table only one side has is read from that side alone.
func (m *Manager) diffTable(
	ctx context.Context,
	left *session,
	leftSide models.CompareSide,
	right *session,
	rightSide models.CompareSide,
	leftObject *models.ObjectInfo,
	rightObject *models.ObjectInfo,
) (models.TableDiff, error) {
	var leftStructure, rightStructure *models.TableStructure
	if leftObject != nil {
		structure, err := left.conn.Structure(ctx, leftSide.Database, leftSide.Schema, leftObject.Name)
		if err != nil {
			return models.TableDiff{}, err
		}
		leftStructure = structure
	}
	if rightObject != nil {
		structure, err := right.conn.Structure(ctx, rightSide.Database, rightSide.Schema, rightObject.Name)
		if err != nil {
			return models.TableDiff{}, err
		}
		rightStructure = structure
	}
	return sqlbase.DiffTable(leftStructure, rightStructure), nil
}

// objectOrNil turns a map lookup into the pointer the reading code wants, so a
// table that is on one side only is a nil structure rather than a zero one.
func objectOrNil(object models.ObjectInfo, found bool) *models.ObjectInfo {
	if !found {
		return nil
	}
	return &object
}

// comparedNames is the union of both sides' table names, in the order the
// comparison lists them.
func comparedNames(left, right map[string]models.ObjectInfo) []string {
	seen := map[string]bool{}
	names := make([]string, 0, len(left)+len(right))
	for name := range left {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for name := range right {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// sideLabel names a side the way the window shows it: the connection it was
// reached through, then the database and schema inside it.
func sideLabel(s *session, side models.CompareSide) string {
	info := s.info()
	parts := []string{info.Name}
	if database := strings.TrimSpace(side.Database); database != "" {
		parts = append(parts, database)
	}
	if schema := strings.TrimSpace(side.Schema); schema != "" {
		parts = append(parts, schema)
	}
	return strings.Join(parts, " · ")
}

// countOf is "1 view" / "2 views", with the kind's own spelling.
func countOf(n int, kind models.ObjectKind) string {
	name := strings.ReplaceAll(string(kind), "_", " ")
	if n == 1 {
		return "1 " + name
	}
	return strconv.Itoa(n) + " " + name + "s"
}
