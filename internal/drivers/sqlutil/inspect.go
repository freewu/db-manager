package sqlutil

import (
	"strconv"
	"strings"

	"dbmanager/internal/models"
)

// Statement kinds returned by Analyze.
const (
	KindQuery   = "query"
	KindDDL     = "ddl"
	KindDML     = "dml"
	KindUnknown = "unknown"
)

// ddlLeaders are the leading keywords that change the schema.
var ddlLeaders = map[string]bool{
	"create":   true,
	"alter":    true,
	"drop":     true,
	"truncate": true,
	"rename":   true,
	"comment":  true,
	"grant":    true,
	"revoke":   true,
	"reindex":  true,
	"vacuum":   true,
	"analyze":  true,
	"attach":   true,
	"detach":   true,
}

// dmlLeaders are the leading keywords that change data.
var dmlLeaders = map[string]bool{
	"insert":  true,
	"update":  true,
	"delete":  true,
	"merge":   true,
	"replace": true,
	"copy":    true,
	"load":    true,
	"upsert":  true,
}

// Analyze inspects a script without running it and reports what each statement
// would do.
//
// The DDL editor uses this to show a dry run before anything is executed: which
// statements are DDL, which mutate data, and which of them would be refused on
// a read-only connection. It is a heuristic (no engine is contacted), so it
// errs towards warning: a false positive costs a glance, a false negative costs
// a dropped table.
func Analyze(sql string, readOnly bool) models.ScriptAnalysis {
	analysis := models.ScriptAnalysis{
		Statements: []models.ScriptStatement{},
		Warnings:   []string{},
		ReadOnly:   readOnly,
	}

	for i, statement := range SplitStatements(sql) {
		entry := Inspect(statement, i)

		if entry.Destructive {
			analysis.Destructive = true
		}
		if entry.Kind == KindUnknown && strings.TrimSpace(stripComments(statement)) != "" {
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": unrecognised statement, it will be sent to the server as-is")
		}
		if entry.Destructive {
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": "+entry.Reason)
		}

		if readOnly && entry.Kind != KindQuery {
			analysis.Refused++
		}
		analysis.Statements = append(analysis.Statements, entry)
	}

	if readOnly && analysis.Refused > 0 {
		analysis.Warnings = append([]string{
			"this connection is read-only: " + strconv.Itoa(analysis.Refused) + " write statement(s) will be refused",
		}, analysis.Warnings...)
	}
	return analysis
}

// Inspect is what Analyze works out about one statement it has already split
// out of a script.
//
// index is the statement's zero-based position in that script, which is what
// the warnings and the preview list number their lines by. Callers that read a
// script from a file rather than a string use it to look at statements one at a
// time, without building the whole analysis in memory.
func Inspect(statement string, index int) models.ScriptStatement {
	entry := models.ScriptStatement{
		Index:   index,
		Preview: preview(statement),
		// A fragment that is only comments cannot normally reach this point,
		// since SplitStatements drops those; it would come back unknown.
		Kind: KindOf(statement),
		// The statement as written, for the change log to record if a window
		// runs it.
		SQL: strings.TrimSpace(statement),
	}

	entry.Destructive, entry.Reason = destructive(stripComments(statement))
	return entry
}

// KindOf classifies a single statement the way Analyze does: KindQuery, KindDDL,
// KindDML, or KindUnknown when the leading keyword is not one this build knows.
//
// It is the same judgement on one statement, for callers that have already split
// the script themselves — the change log, which records the statements that
// change something and leaves the reads alone.
func KindOf(statement string) string {
	word := LeadingWord(statement)
	switch {
	case word == "":
		return KindUnknown
	case queryLeaders[word]:
		return KindQuery
	case ddlLeaders[word]:
		return KindDDL
	case dmlLeaders[word]:
		return KindDML
	}
	return KindUnknown
}

// LeadingWord returns a statement's first keyword, lowercased, with comments and
// leading whitespace skipped. It is how the change log says what a statement
// does ("create", "alter", "drop", …) without pretending to have parsed it.
func LeadingWord(statement string) string {
	return strings.ToLower(leadingWord(stripComments(statement)))
}

// createsTableModifiers are the words that may stand between CREATE and TABLE.
var createsTableModifiers = map[string]bool{
	"or": true, "replace": true,
	"temp": true, "temporary": true, "global": true, "local": true,
	"unlogged": true, "external": true, "volatile": true, "multiset": true,
}

// CreatesTable reports whether a statement creates a table.
//
// It exists for the change log. An entry keeps the object a statement was
// applied to, and a table that is being created is not one: nothing points at it
// yet, and the name it will have is written in the statement itself. Everything
// else the word CREATE introduces — an index, a view, a database — is a change
// to something that already has a name, and is recorded against it.
func CreatesTable(statement string) bool {
	fields := strings.Fields(strings.ToLower(stripComments(statement)))
	if len(fields) < 2 || fields[0] != "create" {
		return false
	}
	for _, word := range fields[1:] {
		if word == "table" {
			return true
		}
		if !createsTableModifiers[word] {
			return false
		}
	}
	return false
}

// destructive reports whether a statement can lose schema or data, plus the
// sentence explaining why.
func destructive(cleaned string) (bool, string) {
	if cleaned == "" {
		return false, ""
	}
	lower := strings.ToLower(cleaned)
	flat := " " + strings.Join(strings.Fields(lower), " ") + " "

	switch strings.ToLower(leadingWord(lower)) {
	case "drop":
		if strings.HasPrefix(flat, " drop database") || strings.HasPrefix(flat, " drop schema") {
			return true, "DROP DATABASE/SCHEMA removes the whole namespace"
		}
		return true, "DROP removes an object and everything in it"
	case "truncate":
		return true, "TRUNCATE empties the table"
	case "delete":
		if !strings.Contains(flat, " where ") {
			return true, "DELETE without WHERE removes every row"
		}
	case "update":
		if !strings.Contains(flat, " where ") {
			return true, "UPDATE without WHERE rewrites every row"
		}
	case "alter":
		if strings.Contains(flat, " drop ") {
			return true, "ALTER … DROP removes a column, constraint or index"
		}
	}
	return false, ""
}

// preview is the one-line summary the analysis panel shows.
func preview(statement string) string {
	const limit = 140
	line := stripComments(statement)
	line = strings.Join(strings.Fields(line), " ")
	if len(line) > limit {
		return line[:limit] + "…"
	}
	return line
}

// stripComments removes -- and /* */ comments, honouring string literals and
// quoted identifiers so a comment marker inside 'text' is untouched.
func stripComments(s string) string {
	var out strings.Builder
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '-' && i+1 < n && s[i+1] == '-':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			i += 2
			for i < n && !(s[i] == '*' && i+1 < n && s[i+1] == '/') {
				i++
			}
			i += 2
		case c == '\'' || c == '"' || c == '`':
			quote := c
			out.WriteByte(c)
			i++
			for i < n {
				out.WriteByte(s[i])
				if s[i] == '\\' && quote != '`' && i+1 < n {
					i++
					out.WriteByte(s[i])
					i++
					continue
				}
				if s[i] == quote {
					if i+1 < n && s[i+1] == quote {
						out.WriteByte(s[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		default:
			out.WriteByte(c)
			i++
		}
	}
	return strings.TrimSpace(out.String())
}
