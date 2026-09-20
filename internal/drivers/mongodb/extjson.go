package mongodb

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Extended JSON wrappers
//
// The v2 driver decodes extended JSON into plain data: with an `any` target
// {"$oid": "..."} arrives as a one-key document rather than as an ObjectID.
// Sent to the server unchanged that is a subdocument where an id was meant,
// which is a filter that quietly matches nothing. unwrapExtJSON walks what was
// parsed and rebuilds the BSON value each wrapper stands for, so extended JSON
// typed by hand (the shell accepts it) and the mongosh literals this package
// expands (see literals.go) both reach the wire as the type the user asked for.
//
// A wrapper that is also a query operator stays a document: {"$regex": "^ab"}
// must keep meaning "the regex operator", and its typed twin is spelled
// {"$regularExpression": {"pattern": ..., "options": ...}}.

// unwrapExtJSON rebuilds the BSON values behind extended JSON wrappers,
// recursing into documents and arrays. A wrapper whose payload does not fit its
// type is an error rather than a document that silently matches nothing.
func unwrapExtJSON(value any) (any, error) {
	switch typed := value.(type) {
	case bson.D:
		converted, ok, err := convertWrapper(typed)
		if err != nil {
			return nil, err
		}
		if ok {
			return converted, nil
		}
		for i := range typed {
			inner, err := unwrapExtJSON(typed[i].Value)
			if err != nil {
				return nil, err
			}
			typed[i].Value = inner
		}
		return typed, nil

	case bson.M, map[string]any:
		doc := mapToDocument(typed)
		converted, ok, err := convertWrapper(doc)
		if err != nil {
			return nil, err
		}
		if ok {
			return converted, nil
		}
		for _, element := range doc {
			unwrapped, err := unwrapExtJSON(element.Value)
			if err != nil {
				return nil, err
			}
			setElement(typed, element.Key, unwrapped)
		}
		return typed, nil

	case bson.A:
		for i := range typed {
			inner, err := unwrapExtJSON(typed[i])
			if err != nil {
				return nil, err
			}
			typed[i] = inner
		}
		return typed, nil

	case []any:
		for i := range typed {
			inner, err := unwrapExtJSON(typed[i])
			if err != nil {
				return nil, err
			}
			typed[i] = inner
		}
		return typed, nil
	}
	return value, nil
}

// setElement writes a rebuilt value back into the map it came from.
func setElement(container any, key string, value any) {
	switch typed := container.(type) {
	case bson.M:
		typed[key] = value
	case map[string]any:
		typed[key] = value
	}
}

// convertWrapper turns a single "$key" document into the BSON value it stands
// for. The middle result says the document was such a wrapper, so a payload
// that does not fit its type can be reported instead of passed on.
func convertWrapper(doc bson.D) (any, bool, error) {
	if len(doc) != 1 || !strings.HasPrefix(doc[0].Key, "$") {
		return nil, false, nil
	}
	key := strings.ToLower(doc[0].Key)
	inner := doc[0].Value

	switch key {
	case "$oid":
		text, ok := textValue(inner)
		if !ok {
			return nil, true, fmt.Errorf("$oid takes the 24 character hex string of an id, got %v", inner)
		}
		id, err := bson.ObjectIDFromHex(text)
		if err != nil {
			return nil, true, fmt.Errorf("$oid %q is not a 24 character hex string", text)
		}
		return id, true, nil

	case "$date":
		millis, ok := dateMillis(inner)
		if !ok {
			return nil, true, fmt.Errorf("$date takes a timestamp or milliseconds since the epoch, got %v", inner)
		}
		return bson.NewDateTimeFromTime(time.UnixMilli(millis).UTC()), true, nil

	case "$timestamp":
		seconds, ok := integerValue(mustElement(inner, "t"))
		if !ok {
			return nil, true, fmt.Errorf("$timestamp needs a numeric t and i, got %v", inner)
		}
		increment, ok := integerValue(mustElement(inner, "i"))
		if !ok {
			return nil, true, fmt.Errorf("$timestamp needs a numeric t and i, got %v", inner)
		}
		return bson.Timestamp{T: uint32(seconds), I: uint32(increment)}, true, nil

	case "$numberlong", "$numberint":
		number, ok := integerValue(inner)
		if !ok {
			return nil, true, fmt.Errorf("%s takes a whole number, got %v", key, inner)
		}
		if key == "$numberint" {
			return int32(number), true, nil
		}
		return number, true, nil

	case "$numberdecimal":
		text, ok := textValue(inner)
		if !ok {
			return nil, true, fmt.Errorf("$numberDecimal takes a number as text, got %v", inner)
		}
		decimal, err := bson.ParseDecimal128(text)
		if err != nil {
			return nil, true, fmt.Errorf("$numberDecimal %q is not a decimal number", text)
		}
		return decimal, true, nil

	case "$binary":
		encoded, ok := textValue(mustElement(inner, "base64"))
		if !ok {
			return nil, true, fmt.Errorf("$binary takes %s, got %v", `{"base64": ..., "subType": "00"}`, inner)
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, true, fmt.Errorf("$binary %q is not base64", encoded)
		}
		subtype := 0
		if text, ok := textValue(mustElement(inner, "subType")); ok {
			parsed, err := strconv.ParseUint(text, 16, 8)
			if err != nil {
				return nil, true, fmt.Errorf("$binary subType %q is not a two digit hex byte", text)
			}
			subtype = int(parsed)
		}
		return bson.Binary{Subtype: byte(subtype), Data: raw}, true, nil

	case "$regularexpression":
		pattern, ok := textValue(mustElement(inner, "pattern"))
		if !ok {
			return nil, true, fmt.Errorf("$regularExpression needs a pattern, got %v", inner)
		}
		options, _ := textValue(mustElement(inner, "options"))
		return bson.Regex{Pattern: pattern, Options: options}, true, nil

	case "$minkey":
		return bson.MinKey{}, true, nil

	case "$maxkey":
		return bson.MaxKey{}, true, nil
	}
	return nil, false, nil
}

// dateMillis reads a $date payload: milliseconds, {"$numberLong": millis} or a
// timestamp string.
func dateMillis(inner any) (int64, bool) {
	if millis, ok := integerValue(inner); ok {
		return millis, true
	}
	if nested := mustElement(inner, "$numberLong"); nested != nil {
		if millis, ok := integerValue(nested); ok {
			return millis, true
		}
	}
	text, ok := textValue(inner)
	if !ok {
		return 0, false
	}
	parsed, ok := parseDateText(text)
	if !ok {
		return 0, false
	}
	return parsed.UnixMilli(), true
}

// parseDateText reads the date spellings this driver accepts: RFC3339 with a
// zone, the same without one, and a plain day. Everything is taken as UTC so a
// date typed without a zone does not move with the machine's clock.
func parseDateText(text string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// mustElement looks up one field of a document-ish value.
func mustElement(value any, key string) any {
	wanted := strings.ToLower(key)
	for _, element := range asElements(value) {
		if strings.ToLower(element.Key) == wanted {
			return element.Value
		}
	}
	return nil
}

func textValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

// integerValue reads a number, including the string form extended JSON uses for
// $numberLong and friends.
func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case string:
		if number, err := strconv.ParseInt(typed, 10, 64); err == nil {
			return number, true
		}
		if number, err := strconv.ParseFloat(typed, 64); err == nil {
			return int64(number), true
		}
	}
	return 0, false
}

// mapToDocument makes a key-ordered document out of a map. A wrapper has
// exactly one key, so the order does not matter: it is either that key or an
// ordinary document that convertWrapper rejects.
func mapToDocument(value any) bson.D {
	switch typed := value.(type) {
	case bson.M:
		doc := make(bson.D, 0, len(typed))
		for key, inner := range typed {
			doc = append(doc, bson.E{Key: key, Value: inner})
		}
		return doc
	case map[string]any:
		doc := make(bson.D, 0, len(typed))
		for key, inner := range typed {
			doc = append(doc, bson.E{Key: key, Value: inner})
		}
		return doc
	}
	return nil
}
