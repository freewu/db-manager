package mongodb

import (
	"context"
	"errors"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Creating a database in MongoDB
//
// There is no CREATE DATABASE: a database is a namespace that starts existing
// with its first collection, and the shell's way of naming one is `use <name>`.
// That is what this window renders, which is also why its options carry no
// character set — MongoDB has none at the database level, and documents are
// BSON whatever the collection is called.
//
// The `use` statement itself is part of the shell language in shell.go, so the
// statement the window shows can also be typed into a query tab, where it
// selects the database for the statements that follow it.

// compile time check: what this window renders is a shell statement, so the two
// halves have to agree.
var _ drivers.DatabaseCreator = (*Conn)(nil)

// DatabaseOptions implements drivers.DatabaseCreator.
func (c *Conn) DatabaseOptions(context.Context) (*models.DatabaseOptions, error) {
	return &models.DatabaseOptions{
		Charsets: []models.DatabaseCharset{},
		Hint: "MongoDB has no CREATE DATABASE: `use <name>` selects the database, and it starts " +
			"existing with its first collection. Documents are BSON, so there is no character " +
			"set or collation to pick here.",
	}, nil
}

// CreateDatabase implements drivers.DatabaseCreator.
func (c *Conn) CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	name, err := validateDatabaseName(req.Name)
	if err != nil {
		return models.DatabasePlan{}, err
	}
	return models.DatabasePlan{
		Statement: "use " + name,
		Warnings: []string{
			"Nothing is stored until the database holds a collection: " +
				"`use " + name + "` selects it, and it appears in the explorer after the first write.",
		},
	}, nil
}

// validateDatabaseName applies MongoDB's own rules for database names, because
// unlike a SQL engine there is no quoting to hide them behind: the name goes
// into the statement as written.
//
// The server enforces these too (InvalidNamespace), but only after the fact, and
// a shell statement is not parameterised — so `use a; db.dropDatabase()` would
// otherwise be two statements where the user asked for one name.
func validateDatabaseName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("the database needs a name")
	}
	if strings.ContainsAny(trimmed, "/\\. \"$*<>:|?") || strings.ContainsRune(trimmed, 0) {
		return "", errors.New("a MongoDB database name cannot contain any of / \\ . \" $ * < > : | ? or a space")
	}
	if len(trimmed) > 63 {
		return "", errors.New("a MongoDB database name is at most 63 bytes")
	}
	return trimmed, nil
}
