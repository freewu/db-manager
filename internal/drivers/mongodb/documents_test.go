package mongodb

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

func doc(t *testing.T, text string) bson.D {
	t.Helper()
	var out bson.D
	if err := bson.UnmarshalExtJSON([]byte(text), false, &out); err != nil {
		t.Fatalf("bad test document %s: %v", text, err)
	}
	return out
}

func TestBsonTypeName(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{nil, "null"},
		{"text", "string"},
		{true, "bool"},
		{int32(1), "int32"},
		{int64(1), "int64"},
		{3.5, "double"},
		{bson.ObjectID{}, "objectId"},
		{bson.NewDateTimeFromTime(time.Unix(0, 0)), "date"},
		{bson.D{{Key: "a", Value: 1}}, "object"},
		{bson.A{1, 2}, "array"},
	}
	for _, tc := range cases {
		if got := bsonTypeName(tc.value); got != tc.want {
			t.Errorf("bsonTypeName(%T) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestCellValue(t *testing.T) {
	id := bson.NewObjectID()
	if got := cellValue(id); got != id.Hex() {
		t.Errorf("cellValue(ObjectID) = %v, want %s", got, id.Hex())
	}

	stamp := time.Date(2026, 9, 20, 12, 30, 0, 0, time.UTC)
	if got := cellValue(bson.NewDateTimeFromTime(stamp)); got != "2026-09-20T12:30:00.000Z" {
		t.Errorf("cellValue(DateTime) = %v", got)
	}

	if got := cellValue(nil); got != nil {
		t.Errorf("cellValue(nil) = %v, want nil", got)
	}

	// Nested values are rendered as extended JSON text so the cell stays a
	// string the grid can display.
	nested, ok := cellValue(doc(t, `{"city": "Shanghai"}`)).(string)
	if !ok || nested != `{"city":"Shanghai"}` {
		t.Errorf("cellValue(document) = %v", nested)
	}

	list, ok := cellValue(bson.A{int32(1), int32(2)}).(string)
	if !ok || list != `[1,2]` {
		t.Errorf("cellValue(array) = %v", list)
	}

	if _, ok := cellValue(bson.D{{Key: "a", Value: 1}}).(string); !ok {
		t.Error("cellValue(document) must be text")
	}
}

func TestParseLiteral(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"5", "5"},
		{"-2.5", "-2.5"},
		{"true", "true"},
		{"null", "null"},
		{`"quoted"`, `"quoted"`},
		{"plain text", `"plain text"`},
		{`"507f1f77bcf86cd799439011"`, `"507f1f77bcf86cd799439011"`},
		{`{"$gt": 5}`, `{"$gt":5}`},
		{`[1, 2]`, `[1,2]`},
		{`{"$oid": "507f1f77bcf86cd799439011"}`, `{"$oid":"507f1f77bcf86cd799439011"}`},
		{`{"unbalanced": `, `"{\"unbalanced\": "`},
	}
	for _, tc := range cases {
		got := describe(parseLiteral(tc.input))
		if got != tc.want {
			t.Errorf("parseLiteral(%q) = %s, want %s", tc.input, got, tc.want)
		}
	}
	if v, ok := parseLiteral("5").(int64); !ok || v != 5 {
		t.Errorf("integers must stay integers, got %T", parseLiteral("5"))
	}
	if v, ok := parseLiteral(`"x"`).(string); !ok || v != "x" {
		t.Errorf("quoted text must lose its quotes, got %#v", parseLiteral(`"x"`))
	}
}

func TestObjectIDShorthand(t *testing.T) {
	const hex = "507f1f77bcf86cd799439011"
	if _, ok := objectIDShorthand("_id", hex).(bson.ObjectID); !ok {
		t.Error("a hex string in _id must become an ObjectID")
	}
	if _, ok := objectIDShorthand("owner._id", hex).(bson.ObjectID); !ok {
		t.Error("a hex string in owner._id must become an ObjectID")
	}
	if _, ok := objectIDShorthand("owner", hex).(string); !ok {
		t.Error("a hex string in another field must stay a string")
	}
	if _, ok := objectIDShorthand("_id", "not-an-id").(string); !ok {
		t.Error("text that is not an ObjectID must stay a string")
	}
}

func TestBuildFilter(t *testing.T) {
	cases := []struct {
		name string
		spec models.FilterSpec
		want string
	}{
		{"equals keeps the type", models.FilterSpec{Column: "age", Operator: sqlutil.OpEq, Value: "42"}, `{"age":42}`},
		{"equals stays text when it is text", models.FilterSpec{Column: "name", Operator: sqlutil.OpEq, Value: "bob"}, `{"name":"bob"}`},
		{"not equal", models.FilterSpec{Column: "age", Operator: sqlutil.OpNe, Value: "42"}, `{"age":{"$ne":42}}`},
		{"greater than", models.FilterSpec{Column: "age", Operator: sqlutil.OpGt, Value: "42"}, `{"age":{"$gt":42}}`},
		{"contains quotes the pattern", models.FilterSpec{Column: "name", Operator: sqlutil.OpContains, Value: "a+b"}, `{"name":{"$regex":"a\\+b"}}`},
		{"starts with anchors", models.FilterSpec{Column: "name", Operator: sqlutil.OpStartsWith, Value: "bo"}, `{"name":{"$regex":"^bo"}}`},
		{"ends with anchors", models.FilterSpec{Column: "name", Operator: sqlutil.OpEndsWith, Value: "ob"}, `{"name":{"$regex":"ob$"}}`},
		{"not contains", models.FilterSpec{Column: "name", Operator: sqlutil.OpNotContains, Value: "bo"}, `{"name":{"$not":{"$regex":"bo"}}}`},
		{"is null", models.FilterSpec{Column: "note", Operator: sqlutil.OpIsNull}, `{"note":null}`},
		{"is not null", models.FilterSpec{Column: "note", Operator: sqlutil.OpIsNotNull}, `{"note":{"$ne":null}}`},
		{"in list", models.FilterSpec{Column: "state", Operator: sqlutil.OpIn, Value: "a, b ,c"}, `{"state":{"$in":["a","b","c"]}}`},
		{"not in list", models.FilterSpec{Column: "state", Operator: sqlutil.OpNotIn, Value: "a,b"}, `{"state":{"$nin":["a","b"]}}`},
		{"between", models.FilterSpec{Column: "age", Operator: sqlutil.OpBetween, Value: "18", Value2: "30"}, `{"age":{"$gte":18,"$lte":30}}`},
		{"empty operator means equals", models.FilterSpec{Column: "name", Value: "bob"}, `{"name":"bob"}`},
		{"the id filter accepts what the grid shows", models.FilterSpec{Column: "_id", Value: "507f1f77bcf86cd799439011"}, `{"_id":{"$oid":"507f1f77bcf86cd799439011"}}`},
		{"operator documents pass through", models.FilterSpec{Column: "age", Value: `{"$exists": true}`}, `{"age":{"$exists":true}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := buildFilter([]models.FilterSpec{tc.spec})
			if err != nil {
				t.Fatalf("buildFilter failed: %v", err)
			}
			if got := describe(filter); got != tc.want {
				t.Errorf("filter = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildFilterErrors(t *testing.T) {
	cases := []struct {
		name string
		spec models.FilterSpec
	}{
		{"no field", models.FilterSpec{Operator: sqlutil.OpEq, Value: "1"}},
		{"unknown operator", models.FilterSpec{Column: "a", Operator: "like", Value: "1"}},
		{"empty in list", models.FilterSpec{Column: "a", Operator: sqlutil.OpIn, Value: " , "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildFilter([]models.FilterSpec{tc.spec}); err == nil {
				t.Fatal("buildFilter succeeded, want an error")
			}
		})
	}
}

func TestBuildSort(t *testing.T) {
	sortSpec := buildSort([]models.SortSpec{
		{Column: "name"},
		{Column: "age", Desc: true},
		{Column: "  "},
	})
	if got, want := describe(sortSpec), `{"name":1,"age":-1}`; got != want {
		t.Errorf("sort = %s, want %s", got, want)
	}
}

func TestKeyFilter(t *testing.T) {
	filter, err := keyFilter([]models.KeyValue{{Column: "_id", Value: "507f1f77bcf86cd799439011"}})
	if err != nil {
		t.Fatalf("keyFilter failed: %v", err)
	}
	if got, want := describe(filter), `{"_id":{"$oid":"507f1f77bcf86cd799439011"}}`; got != want {
		t.Errorf("key filter = %s, want %s", got, want)
	}

	// A row that cannot be identified must not turn into "match everything".
	if _, err := keyFilter(nil); err == nil {
		t.Error("an empty key must be refused")
	}
	if _, err := keyFilter([]models.KeyValue{{Column: " "}}); err == nil {
		t.Error("a key without a field must be refused")
	}
}

func TestInferColumns(t *testing.T) {
	docs := []bson.D{
		doc(t, `{"_id": {"$oid": "507f1f77bcf86cd799439011"}, "name": "a", "age": 30, "tags": ["x"]}`),
		doc(t, `{"_id": {"$oid": "507f1f77bcf86cd799439012"}, "name": "b", "note": null}`),
	}
	columns := inferColumns(docs)
	if len(columns) != 5 {
		t.Fatalf("got %d columns %v, want 5", len(columns), columns)
	}
	if columns[0].Name != idField || !columns[0].PrimaryKey {
		t.Errorf("_id must come first and be the primary key, got %+v", columns[0])
	}
	if columns[0].Nullable {
		t.Error("_id is never missing")
	}

	byName := map[string]models.ColumnInfo{}
	for _, column := range columns {
		byName[column.Name] = column
	}
	if got := byName["age"]; got.DataType != "int32" || !got.Nullable {
		t.Errorf("age = %+v, want an int32 that may be absent", got)
	}
	if got := byName["note"]; got.DataType != "null" || !got.Nullable {
		t.Errorf("note = %+v", got)
	}
	if got := byName["tags"]; got.DataType != "array" {
		t.Errorf("tags = %+v", got)
	}
	if got := byName["name"]; got.Ordinal != 2 {
		t.Errorf("name ordinal = %d, want 2", got.Ordinal)
	}
}

func TestInferColumnsMixedTypes(t *testing.T) {
	docs := []bson.D{
		doc(t, `{"_id": 1, "value": 1}`),
		doc(t, `{"_id": 2, "value": "text"}`),
	}
	byName := map[string]models.ColumnInfo{}
	for _, column := range inferColumns(docs) {
		byName[column.Name] = column
	}
	if got := byName["value"].DataType; got != "int32 | string" {
		t.Errorf("a field holding two types must report both, got %q", got)
	}
}

func TestDocumentsToResult(t *testing.T) {
	docs := []bson.D{
		doc(t, `{"_id": {"$oid": "507f1f77bcf86cd799439011"}, "name": "a", "profile": {"city": "x"}}`),
		doc(t, `{"_id": {"$oid": "507f1f77bcf86cd799439012"}, "age": 21}`),
	}
	result := documentsToResult(docs)
	if !result.HasResultSet || result.RowCount != 2 {
		t.Fatalf("result = %+v", result)
	}
	names := make([]string, 0, len(result.Columns))
	for _, column := range result.Columns {
		names = append(names, column.Name)
	}
	if got, want := len(names), 4; got != want {
		t.Fatalf("columns = %v", names)
	}
	if names[0] != idField {
		t.Errorf("_id must be the first column, got %v", names)
	}

	// Missing fields are empty cells, not shifted ones.
	if result.Rows[0][0] != "507f1f77bcf86cd799439011" {
		t.Errorf("row 0 _id = %v", result.Rows[0][0])
	}
	for i, column := range result.Columns {
		switch column.Name {
		case idField:
			if column.Editable {
				t.Error("_id must not be editable")
			}
		case "profile":
			if column.Editable {
				t.Error("a document column must not be editable as a cell")
			}
		case "name", "age":
			if !column.Editable {
				t.Errorf("%s should be editable", column.Name)
			}
		}
		if column.Name == "age" && result.Rows[1][i] != int32(21) {
			t.Errorf("row 1 age = %v", result.Rows[1][i])
		}
	}
}

func TestDocumentsToResultWithoutID(t *testing.T) {
	// A projection that drops _id: nothing can be written back any more,
	// because no row can be identified.
	result := documentsToResult([]bson.D{doc(t, `{"name": "a"}`)})
	if len(result.Columns) != 1 || result.Columns[0].Editable {
		t.Errorf("columns without _id must stay read-only, got %+v", result.Columns)
	}
}

func TestDocumentsToResultEmpty(t *testing.T) {
	result := documentsToResult(nil)
	if !result.HasResultSet || result.RowCount != 0 || len(result.Columns) != 0 {
		t.Errorf("an empty page must still be a result set, got %+v", result)
	}
}

func TestDefinitionScriptRoundTrip(t *testing.T) {
	script := definitionScript("app", "orders", []models.IndexInfo{
		{Name: "_id_", Primary: true, Columns: []string{"_id ASC"}},
		{Name: "sku_1", Columns: []string{"sku ASC"}, Unique: true},
		{Name: "by_city", Columns: []string{"address.city DESC"}, Comment: "hidden"},
	})

	statements := splitStatements(script)
	if len(statements) != 2 {
		t.Fatalf("generated script has %d statements %q, want 2 (_id_ is not recreated)", len(statements), statements)
	}
	for _, raw := range statements {
		stmt, err := parseStatement(raw)
		if err != nil {
			t.Fatalf("generated statement %q does not parse: %v", raw, err)
		}
		if stmt.collection != "orders" || stmt.command != "createindex" {
			t.Errorf("statement %q parsed as %s.%s", raw, stmt.collection, stmt.command)
		}
	}

	// The generated options survive the round trip: a unique index stays
	// unique, and a descending key stays descending.
	unique, err := parseStatement(statements[0])
	if err != nil {
		t.Fatal(err)
	}
	if got, want := describe(unique.args), `[{"sku":1},{"name":"sku_1","unique":true}]`; got != want {
		t.Errorf("unique index = %s, want %s", got, want)
	}
	descending, err := parseStatement(statements[1])
	if err != nil {
		t.Fatal(err)
	}
	if got, want := describe(descending.args), `[{"address.city":-1},{"name":"by_city","hidden":true}]`; got != want {
		t.Errorf("descending index = %s, want %s", got, want)
	}
}

func TestDefinitionScriptWithoutIndexes(t *testing.T) {
	// Views have no indexes; the script still has to be valid shell input.
	script := definitionScript("app", "my-view", nil)
	statements := splitStatements(script)
	if len(statements) != 1 {
		t.Fatalf("got %q", statements)
	}
	stmt, err := parseStatement(statements[0])
	if err != nil {
		t.Fatalf("generated statement %q does not parse: %v", statements[0], err)
	}
	if stmt.command != "createcollection" {
		t.Errorf("command = %q, want createcollection", stmt.command)
	}
}

func TestCollectionRef(t *testing.T) {
	if got := collectionRef("orders"); got != "db.orders" {
		t.Errorf("collectionRef(orders) = %q", got)
	}
	if got := collectionRef("my-orders"); got != `db.getCollection("my-orders")` {
		t.Errorf("collectionRef(my-orders) = %q", got)
	}
}

func TestCollectionComment(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		option string
		want   string
	}{
		{"plain", "collection", `{}`, ""},
		{"no options at all", "collection", ``, ""},
		{"view", "view", `{"viewOn": "orders"}`, "view on orders"},
		{"view without a source", "view", `{}`, "view"},
		{"time series", "collection", `{"timeseries": {"timeField": "ts", "metaField": "sensor"}}`, "time series on ts, meta sensor"},
		{"time series without a meta field", "collection", `{"timeseries": {"timeField": "ts"}}`, "time series on ts"},
		{"capped with a validator", "collection", `{"capped": true, "validator": {"$jsonSchema": {}}}`, "capped; has a schema validator"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var options bson.Raw
			if tc.option != "" {
				var doc bson.D
				if err := bson.UnmarshalExtJSON([]byte(tc.option), true, &doc); err != nil {
					t.Fatalf("bad options: %v", err)
				}
				options = mustMarshal(doc)
			}
			if got := collectionComment(tc.kind, options); got != tc.want {
				t.Errorf("comment = %q, want %q", got, tc.want)
			}
		})
	}
}
