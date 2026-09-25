package sqlbase

import (
	"strings"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Emptying a table and removing one are one statement each, so these tests are
// about the statement and, more than that, about what each engine is *not* told
// to do: a counter left running, and the engine that has no TRUNCATE at all.

// renderer is the shape PlanDrop and PlanTruncate share, so one helper can plan
// either of them.
type renderer func(drivers.Dialect, *models.TableStructure) (models.DesignPlan, error)

func planOne(t *testing.T, render renderer, dialect drivers.Dialect, structure *models.TableStructure) models.DesignPlan {
	t.Helper()
	plan, err := render(dialect, structure)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

// sqliteStructure is the sample table as a SQLite connection reads it: one
// attached database, no schema in between.
func sqliteStructure() *models.TableStructure {
	structure := serialStructure()
	structure.Object.Database = "main"
	structure.Object.Schema = ""
	return structure
}

func TestPlanDropIsOneStatementPerEngine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dialect   drivers.Dialect
		structure *models.TableStructure
		want      string
	}{
		{"mysql", MySQLDialect{}, serialStructure(), "DROP TABLE `shop`.`orders`"},
		{"postgres", PostgresDialect{}, serialStructure(), `DROP TABLE "public"."orders"`},
		{"sqlite", SQLiteDialect{}, sqliteStructure(), `DROP TABLE "orders"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := planOne(t, PlanDrop, tc.dialect, tc.structure)
			assertStatements(t, plan.Statements, []string{tc.want})
			// Removing a table is the end of it: nothing is added that would
			// take whatever depends on the table along, and no IF EXISTS that
			// would make a table that was already gone look like a success.
			if strings.Contains(tc.want, "CASCADE") || strings.Contains(tc.want, "IF EXISTS") {
				t.Fatalf("the drop must not soften itself: %s", tc.want)
			}
			if !plan.Destructive {
				t.Fatal("a drop is destructive, and the confirmation reads this flag")
			}
			if len(plan.Warnings) != 0 {
				t.Fatalf("there is nothing to warn about beyond the drop itself: %v", plan.Warnings)
			}
		})
	}
}

func TestPlanTruncateIsEmptyingNotDropping(t *testing.T) {
	mysql := planOne(t, PlanTruncate, MySQLDialect{}, serialStructure())
	assertStatements(t, mysql.Statements, []string{"TRUNCATE TABLE `shop`.`orders`"})
	// MySQL empties the table *and* its own AUTO_INCREMENT counter, so the next
	// row is number one again and there is nothing to say.
	if len(mysql.Warnings) != 0 {
		t.Fatalf("MySQL resets the counter on the way, so there is nothing to warn about: %v", mysql.Warnings)
	}
	if !mysql.Destructive {
		t.Fatal("every row in the table goes, which is destructive")
	}

	postgres := planOne(t, PlanTruncate, PostgresDialect{}, serialStructure())
	assertStatements(t, postgres.Statements, []string{`TRUNCATE TABLE "public"."orders"`})
}

func TestPlanTruncateSaysWhatTheEngineLeavesBehind(t *testing.T) {
	// PostgreSQL truncates, but a sequence is not part of the table it numbers:
	// RESTART IDENTITY is a change of its own, so the counter is reported
	// instead of being reset behind the user's back.
	postgres := planOne(t, PlanTruncate, PostgresDialect{}, serialStructure())
	warnings := strings.Join(postgres.Warnings, "\n")
	if len(postgres.Warnings) != 1 || !strings.Contains(warnings, "field id keeps its counter") {
		t.Fatalf("the identity counter must be reported: %v", postgres.Warnings)
	}

	// A table without a counter has nothing left behind, and no warning.
	plain := serialStructure()
	for i := range plain.Columns {
		plain.Columns[i].AutoIncrement = false
	}
	if plan := planOne(t, PlanTruncate, PostgresDialect{}, plain); len(plan.Warnings) != 0 {
		t.Fatalf("there is no counter to warn about: %v", plan.Warnings)
	}

	// SQLite has no TRUNCATE at all: the statement is a DELETE, which is a
	// different thing from what was asked for and is said out loud.
	sqlite := planOne(t, PlanTruncate, SQLiteDialect{}, sqliteStructure())
	assertStatements(t, sqlite.Statements, []string{`DELETE FROM "orders"`})
	warnings = strings.Join(sqlite.Warnings, "\n")
	if len(sqlite.Warnings) != 2 || !strings.Contains(warnings, "SQLite has no TRUNCATE") {
		t.Fatalf("the DELETE must be explained: %v", sqlite.Warnings)
	}
	if !strings.Contains(warnings, "field id keeps its counter") {
		t.Fatalf("the rowid counter is left where it was: %v", sqlite.Warnings)
	}
}

func TestPlanTableOpsRefuseWhatTheyCannotRead(t *testing.T) {
	empty := &models.TableStructure{Object: models.ObjectInfo{Name: "orders"}}

	for _, tc := range []struct {
		name     string
		render   renderer
		verb     string
		structure *models.TableStructure
		dialect  drivers.Dialect
	}{
		{"no table to drop", PlanDrop, "no table to drop", nil, MySQLDialect{}},
		{"no table to empty", PlanTruncate, "no table to empty", empty, MySQLDialect{}},
		{"doris", PlanDrop, "cannot drop doris tables yet", serialStructure(), sqlbaseDorisDialect()},
		{"doris truncate", PlanTruncate, "cannot empty doris tables yet", serialStructure(), sqlbaseDorisDialect()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.render(tc.dialect, tc.structure)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(err.Error(), tc.verb) {
				t.Fatalf("the refusal should say what it is about: %v", err)
			}
		})
	}
}
