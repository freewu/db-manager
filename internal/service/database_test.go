package service

import (
	"context"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Creating a database is the one place where the *session's* driver decides
// what the window looks like: the same request is a charset picker on MySQL, an
// encoding and locale picker on PostgreSQL, and just a name on MongoDB. The
// manager never guesses which one it is talking to — it asks the connection,
// and says so plainly when the connection has no such capability (SQLite).

func TestCreateDatabaseGoesThroughTheSessionConn(t *testing.T) {
	manager, _ := testManager(t)

	// SQLite is a file, not a server that holds databases: the capability is
	// absent, and both calls have to say that instead of rendering SQL for an
	// engine that has no CREATE DATABASE.
	if _, err := manager.DatabaseOptions("s1"); err == nil {
		t.Fatal("expected SQLite to have no database options")
	} else if !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("the error must say the capability is missing: %v", err)
	}
	if _, err := manager.PlanCreateDatabase("s1", models.CreateDatabaseRequest{Name: "shop"}); err == nil {
		t.Fatal("expected SQLite to have no CREATE DATABASE")
	} else if !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("the error must say the capability is missing: %v", err)
	}

	// Swap in a connection that has the capability, the way a real driver does.
	manager.mu.Lock()
	manager.sessions["s1"].conn = creatorConn{Conn: manager.sessions["s1"].conn}
	manager.mu.Unlock()

	options, err := manager.DatabaseOptions("s1")
	if err != nil {
		t.Fatalf("database options: %v", err)
	}
	if len(options.Charsets) != 1 || !options.Charsets[0].Default {
		t.Fatalf("the driver's answer must reach the caller: %+v", options)
	}

	plan, err := manager.PlanCreateDatabase("s1", models.CreateDatabaseRequest{Name: "shop"})
	if err != nil {
		t.Fatalf("plan create database: %v", err)
	}
	if plan.Statement != "CREATE DATABASE `shop`" {
		t.Fatalf("statement = %q", plan.Statement)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("warnings = %+v", plan.Warnings)
	}
}

func TestCreateDatabaseNeedsAKnownSession(t *testing.T) {
	manager, _ := testManager(t)

	if _, err := manager.DatabaseOptions("nope"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
	if _, err := manager.PlanCreateDatabase("", models.CreateDatabaseRequest{Name: "shop"}); err == nil {
		t.Fatal("expected an error for a missing session")
	}
}

// creatorConn adds drivers.DatabaseCreator to an otherwise plain connection.
type creatorConn struct {
	drivers.Conn
}

func (creatorConn) DatabaseOptions(context.Context) (*models.DatabaseOptions, error) {
	return &models.DatabaseOptions{
		Charsets:       []models.DatabaseCharset{{Name: "utf8mb4", Default: true}},
		CharsetLabel:   "Character set",
		CollationLabel: "Collation",
	}, nil
}

func (creatorConn) CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	return models.DatabasePlan{
		Statement: "CREATE DATABASE `" + req.Name + "`",
		Warnings:  []string{"this is a fixture"},
	}, nil
}
