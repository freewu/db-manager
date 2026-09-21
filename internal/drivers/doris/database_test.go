package doris

import (
	"context"
	"strings"
	"testing"

	"dbmanager/internal/models"
)

// Doris creates databases, but nothing about them is a choice: no character
// set, no collation, only `CREATE DATABASE <name>` (and the PROPERTIES the
// cluster sets once). The window therefore shows the name and an explanation
// rather than an empty-looking charset list.

func TestCreateDatabaseIsJustTheName(t *testing.T) {
	plan, err := CreateDatabase(models.CreateDatabaseRequest{Name: "weird`name", Charset: "utf8mb4"})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if plan.Statement != "CREATE DATABASE `weird``name`" {
		t.Fatalf("statement = %q", plan.Statement)
	}
	// A charset the (generic) request carried must not leak into a statement
	// Doris would reject.
	if strings.Contains(plan.Statement, "CHARACTER SET") {
		t.Fatalf("Doris has no character set clause: %q", plan.Statement)
	}
}

func TestDatabaseOptionsExplainWhyThereIsNothingToChoose(t *testing.T) {
	options, err := DatabaseOptions(context.Background(), nil)
	if err != nil {
		t.Fatalf("database options: %v", err)
	}
	if len(options.Charsets) != 0 {
		t.Fatalf("Doris offers no character sets: %+v", options.Charsets)
	}
	if !strings.Contains(options.Hint, "no database level character set") {
		t.Fatalf("the window has to say why: %q", options.Hint)
	}
}
