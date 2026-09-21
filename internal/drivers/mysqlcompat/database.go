package mysqlcompat

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// Creating a database, MySQL family
//
// MySQL, MariaDB, TiDB and Doris all take `CREATE DATABASE <name>` followed by
// engine specific clauses, and the character sets differ per server and per
// version (MariaDB has no utf8mb4_0900_ai_ci, MySQL 5.7 has no utf8mb3 alias).
// The list is therefore read from the server rather than shipped here:
//
//	SHOW CHARACTER SET   Charset | Description | Default collation | Maxlen
//	SHOW COLLATION       Collation | Charset | Id | Default | Compiled | ...
//
// TiDB answers both with a static list of its own; Doris takes the statement
// but has no database level character set at all (its CREATE DATABASE only
// accepts PROPERTIES), which is why it does not use the clause half of this
// file — see the doris package.

// charsetName and collationName are what a character set / collation may look
// like. The values come from the server, but they travel through the UI and
// come back in a request, so what goes into the statement is checked here
// rather than trusted: nothing else in the statement is quoted.
var (
	charsetName   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	collationName = charsetName
)

// DatabaseOptions implements the sqlbase spec hook: it reads what this server
// accepts for a new database.
func DatabaseOptions(ctx context.Context, q sqlbase.Querier) (*models.DatabaseOptions, error) {
	// SHOW CHARACTER SET is the part that matters; a server that refuses it
	// (or a user without the privilege) fails the window instead of showing an
	// empty list, because a charset dropdown with nothing in it is a lie.
	charsetRows, err := ShowRows(ctx, q, "SHOW CHARACTER SET")
	if err != nil {
		return nil, err
	}

	// The collations are the second half of the same choice. Doris answers
	// neither SHOW CHARACTER SET nor SHOW COLLATION, and older servers may
	// refuse one of them; a charset without its collation list is still a
	// usable window, so this failure is not fatal.
	collations := map[string][]string{}
	collationRows, err := ShowRows(ctx, q, "SHOW COLLATION")
	if err != nil {
		collationRows = nil
	} else {
		collations = collationsFromShow(collationRows)
	}

	// character_set_server is the charset a new database gets when the
	// statement says nothing, so it is what the window preselects. Missing
	// (Doris, a stripped down server) simply means nothing is marked.
	serverDefault := ""
	if values, err := ShowMap(ctx, q, "SHOW VARIABLES LIKE 'character_set_server'"); err == nil {
		serverDefault = strings.TrimSpace(values["character_set_server"])
	}

	options := &models.DatabaseOptions{
		Charsets:       charsetsFromShow(charsetRows, collations, serverDefault),
		CharsetLabel:   "Character set",
		CollationLabel: "Collation",
		Hint: "The character set is stored with the database and is inherited by its tables; " +
			"the collation decides how text is compared and sorted.",
	}
	if len(collations) == 0 {
		options.CollationLabel = ""
	}
	return options, nil
}

// CreateDatabase renders `CREATE DATABASE` for the MySQL family.
//
// Only the clauses that were asked for are written: an empty charset means the
// server's own default, which is not the same as naming it (a server default
// can change, and naming it in the statement freezes it).
func CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	charset := strings.TrimSpace(req.Charset)
	collation := strings.TrimSpace(req.Collation)
	if charset != "" && !charsetName.MatchString(charset) {
		return models.DatabasePlan{}, errors.New("not a character set name: " + charset)
	}
	if collation != "" && !collationName.MatchString(collation) {
		return models.DatabasePlan{}, errors.New("not a collation name: " + collation)
	}

	statement := "CREATE DATABASE " + sqlutil.QuoteBacktick(req.Name)
	if charset != "" {
		statement += " DEFAULT CHARACTER SET " + charset
	}
	if collation != "" {
		statement += " COLLATE " + collation
	}
	return models.DatabasePlan{Statement: statement}, nil
}

// charsetsFromShow reads SHOW CHARACTER SET, attaching the collations that
// belong to each charset and marking the server's own default.
func charsetsFromShow(rows []ShowRow, collations map[string][]string, serverDefault string) []models.DatabaseCharset {
	out := make([]models.DatabaseCharset, 0, len(rows))
	for _, row := range rows {
		name := row.Get("Charset")
		if name == "" {
			continue
		}
		entry := models.DatabaseCharset{
			Name:       name,
			Default:    name == serverDefault,
			Collation:  row.Get("Default collation"),
			Collations: collations[name],
		}
		if entry.Collations == nil {
			entry.Collations = []string{}
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// The server's own default leads: it is the answer for most users and
		// it is what the window preselects.
		if out[i].Default != out[j].Default {
			return out[i].Default
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// collationsFromShow reads SHOW COLLATION into the list that goes with each
// charset, keeping the server's ordering inside a charset (its default first).
func collationsFromShow(rows []ShowRow) map[string][]string {
	byCharset := map[string][]string{}
	defaults := map[string]bool{}
	for _, row := range rows {
		charset, collation := row.Get("Charset"), row.Get("Collation")
		if charset == "" || collation == "" {
			continue
		}
		byCharset[charset] = append(byCharset[charset], collation)
		if strings.EqualFold(row.Get("Default"), "yes") {
			defaults[charset+"\x00"+collation] = true
		}
	}
	for charset, list := range byCharset {
		sort.SliceStable(list, func(i, j int) bool {
			a, b := defaults[charset+"\x00"+list[i]], defaults[charset+"\x00"+list[j]]
			if a != b {
				return a
			}
			return list[i] < list[j]
		})
	}
	return byCharset
}
