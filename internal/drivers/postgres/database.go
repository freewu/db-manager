package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strings"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// Creating a database in PostgreSQL
//
// PostgreSQL's CREATE DATABASE is not MySQL's with different keywords, which is
// why this window is its own thing:
//
//   - the character set is an *encoding*, chosen from the list the server
//     documents rather than the list of one table;
//   - the sort order is a *locale* (LC_COLLATE / LC_CTYPE), and locale names
//     come from the operating system — "locale names are specific to the
//     operating system", says the manual — so the list below can only ever be
//     what this cluster already uses, and the window has to accept a typed
//     value;
//   - a new database clones template1, so naming an encoding or a locale is
//     only allowed together with TEMPLATE template0. The renderer adds it;
//     without it the server refuses the statement.

// pgEncodings are the encodings PostgreSQL documents for CREATE DATABASE.
//
// This list is a constant of the *engine*, not of the server: there is no
// catalog to ask (SHOW server_encoding reports the one this cluster uses, and
// pg_encoding_to_char over pg_database only lists what is already installed),
// so a curated list is the honest answer here — the same one psql's own
// documentation table gives. The server's own encoding is marked from
// template1, which is what "no clause" means.
var pgEncodings = []string{
	"UTF8", "SQL_ASCII",
	"BIG5", "EUC_CN", "EUC_JP", "EUC_JIS_2004", "EUC_KR", "EUC_TW",
	"GB18030", "GBK", "JOHAB", "KOI8R", "KOI8U", "LATIN1", "LATIN2", "LATIN3",
	"LATIN4", "LATIN5", "LATIN6", "LATIN7", "LATIN8", "LATIN9", "LATIN10",
	"MULE_INTERNAL", "SJIS", "SHIFT_JIS_2004", "UHC", "WIN866", "WIN874",
	"WIN1250", "WIN1251", "WIN1252", "WIN1253", "WIN1254", "WIN1255", "WIN1256",
	"WIN1257", "WIN1258",
}

// pgOptionName is what an encoding or a locale may contain. Those two are the
// only values in the statement that are written as string literals (the name is
// quoted as an identifier), so they are the ones worth checking: a locale is
// "en_US.UTF-8", "C", "sr_RS@latin", "de_DE.UTF-8@euro" — letters, digits and
// _ . @ + - and nothing else.
var pgOptionName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@+-]*$`)

// alwaysAvailableLocales are the locales every PostgreSQL installation can be
// created with, whatever the operating system provides.
var alwaysAvailableLocales = []string{"C", "POSIX"}

// DatabaseOptions implements the sqlbase spec hook.
func DatabaseOptions(ctx context.Context, q sqlbase.Querier) (*models.DatabaseOptions, error) {
	// template1 is the database a new one is copied from, so its encoding and
	// locale are what the server would use anyway: that is the pair to
	// preselect, and marking it is the whole reason for this query.
	//
	// `datcollate` is nullable, and a cluster this user cannot read pg_database
	// on would otherwise fail the whole window — but a genuinely broken
	// connection must still surface, so only "no such row" is tolerated.
	var defaultEncoding, defaultLocale sql.NullString
	row := q.QueryRowContext(ctx,
		"SELECT pg_encoding_to_char(encoding), datcollate FROM pg_database WHERE datname = 'template1'")
	switch err := row.Scan(&defaultEncoding, &defaultLocale); {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		// A cluster without template1 is broken, but the window should still
		// open with the full encoding list and no preselection.
	default:
		return nil, err
	}

	// Which locales this cluster already uses. It cannot be the full set (that
	// belongs to the operating system), so the window lets the user type one.
	used := []string{}
	distinct, err := q.QueryContext(ctx, "SELECT DISTINCT datcollate FROM pg_database ORDER BY 1")
	if err == nil {
		defer distinct.Close()
		for distinct.Next() {
			var locale sql.NullString
			if err := distinct.Scan(&locale); err != nil {
				break
			}
			if name := strings.TrimSpace(locale.String); name != "" {
				used = append(used, name)
			}
		}
	}

	return databaseOptions(strings.TrimSpace(defaultEncoding.String), strings.TrimSpace(defaultLocale.String), used), nil
}

// databaseOptions assembles the window's choices from the two facts read off
// the server — what template1 uses, and which locales the cluster already has —
// plus the encodings PostgreSQL documents. Any of them can arrive empty (a
// cluster this user cannot read pg_database on), and the window then simply
// offers the documented encodings and the always-available locales.
func databaseOptions(defaultEncoding, defaultLocale string, used []string) *models.DatabaseOptions {
	locales := append([]string{}, alwaysAvailableLocales...)
	seen := map[string]bool{"C": true, "POSIX": true}
	for _, locale := range append(append([]string{}, used...), defaultLocale) {
		name := strings.TrimSpace(locale)
		if name != "" && !seen[name] {
			seen[name] = true
			locales = append(locales, name)
		}
	}
	sort.SliceStable(locales, func(i, j int) bool {
		// The cluster's own locale leads, since it is what the window
		// preselects; then UTF-8 locales, then the rest alphabetically. C is
		// almost never what someone wants for a new database.
		a, b := locales[i], locales[j]
		if (a == defaultLocale) != (b == defaultLocale) {
			return a == defaultLocale
		}
		aUTF, bUTF := strings.Contains(a, "UTF"), strings.Contains(b, "UTF")
		if aUTF != bUTF {
			return aUTF
		}
		return a < b
	})

	charsets := make([]models.DatabaseCharset, 0, len(pgEncodings)+1)
	for _, encoding := range pgEncodings {
		charsets = append(charsets, models.DatabaseCharset{
			Name:    encoding,
			Default: strings.EqualFold(encoding, defaultEncoding),
		})
	}
	// An encoding the cluster uses that this list forgot still belongs in the
	// window: the server is the authority, not this table.
	if defaultEncoding != "" && !containsEncoding(pgEncodings, defaultEncoding) {
		charsets = append(charsets, models.DatabaseCharset{Name: defaultEncoding, Default: true})
	}

	return &models.DatabaseOptions{
		Charsets:          charsets,
		Collations:        locales,
		CharsetLabel:      "Encoding",
		CollationLabel:    "Locale",
		CollationEditable: true,
		Hint: "A new database is copied from template1, so naming an encoding or a locale also " +
			"says TEMPLATE template0 — otherwise PostgreSQL refuses the statement. Locale names " +
			"come from the server's operating system, so a typed value is fine.",
	}
}

// CreateDatabase renders `CREATE DATABASE` for PostgreSQL.
//
// ENCODING is written as a string constant and the locale as LOCALE, which sets
// LC_COLLATE and LC_CTYPE together — the form the manual's own example uses.
func CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	encoding := strings.TrimSpace(req.Charset)
	locale := strings.TrimSpace(req.Collation)
	if encoding != "" && !pgOptionName.MatchString(encoding) {
		return models.DatabasePlan{}, errors.New("not an encoding name: " + encoding)
	}
	if locale != "" && !pgOptionName.MatchString(locale) {
		return models.DatabasePlan{}, errors.New("not a locale name: " + locale)
	}

	statement := "CREATE DATABASE " + sqlutil.QuoteDouble(req.Name)
	if encoding != "" {
		statement += " ENCODING '" + encoding + "'"
	}
	if locale != "" {
		statement += " LOCALE '" + locale + "'"
	}
	if encoding != "" || locale != "" {
		statement += " TEMPLATE template0"
	}
	return models.DatabasePlan{Statement: statement}, nil
}

// containsEncoding reports whether the documented list already has an encoding,
// compared the way PostgreSQL compares encoding names (case insensitively).
func containsEncoding(list []string, name string) bool {
	for _, entry := range list {
		if strings.EqualFold(entry, name) {
			return true
		}
	}
	return false
}
