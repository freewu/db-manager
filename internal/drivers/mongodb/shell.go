package mongodb

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// The shell language
//
// Execute accepts the mongosh methods rather than inventing a SQL dialect that
// would only ever be an approximation. Everything is JSON; extended JSON is
// understood, so {"$oid": "..."}, {"$date": "..."} and friends work as input,
// and JavaScript spellings are rewritten into it before parsing — see
// literals.go: {sku: "a"} is accepted without quoting the key, and so are the
// shell's own constructors (ObjectId, ISODate / new Date, NumberLong,
// Timestamp, UUID, RegExp, BinData, MinKey, MaxKey).
// Supported commands:
//
//	collection level
//	  db.<c>.find(<filter>)[.sort(<doc>)][.projection(<doc>)][.skip(n)][.limit(n)]
//	  db.<c>.findOne(<filter>)
//	  db.<c>.aggregate(<pipeline>)
//	  db.<c>.countDocuments(<filter>)  |  db.<c>.count(<filter>)
//	  db.<c>.distinct(<field>[, <filter>])
//	  db.<c>.insertOne(<doc>)
//	  db.<c>.insertMany(<docs>)
//	  db.<c>.updateOne(<filter>, <update>[, <options>])
//	  db.<c>.updateMany(<filter>, <update>[, <options>])
//	  db.<c>.replaceOne(<filter>, <doc>[, <options>])
//	  db.<c>.deleteOne(<filter>)  |  db.<c>.deleteMany(<filter>)
//	  db.<c>.createIndex(<keys>[, <options>])
//	  db.<c>.createIndexes(<indexes>)
//	  db.<c>.dropIndex(<name> | <keys>)  |  db.<c>.dropIndexes()
//	  db.<c>.drop()
//	  db.<c>.stats()
//
//	database level
//	  db.createCollection(<name>[, <options>])
//	  db.dropDatabase()
//	  db.getCollectionNames()
//	  db.stats()
//	  db.runCommand(<doc>)
//
//	shell shorthands
//	  show dbs | show databases | show collections | show tables
//	  use <database>
//
// Statements are separated by ";" or by a newline at nesting level zero, so a
// multi-line pipeline inside one call stays one statement. "//" starts a
// comment. db.<name> works for names that are valid identifiers; for anything
// else the shell's own db.getCollection("<name>") form is available, and that
// is what generated scripts use.

// statement is one parsed command.
type statement struct {
	// raw is the statement as written, reported back to the UI.
	raw string
	// collection is empty for database level commands.
	collection string
	// command is the lower-cased method name.
	command string
	// args are the call arguments, parsed as BSON values.
	args []any
	// chained holds the calls following a find().
	chained []chainedCall
}

// chainedCall is one ".sort(...)" / ".limit(n)" after a call.
type chainedCall struct {
	name  string
	arg   any
	value int64
}

// writeCommands are the methods a read-only session refuses to run.
var writeCommands = map[string]bool{
	"insertone":         true,
	"insertmany":        true,
	"updateone":         true,
	"updatemany":        true,
	"replaceone":        true,
	"deleteone":         true,
	"deletemany":        true,
	"findoneandupdate":  true,
	"findoneandreplace": true,
	"findoneanddelete":  true,
	"createindex":       true,
	"createindexes":     true,
	"dropindex":         true,
	"dropindexes":       true,
	"drop":              true,
	"renamecollection":  true,
	"createcollection":  true,
	"dropdatabase":      true,
}

// commandName is what refusal and error messages call the statement.
func (s *statement) commandName() string {
	if s.collection == "" {
		return "db." + s.command
	}
	return "db." + s.collection + "." + s.command
}

// isWrite reports whether the statement modifies data or schema.
func (s *statement) isWrite() bool {
	if writeCommands[s.command] {
		return true
	}
	if s.command == "runcommand" && len(s.args) > 0 {
		return isWriteCommandDoc(s.args[0])
	}
	return false
}

// isWriteCommandDoc inspects a runCommand argument. The command name is the
// first key of the document, which is how the shell and the wire protocol both
// read it.
func isWriteCommandDoc(arg any) bool {
	elements := asElements(arg)
	if len(elements) == 0 {
		return false
	}
	return writeCommands[strings.ToLower(elements[0].Key)]
}

// --- script analysis --------------------------------------------------------

// readCommands are the methods that only read. Anything not listed here is
// either a write (see writeCommands) or a method this driver does not run, and
// the dry run says so rather than guessing.
var readCommands = map[string]bool{
	"find":               true,
	"findone":            true,
	"aggregate":          true,
	"count":              true,
	"countdocuments":     true,
	"distinct":           true,
	"stats":              true,
	"getcollectionnames": true,
	"getcollection":      true,
	"show":               true,
	// `use` only selects a database for the statements that follow it: it
	// creates nothing and reads nothing, so the dry run shows it as the query it
	// is and a read-only session keeps it.
	"use": true,
}

// AnalyzeScript is the dry run the DDL editor shows before running a script.
//
// It is the document-store counterpart of sqlutil.Analyze, and it exists
// because a keyword table cannot recognise `db.orders.drop()`: the statements
// are method calls, so this package's own parser classifies them. Like the SQL
// version it is a heuristic that errs towards warning, and it never contacts
// the server.
func (c *Conn) AnalyzeScript(script string, readOnly bool) models.ScriptAnalysis {
	analysis := models.ScriptAnalysis{
		Statements: []models.ScriptStatement{},
		Warnings:   []string{},
		ReadOnly:   readOnly,
	}

	for i, raw := range splitStatements(script) {
		entry := models.ScriptStatement{
			Index:   i,
			Preview: previewStatement(raw),
			Kind:    sqlutil.KindUnknown,
			// The statement as written, for the change log to record if a window
			// runs it — a preview is for showing, not for keeping.
			SQL: strings.TrimSpace(raw),
		}

		st, err := parseStatement(raw)
		if err != nil {
			// A statement the driver cannot parse is the one case worth
			// warning about: the editor is a shell, not a validator.
			entry.Reason = err.Error()
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": "+err.Error())
			analysis.Statements = append(analysis.Statements, entry)
			if readOnly {
				analysis.Refused++
			}
			continue
		}

		switch {
		case readCommands[st.command]:
			entry.Kind = sqlutil.KindQuery
		case st.command == "createcollection" || st.command == "createindex" || st.command == "createindexes" ||
			st.command == "dropindex" || st.command == "dropindexes" || st.command == "drop" ||
			st.command == "dropdatabase" || st.command == "renamecollection":
			entry.Kind = sqlutil.KindDDL
		case st.isWrite():
			entry.Kind = sqlutil.KindDML
		}

		entry.Destructive, entry.Reason = destructiveStatement(st)
		if entry.Destructive {
			analysis.Destructive = true
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": "+entry.Reason)
		}
		if readOnly && entry.Kind != sqlutil.KindQuery {
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

// destructiveStatement reports whether a statement can lose schema or data,
// plus the sentence explaining why.
func destructiveStatement(st *statement) (bool, string) {
	switch st.command {
	case "drop", "dropdatabase":
		return true, st.commandName() + " drops the whole " +
			map[string]string{"drop": "collection", "dropdatabase": "database"}[st.command] +
			" and everything in it"
	case "dropindex", "dropindexes":
		return true, st.commandName() + " removes an index (queries get slower, no data is lost)"
	case "deleteone", "deletemany":
		if emptyFilter(argAt(st.args, 0)) {
			return true, st.commandName() + " with an empty filter removes every document"
		}
		return true, st.commandName() + " removes matching documents"
	case "findoneanddelete":
		return true, "findOneAndDelete removes a document"
	case "replaceone":
		return true, "replaceOne overwrites the whole document"
	case "updateone", "updatemany", "findoneandupdate":
		if emptyFilter(argAt(st.args, 0)) {
			return true, st.commandName() + " with an empty filter rewrites every document"
		}
	}
	return false, ""
}

// emptyFilter reports whether a filter argument selects everything.
func emptyFilter(filter any) bool {
	return len(asElements(filter)) == 0
}

// previewStatement is the single-line summary the analysis panel shows.
func previewStatement(statement string) string {
	const limit = 140
	line := strings.Join(strings.Fields(stripComments(statement)), " ")
	if len(line) > limit {
		return line[:limit] + "…"
	}
	return line
}

// --- splitting and comments -------------------------------------------------

// splitStatements cuts a script into statements.
//
// A statement ends at ";" or at a newline that is not inside a string, a
// document or an array, so a pipeline spread over several lines stays one
// statement. Comments are removed first.
func splitStatements(script string) []string {
	text := stripComments(script)
	var (
		out   []string
		buf   strings.Builder
		depth int
		quote rune
	)
	flush := func() {
		stmt := strings.TrimSpace(buf.String())
		buf.Reset()
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	for _, r := range text {
		if quote != 0 {
			buf.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
			buf.WriteRune(r)
			continue
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				flush()
				continue
			}
		case '\n':
			if depth == 0 {
				flush()
				continue
			}
		}
		buf.WriteRune(r)
	}
	flush()
	return out
}

// stripComments removes "//" and "--" line comments, leaving string contents
// alone.
func stripComments(script string) string {
	var (
		out   strings.Builder
		quote rune
	)
	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			out.WriteRune(r)
			if r == '\\' && i+1 < len(runes) {
				i++
				out.WriteRune(runes[i])
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			out.WriteRune(r)
			continue
		}
		if r == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			if i < len(runes) {
				out.WriteRune('\n')
			}
			continue
		}
		if r == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			if i < len(runes) {
				out.WriteRune('\n')
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// --- parsing ----------------------------------------------------------------

// parseStatement parses one statement of the shell language. Which database it
// runs against is decided by the caller: the database comes from the editor's
// selector or from a `use` statement earlier in the same script.
func parseStatement(raw string) (*statement, error) {
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
	if text == "" {
		return nil, fmt.Errorf("empty statement")
	}
	stmt := &statement{raw: strings.TrimSpace(raw)}

	if fields := strings.Fields(text); len(fields) == 2 && strings.EqualFold(fields[0], "show") {
		stmt.command = "show"
		stmt.args = []any{strings.ToLower(fields[1])}
		return stmt, nil
	}

	// `use <database>` has no parentheses, so it is recognised before the
	// db.<...> path below. The name is read the way the shell reads it — the
	// rest of the line, or a quoted string when it needs spaces — and whatever
	// the server's naming rules say about it is the server's business: this
	// statement only changes which database the following statements address.
	if strings.EqualFold(text, "use") {
		return nil, fmt.Errorf("use needs a database name")
	}
	if name, ok := cutUse(text); ok {
		stmt.command = "use"
		stmt.args = []any{name}
		return stmt, nil
	}

	pos := 0
	name, next, err := readName(text, pos)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(name, "db") {
		return nil, fmt.Errorf("statement must start with db or show: %s", text)
	}
	pos = next
	if pos >= len(text) || text[pos] != '.' {
		return nil, fmt.Errorf("expected a method call after db: %s", text)
	}
	pos++

	// db.<name>.<method>(...) — but db.getCollection("<name>").<method>(...)
	// is how the shell spells names that are not identifiers.
	head, next, err := readName(text, pos)
	if err != nil {
		return nil, err
	}
	pos = next
	if strings.EqualFold(head, "getCollection") {
		inner, next, err := scanCall(text, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		args, err := parseArgs(inner)
		if err != nil {
			return nil, err
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("db.getCollection takes exactly one name")
		}
		collection, err := argString(args[0])
		if err != nil {
			return nil, fmt.Errorf("db.getCollection name: %w", err)
		}
		stmt.collection = collection
		if pos >= len(text) || text[pos] != '.' {
			return nil, fmt.Errorf("expected a method call after db.getCollection(...)")
		}
		pos++
		head, next, err = readName(text, pos)
		if err != nil {
			return nil, err
		}
		pos = next
	} else if pos < len(text) && text[pos] == '.' {
		stmt.collection = head
		pos++
		head, next, err = readName(text, pos)
		if err != nil {
			return nil, err
		}
		pos = next
	}

	// "db.users" and "db.users.name" are both missing the call.
	if pos >= len(text) || text[pos] != '(' {
		return nil, fmt.Errorf("expected a method call after db.%s", head)
	}

	stmt.command = strings.ToLower(head)
	inner, next, err := scanCall(text, pos)
	if err != nil {
		return nil, err
	}
	pos = next
	if stmt.args, err = parseArgs(inner); err != nil {
		return nil, err
	}

	for pos < len(text) {
		if text[pos] != '.' {
			return nil, fmt.Errorf("unexpected %q in %s", string(text[pos]), text)
		}
		pos++
		method, next, err := readName(text, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		inner, next, err := scanCall(text, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		args, err := parseArgs(inner)
		if err != nil {
			return nil, err
		}
		call := chainedCall{name: strings.ToLower(method)}
		switch call.name {
		case "sort", "projection":
			if len(args) != 1 {
				return nil, fmt.Errorf("%s takes one document", method)
			}
			call.arg = args[0]
		case "limit", "skip":
			if len(args) != 1 {
				return nil, fmt.Errorf("%s takes one number", method)
			}
			n, err := argNumber(args[0])
			if err != nil {
				return nil, err
			}
			call.value = n
		default:
			return nil, fmt.Errorf("unsupported method .%s()", method)
		}
		stmt.chained = append(stmt.chained, call)
	}
	return stmt, nil
}

// cutUse splits `use <database>` into the name it names.
//
// A statement that merely *starts* with the letters (db.user.find()) reports
// false, and so does a bare `use`. Everything after the keyword is the name —
// the shell's own rule — except for the quoted form, which is how a name with
// spaces in it is written anywhere else in this language.
func cutUse(text string) (name string, ok bool) {
	if len(text) < 3 || !strings.EqualFold(text[:3], "use") {
		return "", false
	}
	body := text[3:]
	rest := strings.TrimSpace(body)
	if len(rest) == len(body) || rest == "" {
		// No space after the keyword (db.user.find(...)), or nothing at all.
		return "", false
	}
	if rest[0] == '"' || rest[0] == '\'' {
		value, pos, err := readName(rest, 0)
		if err != nil || strings.TrimSpace(rest[pos:]) != "" {
			return "", false
		}
		return value, true
	}
	return rest, true
}

// readName reads an identifier or a quoted name.
func readName(text string, pos int) (string, int, error) {
	if pos >= len(text) {
		return "", pos, fmt.Errorf("unexpected end of statement")
	}
	switch text[pos] {
	case '"', '\'':
		quote := text[pos]
		end := pos + 1
		for end < len(text) && text[end] != quote {
			if text[end] == '\\' {
				end++
			}
			end++
		}
		if end >= len(text) {
			return "", pos, fmt.Errorf("unterminated string")
		}
		value := text[pos+1 : end]
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\'`, `'`)
		return value, end + 1, nil
	}
	end := pos
	for end < len(text) {
		r := text[end]
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '$':
		case r >= '0' && r <= '9' && end > pos:
		default:
			return text[pos:end], end, nil
		}
		end++
	}
	if end == pos {
		return "", pos, fmt.Errorf("expected a name at %q", text[pos:])
	}
	return text[pos:end], end, nil
}

// scanCall returns the raw text between the parentheses that start at pos and
// the position just past the closing one.
func scanCall(text string, pos int) (string, int, error) {
	if pos >= len(text) || text[pos] != '(' {
		return "", pos, fmt.Errorf("expected \"(\" in %s", text)
	}
	depth := 0
	var quote byte
	for i := pos; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 && c == ')' {
				return text[pos+1 : i], i + 1, nil
			}
		}
	}
	return "", pos, fmt.Errorf("unbalanced parentheses in %s", text)
}

// splitArgs splits the argument list at top level commas.
func splitArgs(inner string) []string {
	var (
		out   []string
		buf   strings.Builder
		depth int
		quote byte
	)
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if quote != 0 {
			buf.WriteByte(c)
			if c == '\\' && i+1 < len(inner) {
				i++
				buf.WriteByte(inner[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
			buf.WriteByte(c)
		case '(', '[', '{':
			depth++
			buf.WriteByte(c)
		case ')', ']', '}':
			depth--
			buf.WriteByte(c)
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(buf.String()))
				buf.Reset()
				continue
			}
			buf.WriteByte(c)
		default:
			buf.WriteByte(c)
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// parseArgs parses the arguments of a call as BSON values.
func parseArgs(inner string) ([]any, error) {
	if strings.TrimSpace(inner) == "" {
		return []any{}, nil
	}
	parts := splitArgs(inner)
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		arg, err := parseArg(part)
		if err != nil {
			return nil, err
		}
		out = append(out, arg)
	}
	return out, nil
}

// parseArg parses one argument. Arguments are extended JSON, with single
// quoted strings accepted because that is what the shell uses for names.
func parseArg(text string) (any, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("empty argument")
	}
	if trimmed[0] == '\'' {
		if len(trimmed) < 2 || trimmed[len(trimmed)-1] != '\'' {
			return nil, fmt.Errorf("unterminated string %s", trimmed)
		}
		return trimmed[1 : len(trimmed)-1], nil
	}
	expanded, err := expandShellLiterals(trimmed)
	if err != nil {
		return nil, err
	}
	var value any
	if err := bson.UnmarshalExtJSON([]byte(quoteBareKeys(expanded)), false, &value); err != nil {
		return nil, fmt.Errorf("%s is not a valid JSON value (%v); quote text with \"...\"", trimmed, err)
	}
	unwrapped, err := unwrapExtJSON(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", trimmed, err)
	}
	return unwrapped, nil
}

// --- argument helpers -------------------------------------------------------

func argString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case bson.ObjectID:
		return v.Hex(), nil
	default:
		return "", fmt.Errorf("expected text, got %T", value)
	}
}

func argNumber(value any) (int64, error) {
	switch v := value.(type) {
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", value)
	}
}

// argDocument returns an argument as a document, accepting both an empty
// argument list and a missing one as "no filter".
func argDocument(args []any, index int) (any, error) {
	if index >= len(args) || args[index] == nil {
		return bson.D{}, nil
	}
	if !isDocument(args[index]) {
		return nil, fmt.Errorf("argument %d must be a document", index+1)
	}
	return args[index], nil
}

func argArray(args []any, index int) (bson.A, error) {
	if index >= len(args) || args[index] == nil {
		return nil, fmt.Errorf("argument %d must be an array", index+1)
	}
	switch v := args[index].(type) {
	case bson.A:
		return v, nil
	case []any:
		return bson.A(v), nil
	default:
		return nil, fmt.Errorf("argument %d must be an array", index+1)
	}
}

func isDocument(value any) bool {
	switch value.(type) {
	case bson.D, bson.M, map[string]any:
		return true
	default:
		return false
	}
}

// asElements iterates a document argument, whatever form it arrived in.
func asElements(value any) []bson.E {
	switch v := value.(type) {
	case bson.D:
		return v
	case bson.M:
		out := make([]bson.E, 0, len(v))
		for key, val := range v {
			out = append(out, bson.E{Key: key, Value: val})
		}
		return out
	case map[string]any:
		out := make([]bson.E, 0, len(v))
		for key, val := range v {
			out = append(out, bson.E{Key: key, Value: val})
		}
		return out
	default:
		return nil
	}
}

// optionBool reads a boolean from an options document.
func optionBool(doc any, key string) bool {
	for _, elem := range asElements(doc) {
		if elem.Key != key {
			continue
		}
		value, ok := elem.Value.(bool)
		return ok && value
	}
	return false
}

// --- execution --------------------------------------------------------------

// execStatement runs one parsed statement against one database.
func (c *Conn) execStatement(ctx context.Context, database string, st *statement, maxRows int) (*models.QueryResult, error) {
	if st.collection == "" {
		return c.execDatabaseCommand(ctx, database, st)
	}
	db := c.client.Database(database)
	coll := db.Collection(st.collection)

	switch st.command {
	case "find", "findone":
		filter, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		limit := int64(maxRows)
		if st.command == "findone" {
			limit = 1
		}
		opts := options.Find().SetLimit(limit)
		for _, call := range st.chained {
			switch call.name {
			case "sort":
				opts.SetSort(call.arg)
			case "projection":
				opts.SetProjection(call.arg)
			case "limit":
				if call.value <= 0 {
					return nil, fmt.Errorf("limit must be positive")
				}
				opts.SetLimit(call.value)
				limit = call.value
			case "skip":
				if call.value < 0 {
					return nil, fmt.Errorf("skip must not be negative")
				}
				opts.SetSkip(call.value)
			}
		}
		return c.findResult(ctx, coll, filter, opts, limit)

	case "aggregate":
		pipeline, err := argArray(st.args, 0)
		if err != nil {
			return nil, err
		}
		cursor, err := coll.Aggregate(ctx, pipeline)
		docs, err := collect(ctx, cursor, err)
		if err != nil {
			return nil, err
		}
		result := documentsToResult(trimDocs(docs, maxRows))
		result.Truncated = len(docs) > maxRows
		return result, nil

	case "count", "countdocuments":
		filter, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		total, err := coll.CountDocuments(ctx, filter)
		if err != nil {
			return nil, err
		}
		return singleValue("count", total), nil

	case "distinct":
		if len(st.args) == 0 {
			return nil, fmt.Errorf("distinct needs a field name")
		}
		field, err := argString(st.args[0])
		if err != nil {
			return nil, err
		}
		filter, err := argDocument(st.args, 1)
		if err != nil {
			return nil, err
		}
		res := coll.Distinct(ctx, field, filter)
		var values bson.A
		if err := res.Decode(&values); err != nil {
			return nil, err
		}
		rows := make([][]any, 0, len(values))
		types := map[string]bool{}
		for _, value := range values {
			types[bsonTypeName(value)] = true
			rows = append(rows, []any{cellValue(value)})
		}
		meta := models.ColumnMeta{Name: field, DatabaseType: joinSorted(types), Nullable: true}
		return &models.QueryResult{
			Columns:      []models.ColumnMeta{meta},
			Rows:         rows,
			RowCount:     len(rows),
			HasResultSet: true,
		}, nil

	case "insertone":
		doc, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		res, err := coll.InsertOne(ctx, doc)
		if err != nil {
			return nil, err
		}
		return affected(1, "insertedId: "+describe(res.InsertedID)), nil

	case "insertmany":
		docs, err := argArray(st.args, 0)
		if err != nil {
			return nil, err
		}
		res, err := coll.InsertMany(ctx, docs)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(res.InsertedIDs))
		for _, id := range res.InsertedIDs {
			ids = append(ids, describe(id))
		}
		return affected(int64(len(res.InsertedIDs)), "insertedIds: "+strings.Join(ids, ", ")), nil

	case "updateone", "updatemany":
		filter, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		if len(st.args) < 2 {
			return nil, fmt.Errorf("%s needs an update document", st.commandName())
		}
		update, err := argDocument(st.args, 1)
		if err != nil {
			return nil, err
		}
		upsert := optionBool(argAt(st.args, 2), "upsert")
		var res *mongo.UpdateResult
		if st.command == "updateone" {
			res, err = coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(upsert))
		} else {
			res, err = coll.UpdateMany(ctx, filter, update, options.UpdateMany().SetUpsert(upsert))
		}
		if err != nil {
			return nil, err
		}
		return affected(res.ModifiedCount, updateMessage(res)), nil

	case "replaceone":
		filter, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		if len(st.args) < 2 {
			return nil, fmt.Errorf("replaceOne needs a document")
		}
		replacement, err := argDocument(st.args, 1)
		if err != nil {
			return nil, err
		}
		opts := options.Replace().SetUpsert(optionBool(argAt(st.args, 2), "upsert"))
		res, err := coll.ReplaceOne(ctx, filter, replacement, opts)
		if err != nil {
			return nil, err
		}
		return affected(res.ModifiedCount, updateMessage(res)), nil

	case "deleteone", "deletemany":
		filter, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		var removed int64
		if st.command == "deleteone" {
			res, err := coll.DeleteOne(ctx, filter)
			if err != nil {
				return nil, err
			}
			removed = res.DeletedCount
		} else {
			res, err := coll.DeleteMany(ctx, filter)
			if err != nil {
				return nil, err
			}
			removed = res.DeletedCount
		}
		return affected(removed, fmt.Sprintf("deleted %d document(s)", removed)), nil

	case "createindex":
		if len(st.args) == 0 {
			return nil, fmt.Errorf("createIndex needs a key document")
		}
		spec := append(bson.D{{Key: "key", Value: st.args[0]}}, asElements(argAt(st.args, 1))...)
		return c.createIndexes(ctx, db, st.collection, bson.A{spec})

	case "createindexes":
		specs, err := argArray(st.args, 0)
		if err != nil {
			return nil, err
		}
		return c.createIndexes(ctx, db, st.collection, specs)

	case "dropindex", "dropindexes":
		selector := any("*")
		if len(st.args) > 0 {
			selector = st.args[0]
		}
		cmd := bson.D{
			{Key: "dropIndexes", Value: st.collection},
			{Key: "index", Value: selector},
		}
		if err := runCommand(ctx, db, cmd); err != nil {
			return nil, err
		}
		return affected(0, "index "+describe(selector)+" dropped"), nil

	case "drop":
		if err := coll.Drop(ctx); err != nil {
			return nil, err
		}
		return affected(0, "collection "+st.collection+" dropped"), nil

	case "stats":
		pipeline := bson.A{bson.D{{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{{Key: "scale", Value: 1}}}}}}}
		cursor, err := coll.Aggregate(ctx, pipeline)
		docs, err := collect(ctx, cursor, err)
		if err != nil {
			return nil, err
		}
		if len(docs) == 0 {
			return nil, fmt.Errorf("no statistics for %s", st.collection)
		}
		return documentsToResult(docs), nil

	default:
		return nil, apperr.New(apperr.CodeUnsupported, "unsupported command %s", st.commandName())
	}
}

// execDatabaseCommand handles the db.<method>() form and `show`.
func (c *Conn) execDatabaseCommand(ctx context.Context, database string, st *statement) (*models.QueryResult, error) {
	db := c.client.Database(database)
	switch st.command {
	case "createcollection":
		if len(st.args) == 0 {
			return nil, fmt.Errorf("createCollection needs a name")
		}
		name, err := argString(st.args[0])
		if err != nil {
			return nil, err
		}
		cmd := append(bson.D{{Key: "create", Value: name}}, asElements(argAt(st.args, 1))...)
		if err := runCommand(ctx, db, cmd); err != nil {
			return nil, err
		}
		return affected(0, "collection "+name+" created"), nil

	case "dropdatabase":
		if err := db.Drop(ctx); err != nil {
			return nil, err
		}
		return affected(0, "database "+db.Name()+" dropped"), nil

	case "getcollectionnames":
		names, err := db.ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
		rows := make([][]any, 0, len(names))
		for _, name := range names {
			rows = append(rows, []any{name})
		}
		return &models.QueryResult{
			Columns:      []models.ColumnMeta{{Name: "name", DatabaseType: "string"}},
			Rows:         rows,
			RowCount:     len(rows),
			HasResultSet: true,
		}, nil

	case "stats":
		var out bson.D
		if err := db.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&out); err != nil {
			return nil, err
		}
		return documentsToResult([]bson.D{out}), nil

	case "runcommand":
		doc, err := argDocument(st.args, 0)
		if err != nil {
			return nil, err
		}
		var out bson.D
		if err := db.RunCommand(ctx, doc).Decode(&out); err != nil {
			return nil, err
		}
		return documentsToResult([]bson.D{out}), nil

	case "show":
		if len(st.args) == 0 {
			return nil, fmt.Errorf("show needs a target")
		}
		what, _ := argString(st.args[0])
		switch what {
		case "dbs", "databases":
			names, err := c.Databases(ctx)
			if err != nil {
				return nil, err
			}
			rows := make([][]any, 0, len(names))
			for _, name := range names {
				rows = append(rows, []any{name})
			}
			return &models.QueryResult{
				Columns:      []models.ColumnMeta{{Name: "databases", DatabaseType: "string"}},
				Rows:         rows,
				RowCount:     len(rows),
				HasResultSet: true,
			}, nil
		case "collections", "tables":
			names, err := db.ListCollectionNames(ctx, bson.D{})
			if err != nil {
				return nil, err
			}
			rows := make([][]any, 0, len(names))
			for _, name := range names {
				rows = append(rows, []any{name})
			}
			return &models.QueryResult{
				Columns:      []models.ColumnMeta{{Name: "collections", DatabaseType: "string"}},
				Rows:         rows,
				RowCount:     len(rows),
				HasResultSet: true,
			}, nil
		default:
			return nil, fmt.Errorf("show %s is not supported (dbs, collections)", what)
		}

	default:
		return nil, apperr.New(apperr.CodeUnsupported, "unsupported command %s", st.commandName())
	}
}

// findResult runs a find and renders the page.
func (c *Conn) findResult(ctx context.Context, coll *mongo.Collection, filter any, opts *options.FindOptionsBuilder, limit int64) (*models.QueryResult, error) {
	cursor, err := coll.Find(ctx, filter, opts)
	docs, err := collect(ctx, cursor, err)
	if err != nil {
		return nil, err
	}
	result := documentsToResult(docs)
	result.Truncated = int64(len(docs)) == limit
	return result, nil
}

// createIndexes runs the createIndexes command with index specifications the
// user wrote, so every index option works without this driver knowing it.
func (c *Conn) createIndexes(ctx context.Context, db *mongo.Database, collection string, specs bson.A) (*models.QueryResult, error) {
	named := make(bson.A, 0, len(specs))
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		prepared, name := prepareIndexSpec(spec)
		named = append(named, prepared)
		names = append(names, name)
	}
	cmd := bson.D{
		{Key: "createIndexes", Value: collection},
		{Key: "indexes", Value: named},
	}
	if err := runCommand(ctx, db, cmd); err != nil {
		return nil, err
	}
	return affected(0, "index(es) created: "+strings.Join(names, ", ")), nil
}

// prepareIndexSpec fills in the default index name.
//
// The shell derives it from the keys — createIndex({sku: 1}) creates "sku_1" —
// while the createIndexes command it sends refuses an index without a name, so
// the name has to be worked out here.
func prepareIndexSpec(spec any) (any, string) {
	elements := asElements(spec)
	name := ""
	for _, element := range elements {
		if element.Key == "name" {
			name, _ = element.Value.(string)
		}
	}
	if name != "" {
		return spec, name
	}
	for _, element := range elements {
		if element.Key == "key" {
			name = indexName(element.Value)
		}
	}
	if name == "" {
		return spec, describe(spec)
	}
	prepared := make(bson.D, 0, len(elements)+1)
	prepared = append(prepared, elements...)
	prepared = append(prepared, bson.E{Key: "name", Value: name})
	return prepared, name
}

// indexName is the name the shell gives an index: the key fields joined with
// their direction, "sku_1" for {sku: 1} and "a_1_b_-1" for {a: 1, b: -1}. Text
// and geo keys keep their own word.
func indexName(key any) string {
	parts := make([]string, 0, 2)
	for _, element := range asElements(key) {
		parts = append(parts, element.Key+"_"+indexKeyText(element.Value))
	}
	return strings.Join(parts, "_")
}

// indexKeyText spells one key direction: numbers as written, words such as
// "text" or "2dsphere" as they are.
func indexKeyText(value any) string {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return typed
	default:
		return describe(value)
	}
}

// updateMessage summarises an update result the way the shell does.
func updateMessage(res *mongo.UpdateResult) string {
	message := fmt.Sprintf("matched %d, modified %d", res.MatchedCount, res.ModifiedCount)
	if res.UpsertedCount > 0 {
		message += ", upserted " + describe(res.UpsertedID)
	}
	return message
}

// runCommand executes a raw command and discards the reply's body.
func runCommand(ctx context.Context, db *mongo.Database, cmd bson.D) error {
	var out bson.D
	if err := db.RunCommand(ctx, cmd).Decode(&out); err != nil {
		return err
	}
	return nil
}

// collect drains the cursor returned by a query call. The error is passed in
// rather than checked at every call site: the driver hands back a cursor and
// the error it already knows about, and a read that failed has nothing to drain.
func collect(ctx context.Context, cursor *mongo.Cursor, err error) ([]bson.D, error) {
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// trimDocs enforces the row cap of a result set.
func trimDocs(docs []bson.D, maxRows int) []bson.D {
	if maxRows > 0 && len(docs) > maxRows {
		return docs[:maxRows]
	}
	return docs
}

// --- result helpers ---------------------------------------------------------

func singleValue(name string, value any) *models.QueryResult {
	return &models.QueryResult{
		Columns:      []models.ColumnMeta{{Name: name, Nullable: true}},
		Rows:         [][]any{{cellValue(value)}},
		RowCount:     1,
		HasResultSet: true,
	}
}

// affected builds the result of a statement that changed something.
func affected(rows int64, messages ...string) *models.QueryResult {
	return &models.QueryResult{
		Columns:      []models.ColumnMeta{},
		Rows:         [][]any{},
		AffectedRows: rows,
		Messages:     messages,
	}
}

func argAt(args []any, index int) any {
	if index < 0 || index >= len(args) {
		return nil
	}
	return args[index]
}

// joinSorted renders a key set in a stable order, so a column type does not
// change between two runs.
func joinSorted(set map[string]bool) string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return strings.Join(out, " | ")
}
