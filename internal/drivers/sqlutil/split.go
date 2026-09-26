package sqlutil

import (
	"bufio"
	"io"
	"strings"
)

// defaultDelimiter is the separator a script starts with.
const defaultDelimiter = ";"

// maxDelimiter and maxTag bound what a DELIMITER directive and a dollar-quoted
// string may claim. Both are spellings of a marker, and a marker longer than
// this is not one this build will follow.
const (
	maxDelimiter = 32
	maxTag       = 64
)

// ScanStatements reads a script from r and hands every statement it finds to
// each, in the order the script has them.
//
// consumed is the number of bytes of r the scanner had read when that statement
// ended. It is what a window showing progress on a large file needs: the number
// of statements is not known until the file has been read, but its size is.
//
// It is SplitStatements over a stream — nothing except the statement being
// built is ever held, so a dump that does not fit in memory can be run one
// statement at a time. The first error from each stops the scan and comes back;
// so does a read error.
func ScanStatements(r io.Reader, each func(statement string, consumed int64) error) error {
	s := &scanner{in: bufio.NewReaderSize(r, 64<<10), delim: defaultDelimiter}

	for {
		if s.err != nil {
			return s.err
		}
		c, ok := s.peek(0)
		if !ok {
			break
		}

		var err error
		switch {
		case c == '-' && s.at("--"):
			s.lineComment()
		case c == '/' && s.at("/*"):
			s.blockComment()
		case c == '\'' || c == '"' || c == '`':
			s.quoted(c)
		case !s.hasCode && s.delimiterDirective():
			// A client side directive: it selects the separator, it is not a
			// statement, so there is nothing to hand over.
		case s.at(s.delim):
			// The separator is dropped, not kept: it ends the statement, it is
			// not part of it. The offset it is handed over with is the one past
			// it, so the last statement of a file ends at the end of the file.
			err = s.flush(each, s.consumed+int64(len(s.delim)))
			if err == nil {
				s.drop(len(s.delim))
			}
		case c == '$':
			if n, ok := s.dollarTag(); ok {
				s.dollarQuoted(n)
				continue
			}
			s.take(1)
		default:
			switch {
			case c == '\n':
				// Nothing to do beyond keeping the byte.
			case !isBlank(c):
				s.hasCode = true
			}
			s.take(1)
		}
		if err != nil {
			return err
		}
	}

	if s.err != nil {
		return s.err
	}
	// Tail without a trailing separator.
	return s.flush(each, s.consumed)
}

// SplitStatements splits a script into individual statements on semicolons
// while respecting string literals, quoted identifiers and comments.
//
// Trailing semicolons and whitespace-only fragments are dropped. A script with
// no trailing semicolon still yields its final statement. A DELIMITER line
// changes the separator from that point on, and a PostgreSQL dollar-quoted
// string ($$…$$ or $tag$…$tag$) is one statement however many semicolons it
// holds.
func SplitStatements(script string) []string {
	out := []string{}
	// A string cannot fail to be read, so the error is always nil here.
	_ = ScanStatements(strings.NewReader(script), func(statement string, _ int64) error {
		out = append(out, statement)
		return nil
	})
	return out
}

// scanner is the state of one ScanStatements call.
type scanner struct {
	in       *bufio.Reader
	stmt     []byte // the statement being read
	consumed int64  // bytes of the reader scanned so far
	delim    string // the separator in force
	hasCode  bool   // the statement being read holds something that is not blank
	err      error  // the read error to report once scanning stops
}

// peek returns the byte off bytes ahead without consuming it.
func (s *scanner) peek(off int) (byte, bool) {
	data, err := s.in.Peek(off + 1)
	if off < len(data) {
		return data[off], true
	}
	if err != nil && err != io.EOF {
		s.err = err
	}
	return 0, false
}

// at reports whether the reader stands on text.
func (s *scanner) at(text string) bool {
	for i := 0; i < len(text); i++ {
		c, ok := s.peek(i)
		if !ok || c != text[i] {
			return false
		}
	}
	return true
}

// foldAt is at, ignoring case. Used for DELIMITER, which is a client side word
// rather than part of any SQL dialect.
func (s *scanner) foldAt(text string) bool {
	for i := 0; i < len(text); i++ {
		c, ok := s.peek(i)
		if !ok || lowerByte(c) != lowerByte(text[i]) {
			return false
		}
	}
	return true
}

// take consumes n bytes into the statement being built.
func (s *scanner) take(n int) {
	for i := 0; i < n; i++ {
		c, err := s.in.ReadByte()
		if err != nil {
			if err != io.EOF {
				s.err = err
			}
			return
		}
		s.stmt = append(s.stmt, c)
		s.consumed++
	}
}

// drop consumes n bytes and throws them away: they are part of the script but
// not of any statement.
func (s *scanner) drop(n int) {
	for i := 0; i < n; i++ {
		if _, err := s.in.ReadByte(); err != nil {
			if err != io.EOF {
				s.err = err
			}
			return
		}
		s.consumed++
	}
}

// flush hands over the statement read so far and starts the next one. at is the
// offset the statement is reported to have ended at.
func (s *scanner) flush(each func(string, int64) error, at int64) error {
	statement := strings.TrimSpace(string(s.stmt))
	// A statement that filled its buffer would otherwise keep that memory for
	// the rest of the run.
	if cap(s.stmt) > 1<<20 {
		s.stmt = nil
	} else {
		s.stmt = s.stmt[:0]
	}
	s.hasCode = false

	if statement == "" || isOnlyComments(statement) {
		return nil
	}
	return each(statement, at)
}

// lineComment consumes a -- comment, including the newline that ends it.
func (s *scanner) lineComment() {
	s.take(2)
	for {
		c, ok := s.peek(0)
		if !ok {
			return
		}
		s.take(1)
		if c == '\n' {
			return
		}
	}
}

// blockComment consumes a /* */ comment. Most engines do not nest them, so
// neither does this.
func (s *scanner) blockComment() {
	s.take(2)
	for {
		if _, ok := s.peek(0); !ok {
			return
		}
		if s.at("*/") {
			s.take(2)
			return
		}
		s.take(1)
	}
}

// quoted consumes a string literal or a quoted identifier, from the opening
// quote to the closing one.
func (s *scanner) quoted(quote byte) {
	s.hasCode = true
	s.take(1)
	for {
		c, ok := s.peek(0)
		if !ok {
			return
		}
		// A backslash escapes in every dialect that has one inside a string;
		// backtick identifiers (MySQL) do not use it.
		if c == '\\' && quote != '`' {
			s.take(2)
			continue
		}
		if c == quote {
			// A doubled quote is an escaped quote, not the end.
			if s.at(string([]byte{quote, quote})) {
				s.take(2)
				continue
			}
			s.take(1)
			return
		}
		s.take(1)
	}
}

// dollarTag reports the length of the opening $tag$ standing here, if one does.
//
// PostgreSQL is the engine with dollar-quoted strings, and a body may hold
// semicolons, so it has to be read as one statement. The tag follows the rules
// of an unquoted identifier, which also keeps $1 (a parameter) out of it.
func (s *scanner) dollarTag() (int, bool) {
	for i := 1; i <= maxTag; i++ {
		c, ok := s.peek(i)
		if !ok {
			return 0, false
		}
		if c == '$' {
			return i + 1, true
		}
		if i == 1 && c >= '0' && c <= '9' {
			return 0, false
		}
		if !isTagByte(c) {
			return 0, false
		}
	}
	return 0, false
}

// dollarQuoted consumes a dollar-quoted string, opening tag included. n is the
// length of that tag as dollarTag reported it.
func (s *scanner) dollarQuoted(n int) {
	data, err := s.in.Peek(n)
	if len(data) < n {
		if err != nil && err != io.EOF {
			s.err = err
		}
		return
	}
	tag := string(data)
	s.hasCode = true
	s.take(n)

	for {
		if _, ok := s.peek(0); !ok {
			return
		}
		if s.at(tag) {
			s.take(len(tag))
			return
		}
		s.take(1)
	}
}

// delimiterDirective consumes a DELIMITER line and reports whether it did. The
// word is a directive of the mysql command line client — it says where the next
// statement ends, it is not part of the script's SQL, so it is dropped rather
// than run.
func (s *scanner) delimiterDirective() bool {
	const word = "DELIMITER"
	if !s.foldAt(word) {
		return false
	}
	// The word has to stand alone: "delimiterx" is an identifier.
	if c, ok := s.peek(len(word)); ok && !isBlank(c) {
		return false
	}
	s.drop(len(word))
	s.skipBlanks()

	// The separator is the word that follows, which is ";" for the usual reset
	// and ";;" or "$$" for the bodies that need one. Whatever is left on the
	// line stays in the script: a script that changes the separator and carries
	// on with a statement has not lost that statement.
	var token []byte
	for {
		c, ok := s.peek(0)
		if !ok || isBlank(c) {
			break
		}
		token = append(token, c)
		s.drop(1)
	}
	s.skipBlanks()

	// A directive that names nothing usable leaves the separator alone: the
	// rest of the file is then read the way it was before.
	if len(token) > 0 && len(token) <= maxDelimiter {
		s.delim = string(token)
	}
	return true
}

// skipBlanks consumes spaces and tabs, but not a newline.
func (s *scanner) skipBlanks() {
	for {
		c, ok := s.peek(0)
		if !ok || (c != ' ' && c != '\t' && c != '\r') {
			return
		}
		s.drop(1)
	}
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isTagByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		return true
	}
	return false
}

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
