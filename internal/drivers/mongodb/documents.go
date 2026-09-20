package mongodb

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// idField is MongoDB's implicit primary key: every document has one, it is
// unique per collection and it is the only thing a row identity can be built
// from.
const idField = "_id"

// marshalExtJSON renders a decoded value as extended JSON. Documents are
// printed with the "$oid"/"$date" wrappers, which is also what the shell
// language accepts as input, so a value can be copied from the grid into a
// filter and mean the same thing.
func marshalExtJSON(value any) (string, error) {
	raw, err := bson.MarshalExtJSON(value, false, false)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// bsonTypeName names the BSON type of a decoded value. It is used as the
// column type in the structure pane and as ColumnMeta.DatabaseType in the data
// grid, where the SQL drivers put the server's type name.
func bsonTypeName(value any) string {
	switch value.(type) {
	case nil, bson.Null:
		return "null"
	case string:
		return "string"
	case bool:
		return "bool"
	case int:
		return "int"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case float32, float64:
		return "double"
	case bson.ObjectID:
		return "objectId"
	case bson.DateTime:
		return "date"
	case bson.Binary:
		return "binData"
	case bson.Decimal128:
		return "decimal"
	case bson.Timestamp:
		return "timestamp"
	case bson.Regex:
		return "regex"
	case bson.JavaScript:
		return "javascript"
	case bson.Symbol:
		return "symbol"
	case bson.MinKey:
		return "minKey"
	case bson.MaxKey:
		return "maxKey"
	case bson.Undefined:
		return "undefined"
	case bson.D, bson.M, map[string]any:
		return "object"
	case bson.A, []any:
		return "array"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// cellValue converts a decoded BSON value into something the UI can render.
//
// Values travel to the frontend as JSON, so anything BSON-specific has to be
// flattened first: an ObjectID is a [12]byte and would otherwise arrive as an
// array of numbers. Nested documents and arrays are kept, but as extended JSON
// text, because the grid edits one column of one row and a string is the only
// thing a cell can hold without inventing a document editor.
func cellValue(value any) any {
	switch v := value.(type) {
	case nil, bson.Null, bson.Undefined:
		return nil
	case bson.ObjectID:
		return v.Hex()
	case bson.DateTime:
		return v.Time().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	case bson.Binary:
		return describe(v)
	case bson.Decimal128:
		return v.String()
	case bson.Timestamp:
		return fmt.Sprintf("Timestamp(%d, %d)", v.T, v.I)
	case bson.Regex:
		if v.Options == "" {
			return v.Pattern
		}
		return "/" + v.Pattern + "/" + v.Options
	case bson.D, bson.M, bson.A, map[string]any, []any:
		return describe(v)
	default:
		return v
	}
}

// isScalar reports whether a value can be edited in a grid cell. Documents and
// arrays cannot: replacing them with a string would silently destroy data, so
// the grid keeps those cells read-only.
func isScalar(value any) bool {
	switch value.(type) {
	case bson.D, bson.M, bson.A, map[string]any, []any:
		return false
	default:
		return true
	}
}

// fieldStat accumulates what a sample of documents says about one field.
type fieldStat struct {
	types  map[string]bool
	seen   int
	scalar bool
}

func (f *fieldStat) add(value any) {
	if f.types == nil {
		f.types = map[string]bool{}
		f.scalar = true
	}
	f.types[bsonTypeName(value)] = true
	f.seen++
	if !isScalar(value) {
		f.scalar = false
	}
}

// typeText renders the observed types of a field. A field holding both int32
// and string (which MongoDB allows) is reported as "int32 | string" rather
// than pretending it has one type.
func (f *fieldStat) typeText() string {
	types := make([]string, 0, len(f.types))
	for t := range f.types {
		types = append(types, t)
	}
	sort.Strings(types)
	return strings.Join(types, " | ")
}

// fieldStats collects per-field statistics in first-seen order.
type fieldStats struct {
	order  []string
	byName map[string]*fieldStat
}

func (s *fieldStats) add(name string, value any) {
	if s.byName == nil {
		s.byName = map[string]*fieldStat{}
	}
	stat, ok := s.byName[name]
	if !ok {
		stat = &fieldStat{}
		s.byName[name] = stat
		s.order = append(s.order, name)
	}
	stat.add(value)
}

// inferColumns turns a document sample into a field list. The "_id" field is
// always first because it is the row identity the grid needs.
func inferColumns(docs []bson.D) []models.ColumnInfo {
	stats := &fieldStats{}
	for _, doc := range docs {
		for _, elem := range doc {
			stats.add(elem.Key, elem.Value)
		}
	}

	names := make([]string, 0, len(stats.order))
	for _, name := range stats.order {
		if name != idField {
			names = append(names, name)
		}
	}
	if _, ok := stats.byName[idField]; ok {
		names = append([]string{idField}, names...)
	}

	columns := make([]models.ColumnInfo, 0, len(names))
	for i, name := range names {
		stat := stats.byName[name]
		columns = append(columns, models.ColumnInfo{
			Name:       name,
			Ordinal:    i + 1,
			DataType:   stat.typeText(),
			ColumnType: stat.typeText(),
			// A field missing from some documents is optional; MongoDB never
			// enforces NOT NULL, so "nullable" means "may be absent".
			Nullable:   stat.seen < len(docs) || stat.types["null"],
			PrimaryKey: name == idField,
		})
	}
	return columns
}

// documentsToResult flattens a page of documents into a result set.
//
// The columns are the union of the top level fields of the page (not of the
// whole collection, which would need a full scan): a page is what the grid
// shows, and a column that appears in the next page shows up then.
func documentsToResult(docs []bson.D) *models.QueryResult {
	stats := &fieldStats{}
	for _, doc := range docs {
		for _, elem := range doc {
			stats.add(elem.Key, elem.Value)
		}
	}

	names := make([]string, 0, len(stats.order))
	for _, name := range stats.order {
		if name != idField {
			names = append(names, name)
		}
	}
	hasID := false
	if _, ok := stats.byName[idField]; ok {
		hasID = true
		names = append([]string{idField}, names...)
	}

	index := make(map[string]int, len(names))
	columns := make([]models.ColumnMeta, 0, len(names))
	for i, name := range names {
		index[name] = i
		stat := stats.byName[name]
		columns = append(columns, models.ColumnMeta{
			Name:         name,
			DatabaseType: stat.typeText(),
			Nullable:     stat.seen < len(docs) || stat.types["null"],
			IsPrimaryKey: name == idField,
			// Only top level scalars can be written back by a cell edit, and
			// only when the row can be identified again (_id present). The id
			// itself is immutable in MongoDB.
			Editable: hasID && stat.scalar && name != idField,
		})
	}

	out := make([][]any, 0, len(docs))
	for _, doc := range docs {
		record := make([]any, len(names))
		for _, elem := range doc {
			pos, ok := index[elem.Key]
			if !ok {
				continue
			}
			record[pos] = cellValue(elem.Value)
		}
		out = append(out, record)
	}

	return &models.QueryResult{
		Columns:      columns,
		Rows:         out,
		RowCount:     len(out),
		HasResultSet: true,
	}
}

// buildFilter compiles the data grid's filters into a MongoDB filter document.
//
// The value is parsed as a JSON literal, so a filter can carry a real number,
// a date, a regex or a whole operator document ({"$gt": 5} typed as the value
// of "="). A bare word stays a string, which keeps the common case — filter by
// name — free of quoting.
func buildFilter(filters []models.FilterSpec) (bson.D, error) {
	out := make(bson.D, 0, len(filters))
	for i, f := range filters {
		column := strings.TrimSpace(f.Column)
		if column == "" {
			return nil, fmt.Errorf("filter %d: missing field", i+1)
		}
		op := f.Operator
		if op == "" {
			op = sqlutil.OpEq
		}
		value := filterValue(column, f.Value)
		value2 := filterValue(column, f.Value2)

		var cond any
		switch op {
		case sqlutil.OpEq:
			cond = value
		case sqlutil.OpNe:
			cond = bson.D{{Key: "$ne", Value: value}}
		case sqlutil.OpGt:
			cond = bson.D{{Key: "$gt", Value: value}}
		case sqlutil.OpGte:
			cond = bson.D{{Key: "$gte", Value: value}}
		case sqlutil.OpLt:
			cond = bson.D{{Key: "$lt", Value: value}}
		case sqlutil.OpLte:
			cond = bson.D{{Key: "$lte", Value: value}}
		case sqlutil.OpContains:
			cond = regexCond(regexp.QuoteMeta(f.Value))
		case sqlutil.OpNotContains:
			cond = bson.D{{Key: "$not", Value: regexCond(regexp.QuoteMeta(f.Value))}}
		case sqlutil.OpStartsWith:
			cond = regexCond("^" + regexp.QuoteMeta(f.Value))
		case sqlutil.OpEndsWith:
			cond = regexCond(regexp.QuoteMeta(f.Value) + "$")
		case sqlutil.OpIsNull:
			cond = nil
		case sqlutil.OpIsNotNull:
			cond = bson.D{{Key: "$ne", Value: nil}}
		case sqlutil.OpIn, sqlutil.OpNotIn:
			values := make(bson.A, 0, 4)
			for _, part := range strings.Split(f.Value, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				values = append(values, filterValue(column, part))
			}
			if len(values) == 0 {
				return nil, fmt.Errorf("filter %d: %s needs at least one value", i+1, op)
			}
			key := "$in"
			if op == sqlutil.OpNotIn {
				key = "$nin"
			}
			cond = bson.D{{Key: key, Value: values}}
		case sqlutil.OpBetween:
			cond = bson.D{{Key: "$gte", Value: value}, {Key: "$lte", Value: value2}}
		default:
			return nil, fmt.Errorf("filter %d: unsupported operator %q", i+1, op)
		}
		out = append(out, bson.E{Key: column, Value: cond})
	}
	return out, nil
}

// regexCond builds a regular expression condition from an already quoted
// pattern.
//
// The match is case sensitive: MySQL's LIKE is not and PostgreSQL's is, so
// there is no shared behaviour to copy and PostgreSQL's rule — what you typed
// is what you match — is the least surprising one.
func regexCond(pattern string) bson.D {
	return bson.D{{Key: "$regex", Value: pattern}}
}

// filterValue parses one filter operand, with the ObjectID shorthand applied.
func filterValue(column, raw string) any {
	value := parseLiteral(raw)
	return objectIDShorthand(column, value)
}

// objectIDShorthand upgrades a 24 character hex string to an ObjectID when the
// field is (or ends with) "_id".
//
// mongosh would need ObjectId("...") here, but a data grid that shows ids as
// hex strings has to accept what it shows; anything else makes copying an id
// out of the grid into the filter fail for no visible reason.
func objectIDShorthand(column string, value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	if column != idField && !strings.HasSuffix(column, "."+idField) {
		return value
	}
	if id, err := bson.ObjectIDFromHex(text); err == nil {
		return id
	}
	return value
}

// buildSort compiles the grid's sort specs.
func buildSort(specs []models.SortSpec) bson.D {
	out := make(bson.D, 0, len(specs))
	for _, s := range specs {
		column := strings.TrimSpace(s.Column)
		if column == "" {
			continue
		}
		dir := 1
		if s.Desc {
			dir = -1
		}
		out = append(out, bson.E{Key: column, Value: dir})
	}
	return out
}

// parseLiteral turns filter and editor text into a BSON value.
//
// The rule is deliberately narrow: text that is valid JSON keeps its type
// (numbers, booleans, null, quoted strings, documents, arrays and every
// extended JSON form such as {"$oid": "..."} or {"$gt": 5}), everything else
// is a literal string. That makes "active" and {"$exists": true} both work
// without a type selector.
func parseLiteral(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	switch trimmed[0] {
	case '{', '[':
		var value any
		if err := bson.UnmarshalExtJSON([]byte(trimmed), false, &value); err != nil {
			return raw
		}
		// {"$oid": "..."} has to become an ObjectID, not a subdocument: see
		// unwrapExtJSON. A wrapper with a bad payload is not valid JSON either,
		// so the text is taken as the literal string it looks like.
		unwrapped, err := unwrapExtJSON(value)
		if err != nil {
			return raw
		}
		return unwrapped
	case '"':
		var value string
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			return raw
		}
		return value
	}

	switch strings.ToLower(trimmed) {
	case "null":
		return nil
	case "true":
		return true
	case "false":
		return false
	}

	// The whole text has to be a number: a decoder that stops at the first
	// value it understands would read the id "507f1f77bcf86cd799439011" as
	// 507 and quietly match nothing.
	var number json.Number
	if err := json.Unmarshal([]byte(trimmed), &number); err == nil {
		if i, err := number.Int64(); err == nil {
			return i
		}
		if f, err := number.Float64(); err == nil {
			return f
		}
	}
	return raw
}

// writeValue converts text coming back from the grid into a BSON value: a row
// identity component when it is a key, the new cell contents when it is an
// edit. The grid sends what it renders, so numbers arrive as JSON numbers and
// ids as hex strings.
func writeValue(column string, value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return objectIDShorthand(column, parseLiteral(v))
	default:
		return value
	}
}

// keyFilter builds the filter that identifies exactly one document.
//
// An empty key is refused: it would match every document in the collection,
// and a cell edit or a delete that hits everything is never what the user
// meant.
func keyFilter(key []models.KeyValue) (bson.D, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("no row identity given")
	}
	out := make(bson.D, 0, len(key))
	for _, kv := range key {
		column := strings.TrimSpace(kv.Column)
		if column == "" {
			continue
		}
		out = append(out, bson.E{Key: column, Value: writeValue(column, kv.Value)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no row identity given")
	}
	return out, nil
}

// formatTime renders a timestamp for messages.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }
