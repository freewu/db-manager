// Exporting a namespace to a file.
//
// This is the one read that does not come back across the bridge: a database
// dump is the rows themselves, so the run takes a destination path, reads each
// table through the driver and writes what it reads, one row at a time. What
// returns to the window is a summary — the path, how many tables and rows landed
// in it, and what could not be written — plus a stream of progress ticks while
// it runs, because a dump of a real database takes long enough that a window
// with no news in it looks broken.
//
// Two shapes of read meet here. The table's *shape* is read through
// Conn.Structure, which is the same call the structure pane makes and is
// whatever the engine says (MySQL answers with its own SHOW CREATE TABLE).
// Its *rows* are read through drivers.RowStreamer, the optional capability that
// reads a table once without collecting it — a dump that paged through OFFSET
// would ask the server to re-send everything it had already sent.
package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// exportTick is the shortest gap between two progress reports. A report per row
// would spend more time crossing to the interface than reading the table.
const exportTick = 100 * time.Millisecond

// ExportDatabase writes the tables named by the request into the file it names.
//
// progress, when it is not nil, is called from this same goroutine: the run is
// the caller's goroutine (Wails gives every bound method its own), so a tick
// cannot arrive after the summary that ended the run. The App's progress
// function posts the tick to the interface; a test's records it.
//
// A table that cannot be written is a warning, not the end of the run: losing
// the eleven tables that can be read because the twelfth is not readable would
// be a worse answer than the eleven plus a sentence about the twelfth. A failure
// that is not about one table — no session, no destination, a full disk — is an
// error, and the file already written is left where the user asked for it.
func (m *Manager) ExportDatabase(req models.ExportRequest, progress func(models.ExportProgress)) (models.ExportResult, error) {
	mode, format, err := exportPlan(req)
	if err != nil {
		return models.ExportResult{}, err
	}

	s, err := m.session(req.SessionID)
	if err != nil {
		return models.ExportResult{}, err
	}

	// Rows are read by the driver's own capability rather than by assembling a
	// SELECT here, so an engine that cannot stream says so instead of being
	// exported a page at a time.
	streamer, _ := s.conn.(drivers.RowStreamer)
	if mode != models.ExportStructure && streamer == nil {
		return models.ExportResult{}, apperr.New(
			apperr.CodeUnsupported,
			"%s cannot stream rows, so its data cannot be exported",
			s.driver.Info().DisplayName,
		)
	}

	// No deadline, for the same reason the stream has none: a large table is not
	// a slow query, it is a large table. The context is here to be cancelled —
	// by the window's Stop button, or by the application closing, which cancels
	// every method's context at once.
	ctx, cancel := context.WithCancel(m.baseCtx)
	defer cancel()

	// The picked fields are checked against the catalog now, before the file
	// exists. Waiting until the rows are read would make a misspelled field name a
	// warning on a half-written dump — the one failure the per-table rule must not
	// swallow, because it is not about that table: it is about the request.
	if mode == models.ExportData && len(req.Columns) > 0 {
		for _, table := range req.Tables {
			structure, err := s.conn.Structure(ctx, req.Database, table.Schema, table.Name)
			if err != nil {
				return models.ExportResult{}, err
			}
			if _, err := pickedColumns(structure, req.Columns); err != nil {
				return models.ExportResult{}, err
			}
		}
	}

	run := &exportRun{
		session:   s,
		req:       req,
		mode:      mode,
		format:    format,
		streamer:  streamer,
		dialect:   s.conn.Dialect(),
		cancel:    cancel,
		startedAt: time.Now(),
		total:     len(req.Tables),
		jsonFirst: true,
	}
	m.registerExport(run.id(), run.stop)
	defer m.unregisterExport(run.id())

	return run.run(ctx, progress)
}

// CancelExport stops the run with this id, if it is still going.
//
// Cancelling a run that has already finished is not an error: the window and the
// run are not in step — the window may ask to stop in the same instant the run
// reports that it is done — and "that export is no longer running" is exactly
// what the caller wanted to hear.
func (m *Manager) CancelExport(id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "no export id")
	}
	m.exportsMu.Lock()
	cancel := m.exports[id]
	m.exportsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// registerExport remembers how to stop one run, so CancelExport can find it
// while it is going.
func (m *Manager) registerExport(id string, cancel func()) {
	if cancel == nil {
		return
	}
	m.exportsMu.Lock()
	defer m.exportsMu.Unlock()
	if m.exports == nil {
		m.exports = map[string]func(){}
	}
	m.exports[id] = cancel
}

func (m *Manager) unregisterExport(id string) {
	m.exportsMu.Lock()
	defer m.exportsMu.Unlock()
	delete(m.exports, id)
}

// exportPlan reads the request as a decision, refusing anything the window
// should not have been able to send.
//
// These refusals are for the window's bugs, not for the user's mistakes: the
// dialog only offers combinations that make sense (a structure export is always
// SQL, a CSV export is always one table), so a request that breaks those rules
// is a request that never came from the form.
func exportPlan(req models.ExportRequest) (models.ExportMode, models.ExportFormat, error) {
	if strings.TrimSpace(req.ID) == "" {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "no export id")
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "no session specified")
	}
	if strings.TrimSpace(req.Path) == "" {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "no destination file")
	}
	if len(req.Tables) == 0 {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "no tables selected")
	}

	mode := req.Mode
	switch mode {
	case models.ExportStructure, models.ExportStructureData, models.ExportData:
	default:
		return "", "", apperr.New(apperr.CodeInvalidConfig, "unknown export mode %q", req.Mode)
	}

	format := req.Format
	if format == "" {
		format = models.ExportSQL
	}
	switch format {
	case models.ExportSQL, models.ExportCSV, models.ExportJSON, models.ExportJSONL:
	default:
		return "", "", apperr.New(apperr.CodeInvalidConfig, "unknown export format %q", req.Format)
	}

	// The structure modes write a script, and a script is SQL.
	if mode != models.ExportData && format != models.ExportSQL {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "%s can only be written as SQL", mode)
	}
	// CSV, JSON and JSONL have nowhere to say which table a row came from, and
	// no comment syntax to say it in: one file is one table's worth of rows.
	if format != models.ExportSQL && len(req.Tables) != 1 {
		return "", "", apperr.New(apperr.CodeInvalidConfig, "a %s export writes one table", format)
	}

	for _, table := range req.Tables {
		if strings.TrimSpace(table.Name) == "" {
			return "", "", apperr.New(apperr.CodeInvalidConfig, "a table with no name")
		}
	}
	return mode, format, nil
}

// exportRun is one export in flight: the state the writer threads through the
// tables it writes.
type exportRun struct {
	session *session
	req     models.ExportRequest

	mode   models.ExportMode
	format models.ExportFormat

	// streamer and dialect are the session's connection seen as a row source and
	// as a way of spelling a literal. Both are read once: neither changes under
	// a live session.
	streamer drivers.RowStreamer
	dialect  drivers.Dialect

	cancel func()
	// stopped is set by CancelExport before it cancels the context, so the run
	// can tell "the user pressed Stop" from "the application is closing" — the
	// context cannot tell the two apart, and the answer to "did I finish?" is
	// different for each.
	stopped atomic.Bool

	file *os.File
	out  *bufio.Writer
	// err is the first failure of the file itself. A buffer that could not be
	// written to cannot be written to later either, so the run stops at the
	// first news of it rather than at the end.
	err error

	startedAt time.Time
	// current is the table being written, for progress ticks.
	current string
	// progress is where ticks go, or nil when nobody is listening (a test that
	// only looks at the file). It is set by run.
	progress func(models.ExportProgress)

	total    int
	done     int
	tables   int
	rows     int64
	bytes    int64
	warnings []string
	lastTick time.Time
	// jsonFirst tracks whether the JSON array already holds an object, which is
	// what tells a comma from an opening bracket.
	jsonFirst bool
	// fatal is a failure that is about the request rather than about one table, and
	// therefore ends the run instead of becoming one table's warning.
	fatal error
}

func (r *exportRun) id() string { return r.req.ID }

// stop is what CancelExport holds: the two facts of a user-pressed Stop, in the
// order that matters — the flag first, so the run cannot be read as "finished"
// by the time it notices the cancellation.
func (r *exportRun) stop() {
	r.stopped.Store(true)
	r.cancel()
}

// run writes the whole export and returns its summary.
func (r *exportRun) run(ctx context.Context, progress func(models.ExportProgress)) (models.ExportResult, error) {
	r.progress = progress
	if err := r.open(); err != nil {
		return models.ExportResult{}, err
	}
	// The file is closed whatever happened, including a stop: what has been
	// written is what the user asked for, and deleting it would be a second
	// decision they did not make.
	defer r.close()

	// A SQL file is read by people as well as by a server, so it says what it is
	// and when it was taken. The row-oriented formats have no comment syntax —
	// a line that is not data would corrupt them — so they get no header.
	if r.format == models.ExportSQL {
		r.writeHeader()
	}
	if r.format == models.ExportJSON {
		r.write("[\n")
	}
	r.tick(true)

	for _, table := range r.req.Tables {
		if r.stopped.Load() {
			break
		}
		r.current = exportTableLabel(table)
		if err := r.writeTable(ctx, table); err != nil {
			if r.stopped.Load() {
				break
			}
			if apperr.Is(err, apperr.CodeInvalidConfig) {
				// The request is what is wrong, and every table after this one
				// would be written with the same wrong request. (The picked fields
				// are checked before the file is created, so this is a backstop
				// rather than the usual path.)
				r.fatal = err
				break
			}
			if r.err != nil {
				// The file is the problem, not the table: every table after this
				// one would fail the same way, so the run stops and says so
				// instead of collecting one warning per table.
				break
			}
			// One table that cannot be read does not end the run, but it is
			// remembered: the summary has to be able to say what is missing.
			r.warnings = append(r.warnings, fmt.Sprintf("%s: %s", r.current, apperr.Message(err)))
		} else {
			r.tables++
		}
		r.done++
		r.tick(true)
	}

	if r.fatal != nil {
		return models.ExportResult{}, r.fatal
	}
	if r.format == models.ExportJSON {
		// Closed even when the run was stopped: a truncated array is not JSON,
		// and the rows that did land should still be readable.
		if !r.jsonFirst {
			r.write("\n")
		}
		r.write("]\n")
	}
	if flushErr := r.out.Flush(); flushErr != nil && r.err == nil {
		r.err = flushErr
	}
	if r.err != nil {
		return models.ExportResult{}, apperr.Wrap(apperr.CodeInternal, r.err, "write %s", r.req.Path)
	}

	return models.ExportResult{
		Path:      r.req.Path,
		Tables:    r.tables,
		Rows:      r.rows,
		Bytes:     r.bytes,
		Cancelled: r.stopped.Load(),
		Warnings:  r.warnings,
	}, nil
}

// open creates the destination file. The user picked it in a save dialog a
// moment ago, so an existing file is theirs to replace — that is what the
// dialog's confirmation was about.
func (r *exportRun) open() error {
	file, err := os.Create(r.req.Path)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "create %s", r.req.Path)
	}
	r.file = file
	// Sixty-four kilobytes: the writer's job is to keep the file from being
	// written a row at a time, and the rows are what there are a lot of.
	r.out = bufio.NewWriterSize(file, 64<<10)
	return nil
}

func (r *exportRun) close() {
	if r.file == nil {
		return
	}
	if r.out != nil {
		if err := r.out.Flush(); err != nil && r.err == nil {
			r.err = err
		}
	}
	_ = r.file.Close()
	r.file = nil
}

// write appends text to the file, keeping the first failure. Every write in this
// file goes through here, so no caller has to think about what a failing disk
// does to the row after it.
func (r *exportRun) write(text string) {
	if r.err != nil {
		return
	}
	n, err := r.out.WriteString(text)
	r.bytes += int64(n)
	if err != nil {
		r.err = err
	}
}

// writeHeader names the file, the database and the moment.
func (r *exportRun) writeHeader() {
	r.write("--\n")
	r.write("-- db-manager export\n")
	r.write("-- Database: " + headerValue(r.req.Database) + "\n")
	r.write("-- Driver: " + string(r.session.driver.Info().Type) + "\n")
	r.write("-- Exported: " + r.startedAt.UTC().Format(time.RFC3339) + "\n")
	r.write("-- Tables: " + strconv.Itoa(r.total) + "\n")
	r.write("--\n")
}

// writeTable writes one table: its definition, its rows, or both, according to
// the mode.
func (r *exportRun) writeTable(ctx context.Context, table models.ExportTable) error {
	label := exportTableLabel(table)

	structure, err := r.session.conn.Structure(ctx, r.req.Database, table.Schema, table.Name)
	if err != nil {
		return err
	}

	// A SQL file is read by people too, so each table's part announces itself.
	// This comes after the structure call on purpose: a table that cannot be read
	// leaves no comment promising rows that are not there.
	if r.format == models.ExportSQL {
		r.write("\n-- Table: " + label + "\n")
	}

	if r.mode != models.ExportData {
		if err := r.writeStructure(table, structure); err != nil {
			return err
		}
	}
	if r.mode == models.ExportStructure {
		return nil
	}

	columns, err := r.columnsOf(structure)
	if err != nil {
		return err
	}
	return r.writeRows(ctx, table, columns)
}

// writeStructure writes the engine's own CREATE statement for one table.
//
// The statement comes from the catalog rather than from this application's
// renderer, so a dump carries what the engine actually holds: the keys, the
// options, the storage clauses — everything a reconstruction would lose. An
// engine that cannot answer with its own text is answered for by
// Conn.Structure's fallback, which is the same text the structure pane shows.
func (r *exportRun) writeStructure(table models.ExportTable, structure *models.TableStructure) error {
	ddl := strings.TrimSpace(structure.DDL)
	if ddl == "" {
		return fmt.Errorf("%s has no definition to write", exportTableLabel(table))
	}
	r.write(terminate(ddl) + "\n")
	return r.err
}

// columnsOf decides which fields a data export writes, in the order it writes
// them: the ones the user picked, or every field in the catalog's own order.
//
// The list comes from the structure rather than from the result set because a
// result set is a bag of values with names on it, and the file has to say which
// field each value belongs to — the CSV header, the INSERT's column list — so the
// fields have to be known before the first row arrives.
func (r *exportRun) columnsOf(structure *models.TableStructure) ([]string, error) {
	if len(r.req.Columns) == 0 {
		return catalogColumns(structure), nil
	}
	return pickedColumns(structure, r.req.Columns)
}

// catalogColumns is every field of a table, in the order the catalog declares
// them — which is also the order the driver's rows arrive in.
func catalogColumns(structure *models.TableStructure) []string {
	out := make([]string, 0, len(structure.Columns))
	for _, column := range structure.Columns {
		out = append(out, column.Name)
	}
	return out
}

// pickedColumns turns the field names the user chose into the table's own
// spelling of them, in the order they chose.
//
// The names are matched back to the catalog (engines are case-insensitive about
// identifiers, but a file that quoted a field differently from the table would
// not read back), and a name that is not there is refused by name rather than
// dropped: a column silently missing from a dump is discovered at restore time.
func pickedColumns(structure *models.TableStructure, picked []string) ([]string, error) {
	known := catalogColumns(structure)
	out := make([]string, 0, len(picked))
	for _, wanted := range picked {
		name := strings.TrimSpace(wanted)
		found := ""
		for _, column := range known {
			if strings.EqualFold(column, name) {
				found = column
				break
			}
		}
		if found == "" {
			return nil, apperr.New(apperr.CodeInvalidConfig, "table %s has no field %q", structure.Object.Name, name)
		}
		out = append(out, found)
	}
	if len(out) == 0 {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no fields selected")
	}
	return out, nil
}

// writeRows streams one table's rows into the file.
func (r *exportRun) writeRows(ctx context.Context, table models.ExportTable, columns []string) error {
	// A CSV file's first line names its columns; there is no other place to put
	// them, and a file of numbers with no headings is a file nobody can use.
	if r.format == models.ExportCSV {
		r.write(csvRow(columnNames(columns)))
	}

	err := r.streamer.StreamRows(ctx, drivers.StreamRequest{
		Database: r.req.Database,
		Schema:   table.Schema,
		Object:   table.Name,
		Columns:  columns,
	}, func(row []any) error {
		return r.writeRow(table, columns, row)
	})
	if err != nil {
		return err
	}
	return r.err
}

// writeRow writes one row in the chosen format.
func (r *exportRun) writeRow(table models.ExportTable, columns []string, row []any) error {
	if len(row) != len(columns) {
		return apperr.New(
			apperr.CodeInternal,
			"a row of %s has %d values for %d fields",
			exportTableLabel(table), len(row), len(columns),
		)
	}

	switch r.format {
	case models.ExportSQL:
		r.write(insertStatement(r.dialect, r.req.Database, table, columns, row))
	case models.ExportCSV:
		r.write(csvRow(row))
	case models.ExportJSON:
		if r.jsonFirst {
			r.jsonFirst = false
		} else {
			r.write(",\n")
		}
		r.write(jsonRow(columns, row))
	case models.ExportJSONL:
		r.write(jsonRow(columns, row) + "\n")
	}

	r.rows++
	r.tick(false)
	return r.err
}

// tick reports progress, at most every exportTick unless it is forced.
//
// The forced ticks (start of the run, end of a table) are the ones that must not
// be swallowed by the throttle: they are what tells a window that a table it is
// waiting on has landed.
func (r *exportRun) tick(force bool) {
	if r.progress == nil {
		return
	}
	if !force && time.Since(r.lastTick) < exportTick {
		return
	}
	r.lastTick = time.Now()
	r.progress(models.ExportProgress{
		ID:    r.req.ID,
		Done:  r.done,
		Total: r.total,
		Table: r.current,
		Rows:  r.rows,
		Bytes: r.bytes,
	})
}

// --- rendering -------------------------------------------------------------

// insertStatement renders one row as an INSERT.
//
// One statement per row, rather than a batched multi-row VALUES: a batch is
// smaller for short rows, but it has to be cut somewhere (MySQL's
// max_allowed_packet, SQLite's SQL length limit), a failure then loses a whole
// batch instead of a row, and a file of single-row statements can be read and
// resumed by eye. The column list is spelled out on every row so the file does
// not depend on the table's column order to be reloaded.
func insertStatement(d drivers.Dialect, database string, table models.ExportTable, columns []string, row []any) string {
	var b strings.Builder
	b.Grow(64 + len(columns)*8 + len(row)*12)
	b.WriteString("INSERT INTO ")
	b.WriteString(d.Qualify(database, table.Schema, table.Name))
	b.WriteString(" (")
	for i, name := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(d.Quote(name))
	}
	b.WriteString(") VALUES (")
	for i, value := range row {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlbase.Literal(d, value))
	}
	b.WriteString(");\n")
	return b.String()
}

// jsonRow renders one row as a JSON object, with the fields in the order they
// were read.
//
// The object is assembled by hand rather than by marshalling a map, because a
// map has no order and a JSON file whose fields shuffle between rows is a file
// that cannot be diffed.
func jsonRow(columns []string, row []any) string {
	var b strings.Builder
	b.Grow(32 + len(columns)*12)
	b.WriteByte('{')
	for i, name := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteJSON(name))
		b.WriteString(": ")
		b.WriteString(jsonValue(row[i]))
	}
	b.WriteByte('}')
	return b.String()
}

// jsonValue spells one value as JSON.
//
// The values arrive already decoded by the driver (see sqlbase.coerceValue):
// text, whole numbers, floats, booleans, nil, and the occasional time or blob
// rendered as text. Numbers are written as numbers — a dump whose counts come
// back quoted is a file every reader has to clean up first — and text goes
// through encoding/json so the escaping is the standard one.
func jsonValue(value any) string {
	if number, ok := jsonNumber(value); ok {
		return number
	}
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(typed)
	default:
		// A time, a blob, or a value only this driver knows: written as the text
		// the other formats would write for it.
		return quoteJSON(textOf(value))
	}
}

// jsonNumber renders any of Go's numbers as JSON, and says whether the value was
// one.
//
// The list is long because the driver hands back whatever the engine's driver
// decided, and it is written out rather than reflected over because it is the
// same list sqlbase's literal renderer writes — a number that is a number in a
// statement has to stay a number in a file.
func jsonNumber(value any) (string, bool) {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return floatJSON(float64(typed)), true
	case float64:
		return floatJSON(typed), true
	case json.Number:
		// A driver reading in raw mode can hand these back, and the text is
		// already JSON's own spelling of the number.
		return typed.String(), true
	default:
		return "", false
	}
}

// floatJSON spells a float the way JSON does — through encoding/json, so the
// exponent rule is the standard one — and falls back to a quoted word for the
// three values JSON has no spelling for. A file that cannot be parsed is worse
// than one that says "NaN": nobody can act on a NaN either way.
func floatJSON(value float64) string {
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return quoteJSON(strconv.FormatFloat(value, 'g', -1, 64))
}

// quoteJSON renders text as a JSON string.
func quoteJSON(text string) string {
	encoded, err := json.Marshal(text)
	if err != nil {
		// json.Marshal cannot fail on a string; this is here so that a value
		// that somehow cannot be encoded is written down rather than dropped.
		return strconv.Quote(text)
	}
	return string(encoded)
}

// csvRow renders one row as RFC 4180 text: cells separated by commas, the line
// ended by CRLF, and a cell quoted when it holds a comma, a quote or a line
// break.
//
// It is deliberately the same rule the result grid's export follows
// (frontend/src/lib/export.ts). The two are not shared because they run on
// opposite sides of the bridge: the grid has the rows in hand and writes a file
// from the window, while this one never sees the rows outside the file it is
// writing. Duplicating forty lines is cheaper than moving either one.
func csvRow(values []any) string {
	var b strings.Builder
	b.Grow(16 + len(values)*8)
	for i, value := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(csvCell(value))
	}
	b.WriteString("\r\n")
	return b.String()
}

func csvCell(value any) string {
	text := textOf(value)
	if strings.ContainsAny(text, ",\"\r\n") {
		return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	}
	return text
}

// textOf renders a value as plain text, for the formats that are text rather
// than SQL. The cases are the ones a driver sends back after decoding; anything
// else is written as Go spells it, which is at least reproducible.
func textOf(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	case bool:
		return strconv.FormatBool(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return floatText(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

// floatText spells a float the way a person reads it: a whole number is written
// out (1000000, not 1e+06), and only a number far enough from one to be
// unreadable in full falls back to an exponent.
func floatText(value float64) string {
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// terminate makes sure a statement ends in a semicolon.
//
// Engines that answer with their own CREATE often leave it off (MySQL's SHOW
// CREATE TABLE has none), and a file of statements has to be loadable as one
// script — which for most clients means every statement ends in a semicolon.
func terminate(statement string) string {
	if strings.HasSuffix(statement, ";") {
		return statement
	}
	return statement + ";"
}

// exportTableLabel spells a table for a comment or a warning: qualified when
// there is a schema to qualify it with, bare otherwise. It is for people, not
// for the engine — a statement is built from the parts, never from this.
func exportTableLabel(table models.ExportTable) string {
	if table.Schema == "" {
		return table.Name
	}
	return table.Schema + "." + table.Name
}

// columnNames is the label of each column, for a CSV header.
func columnNames(columns []string) []any {
	out := make([]any, len(columns))
	for i, name := range columns {
		out[i] = name
	}
	return out
}

// headerValue keeps a name from breaking the comment it is written into: a
// comment ends at the end of its line, and an identifier is allowed to contain a
// line break.
func headerValue(text string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
}
