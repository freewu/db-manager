package mongodb

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "semicolon separates",
			input: `db.a.find({}); db.b.countDocuments({})`,
			want:  []string{`db.a.find({})`, `db.b.countDocuments({})`},
		},
		{
			name:  "newline separates",
			input: "db.a.find({})\ndb.b.find({})\n",
			want:  []string{`db.a.find({})`, `db.b.find({})`},
		},
		{
			name:  "newline inside a call does not separate",
			input: "db.a.aggregate([\n  {\"$match\": {}},\n  {\"$limit\": 5}\n])",
			want:  []string{"db.a.aggregate([\n  {\"$match\": {}},\n  {\"$limit\": 5}\n])"},
		},
		{
			name:  "semicolon and braces inside a string stay put",
			input: `db.a.find({"name": "x;y {z}"})`,
			want:  []string{`db.a.find({"name": "x;y {z}"})`},
		},
		{
			name:  "comments are dropped",
			input: "// a leading comment\ndb.a.find({}) // trailing\n-- another style\ndb.a.count({})",
			want:  []string{`db.a.find({})`, `db.a.count({})`},
		},
		{
			name:  "blank statements disappear",
			input: ";;\ndb.a.find({});;\n\n",
			want:  []string{`db.a.find({})`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitStatements(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d statements %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if strings.TrimSpace(got[i]) != strings.TrimSpace(tc.want[i]) {
					t.Errorf("statement %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSplitArgs(t *testing.T) {
	got := splitArgs(`{"a": 1, "b": [1, 2]}, 'text, with comma', 5`)
	want := []string{`{"a": 1, "b": [1, 2]}`, `'text, with comma'`, `5`}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("argument %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseStatement(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		collection string
		command    string
		args       string // extended JSON rendering of the parsed arguments
		chained    int
	}{
		{
			name:       "find with a filter",
			input:      `db.users.find({"age": {"$gt": 21}})`,
			collection: "users",
			command:    "find",
			args:       `[{"age":{"$gt":21}}]`,
		},
		{
			name:       "chained calls",
			input:      `db.users.find({"active": true}).sort({"age": -1}).skip(10).limit(5)`,
			collection: "users",
			command:    "find",
			args:       `[{"active":true}]`,
			chained:    3,
		},
		{
			name:       "no filter means no arguments",
			input:      `db.users.countDocuments()`,
			collection: "users",
			command:    "countdocuments",
			args:       `[]`,
		},
		{
			name:       "getCollection for names that are not identifiers",
			input:      `db.getCollection("my-coll").findOne({})`,
			collection: "my-coll",
			command:    "findone",
			args:       `[{}]`,
		},
		{
			name:       "single quotes are accepted",
			input:      `db.getCollection('logs-2026').count({})`,
			collection: "logs-2026",
			command:    "count",
			args:       `[{}]`,
		},
		{
			name:    "database level command",
			input:   `db.createCollection("events", {"capped": true})`,
			command: "createcollection",
			args:    `["events",{"capped":true}]`,
		},
		{
			name:    "shell shorthand",
			input:   `show collections`,
			command: "show",
			args:    `["collections"]`,
		},
		{
			name:    "use switches the database of what follows",
			input:   `use shop`,
			command: "use",
			args:    `["shop"]`,
		},
		{
			name:    "a database name may carry the characters MongoDB allows in one",
			input:   `use logs-2026_v2`,
			command: "use",
			args:    `["logs-2026_v2"]`,
		},
		{
			name:    "a quoted name is how spaces survive",
			input:   `use "my db"`,
			command: "use",
			args:    `["my db"]`,
		},
		{
			name:       "a collection whose name starts with use is still a method call",
			input:      `db.users.find({})`,
			collection: "users",
			command:    "find",
			args:       `[{}]`,
		},
		{
			name:       "extended json argument",
			input:      `db.users.deleteOne({"_id": {"$oid": "507f1f77bcf86cd799439011"}})`,
			collection: "users",
			command:    "deleteone",
			args:       `[{"_id":{"$oid":"507f1f77bcf86cd799439011"}}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := parseStatement(tc.input)
			if err != nil {
				t.Fatalf("parseStatement(%q) failed: %v", tc.input, err)
			}
			if st.collection != tc.collection {
				t.Errorf("collection = %q, want %q", st.collection, tc.collection)
			}
			if st.command != tc.command {
				t.Errorf("command = %q, want %q", st.command, tc.command)
			}
			if got := describe(st.args); got != tc.args {
				t.Errorf("args = %s, want %s", got, tc.args)
			}
			if len(st.chained) != tc.chained {
				t.Errorf("chained = %d, want %d", len(st.chained), tc.chained)
			}
		})
	}
}

func TestParseStatementErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"no db prefix", `users.find({})`, "must start with db"},
		{"no method", `db.users`, "expected a method call"},
		{"no method on a field", `db.users.name`, "expected a method call"},
		{"unbalanced parentheses", `db.users.find({}`, "unbalanced parentheses"},
		{"unknown chained method", `db.users.find({}).sortBy({})`, "unsupported method"},
		{"chained number that is not a number", `db.users.find({}).limit("x")`, "expected a number"},
		{"unquoted text argument", `db.users.find(active)`, "not a valid JSON value"},
		{"getCollection without a method", `db.getCollection("x")`, "expected a method call"},
		{"show without a target", `show`, "must start with db or show"},
		{"use without a name", `use`, "use needs a database name"},
		{"a quoted use name with a tail", `use "a" "b"`, "must start with db or show"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseStatement(tc.input)
			if err == nil {
				t.Fatalf("parseStatement(%q) succeeded, want an error", tc.input)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestPrepareIndexSpec(t *testing.T) {
	// createIndex({sku: 1}) has to reach the server as
	// {key: {sku: 1}, name: "sku_1"}: the shell works the name out, the
	// createIndexes command demands it.
	cases := []struct {
		name string
		spec bson.D
		want string
	}{
		{
			name: "single key",
			spec: bson.D{{Key: "key", Value: bson.D{{Key: "sku", Value: int32(1)}}}},
			want: "sku_1",
		},
		{
			name: "compound key keeps its order",
			spec: bson.D{{Key: "key", Value: bson.D{{Key: "a", Value: int32(1)}, {Key: "b", Value: int32(-1)}}}},
			want: "a_1_b_-1",
		},
		{
			name: "options are kept",
			spec: bson.D{
				{Key: "key", Value: bson.D{{Key: "sku", Value: int32(1)}}},
				{Key: "unique", Value: true},
			},
			want: "sku_1",
		},
		{
			name: "a word key keeps its word",
			spec: bson.D{{Key: "key", Value: bson.D{{Key: "title", Value: "text"}}}},
			want: "title_text",
		},
		{
			name: "an explicit name wins",
			spec: bson.D{
				{Key: "key", Value: bson.D{{Key: "sku", Value: int32(1)}}},
				{Key: "name", Value: "sku_idx"},
			},
			want: "sku_idx",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prepared, got := prepareIndexSpec(tc.spec)
			if got != tc.want {
				t.Fatalf("name = %q, want %q", got, tc.want)
			}
			named := false
			for _, element := range asElements(prepared) {
				if element.Key == "name" && element.Value == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("prepared spec %v has no name field", prepared)
			}
		})
	}
}

func TestStatementIsWrite(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{`db.users.find({})`, false},
		{`db.users.countDocuments({})`, false},
		{`db.users.aggregate([])`, false},
		{`db.users.insertOne({})`, true},
		{`db.users.updateMany({}, {"$set": {"a": 1}})`, true},
		{`db.users.deleteOne({})`, true},
		{`db.users.createIndex({"a": 1})`, true},
		{`db.users.drop()`, true},
		{`db.createCollection("x")`, true},
		{`db.dropDatabase()`, true},
		{`db.getCollectionNames()`, false},
		{`db.runCommand({"serverStatus": 1})`, false},
		{`db.runCommand({"drop": "users"})`, true},
		{`db.runCommand({"createIndexes": "users"})`, true},
		{`show collections`, false},
	}
	for _, tc := range cases {
		st, err := parseStatement(tc.input)
		if err != nil {
			t.Fatalf("parseStatement(%q) failed: %v", tc.input, err)
		}
		if got := st.isWrite(); got != tc.want {
			t.Errorf("%s isWrite() = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestCommandName(t *testing.T) {
	st, err := parseStatement(`db.users.find({})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.commandName(); got != "db.users.find" {
		t.Errorf("commandName() = %q", got)
	}
	st, err = parseStatement(`show dbs`)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.commandName(); got != "db.show" {
		t.Errorf("commandName() = %q", got)
	}
}

func TestAnalyzeScript(t *testing.T) {
	cases := []struct {
		name        string
		script      string
		wantKind    string
		destructive bool
		reason      string
	}{
		{"find", `db.orders.find({"a": 1})`, "query", false, ""},
		{"count", `db.orders.countDocuments({})`, "query", false, ""},
		{"show", `show collections`, "query", false, ""},
		{"use", `use shop`, "query", false, ""},
		{"insert", `db.orders.insertOne({"a": 1})`, "dml", false, ""},
		{"update with a filter", `db.orders.updateMany({"a": 1}, {"$set": {"b": 2}})`, "dml", false, ""},
		{"update without a filter", `db.orders.updateMany({}, {"$set": {"b": 2}})`, "dml", true, "rewrites every document"},
		{"delete with a filter", `db.orders.deleteMany({"a": 1})`, "dml", true, "removes matching documents"},
		{"delete without a filter", `db.orders.deleteMany({})`, "dml", true, "removes every document"},
		{"replace", `db.orders.replaceOne({"a": 1}, {"a": 2})`, "dml", true, "overwrites the whole document"},
		{"create collection", `db.createCollection("orders")`, "ddl", false, ""},
		{"create index", `db.orders.createIndex({"sku": 1})`, "ddl", false, ""},
		{"drop index", `db.orders.dropIndex("sku_1")`, "ddl", true, "removes an index"},
		{"drop collection", `db.orders.drop()`, "ddl", true, "drops the whole collection"},
		{"drop database", `db.dropDatabase()`, "ddl", true, "drops the whole database"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			analysis := (&Conn{}).AnalyzeScript(tc.script, false)
			if len(analysis.Statements) != 1 {
				t.Fatalf("statements = %+v", analysis.Statements)
			}
			entry := analysis.Statements[0]
			if entry.Kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", entry.Kind, tc.wantKind)
			}
			if entry.Destructive != tc.destructive {
				t.Errorf("destructive = %v, want %v", entry.Destructive, tc.destructive)
			}
			if tc.reason != "" && !strings.Contains(entry.Reason, tc.reason) {
				t.Errorf("reason = %q, want it to mention %q", entry.Reason, tc.reason)
			}
			if analysis.Destructive != tc.destructive {
				t.Errorf("script destructive = %v", analysis.Destructive)
			}
			// Nothing is run, so a clean script has nothing to say about
			// itself beyond the statements.
			if !tc.destructive && len(analysis.Warnings) != 0 {
				t.Errorf("warnings = %v, want none", analysis.Warnings)
			}
		})
	}
}

func TestAnalyzeScriptReadOnlyAndUnknown(t *testing.T) {
	analysis := (&Conn{}).AnalyzeScript("db.orders.find({});\ndb.orders.insertOne({\"a\": 1})", true)
	if analysis.Refused != 1 {
		t.Errorf("refused = %d, want 1", analysis.Refused)
	}
	if len(analysis.Warnings) == 0 || !strings.Contains(analysis.Warnings[0], "read-only") {
		t.Errorf("warnings = %v, want a read-only notice first", analysis.Warnings)
	}

	// A statement that is not shell at all is reported, not silently accepted.
	bad := (&Conn{}).AnalyzeScript("select 1", false)
	if len(bad.Statements) != 1 || bad.Statements[0].Kind != "unknown" {
		t.Fatalf("analysis = %+v", bad.Statements)
	}
	if len(bad.Warnings) != 1 || !strings.Contains(bad.Warnings[0], "statement 1") {
		t.Errorf("warnings = %v", bad.Warnings)
	}
	if bad.Refused != 0 {
		t.Errorf("refused = %d, want 0 on a writable session", bad.Refused)
	}
}
