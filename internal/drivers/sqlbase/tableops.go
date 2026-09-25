package sqlbase

import (
	"fmt"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// The explorer's table menu asks for two things that are each one statement:
// emptying a table and removing it. Both render from the live catalog, the way a
// copy does, so nothing the window holds can disagree with what the engine is
// actually asked to do — and both come with whatever the engine leaves behind or
// refuses, spelled out in Warnings rather than discovered halfway through.

// PlanDrop renders the statement that removes a table.
//
// One DROP TABLE, and no clause that would soften it: no IF EXISTS, which would
// turn "it is gone" and "it was already gone" into the same success, and no
// CASCADE, which would quietly take the views and constraints that depend on the
// table with it. An engine that refuses because something depends on the table
// names that something, and that sentence is a better warning than anything this
// renderer could guess.
func PlanDrop(d drivers.Dialect, current *models.TableStructure) (models.DesignPlan, error) {
	table, err := tableToWrite(d, current, "drop")
	if err != nil {
		return emptyPlan(), err
	}
	return models.DesignPlan{
		Statements: []string{"DROP TABLE " + table},
		Warnings:   []string{},
		// Nothing about a drop is recoverable, and the flag is what the
		// confirmation is drawn from: it does not read the verb.
		Destructive: true,
	}, nil
}

// PlanTruncate renders the script that empties a table: every row goes, the
// table, its fields and its indexes stay.
//
// Only the MySQL family spells this in one statement that means one thing.
// PostgreSQL truncates but leaves the sequence behind an identity or serial
// column where it was, and SQLite has no TRUNCATE at all and is given a DELETE
// FROM; in both cases the counter that is *not* reset is said out loud, because
// a table that empties and then hands out numbers that were just deleted is
// exactly the kind of surprise a preview exists to prevent.
//
// Resetting the counter is deliberately left to the user rather than added as a
// second statement. On PostgreSQL that would mean TRUNCATE ... RESTART IDENTITY,
// which is a change of its own and a different thing from what was asked for; on
// SQLite it would mean DELETE FROM sqlite_sequence, and that table only exists in
// a database that has used AUTOINCREMENT at least once — the statement would
// fail on the tables that never did, *after* the rows were already gone. A
// script that reports what it did not do is worth more than one that half-runs.
func PlanTruncate(d drivers.Dialect, current *models.TableStructure) (models.DesignPlan, error) {
	table, err := tableToWrite(d, current, "empty")
	if err != nil {
		return emptyPlan(), err
	}

	switch d.Name() {
	case models.DriverMySQL, models.DriverTiDB:
		// TRUNCATE resets the table's own AUTO_INCREMENT counter on the way,
		// and there is nothing else it leaves behind.
		return models.DesignPlan{
			Statements:  []string{"TRUNCATE TABLE " + table},
			Warnings:    []string{},
			Destructive: true,
		}, nil
	case models.DriverPostgres:
		return models.DesignPlan{
			Statements:  []string{"TRUNCATE TABLE " + table},
			Warnings:    warnNonEmpty(counterWarning(current)),
			Destructive: true,
		}, nil
	case models.DriverSQLite:
		warnings := []string{
			"SQLite has no TRUNCATE: the rows are removed with DELETE FROM, which leaves the table, its fields and its indexes in place",
		}
		return models.DesignPlan{
			Statements:  []string{"DELETE FROM " + table},
			Warnings:    append(warnings, warnNonEmpty(counterWarning(current))...),
			Destructive: true,
		}, nil
	default:
		return emptyPlan(), fmt.Errorf("the explorer cannot empty %s tables yet", d.Name())
	}
}

// tableToWrite validates the table a one-statement script is about to be
// rendered for and returns its qualified name.
//
// The engines it accepts are the ones that have a renderer here — which is the
// same set the table designer can write for, and the reason the explorer's menu
// asks for the same capability before it offers these two entries at all.
func tableToWrite(d drivers.Dialect, current *models.TableStructure, verb string) (string, error) {
	if current == nil || len(current.Columns) == 0 {
		return "", fmt.Errorf("there is no table to %s", verb)
	}
	switch d.Name() {
	case models.DriverMySQL, models.DriverTiDB, models.DriverPostgres, models.DriverSQLite:
		return d.Qualify(current.Object.Database, current.Object.Schema, current.Object.Name), nil
	default:
		return "", fmt.Errorf("the explorer cannot %s %s tables yet", verb, d.Name())
	}
}

// counterWarning is the sentence for a table whose number-generating fields keep
// their counters, and is empty for a table without any.
//
// It covers both halves of the promise: what was not reset, and what that means
// for the next row that is written.
func counterWarning(current *models.TableStructure) string {
	names := make([]string, 0, 2)
	for _, c := range current.Columns {
		if c.AutoIncrement {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return fmt.Sprintf("field %s keeps its counter: emptying the table does not reset it, so the rows written afterwards carry on from the numbers that were just removed",
			names[0])
	}
	return fmt.Sprintf("fields %s keep their counters: emptying the table does not reset them, so the rows written afterwards carry on from the numbers that were just removed",
			strings.Join(names, ", "))
}

// emptyPlan is the plan shape every renderer starts from: no statements, no
// warnings, nothing dropped. It exists so an error return never carries a nil
// slice the window would have to guard against.
func emptyPlan() models.DesignPlan {
	return models.DesignPlan{Statements: []string{}, Warnings: []string{}}
}

// warnNonEmpty drops an empty sentence, so a table with no counter to warn
// about gets no warning rather than a blank line.
func warnNonEmpty(warning string) []string {
	if warning == "" {
		return []string{}
	}
	return []string{warning}
}
