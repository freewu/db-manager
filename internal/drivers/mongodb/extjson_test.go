package mongodb

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The driver parses extended JSON into plain data, so a wrapper such as
// {"$oid": "..."} stays a one-key document unless something rebuilds the BSON
// value it stands for. Sending the wrapper on unchanged stores (or matches) a
// subdocument where an id was meant, which is a filter that silently returns
// nothing — the same failure mode as reading the id "507f..." as the number
// 507, so it is worth pinning down.

// sampleID is the id used across these tests: a valid 24 character hex string.
const sampleIDHex = "507f1f77bcf86cd799439011"

var sampleID = func() bson.ObjectID {
	id, err := bson.ObjectIDFromHex(sampleIDHex)
	if err != nil {
		panic(err)
	}
	return id
}()

// valuesEqual compares two decoded values the way the server would see them.
func valuesEqual(left, right any) bool {
	encodedLeft, err := bson.Marshal(bson.D{{Key: "v", Value: left}})
	if err != nil {
		return false
	}
	encodedRight, err := bson.Marshal(bson.D{{Key: "v", Value: right}})
	if err != nil {
		return false
	}
	return bytes.Equal(encodedLeft, encodedRight)
}

func TestParseLiteralRebuildsExtendedJSONValues(t *testing.T) {
	cases := []struct {
		raw  string
		want any
	}{
		// The wrappers the grid shows, typed back in by a user.
		{`{"$oid": "507f1f77bcf86cd799439011"}`, sampleID},
		{`{"$date": {"$numberLong": "1767323045000"}}`, bson.NewDateTimeFromTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))},
		{`{"$date": "2026-01-02T03:04:05Z"}`, bson.NewDateTimeFromTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))},
		{`{"$numberLong": "7"}`, int64(7)},
		{`{"$numberInt": "7"}`, int32(7)},
		{`{"$timestamp": {"t": 1767323045, "i": 3}}`, bson.Timestamp{T: 1767323045, I: 3}},
		{`{"$binary": {"base64": "AAECAw==", "subType": "00"}}`, bson.Binary{Subtype: 0, Data: []byte{0, 1, 2, 3}}},
		{`{"$regularExpression": {"pattern": "^ab", "options": "i"}}`, bson.Regex{Pattern: "^ab", Options: "i"}},
		{`{"$minKey": 1}`, bson.MinKey{}},
		{`{"$maxKey": 1}`, bson.MaxKey{}},

		// Wrappers nested in a document keep working, and the document around
		// them is not disturbed.
		{`{"_id": {"$oid": "507f1f77bcf86cd799439011"}, "n": 1}`,
			bson.D{{Key: "_id", Value: sampleID}, {Key: "n", Value: 1}}},
		{`[{"$oid": "507f1f77bcf86cd799439011"}]`,
			bson.A{sampleID}},

		// Query operators have to stay documents: {"$regex": "..."} is the
		// operator, {"$regularExpression": ...} is the value.
		{`{"$gt": 5}`, bson.D{{Key: "$gt", Value: 5}}},
		{`{"$regex": "^ab", "$options": "i"}`, bson.D{{Key: "$regex", Value: "^ab"}, {Key: "$options", Value: "i"}}},
		{`{"$exists": true}`, bson.D{{Key: "$exists", Value: true}}},
	}
	for _, tc := range cases {
		got := parseLiteral(tc.raw)
		if !valuesEqual(got, tc.want) {
			t.Errorf("parseLiteral(%s) = %#v (%T), want %#v (%T)", tc.raw, got, got, tc.want, tc.want)
		}
	}
}

func TestParseLiteralKeepsBadWrappersAsText(t *testing.T) {
	// A wrapper whose payload does not fit its type is neither usable JSON nor
	// the value the user meant. The grid's rule for text that is not JSON is
	// "it is a literal string", so it is reported instead of quietly becoming a
	// subdocument that matches nothing.
	for _, raw := range []string{
		`{"$oid": "nope"}`,
		`{"$date": "yesterday"}`,
		`{"$numberLong": "seven"}`,
	} {
		if got := parseLiteral(raw); got != raw {
			t.Errorf("parseLiteral(%s) = %#v, want the raw text", raw, got)
		}
	}
}

func TestShellRejectsBadExtendedJSONWrappers(t *testing.T) {
	cases := []struct {
		arg  string
		want string
	}{
		{`ObjectId("nope")`, "not a 24 character hex string"},
		{`{_id: ObjectId("nope")}`, "not a valid ObjectID"},
		{`NumberLong("seven")`, "is not a number"},
		{`ISODate("yesterday")`, "is not a date"},
		{`BinData(0, "not base64!")`, "is not base64"},
	}
	for _, tc := range cases {
		if _, err := parseArg(tc.arg); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("parseArg(%s) error = %v, want %q", tc.arg, err, tc.want)
		}
	}
}

func TestShellReadsExtendedJSONValues(t *testing.T) {
	// Values a user writes by hand, including the wrappers the grid shows, have
	// to reach the server as BSON types rather than as subdocuments.
	value, err := parseArg(`{_id: {"$oid": "507f1f77bcf86cd799439011"}, tags: ["a", 2]}`)
	if err != nil {
		t.Fatalf("parseArg: %v", err)
	}
	document, ok := value.(bson.D)
	if !ok {
		t.Fatalf("parseArg returned %T, want a document", value)
	}
	if !valuesEqual(document[0].Value, sampleID) {
		t.Errorf("field _id = %#v, want an ObjectID", document[0].Value)
	}
	if !valuesEqual(document[1].Value, bson.A{"a", 2}) {
		t.Errorf("field tags = %#v, want [a 2]", document[1].Value)
	}
}
