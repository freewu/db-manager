// The change log: the record of what this application ran against the databases
// it was pointed at.
//
// Entries are written by the layer that executes statements, at the moment they
// are executed, so a change cannot be made through the application without
// leaving a line behind. There are exactly three places that can write one, and
// they are the three places that execute something other than a read:
//
//   - Manager.Execute, for a script somebody wrote (the query window, the DDL
//     editor, a database being created) — one entry per write statement;
//   - Manager.applyPlan, for the structure page saving a draft, which runs the
//     planned statements one at a time and therefore knows each outcome;
//   - nothing else: a cell edited in the grid and a batch of generated rows are
//     statements this application builds itself, with bound values and no text a
//     user ever read, so they are out of scope for a log whose every line is
//     supposed to be a statement that was run.
//
// The file itself lives in the data directory and is written by the config store
// (see config/changelog.go); this file is only about what goes in it.
package service

import (
	"net"
	"strconv"
	"time"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

const (
	// changeLogEntryVersion is written on every entry, so a later build can tell
	// how to read a line this one wrote.
	changeLogEntryVersion = 1
	// changeLogDefaultPage is what a window gets when it does not say how much it
	// wants. It is more than the list can usefully be scrolled through and far
	// less than the file can hold.
	changeLogDefaultPage = 200
	// changeLogMaxPage bounds one answer, whatever a caller asks for: the whole
	// log travels over the bridge in one message, and a window has no use for
	// more lines than this anyway.
	changeLogMaxPage = 2000
)

// ListChangeLog returns the newest entries first, with the total behind them.
func (m *Manager) ListChangeLog(limit int) (models.ChangeLog, error) {
	if limit <= 0 {
		limit = changeLogDefaultPage
	}
	if limit > changeLogMaxPage {
		limit = changeLogMaxPage
	}

	store := m.storeRef()
	if store == nil {
		// A manager built without a store has nowhere to keep a log; only a test
		// constructs one that way, and an empty log is the honest answer.
		return models.ChangeLog{Entries: []models.ChangeLogEntry{}}, nil
	}
	entries, total, err := store.ChangeLog(limit)
	if err != nil {
		return models.ChangeLog{}, err
	}
	if entries == nil {
		entries = []models.ChangeLogEntry{}
	}
	return models.ChangeLog{Entries: entries, Total: total}, nil
}

// statementPlace is where a logged statement was written: the object the window
// that ran it was open on, when it was open on one, plus which window it was.
type statementPlace struct {
	database string
	schema   string
	object   string
	source   string
}

// logStatement records one statement that has just been executed.
//
// `place` is where the statement was written — taken from the window that ran
// it, never parsed out of the text: nothing here reads SQL to find out what a
// statement touched. A statement that creates a table is recorded without an
// object, because the table it names does not exist yet and the name is in the
// statement itself — so the field says "what this was applied to", not "what it
// mentions".
func (m *Manager) logStatement(
	s *session,
	place statementPlace,
	statement, kind string,
	runErr error,
) {
	table := place.object
	if sqlutil.CreatesTable(statement) {
		table = ""
	}
	m.appendChange(s, models.ChangeLogEntry{
		Database:  place.database,
		Schema:    place.schema,
		Table:     table,
		Kind:      kind,
		Source:    place.source,
		Statement: statement,
		Error:     errorText(runErr),
	})
}

// logScript records the write statements of a script that was just run.
//
// A read is not a change and is not logged, and neither is a statement this build
// does not recognise (a session variable, a `USE`, a transaction boundary): the
// log answers "what was done to this database", and lines that cannot be read as
// an answer to that question make it harder to read.
func (m *Manager) logScript(s *session, req models.ExecRequest, runErr error) {
	place := statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   models.ChangeSourceScript,
	}
	for _, statement := range statementsOf(s, req.SQL) {
		switch statement.Kind {
		case sqlutil.KindDDL, sqlutil.KindDML:
			m.logStatement(s, place, statement.SQL, statementKind(statement.SQL, statement.Kind), runErr)
		}
	}
}

// statementsOf splits and classifies a script the way its engine understands it:
// a driver that cannot be described by SQL keywords describes itself, which is
// what makes a document store's `db.orders.insertOne(…)` a write statement here
// exactly as it is in the DDL editor's dry run.
func statementsOf(s *session, script string) []models.ScriptStatement {
	if analyzer, ok := s.conn.(drivers.Analyzer); ok {
		return analyzer.AnalyzeScript(script, s.readOnly).Statements
	}
	return sqlutil.Analyze(script, s.readOnly).Statements
}

// statementKind is what the log's tag says a statement did.
//
// It is the statement's own leading keyword when the keyword table can read it
// — `create`, `alter`, `insert`, … — which is what a reader scans the list by.
// A statement that cannot be classified that way is one written in another
// syntax (`db.orders.insertOne(…)` has no keyword to show: its first word is
// `db`, and calling it that would be worse than saying nothing), so the
// classification its own driver produced is the honest name for what it did.
func statementKind(statement, fallback string) string {
	if sqlutil.KindOf(statement) == sqlutil.KindUnknown {
		return fallback
	}
	return sqlutil.LeadingWord(statement)
}

// appendChange writes one entry, filling in when it happened and where.
//
// Everything in here is best effort. The statement has already run by the time
// this is called, so a log that cannot be written must not be reported as a
// change that did not happen: the user would go looking for a table that is
// there. What is lost is a line of history, which is the smaller of the two
// losses.
func (m *Manager) appendChange(s *session, entry models.ChangeLogEntry) {
	store := m.storeRef()
	if store == nil {
		return
	}
	entry.Version = changeLogEntryVersion
	if entry.At == 0 {
		entry.At = time.Now().UnixMilli()
	}
	entry.Connection = connectionSummary(s.cfg)
	_ = store.AppendChangeLog(entry)
}

// connectionSummary is what an entry remembers about the connection it ran on.
//
// It is copied at write time rather than looked up when the log is read: a
// profile can be renamed, re-pointed at another server or deleted, and a log that
// changes its mind about where a statement ran is not a log. Nothing secret is
// kept — the address and the user name are what tells two servers apart.
func connectionSummary(cfg models.ConnectionConfig) models.ChangeLogConnection {
	address := cfg.FilePath
	if address == "" && cfg.Host != "" {
		address = cfg.Host
		if cfg.Port > 0 {
			address = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
		}
	}
	return models.ChangeLogConnection{
		ID:      cfg.ID,
		Name:    cfg.Name,
		Driver:  cfg.Driver,
		Address: address,
		User:    cfg.Username,
	}
}

// errorText is an error as an entry keeps it: absent when there was none.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
