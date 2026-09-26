// Running a SQL file: the window behind the connection tree's "run SQL file"
// action.
//
// The file is read and run in the backend rather than in the window, for the
// same reason the export writes its file there: a dump is routinely larger than
// the bridge wants to carry, and it is on the same machine as the engine it is
// being written to. The statements are read one at a time and run one at a
// time, so a file that does not fit in memory can still be imported, progress
// can be reported as the file is consumed, and the run can stop between two
// statements.
//
// Two consequences of running one statement at a time are worth stating, because
// they are choices rather than accidents:
//
//   - Every statement goes through Manager.executeScript, so it is read-only
//     checked, timed and logged exactly as it would be from the query window —
//     with its own change log entry, and with the row count the engine returned
//     for that one statement instead of a count for the whole file.
//   - Nothing wraps the file in a transaction. The drivers have no transaction
//     handle, DDL cannot be rolled back on every engine, and a dump's own BEGIN
//     and COMMIT statements are just statements like any other. A run that stops
//     halfway leaves the statements that ran in place, and the summary says how
//     many that was.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

const (
	// sqlFileTick is how often progress travels to the window while a file is
	// running. The run happens on the goroutine that was called, so this is a
	// throttle rather than a queue: a file of ten thousand statements must not
	// send ten thousand events across the bridge.
	sqlFileTick = 100 * time.Millisecond
	// sqlFileShown is how many statements the analysis describes one by one. The
	// window shows what is coming, and a dump of a hundred thousand INSERTs is
	// not something to send a window a hundred thousand previews of.
	sqlFileShown = 50
	// sqlFileWarningLimit caps the analysis warnings and the list of destructive
	// statements: enough to see the pattern, few enough to read.
	sqlFileWarningLimit = 20
	// sqlFileErrorLimit caps the failures a result carries, and
	// sqlFileMessageLimit the length of one engine message — an import where
	// everything fails should not answer with a megabyte of the same sentence.
	sqlFileErrorLimit   = 20
	sqlFileMessageLimit = 400
)

// errSQLFileStopped ends a scan early: the user asked to stop, or a statement
// failed and the run was told to stop at the first failure. It never leaves this
// file — the run answers with a result that says what happened instead.
var errSQLFileStopped = errors.New("the run was stopped")

// AnalyzeSQLFile reports what a file holds without running any of it.
//
// It is the dry run the window shows before the user commits to a file: how many
// statements there are, what kinds, which of them can lose schema or data, and
// whether a read-only connection would refuse any of them. Nothing contacts the
// engine, so this is not a syntax check — it is what the file says about itself.
func (m *Manager) AnalyzeSQLFile(req models.SQLFileRequest) (*models.SQLFileAnalysis, error) {
	s, err := m.sqlFileSession(req.SessionID)
	if err != nil {
		return nil, err
	}
	file, size, err := sqlFileOpen(req.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	analysis := models.SQLFileAnalysis{Path: req.Path, Size: size, Shown: []models.ScriptStatement{}}
	warn := func(format string, args ...any) {
		if len(analysis.Warnings) >= sqlFileWarningLimit {
			return
		}
		analysis.Warnings = append(analysis.Warnings, fmt.Sprintf(format, args...))
	}

	index := 0
	scanErr := sqlutil.ScanStatements(file, func(statement string, _ int64) error {
		entry := sqlutil.Inspect(statement, index)
		if index < sqlFileShown {
			analysis.Shown = append(analysis.Shown, entry)
		}
		if entry.Destructive {
			analysis.Destructive++
			if len(analysis.DestructiveIndexes) < sqlFileWarningLimit {
				analysis.DestructiveIndexes = append(analysis.DestructiveIndexes, index+1)
			}
			warn("statement %d: %s", index+1, entry.Reason)
		}
		if s.readOnly && entry.Kind != sqlutil.KindQuery {
			analysis.Refused++
		}

		name, isUse, useErr := sqlFileTarget(entry.Preview)
		switch {
		case isUse && useErr != nil:
			warn("statement %d: %s", index+1, apperr.Message(useErr))
		case isUse:
			warn("statement %d: USE %s — the statements after it are sent to %s", index+1, name, name)
		case entry.Kind == sqlutil.KindUnknown:
			warn("statement %d: unrecognised statement, it will be sent to the server as-is", index+1)
		}

		index++
		return nil
	})
	if scanErr != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, scanErr, "read %s", filepath.Base(req.Path))
	}
	analysis.Statements = index
	return &analysis, nil
}

// RunSQLFile runs a file statement by statement and answers with a summary.
//
// progress is called on this goroutine while the file is being read, and may be
// nil. The run can be stopped with CancelSQLFile from another goroutine.
func (m *Manager) RunSQLFile(req models.SQLFileRequest, progress func(models.SQLFileProgress)) (*models.SQLFileResult, error) {
	if strings.TrimSpace(req.ID) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no run id")
	}
	if _, err := m.sqlFileSession(req.SessionID); err != nil {
		return nil, err
	}
	file, size, err := sqlFileOpen(req.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// The run's own context, not the manager's: it is what the stop button
	// cancels, and every statement's own timeout hangs off it so that stopping a
	// file also stops the statement that is running.
	ctx, cancel := context.WithCancel(m.baseCtx)
	defer cancel()
	m.registerRun(req.ID, cancel)
	defer m.unregisterRun(req.ID)

	run := &sqlFileRun{
		manager:   m,
		req:       req,
		size:      size,
		database:  req.Database,
		startedAt: time.Now(),
	}
	return run.run(ctx, file, progress)
}

// CancelSQLFile stops the run with this id, if it is still going. See cancelRun
// for what "still going" is allowed to mean.
func (m *Manager) CancelSQLFile(id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "no run id")
	}
	m.cancelRun(id)
	return nil
}

// sqlFileSession resolves the session a file is to be run against, refusing the
// engines that do not have statements to run.
func (m *Manager) sqlFileSession(sessionID string) (*session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no session specified")
	}
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	// Asked as a capability rather than by engine name: an engine that is not
	// relational has no SQL file to run, and says so instead of failing on the
	// first statement.
	if !s.driver.Info().Relational {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s runs its own scripts rather than SQL, so a file of statements is not something to run against it",
			s.driver.Info().DisplayName)
	}
	return s, nil
}

// sqlFileOpen opens the file a request names and reports its size.
//
// The size is read here rather than while scanning, because it is what progress
// is a fraction of: the number of statements is not known until the file has
// been read to the end.
func sqlFileOpen(path string) (*os.File, int64, error) {
	if strings.TrimSpace(path) == "" {
		return nil, 0, apperr.New(apperr.CodeInvalidConfig, "no SQL file chosen")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeInvalidConfig, err, "open %s", filepath.Base(path))
	}
	if info.IsDir() {
		return nil, 0, apperr.New(apperr.CodeInvalidConfig, "%s is a folder, not a file", filepath.Base(path))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeInvalidConfig, err, "open %s", filepath.Base(path))
	}
	return file, info.Size(), nil
}

// sqlFileRun is the state of one run.
type sqlFileRun struct {
	manager *Manager
	req     models.SQLFileRequest
	size    int64
	// database is what the statements are sent to. A USE in the file changes it:
	// a file written for another client says which database it is about that
	// way, and this window can honour it because it names the database with
	// every statement it sends.
	database string

	statements int
	failed     int
	rows       int64
	errors     []models.SQLFileError
	truncated  bool
	recent     string
	// consumed is how many bytes of the file the scanner has read, which is what
	// progress draws its bar from.
	consumed  int64
	lastTick  time.Time
	startedAt time.Time
	// stopped says the run ended at a failed statement rather than at the end of
	// the file. cancelled says the user asked it to stop.
	stopped   bool
	cancelled bool
}

func (r *sqlFileRun) run(ctx context.Context, file *os.File, progress func(models.SQLFileProgress)) (*models.SQLFileResult, error) {
	r.tick(progress, 0, true)
	r.lastTick = time.Now()

	scanErr := sqlutil.ScanStatements(file, func(statement string, consumed int64) error {
		if ctx.Err() != nil {
			r.cancelled = true
			return errSQLFileStopped
		}
		r.consumed = consumed
		if !r.step(ctx, statement, progress, consumed) {
			return errSQLFileStopped
		}
		return nil
	})

	if scanErr != nil && !errors.Is(scanErr, errSQLFileStopped) {
		return nil, apperr.Wrap(apperr.CodeInternal, scanErr, "read %s", filepath.Base(r.req.Path))
	}

	// The file has either been read to the end or abandoned mid-way, and the
	// last tick says which: a bar that stops short of the end is the truth about
	// a run that was stopped.
	end := r.consumed
	if scanErr == nil {
		end = r.size
	}
	r.tick(progress, end, true)

	return &models.SQLFileResult{
		Path:            r.req.Path,
		Statements:      r.statements,
		Failed:          r.failed,
		Rows:            r.rows,
		Cancelled:       r.cancelled,
		StoppedOnError:  r.stopped,
		DurationMS:      time.Since(r.startedAt).Milliseconds(),
		Errors:          r.errors,
		ErrorsTruncated: r.truncated,
	}, nil
}

// step runs one statement and reports whether the run should carry on.
func (r *sqlFileRun) step(ctx context.Context, statement string, progress func(models.SQLFileProgress), consumed int64) bool {
	entry := sqlutil.Inspect(statement, r.statements)
	r.statements++
	r.recent = entry.Preview

	// A USE is the file saying which database it is about. It is not sent to the
	// engine — every statement this window sends names its database itself, so
	// the server's own idea of "current database" is not used — it changes the
	// name the statements after it are sent with.
	if name, isUse, useErr := sqlFileTarget(entry.Preview); isUse {
		if useErr == nil {
			r.database = name
		} else {
			r.record(entry, useErr)
			if r.req.StopOnError {
				r.stopped = true
				return false
			}
		}
		r.tick(progress, consumed, false)
		return true
	}

	res, err := r.manager.executeScript(ctx, models.ExecRequest{
		SessionID: r.req.SessionID,
		Database:  r.database,
		SQL:       statement,
		TimeoutMS: r.req.TimeoutMS,
	}, models.ChangeSourceSQLFile)
	if res != nil {
		r.rows += res.AffectedRows
	}

	if err != nil {
		if ctx.Err() != nil {
			// The stop button interrupted this statement rather than the engine
			// refusing it. It is recorded as a cancelled run, not as a failed
			// statement: the statement did not get to answer.
			r.cancelled = true
			return false
		}
		r.record(entry, err)
		if r.req.StopOnError {
			r.stopped = true
			return false
		}
	}

	r.tick(progress, consumed, false)
	return true
}

// record keeps one failure for the summary.
func (r *sqlFileRun) record(entry models.ScriptStatement, err error) {
	r.failed++
	if len(r.errors) >= sqlFileErrorLimit {
		r.truncated = true
		return
	}
	r.errors = append(r.errors, models.SQLFileError{
		Index:     entry.Index + 1,
		Statement: entry.Preview,
		Message:   sqlFileMessage(err),
	})
}

// tick sends one progress event, unless the last one was sent a moment ago:
// progress is a readout, and nothing is gained by sending it faster than a
// window can paint it.
func (r *sqlFileRun) tick(progress func(models.SQLFileProgress), consumed int64, force bool) {
	if progress == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(r.lastTick) < sqlFileTick {
		return
	}
	r.lastTick = now
	progress(models.SQLFileProgress{
		ID:        r.req.ID,
		Bytes:     consumed,
		Size:      r.size,
		Done:      r.statements,
		Failed:    r.failed,
		Statement: r.recent,
		Rows:      r.rows,
	})
}

// sqlFileTarget reads the database a USE names.
//
// isUse is false when the statement is not a USE at all, which is the common
// case. err says why a statement that is one could not be read, in which case
// the run reports that statement as failed rather than guessing: sending the
// rest of the file to the wrong database is worse than not running it.
func sqlFileTarget(preview string) (name string, isUse bool, err error) {
	fields := strings.Fields(preview)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "use") {
		return "", false, nil
	}
	rest := strings.TrimSpace(preview[len(fields[0]):])

	// A name may be quoted, which is how an identifier with a space in it is
	// written; a name that is not quoted has to be one word.
	quoted := false
	if n := len(rest); n >= 2 {
		switch {
		case rest[0] == '`' && rest[n-1] == '`':
			quoted = true
		case rest[0] == '"' && rest[n-1] == '"':
			quoted = true
		case rest[0] == '[' && rest[n-1] == ']':
			quoted = true
		}
		if quoted {
			rest = rest[1 : n-1]
		}
	}
	name = strings.TrimSpace(rest)

	refuse := func() (string, bool, error) {
		return "", true, apperr.New(apperr.CodeInvalidConfig,
			"cannot read the database name in %s", preview)
	}
	if name == "" || strings.ContainsAny(name, "`\"'[]") {
		return refuse()
	}
	if !quoted && strings.ContainsAny(name, " \t") {
		return refuse()
	}
	return name, true, nil
}

// sqlFileMessage is one engine message as a single short line: an error from a
// server can be several lines and several hundred characters, and the summary
// shows a list of them.
func sqlFileMessage(err error) string {
	text := strings.Join(strings.Fields(apperr.Message(err)), " ")
	if len(text) > sqlFileMessageLimit {
		return text[:sqlFileMessageLimit] + "…"
	}
	return text
}
