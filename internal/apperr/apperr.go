// Package apperr provides an error type that survives the Wails JSON bridge
// with its message intact.
//
// Wails marshals Go errors to JS as plain strings, so a rich error type buys us
// nothing on the wire; what matters is that messages are human readable and
// that we never leak credentials (a DSN containing a password, for instance).
package apperr

import (
	"errors"
	"fmt"
	"regexp"
)

// Error is a classified application error.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// New builds an Error with a code and a formatted message.
func New(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap annotates an existing error with a code and a formatted prefix.
func Wrap(code string, err error, format string, args ...any) *Error {
	if err == nil {
		return nil
	}
	prefix := fmt.Sprintf(format, args...)
	return &Error{Code: code, Message: prefix + ": " + err.Error(), Err: err}
}

// credentialRe matches "password=...", "pwd:..." and URI userinfo sections so
// that they can be redacted from messages shown in the UI.
var credentialRe = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*[^\s;&,]*`)
var userinfoRe = regexp.MustCompile(`://[^:/@\s]+:[^@/\s]+@`)

// Sanitize strips anything that looks like a credential from a message.
func Sanitize(msg string) string {
	out := credentialRe.ReplaceAllString(msg, "$1=***")
	out = userinfoRe.ReplaceAllString(out, "://***:***@")
	return out
}

// Message extracts a user facing message from any error.
func Message(err error) string {
	if err == nil {
		return ""
	}
	return Sanitize(err.Error())
}

// Is reports whether err (or anything it wraps) is an *Error with the code.
func Is(err error, code string) bool {
	var target *Error
	if !errors.As(err, &target) {
		return false
	}
	return target.Code == code
}

// Common error codes.
const (
	CodeNotFound       = "not_found"
	CodeInvalidConfig  = "invalid_config"
	CodeUnsupported    = "unsupported"
	CodeConnectionFail = "connection_failed"
	CodeQueryFailed    = "query_failed"
	CodeReadOnly       = "read_only"
	CodeTimeout        = "timeout"
	CodeInternal       = "internal"
)
