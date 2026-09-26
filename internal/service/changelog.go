// The change log: the record of what this application ran against the databases
// it was pointed at.
//
// Entries are written by the layer that executes statements, at the moment they
// are executed, so a change cannot be made through the application without
// leaving a line behind. Every place that executes something other than a read
// writes one:
//
//   - Manager.Execute, for a script somebody wrote (the query window, the DDL
//     editor, a database being created) — one entry per write statement;
//   - Manager.applyPlan, for the structure page saving a draft, which runs the
//     planned statements one at a time and therefore knows each outcome;
//   - Manager.UpdateRow and Manager.DeleteRow, for a row edited or removed from
//     the result grid: the statement is built by this application rather than
//     typed by the user, and it is logged as the statement that was run;
//   - Manager.InsertRows, for a batch of rows the data generation window made
//     up — one entry per batch, since a batch is what the engine is handed.
//
// Every entry carries how much data the statement changed, as the engine counted
// it (`Rows`, in models.ChangeLogEntry). That number is the point of the record
// as much as the statement is: "an UPDATE ran" and "an UPDATE rewrote 4,120
// rows" are different pieces of news.
//
// The file itself lives in the data directory and is written by the config store
// (see config/changelog.go); this file is only about what goes in it.
package service

import (
	"net"
	"strconv"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
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

// ListChangeLog returns the newest entries of one file first, the files the log
// is spread over, and how many entries the file read holds.
//
// An empty file name means the file being written; an archived log is read by
// the name the window got from the listing. The file list travels with the page
// because a reader who is looking at an archive still has to be able to see that
// a newer one exists and go back to it.
func (m *Manager) ListChangeLog(file string, limit int) (models.ChangeLog, error) {
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
		return models.ChangeLog{File: changeLogFileOf(file), Entries: []models.ChangeLogEntry{}, Files: []models.ChangeLogFile{}}, nil
	}
	name, err := config.ChangeLogName(file)
	if err != nil {
		return models.ChangeLog{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "open the change log")
	}
	log, err := store.ChangeLog(name, limit)
	if err != nil {
		return models.ChangeLog{}, err
	}
	if log.Entries == nil {
		log.Entries = []models.ChangeLogEntry{}
	}
	if log.Files == nil {
		log.Files = []models.ChangeLogFile{}
	}
	return log, nil
}

// ChangeLogSettings reports how the log is kept, and what it may be set to.
func (m *Manager) ChangeLogSettings() (models.ChangeLogSettings, error) {
	store := m.storeRef()
	if store == nil {
		// A manager built without a store has no settings file to read; only a
		// test constructs one that way, and the defaults are the honest answer.
		return config.DefaultChangeLogSettings(), nil
	}
	return store.ChangeLogSettings()
}

// SaveChangeLogSettings stores how many statements one log file holds before it
// is archived.
//
// The bounds are checked here rather than clamped quietly: the number is how
// much history one file may hold, and a value the user did not choose is a worse
// answer than being told what the limits are.
func (m *Manager) SaveChangeLogSettings(settings models.ChangeLogSettings) (models.ChangeLogSettings, error) {
	current, err := m.ChangeLogSettings()
	if err != nil {
		return models.ChangeLogSettings{}, err
	}
	if settings.MaxEntries < current.Min || settings.MaxEntries > current.Max {
		return models.ChangeLogSettings{}, apperr.New(apperr.CodeInvalidConfig,
			"a change log file has to hold between %d and %d statements", current.Min, current.Max)
	}
	store := m.storeRef()
	if store == nil {
		return current, nil
	}
	if err := store.SaveChangeLogSettings(settings); err != nil {
		return models.ChangeLogSettings{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "save the change log settings")
	}
	return m.ChangeLogSettings()
}

// changeLogFileOf is the file name a listing is about when there is no store to
// ask: the live log, which is what an empty name selects.
func changeLogFileOf(file string) string {
	if name, err := config.ChangeLogName(file); err == nil {
		return name
	}
	return file
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
//
// `rows` is what the engine reported as affected, and zero when nothing reported
// one: DDL has no count, and a statement that failed changed nothing.
func (m *Manager) logStatement(
	s *session,
	place statementPlace,
	statement, kind string,
	rows int64,
	runErr error,
) {
	table := place.object
	if sqlutil.CreatesTable(statement) {
		table = ""
	}
	if runErr != nil {
		// A statement that did not go through changed nothing, whatever a count
		// from an earlier step of the same run happens to say.
		rows = 0
	}
	m.appendChange(s, models.ChangeLogEntry{
		Database:  place.database,
		Schema:    place.schema,
		Table:     table,
		Kind:      kind,
		Source:    place.source,
		Statement: statement,
		Rows:      rows,
		Error:     errorText(runErr),
	})
}

// logScript records the write statements of a script that was just run.
//
// source is the window the script came from, which is what the entry says about
// where it was written: a query window and a file being imported both run text
// this application did not write, and the log is the only place that tells them
// apart afterwards.
//
// A read is not a change and is not logged, and neither is a statement this build
// does not recognise (a session variable, a `USE`, a transaction boundary): the
// log answers "what was done to this database", and lines that cannot be read as
// an answer to that question make it harder to read.
//
// The row count is the engine's, and the engine answers a script with one result
// for the whole run — so the count can only be attributed when the script holds a
// single write statement. With two of them there is no way to say which changed
// the rows, and a number on the wrong line is worse than no number at all.
// Nothing is lost from the record itself: a statement that changed rows says so,
// it just says it without a figure.
func (m *Manager) logScript(s *session, req models.ExecRequest, source string, res *models.QueryResult, runErr error) {
	place := statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   source,
	}
	writes := make([]models.ScriptStatement, 0, 4)
	for _, statement := range statementsOf(s, req.SQL) {
		switch statement.Kind {
		case sqlutil.KindDDL, sqlutil.KindDML:
			writes = append(writes, statement)
		}
	}
	var rows int64
	if len(writes) == 1 && res != nil {
		rows = res.AffectedRows
	}
	for _, statement := range writes {
		m.logStatement(s, place, statement.SQL, statementKind(statement.SQL, statement.Kind), rows, runErr)
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
