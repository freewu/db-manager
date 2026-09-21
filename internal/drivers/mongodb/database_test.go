package mongodb

import (
	"context"
	"strings"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// MongoDB has no CREATE DATABASE, so the "New database" window renders `use` —
// the statement a user would type in the shell, and one this driver can also
// run.

func TestCreateDatabaseRendersUse(t *testing.T) {
	plan, err := (&Conn{}).CreateDatabase(models.CreateDatabaseRequest{Name: "shop"})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if plan.Statement != "use shop" {
		t.Fatalf("statement = %q", plan.Statement)
	}
	// The window says what the statement actually does: `use` alone stores
	// nothing, which is the one thing a user of a CREATE DATABASE button would
	// otherwise expect it to do.
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "first write") {
		t.Fatalf("warnings = %+v", plan.Warnings)
	}
}

func TestCreateDatabaseRefusesNamesMongoDBItselfRefuses(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"", "needs a name"},
		{"   ", "needs a name"},
		// A name goes into the statement as written — there is no quoting to
		// hide a second statement behind.
		{"shop; db.dropDatabase()", "cannot contain"},
		{"shop\"", "cannot contain"},
		{"my db", "cannot contain"},
		{"a/b", "cannot contain"},
		{"a.b", "cannot contain"},
		{strings.Repeat("x", 64), "at most 63 bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := (&Conn{}).CreateDatabase(models.CreateDatabaseRequest{Name: tc.name})
			if err == nil {
				t.Fatalf("expected %q to be refused, got %q", tc.name, plan.Statement)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// A `use` statement never reaches the server: it moves the script to another
// database, which is why a script that is nothing but a `use` runs without a
// client at all.
func TestExecuteReportsAUseWithoutContactingTheServer(t *testing.T) {
	result, err := (&Conn{}).Execute(context.Background(), drivers.ExecRequest{SQL: "use demo"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0] != "#1: switched to database demo" {
		t.Fatalf("messages = %v", result.Messages)
	}
	if result.SQL != "use demo" {
		t.Fatalf("the result should point at the statement that produced it: %q", result.SQL)
	}
	if result.HasResultSet {
		t.Fatalf("nothing was read: %+v", result)
	}
}
