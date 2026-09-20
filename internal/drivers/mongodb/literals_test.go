package mongodb

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The shell accepts the mongosh constructors because that is what a script
// copied out of the real shell is full of. They are only respelled as extended
// JSON, so the value is still validated by the BSON parser.

func TestExpandShellLiterals(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`ObjectId("507f1f77bcf86cd799439011")`, `{"$oid":"507f1f77bcf86cd799439011"}`},
		{`ISODate("2026-01-02T03:04:05Z")`, `{"$date":{"$numberLong":"1767323045000"}}`},
		{`new Date("2026-01-02")`, `{"$date":{"$numberLong":"1767312000000"}}`},
		{`new Date(1735787045000)`, `{"$date":{"$numberLong":"1735787045000"}}`},
		{`NumberLong(7)`, `{"$numberLong":"7"}`},
		{`NumberDecimal("12.50")`, `{"$numberDecimal":"12.50"}`},
		{`Timestamp(1735787045, 3)`, `{"$timestamp":{"t":1735787045,"i":3}}`},
		{`RegExp("^ab", "i")`, `{"$regularExpression":{"pattern":"^ab","options":"i"}}`},
		{`RegExp("^ab")`, `{"$regularExpression":{"pattern":"^ab","options":""}}`},
		{`UUID("00010203-0405-0607-0809-0a0b0c0d0e0f")`,
			`{"$binary":{"base64":"AAECAwQFBgcICQoLDA0ODw==","subType":"04"}}`},
		{`BinData(0, "AAECAw==")`, `{"$binary":{"base64":"AAECAw==","subType":"00"}}`},
		{`MinKey()`, `{"$minKey":1}`},
		{`MaxKey()`, `{"$maxKey":1}`},
		// Nesting, quoting and untouched text.
		{`{ at: ISODate("2026-01-02"), _id: ObjectId("507f1f77bcf86cd799439011") }`,
			`{ at: {"$date":{"$numberLong":"1767312000000"}}, _id: {"$oid":"507f1f77bcf86cd799439011"} }`},
		{`[NumberLong(1), NumberLong(2)]`, `[{"$numberLong":"1"}, {"$numberLong":"2"}]`},
		{`{"$oid":"507f1f77bcf86cd799439011"}`, `{"$oid":"507f1f77bcf86cd799439011"}`},
		{`{ sku: "a", $gt: NumberLong(2) }`, `{ sku: "a", $gt: {"$numberLong":"2"} }`},
		{`"ISODate(\"2026-01-02\")"`, `"ISODate(\"2026-01-02\")"`},
		{`sum("total")`, `sum("total")`},
	}
	for _, testCase := range cases {
		got, err := expandShellLiterals(testCase.input)
		if err != nil {
			t.Errorf("expand %s: %v", testCase.input, err)
			continue
		}
		if got != testCase.want {
			t.Errorf("expand %s\n got %s\nwant %s", testCase.input, got, testCase.want)
		}
	}
}

func TestExpandShellLiteralsRejectsBadCalls(t *testing.T) {
	cases := []struct {
		input string
		hint  string
	}{
		{`ObjectId()`, "generate a new id"},
		{`ObjectId("507f1f77bcf86cd799439011", "extra")`, "takes 1 argument"},
		{`ISODate()`, "needs a date"},
		{`ISODate("yesterday")`, "is not a date"},
		{`NumberLong("many")`, "is not a number"},
		{`Timestamp(1)`, "takes 2 argument"},
		{`UUID("nope")`, "16 byte hex"},
		{`BinData(300, "AA==")`, "out of range"},
		{`RegExp()`, "takes 1 or 2 arguments"},
		{`MaxKey(1)`, "takes no arguments"},
		{`ObjectId(at)`, "expects a quoted string"},
		{`ObjectId("unterminated)`, "unbalanced parentheses"},
	}
	for _, testCase := range cases {
		if _, err := expandShellLiterals(testCase.input); err == nil {
			t.Errorf("expand %s: expected an error mentioning %q", testCase.input, testCase.hint)
		} else if !strings.Contains(err.Error(), testCase.hint) {
			t.Errorf("expand %s: error %q does not mention %q", testCase.input, err, testCase.hint)
		}
	}
}

// parseArg is the door: what comes out is BSON, not text.
func TestParseArgUnderstandsShellLiterals(t *testing.T) {
	id, err := parseArg(`ObjectId("507f1f77bcf86cd799439011")`)
	if err != nil {
		t.Fatalf("parse ObjectId: %v", err)
	}
	if objectID, ok := id.(bson.ObjectID); !ok || objectID.Hex() != "507f1f77bcf86cd799439011" {
		t.Fatalf("ObjectId parsed as %#v", id)
	}

	at, err := parseArg(`ISODate("2026-01-02T03:04:05Z")`)
	if err != nil {
		t.Fatalf("parse ISODate: %v", err)
	}
	stamp, ok := at.(bson.DateTime)
	if !ok {
		t.Fatalf("ISODate parsed as %#v", at)
	}
	if text := stamp.Time().UTC().Format("2006-01-02T15:04:05Z"); text != "2026-01-02T03:04:05Z" {
		t.Fatalf("ISODate parsed as %s", text)
	}

	doc, err := parseArg(`{ _id: ObjectId("507f1f77bcf86cd799439011"), at: new Date("2026-01-02"), qty: NumberLong(7) }`)
	if err != nil {
		t.Fatalf("parse JavaScript document: %v", err)
	}
	if len(asElements(doc)) != 3 {
		t.Fatalf("JavaScript document parsed as %#v", doc)
	}

	// Nested objects with unquoted keys, and a boolean that must stay one.
	plain, err := parseArg(`{ sku: "a", active: true, nested: { deep: null } }`)
	if err != nil {
		t.Fatalf("parse unquoted keys: %v", err)
	}
	elements := asElements(plain)
	if len(elements) != 3 || elements[0].Key != "sku" || elements[1].Value != true {
		t.Fatalf("unquoted keys parsed as %#v", plain)
	}

	doc, err = parseArg(`{ _id: ObjectId("507f1f77bcf86cd799439011"), at: new Date("2026-01-02"), qty: NumberLong(7) }`)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	elements = asElements(doc)
	if len(elements) != 3 {
		t.Fatalf("document parsed as %#v", doc)
	}
	if _, ok := elements[0].Value.(bson.ObjectID); !ok {
		t.Errorf("_id parsed as %#v", elements[0].Value)
	}
	if _, ok := elements[1].Value.(bson.DateTime); !ok {
		t.Errorf("at parsed as %#v", elements[1].Value)
	}
	if number, ok := elements[2].Value.(int64); !ok || number != 7 {
		t.Errorf("qty parsed as %#v", elements[2].Value)
	}

	// Ordinary JSON keeps working, and a bad value is still rejected by bson.
	if value, err := parseArg(`{"a": 1}`); err != nil {
		t.Fatalf("parse plain JSON: %v", err)
	} else if elements := asElements(value); len(elements) != 1 {
		t.Fatalf("plain JSON parsed as %#v", value)
	}
	if _, err := parseArg(`ObjectId("nope")`); err == nil {
		t.Fatal("a malformed ObjectID must be rejected")
	}
}
