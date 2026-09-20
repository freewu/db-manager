package mongodb

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// mongosh literals
//
// A snippet copied out of the real shell is full of values that are not JSON:
// ObjectId("..."), ISODate("..."), new Date(...), NumberLong(7), UUID("..."),
// Timestamp(1, 2). They are rewritten into the extended JSON they stand for
// before the argument is parsed, so a script written for mongosh runs here
// unchanged. Only the spelling changes — the extended JSON parser still
// validates every value, so ObjectId("nope") is rejected by bson rather than
// by a second, looser implementation of what an ObjectID is.
//
// The rewrites are textual and quote-aware, so they work at any nesting depth:
// { at: ISODate("2026-01-02") } is one document with one date in it.
//
// The same pass quotes JavaScript object keys, because the shell is a
// JavaScript dialect and extended JSON — the thing actually parsed — is not:
// {sku: "a"} is what people type, {"sku": "a"} is what bson accepts.

// shellConstructors maps a lower-cased constructor name to the extended JSON it
// stands for.
var shellConstructors = map[string]func(args []string) (string, error){
	"objectid":      objectIDLiteral,
	"isodate":       dateLiteral,
	"date":          dateLiteral,
	"numberlong":    numberLiteral("NumberLong", "$numberLong"),
	"numberint":     numberLiteral("NumberInt", "$numberInt"),
	"numberdecimal": numberLiteral("NumberDecimal", "$numberDecimal"),
	"timestamp":     timestampLiteral,
	"regexp":        regexLiteral,
	"uuid":          uuidLiteral,
	"bindata":       binDataLiteral,
	"minkey":        constantLiteral(`{"$minKey":1}`),
	"maxkey":        constantLiteral(`{"$maxKey":1}`),
}

// expandShellLiterals rewrites every known constructor call in an argument into
// extended JSON.
func expandShellLiterals(text string) (string, error) {
	var out strings.Builder
	i := 0
	for i < len(text) {
		switch text[i] {
		case '"', '\'':
			end, err := copyQuoted(&out, text, i)
			if err != nil {
				return "", err
			}
			i = end
			continue
		}

		// The real shell writes the date constructor as `new Date(...)`.
		prefix := ""
		if strings.HasPrefix(text[i:], "new ") {
			prefix = "new "
			i += len("new ")
			for i < len(text) && text[i] == ' ' {
				prefix += " "
				i++
			}
		}
		if i >= len(text) {
			out.WriteString(prefix)
			break
		}
		if !isNameStart(text[i]) {
			out.WriteString(prefix)
			out.WriteByte(text[i])
			i++
			continue
		}

		name := readIdentifier(text, i)
		open := i + len(name)
		spaces := ""
		for open < len(text) && text[open] == ' ' {
			spaces += " "
			open++
		}
		build, known := shellConstructors[strings.ToLower(name)]
		if !known || open >= len(text) || text[open] != '(' {
			out.WriteString(prefix + name + spaces)
			i = open
			continue
		}

		inner, after, err := scanCall(text, open)
		if err != nil {
			return "", err
		}
		literal, err := build(splitArgs(inner))
		if err != nil {
			return "", err
		}
		out.WriteString(literal)
		i = after
	}
	return out.String(), nil
}

// quoteBareKeys turns JavaScript object keys into JSON ones, so {sku: "a"}
// parses while {active: true} keeps its boolean.
//
// Only a bare name directly followed by ":" is quoted, because that is the only
// place a name can be a key in this grammar (there are no labels, and a ternary
// does not fit in a document). A bare name anywhere else is left alone: quoting
// it would silently turn a typo into a string.
func quoteBareKeys(text string) string {
	var out strings.Builder
	i := 0
	for i < len(text) {
		c := text[i]
		if c == '"' || c == '\'' {
			end, err := copyQuoted(&out, text, i)
			if err != nil {
				out.WriteString(text[i:])
				return out.String()
			}
			i = end
			continue
		}
		if !isNameStart(c) {
			out.WriteByte(c)
			i++
			continue
		}

		name := readIdentifier(text, i)
		after := i + len(name)
		colon := after
		for colon < len(text) && text[colon] == ' ' {
			colon++
		}
		if colon < len(text) && text[colon] == ':' {
			out.WriteString(quoteJSON(name))
			out.WriteString(text[after : colon+1])
			i = colon + 1
			continue
		}
		out.WriteString(name)
		i = after
	}
	return out.String()
}

// copyQuoted copies one string literal, quotes included, and returns the
// position after it.
func copyQuoted(out *strings.Builder, text string, pos int) (int, error) {
	quote := text[pos]
	for i := pos; i < len(text); i++ {
		c := text[i]
		if c == '\\' && i+1 < len(text) {
			out.WriteByte(c)
			out.WriteByte(text[i+1])
			i++
			continue
		}
		out.WriteByte(c)
		if c == quote && i > pos {
			return i + 1, nil
		}
	}
	return pos, fmt.Errorf("unterminated string in %s", text)
}

func isNameStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// readIdentifier reads a constructor or function name.
func readIdentifier(text string, pos int) string {
	end := pos
	for end < len(text) && (isNameStart(text[end]) || (text[end] >= '0' && text[end] <= '9')) {
		end++
	}
	return text[pos:end]
}

// --- the constructors -------------------------------------------------------

func objectIDLiteral(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("ObjectId() with no argument would generate a new id; write the 24 character hex string instead")
	}
	if err := expectArgs("ObjectId", args, 1); err != nil {
		return "", err
	}
	value, err := textArg("ObjectId", args[0])
	if err != nil {
		return "", err
	}
	return `{"$oid":` + quoteJSON(value) + `}`, nil
}

func dateLiteral(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("ISODate needs a date, for example ISODate(\"2026-01-02T03:04:05Z\")")
	}
	if len(args) != 1 {
		return "", fmt.Errorf("ISODate (or Date) takes 1 argument, got %d", len(args))
	}
	raw := strings.TrimSpace(args[0])
	if raw == "" {
		return "", fmt.Errorf("ISODate needs a date, for example ISODate(\"2026-01-02T03:04:05Z\")")
	}
	// A bare number is milliseconds since the epoch, which is how the shell
	// spells `new Date(1735787045000)`.
	if millis, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return dateMillisLiteral(millis), nil
	}
	value, err := textArg("ISODate", raw)
	if err != nil {
		return "", err
	}
	if millis, err := strconv.ParseInt(value, 10, 64); err == nil {
		return dateMillisLiteral(millis), nil
	}
	// Anything else is a date the shell would have parsed at run time, so it is
	// read here and written as milliseconds: extended JSON only takes a full
	// timestamp, while people type ISODate("2026-01-02") as often as the long
	// form.
	parsed, ok := parseDateText(value)
	if !ok {
		return "", fmt.Errorf("ISODate(%s) is not a date, for example ISODate(\"2026-01-02T03:04:05Z\")", args[0])
	}
	return dateMillisLiteral(parsed.UnixMilli()), nil
}

// dateMillisLiteral spells a date the way canonical extended JSON does.
func dateMillisLiteral(millis int64) string {
	return `{"$date":{"$numberLong":` + quoteJSON(strconv.FormatInt(millis, 10)) + `}}`
}

func numberLiteral(name, tag string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		if err := expectArgs(name, args, 1); err != nil {
			return "", err
		}
		// Quoted or bare: NumberLong(7) and NumberLong("7") are the same thing.
		value := strings.TrimSpace(args[0])
		if unquoted, err := textArg(name, value); err == nil {
			value = unquoted
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "", fmt.Errorf("%s(%s) is not a number", name, args[0])
		}
		return `{"` + tag + `":` + quoteJSON(value) + `}`, nil
	}
}

func timestampLiteral(args []string) (string, error) {
	if err := expectArgs("Timestamp", args, 2); err != nil {
		return "", err
	}
	seconds, err := intArg("Timestamp", args[0])
	if err != nil {
		return "", err
	}
	increment, err := intArg("Timestamp", args[1])
	if err != nil {
		return "", err
	}
	return `{"$timestamp":{"t":` + strconv.FormatInt(seconds, 10) +
		`,"i":` + strconv.FormatInt(increment, 10) + `}}`, nil
}

func regexLiteral(args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf("RegExp takes 1 or 2 arguments, got %d", len(args))
	}
	pattern, err := textArg("RegExp", args[0])
	if err != nil {
		return "", err
	}
	options := ""
	if len(args) == 2 {
		if options, err = textArg("RegExp", args[1]); err != nil {
			return "", err
		}
	}
	return `{"$regularExpression":{"pattern":` + quoteJSON(pattern) +
		`,"options":` + quoteJSON(options) + `}}`, nil
}

func uuidLiteral(args []string) (string, error) {
	if err := expectArgs("UUID", args, 1); err != nil {
		return "", err
	}
	value, err := textArg("UUID", args[0])
	if err != nil {
		return "", err
	}
	compact := strings.ReplaceAll(value, "-", "")
	if len(compact) != 32 {
		return "", fmt.Errorf("UUID(%s) is neither a dashed nor a plain 16 byte hex string", args[0])
	}
	raw, err := hex.DecodeString(compact)
	if err != nil {
		return "", fmt.Errorf("UUID(%s) is not hex", args[0])
	}
	return `{"$binary":{"base64":` + quoteJSON(base64.StdEncoding.EncodeToString(raw)) + `,"subType":"04"}}`, nil
}

func binDataLiteral(args []string) (string, error) {
	if err := expectArgs("BinData", args, 2); err != nil {
		return "", err
	}
	subtype, err := intArg("BinData", args[0])
	if err != nil {
		return "", err
	}
	if subtype < 0 || subtype > 255 {
		return "", fmt.Errorf("BinData subtype %d is out of range", subtype)
	}
	data, err := textArg("BinData", args[1])
	if err != nil {
		return "", err
	}
	return `{"$binary":{"base64":` + quoteJSON(data) +
		`,"subType":` + quoteJSON(fmt.Sprintf("%02x", subtype)) + `}}`, nil
}

func constantLiteral(literal string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("this constructor takes no arguments, got %d", len(args))
		}
		return literal, nil
	}
}

// --- argument helpers -------------------------------------------------------

func expectArgs(name string, args []string, want int) error {
	if len(args) != want {
		return fmt.Errorf("%s takes %d argument(s), got %d", name, want, len(args))
	}
	return nil
}

// textArg unquotes one argument. Both quoting styles are accepted because the
// shell uses single quotes and JSON uses double ones.
func textArg(name, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 {
		quote := trimmed[0]
		if (quote == '"' || quote == '\'') && trimmed[len(trimmed)-1] == quote {
			var value string
			if err := json.Unmarshal([]byte(trimmed), &value); err == nil {
				return value, nil
			}
			body := trimmed[1 : len(trimmed)-1]
			body = strings.ReplaceAll(body, `\"`, `"`)
			return strings.ReplaceAll(body, `\'`, `'`), nil
		}
	}
	return "", fmt.Errorf("%s expects a quoted string, got %s", name, raw)
}

func intArg(name, raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s expects a whole number, got %s", name, raw)
	}
	return value, nil
}

func quoteJSON(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		// Marshalling a string cannot fail; keep the compiler honest.
		return `""`
	}
	return string(encoded)
}
