// Package service is the application layer sitting between the Wails bound
// methods and the database drivers. It owns the session registry, the
// connection profile store and the error/redaction policy.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/planned"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// ConnectTimeout bounds a single connection attempt.
const ConnectTimeout = 20 * time.Second

// Manager owns every live session.
type Manager struct {
	baseCtx context.Context

	mu       sync.RWMutex
	sessions map[string]*session
	store    *config.Store

	// exportsMu guards the runs in flight, which is a separate lock from the
	// session lock on purpose: an export is stopped from the window's thread
	// while the export itself is reading a table, and neither has anything to
	// do with the other.
	exportsMu sync.Mutex
	exports   map[string]func()
}

type session struct {
	id          string
	cfg         models.ConnectionConfig
	driver      drivers.Driver
	conn        drivers.Conn
	readOnly    bool
	connectedAt int64
	version     string
}

// New creates a Manager and opens the profile store.
func New() (*Manager, error) {
	store, err := config.New()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "open config directory")
	}
	return &Manager{
		baseCtx:  context.Background(),
		sessions: map[string]*session{},
		store:    store,
		exports:  map[string]func(){},
	}, nil
}

// SetContext installs the Wails lifetime context so in-flight queries are
// cancelled when the window closes.
func (m *Manager) SetContext(ctx context.Context) {
	if ctx != nil {
		m.baseCtx = ctx
	}
}

// ConfigDir exposes the profile directory for the welcome screen.
func (m *Manager) ConfigDir() string { return m.storeRef().Dir() }

// storeRef is how the rest of the manager reaches the profile store.
//
// The pointer can be swapped (the settings page moves the data directory), so
// the field is read under the same lock that guards the swap rather than being
// captured at construction time: a store that has already been moved away from
// would otherwise keep writing into an empty directory.
func (m *Manager) storeRef() *config.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

func (m *Manager) ctx(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(m.baseCtx, timeout)
}

// Shutdown closes every session. Called on application exit.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	sessions := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = map[string]*session{}
	m.mu.Unlock()

	for _, s := range sessions {
		_ = s.conn.Close()
	}
}

// --- drivers ---------------------------------------------------------------

// Drivers returns implemented drivers followed by the roadmap entries.
func (m *Manager) Drivers() []models.DriverInfo {
	infos := drivers.Infos()
	infos = append(infos, planned.Infos()...)
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].SortOrder < infos[j].SortOrder })
	return infos
}

// DriverInfos is an alias kept for symmetry with the frontend API.
func (m *Manager) DriverInfos() []models.DriverInfo { return m.Drivers() }

// --- connection profiles ---------------------------------------------------

// Connections lists every saved profile, redacted.
func (m *Manager) Connections() ([]models.ConnectionConfig, error) {
	list, err := m.storeRef().Load()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "read connection profiles")
	}
	out := make([]models.ConnectionConfig, 0, len(list))
	for _, c := range list {
		out = append(out, c.Redacted())
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// SaveConnection validates and persists a profile.
func (m *Manager) SaveConnection(cfg models.ConnectionConfig) (models.ConnectionConfig, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return cfg, apperr.New(apperr.CodeInvalidConfig, "connection name is required")
	}
	drv, ok := drivers.Get(cfg.Driver)
	if !ok {
		return cfg, apperr.New(apperr.CodeUnsupported, "driver %q is not available", cfg.Driver)
	}
	if err := drv.Normalize(&cfg); err != nil {
		return cfg, err
	}
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}

	stored, err := m.storeRef().Upsert(cfg)
	if err != nil {
		return cfg, apperr.Wrap(apperr.CodeInternal, err, "save connection profile")
	}
	return stored.Redacted(), nil
}

// DeleteConnection removes a profile.
func (m *Manager) DeleteConnection(id string) error {
	if id == "" {
		return apperr.New(apperr.CodeInvalidConfig, "connection id is required")
	}
	if err := m.storeRef().Delete(id); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "delete connection profile")
	}
	return nil
}

// TestConnection validates credentials without registering a session. It never
// returns a Go error: the outcome is part of the payload so the UI can show a
// friendly banner.
func (m *Manager) TestConnection(cfg models.ConnectionConfig) models.TestResult {
	result := models.TestResult{}

	drv, ok := drivers.Get(cfg.Driver)
	if !ok {
		result.Message = "driver " + string(cfg.Driver) + " is not available yet"
		return result
	}
	if err := drv.Normalize(&cfg); err != nil {
		result.Message = apperr.Message(err)
		return result
	}

	ctx, cancel := m.ctx(ConnectTimeout)
	defer cancel()

	started := time.Now()
	conn, err := drv.Open(ctx, cfg)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Message = apperr.Message(err)
		return result
	}
	defer conn.Close()

	if v, err := conn.Version(ctx); err == nil {
		result.ServerVer = v
	}
	if db, err := conn.CurrentDatabase(ctx); err == nil {
		result.ConnectedDB = db
	}
	if names, err := conn.Databases(ctx); err == nil {
		result.Databases = len(names)
	}
	result.OK = true
	result.Message = "Connection succeeded"
	return result
}

// --- sessions --------------------------------------------------------------

// Open establishes (or reuses) a session.
func (m *Manager) Open(req models.OpenRequest) (models.SessionInfo, error) {
	cfg, err := m.resolveConfig(req)
	if err != nil {
		return models.SessionInfo{}, err
	}

	// Reuse an existing session for the same profile so repeated double clicks
	// do not leak pools.
	if cfg.ID != "" {
		m.mu.RLock()
		for _, s := range m.sessions {
			if s.cfg.ID == cfg.ID {
				info := s.info()
				m.mu.RUnlock()
				return info, nil
			}
		}
		m.mu.RUnlock()
	}

	drv, ok := drivers.Get(cfg.Driver)
	if !ok {
		return models.SessionInfo{}, apperr.New(apperr.CodeUnsupported, "driver %q is not available", cfg.Driver)
	}
	if err := drv.Normalize(&cfg); err != nil {
		return models.SessionInfo{}, err
	}
	if req.ReadOnly {
		cfg.ReadOnly = true
	}

	ctx, cancel := m.ctx(ConnectTimeout)
	defer cancel()

	conn, err := drv.Open(ctx, cfg)
	if err != nil {
		return models.SessionInfo{}, err
	}

	// A secret typed into the connect prompt is written back when the profile
	// opted into "remember password", so the next connect does not have to ask
	// again (the dialog promises exactly that). The store keeps the password only
	// when SavePassword is set, so this is a no-op for every other profile — and
	// it is best effort: the session is already up, failing the connect because
	// the file could not be rewritten would be worse than asking once more.
	if req.Password != "" && cfg.SavePassword && cfg.ID != "" {
		if stored, err := m.storeRef().Upsert(cfg); err == nil {
			cfg.Password = stored.Password
		}
	}

	s := &session{
		id:          uuid.NewString(),
		cfg:         cfg,
		driver:      drv,
		conn:        conn,
		readOnly:    cfg.ReadOnly,
		connectedAt: time.Now().UnixMilli(),
	}
	if v, err := conn.Version(ctx); err == nil {
		s.version = v
	}

	m.mu.Lock()
	m.sessions[s.id] = s
	m.mu.Unlock()

	return s.info(), nil
}

func (m *Manager) resolveConfig(req models.OpenRequest) (models.ConnectionConfig, error) {
	if req.Config != nil {
		cfg := *req.Config
		if req.Password != "" {
			cfg.Password = req.Password
		}
		if req.Database != "" {
			cfg.Database = req.Database
		}
		return cfg, nil
	}
	if req.ConnectionID == "" {
		return models.ConnectionConfig{}, apperr.New(apperr.CodeInvalidConfig, "no connection specified")
	}
	cfg, found, err := m.storeRef().Find(req.ConnectionID)
	if err != nil {
		return models.ConnectionConfig{}, apperr.Wrap(apperr.CodeInternal, err, "read connection profile")
	}
	if !found {
		return models.ConnectionConfig{}, apperr.New(apperr.CodeNotFound, "connection profile not found")
	}
	if req.Password != "" {
		cfg.Password = req.Password
	}
	if req.Database != "" {
		cfg.Database = req.Database
	}
	return cfg, nil
}

// Close tears a session down. Unknown ids are a no-op so the UI can call this
// defensively.
func (m *Manager) Close(sessionID string) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}
	if err := s.conn.Close(); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "close session")
	}
	return nil
}

// Sessions lists the live sessions.
func (m *Manager) Sessions() []models.SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]models.SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s.info())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ConnectedAt < out[j].ConnectedAt })
	return out
}

func (s *session) info() models.SessionInfo {
	return models.SessionInfo{
		ID:            s.id,
		Name:          s.cfg.Name,
		Driver:        s.cfg.Driver,
		ConnectionID:  s.cfg.ID,
		Database:      s.cfg.Database,
		ServerVersion: s.version,
		ReadOnly:      s.readOnly,
		ConnectedAt:   s.connectedAt,
	}
}

func (m *Manager) session(id string) (*session, error) {
	if id == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no session specified")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound, "session is no longer open")
	}
	return s, nil
}

// Session exposes one session's metadata.
func (m *Manager) Session(id string) (models.SessionInfo, error) {
	s, err := m.session(id)
	if err != nil {
		return models.SessionInfo{}, err
	}
	return s.info(), nil
}

// --- metadata --------------------------------------------------------------

// Databases lists catalogs visible in a session.
func (m *Manager) Databases(sessionID string) ([]string, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(30 * time.Second)
	defer cancel()

	names, err := s.conn.Databases(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Schemas lists schemas inside a database.
func (m *Manager) Schemas(sessionID, database string) ([]string, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.driver.Info().SupportsSchema {
		return []string{}, nil
	}
	ctx, cancel := m.ctx(30 * time.Second)
	defer cancel()

	names, err := s.conn.Schemas(ctx, database)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Objects lists tables/views inside a namespace.
func (m *Manager) Objects(sessionID, database, schema string) ([]models.ObjectInfo, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	return s.conn.Objects(ctx, database, schema)
}

// Structure returns the full description (and DDL) of one object.
func (m *Manager) Structure(sessionID, database, schema, object string) (*models.TableStructure, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "object name is required")
	}
	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	return s.conn.Structure(ctx, database, schema, object)
}

// --- runtime overview ------------------------------------------------------

// Overview reports how the server behind a live session is doing right now.
//
// The engine-specific part comes from the driver (drivers.Overviewer). Three
// situations are worth answering with a page instead of an error, because the
// session itself is perfectly healthy:
//
//   - an engine that cannot report runtime state at all,
//   - a user without the privileges a status page needs,
//
// so those arrive as Warnings on an otherwise empty page, while a hard failure
// (a dropped connection) still returns an error.
func (m *Manager) Overview(sessionID string) (*models.ServerOverview, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()

	info := s.info()
	page := &models.ServerOverview{
		SessionID:     info.ID,
		Name:          info.Name,
		Driver:        string(info.Driver),
		ServerVersion: info.ServerVersion,
		Database:      info.Database,
		ReadOnly:      info.ReadOnly,
		ConnectedAt:   info.ConnectedAt,
		CollectedAt:   time.Now().UnixMilli(),
		Warnings:      []string{},
	}

	started := time.Now()
	reporter, ok := s.conn.(drivers.Overviewer)
	switch {
	case !ok:
		page.Warnings = append(page.Warnings, fmt.Sprintf(
			"%s does not report runtime state yet.", s.driver.Info().DisplayName))
	default:
		collected, err := reporter.Overview(ctx)
		switch {
		case err == nil:
			// The driver only fills in its engine field and server-side values;
			// the session fields above stay authoritative.
			page.MySQL = collected.MySQL
			page.Postgres = collected.Postgres
			page.SQLite = collected.SQLite
			page.Mongo = collected.Mongo
			page.Supported = collected.Supported
			if collected.ServerVersion != "" {
				page.ServerVersion = collected.ServerVersion
			}
			page.Warnings = append(page.Warnings, collected.Warnings...)
		case apperr.Is(err, apperr.CodeUnsupported):
			page.Warnings = append(page.Warnings, err.Error())
		default:
			return nil, err
		}
	}
	page.ElapsedMS = time.Since(started).Milliseconds()
	return page, nil
}

// --- creating databases ----------------------------------------------------

// DatabaseOptions reports what this session's server accepts for a new
// database: the character sets and collations (MySQL family), the encodings and
// locales (PostgreSQL), or just the sentences that explain why there is nothing
// to choose (Doris, MongoDB).
//
// The list is always read from the live server, never from a table in the UI:
// which character sets exist is a property of the release and the
// configuration, not of the driver.
func (m *Manager) DatabaseOptions(sessionID string) (*models.DatabaseOptions, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	creator, ok := s.conn.(drivers.DatabaseCreator)
	if !ok {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s does not create databases", s.driver.Info().DisplayName)
	}

	ctx, cancel := m.ctx(30 * time.Second)
	defer cancel()
	return creator.DatabaseOptions(ctx)
}

// PlanCreateDatabase renders the statement that creates a database. Nothing is
// executed: the window shows this string and runs that exact string through
// Execute, so the preview can never differ from what happens.
func (m *Manager) PlanCreateDatabase(sessionID string, req models.CreateDatabaseRequest) (*models.DatabasePlan, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	creator, ok := s.conn.(drivers.DatabaseCreator)
	if !ok {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s does not create databases", s.driver.Info().DisplayName)
	}

	plan, err := creator.CreateDatabase(req)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// Graph describes a whole namespace for the ER diagram: objects, columns and
// the foreign keys between them.
//
// Drivers that implement drivers.Grapher answer in one bounded round trip; for
// the others the same information is assembled from per-object Structure calls,
// so every shipped engine gets a diagram even before it grows a Grapher.
func (m *Manager) Graph(sessionID, database, schema string) (*models.SchemaGraph, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(120 * time.Second)
	defer cancel()

	if grapher, ok := s.conn.(drivers.Grapher); ok {
		return grapher.Graph(ctx, database, schema)
	}
	return m.graphByStructure(ctx, s, database, schema)
}

// graphByStructure builds a graph out of one Structure call per object. It runs
// the same job as sqlbase.Graph, with the same bounded concurrency, for drivers
// that cannot describe a namespace themselves.
func (m *Manager) graphByStructure(ctx context.Context, s *session, database, schema string) (*models.SchemaGraph, error) {
	objects, err := s.conn.Objects(ctx, database, schema)
	if err != nil {
		return nil, err
	}
	graph := &models.SchemaGraph{
		Driver:   string(s.driver.Info().Type),
		Database: database,
		Schema:   schema,
		Nodes:    make([]models.GraphNode, 0, len(objects)),
		Edges:    []models.GraphEdge{},
		Warnings: []string{},
	}

	const workers = 4
	type result struct {
		structure *models.TableStructure
		err       error
	}
	results := make([]result, len(objects))
	indexes := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				structure, err := s.conn.Structure(ctx, database, schema, objects[i].Name)
				results[i] = result{structure: structure, err: err}
			}
		}()
	}
	for i := range objects {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	known := make(map[string]string, len(objects))
	for _, object := range objects {
		known[strings.ToLower(object.Name)] = object.Name
	}

	for i, object := range objects {
		graph.Nodes = append(graph.Nodes, models.GraphNode{
			Name:    object.Name,
			Kind:    object.Kind,
			Comment: object.Comment,
			Columns: []models.GraphColumn{},
		})
		if results[i].err != nil || results[i].structure == nil {
			message := object.Name + ": structure is unavailable"
			if results[i].err != nil {
				message = object.Name + ": " + results[i].err.Error()
			}
			graph.Warnings = append(graph.Warnings, message)
			continue
		}

		structure := results[i].structure
		for _, column := range structure.Columns {
			graph.Nodes[len(graph.Nodes)-1].Columns = append(
				graph.Nodes[len(graph.Nodes)-1].Columns,
				models.GraphColumn{
					Name:       column.Name,
					Type:       column.DataType,
					Nullable:   column.Nullable,
					PrimaryKey: column.PrimaryKey,
				})
		}
		for _, fk := range structure.ForeignKeys {
			graph.Edges = append(graph.Edges, models.GraphEdge{
				From:       object.Name,
				FromColumn: fk.Columns,
				To:         fk.ReferencedTable,
				ToColumn:   fk.ReferencedColumns,
				Name:       fk.Name,
				OnDelete:   fk.OnDelete,
				OnUpdate:   fk.OnUpdate,
			})
		}
	}

	sort.Strings(graph.Warnings)
	return graph, nil
}

// Indexes lists every index of a namespace (used by the explorer tree).
func (m *Manager) Indexes(sessionID, database, schema string) ([]models.IndexEntry, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	return s.conn.Indexes(ctx, database, schema)
}

// --- data ------------------------------------------------------------------

// Fetch returns a page of rows for the data grid.
func (m *Manager) Fetch(req models.FetchRequest) (*models.FetchResult, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "object name is required")
	}
	ctx, cancel := m.ctx(QueryTimeout(req.TimeoutMS))
	defer cancel()

	return s.conn.Fetch(ctx, drivers.FetchRequest{
		Database:   req.Database,
		Schema:     req.Schema,
		Object:     req.Object,
		Limit:      req.Limit,
		Offset:     req.Offset,
		OrderBy:    req.OrderBy,
		Filters:    req.Filters,
		CountTotal: req.CountTotal,
		TimeoutMS:  req.TimeoutMS,
	})
}

// Execute runs a script.
func (m *Manager) Execute(req models.ExecRequest) (*models.QueryResult, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.SQL) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "nothing to execute")
	}
	ctx, cancel := m.ctx(QueryTimeout(req.TimeoutMS))
	defer cancel()

	readOnly := req.ReadOnly || s.readOnly
	res, err := s.conn.Execute(ctx, drivers.ExecRequest{
		Database:  req.Database,
		SQL:       req.SQL,
		MaxRows:   req.MaxRows,
		TimeoutMS: req.TimeoutMS,
		ReadOnly:  readOnly,
	})
	// Logged after the call rather than before it, so a statement that never
	// reached the server is not recorded as a change; the failure itself is
	// recorded, on the entry, because a script that stopped halfway is part of
	// what happened to the database.
	m.logScript(s, req, res, err)
	return res, err
}

// --- scripts (DDL editor) --------------------------------------------------

// AnalyzeScript reports what a script would do without running it. It is the
// dry run the DDL editor shows next to the editor.
func (m *Manager) AnalyzeScript(sessionID, sql string) (*models.ScriptAnalysis, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sql) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "there is nothing to analyse yet")
	}

	// A driver that cannot be described by SQL keywords describes itself.
	var analysis models.ScriptAnalysis
	if analyzer, ok := s.conn.(drivers.Analyzer); ok {
		analysis = analyzer.AnalyzeScript(sql, s.readOnly)
	} else {
		analysis = sqlutil.Analyze(sql, s.readOnly)
	}
	return &analysis, nil
}

// Explain asks the engine how it would run a statement, without running it.
//
// One statement only: a plan describes one statement, and an engine handed a
// script would either explain the first one silently or answer with several
// plans the window has no place for side by side. Nothing is measured (no
// EXPLAIN ANALYZE anywhere), so this is safe on a read-only session and safe on
// a statement that writes — which is the point: a plan is what you look at
// before deciding whether to run something.
func (m *Manager) Explain(req models.ExplainRequest) (*models.ExplainResult, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.SQL) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "there is nothing to explain yet")
	}

	// The driver is asked first, so a document store says "this engine has no
	// plan" rather than being told about its statement count.
	explainer, ok := s.conn.(drivers.Explainer)
	if !ok {
		return nil, apperr.New(apperr.CodeUnsupported, "this driver cannot explain a statement")
	}

	statements := sqlutil.SplitStatements(req.SQL)
	if len(statements) != 1 {
		if len(statements) == 0 {
			return nil, apperr.New(apperr.CodeInvalidConfig, "there is nothing to explain yet")
		}
		return nil, apperr.New(apperr.CodeInvalidConfig,
			"a plan describes one statement, but this editor holds %d — run the selection instead",
			len(statements))
	}

	ctx, cancel := m.ctx(QueryTimeout(req.TimeoutMS))
	defer cancel()

	return explainer.Explain(ctx, drivers.ExplainRequest{
		Database:  req.Database,
		SQL:       statements[0],
		TimeoutMS: req.TimeoutMS,
	})
}

// --- table designer --------------------------------------------------------

// PlanDesign renders the script that turns the live table into the designer's
// draft. Nothing is executed: this is the SQL preview the designer shows next
// to the fields.
func (m *Manager) PlanDesign(design models.TableDesign) (*models.DesignPlan, error) {
	s, err := m.session(design.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(design.Object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "object name is required")
	}

	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	current, err := s.conn.Structure(ctx, design.Database, design.Schema, design.Object)
	if err != nil {
		return nil, err
	}

	plan, err := sqlbase.PlanAlter(s.conn.Dialect(), current, design)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot design table %s", design.Object)
	}
	return &plan, nil
}

// ApplyDesign plans the draft again and runs the resulting statements one by
// one.
//
// The draft is planned here rather than sent as SQL by the window, so the script
// that runs is produced by the same code that produced the preview the user
// approved. Because DDL cannot be rolled back on every engine, the statements
// run one at a time and the result says exactly how far the script got.
func (m *Manager) ApplyDesign(design models.TableDesign) (*models.DesignResult, error) {
	s, err := m.session(design.SessionID)
	if err != nil {
		return nil, err
	}
	if s.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}

	plan, err := m.PlanDesign(design)
	if err != nil {
		return nil, err
	}
	return m.applyPlan(s, statementPlace{
		database: design.Database,
		schema:   design.Schema,
		object:   design.Object,
		source:   models.ChangeSourceDesign,
	}, plan), nil
}

// PlanCreateDesign renders the script that creates a table that does not exist
// yet. Nothing is executed: the designer shows this next to the fields, the
// same way PlanDesign does for a table that is already there.
func (m *Manager) PlanCreateDesign(design models.TableDesign) (*models.DesignPlan, error) {
	s, err := m.session(design.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(design.Object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "table name is required")
	}

	plan, err := sqlbase.PlanCreate(s.conn.Dialect(), design)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot design table %s", design.Object)
	}
	return &plan, nil
}

// ApplyCreateDesign creates a table from a design, running the statements one by
// one for the same reason ApplyDesign does: the engine may refuse the second
// statement after the first one has already run, and the user has to be told
// exactly how far it got.
func (m *Manager) ApplyCreateDesign(design models.TableDesign) (*models.DesignResult, error) {
	s, err := m.session(design.SessionID)
	if err != nil {
		return nil, err
	}
	if s.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}

	plan, err := m.PlanCreateDesign(design)
	if err != nil {
		return nil, err
	}
	return m.applyPlan(s, statementPlace{
		database: design.Database,
		schema:   design.Schema,
		object:   design.Object,
		source:   models.ChangeSourceCreate,
	}, plan), nil
}

// --- table copy ------------------------------------------------------------

// PlanCopyTable renders the script that duplicates a table into a new one.
// Nothing is executed: this is the preview the explorer's copy window shows
// beside the name being typed.
//
// The structure is read here rather than sent by the window, for the same reason
// the designer reads it: the copy is rendered from the live catalog, so what the
// window shows and what runs cannot disagree about what a table's fields are.
func (m *Manager) PlanCopyTable(req models.CopyTableRequest) (*models.DesignPlan, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "object name is required")
	}
	if strings.TrimSpace(req.Target) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "the copy needs a name")
	}

	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	current, err := s.conn.Structure(ctx, req.Database, req.Schema, req.Object)
	if err != nil {
		return nil, err
	}

	plan, err := sqlbase.PlanCopy(s.conn.Dialect(), current, req.Target, req.WithData)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot copy table %s", req.Object)
	}
	return &plan, nil
}

// CopyTable plans the copy again and runs it statement by statement.
//
// It is deliberately the same shape as ApplyCreateDesign: the window sends what
// it wants, never SQL, and a copy is planned here so the script that runs is
// produced by the code that produced the preview. Copying the rows is part of the
// same script, because a copy that failed between its CREATE and its INSERT is
// exactly the kind of half-done table the result has to be able to describe.
func (m *Manager) CopyTable(req models.CopyTableRequest) (*models.DesignResult, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if s.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}

	plan, err := m.PlanCopyTable(req)
	if err != nil {
		return nil, err
	}
	// The statements are all about the table being created, so that is the object
	// they are logged against; the table being copied is named in the statements
	// themselves.
	return m.applyPlan(s, statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Target,
		source:   models.ChangeSourceCopy,
	}, plan), nil
}

// --- table operations ------------------------------------------------------

// tableOp is one of the two things the explorer's table menu asks for, as this
// layer needs it: the verb it is reported with, and the renderer that writes its
// statement. The two are meant to go together — a message that says "drop"
// about the script that empties a table would be the only place the user could
// tell them apart.
type tableOp struct {
	verb   string
	render func(drivers.Dialect, *models.TableStructure) (models.DesignPlan, error)
}

var (
	opDropTable  = tableOp{verb: "drop", render: sqlbase.PlanDrop}
	opEmptyTable = tableOp{verb: "empty", render: sqlbase.PlanTruncate}
)

// PlanDropTable renders the statement that would remove a table. Nothing is
// executed: this is the preview the explorer shows before it asks.
func (m *Manager) PlanDropTable(req models.TableOpRequest) (*models.DesignPlan, error) {
	return m.planTableOp(req, opDropTable)
}

// DropTable plans the drop again and runs it.
//
// Like every other write, the statement is rendered here from the live catalog
// rather than sent as SQL by the explorer, so the statement that runs is the one
// that was previewed. It is a single statement, so the result's FailedIndex is
// either -1 or 0: there is nothing in between for a drop to stop at.
func (m *Manager) DropTable(req models.TableOpRequest) (*models.DesignResult, error) {
	return m.runTableOp(req, opDropTable)
}

// PlanTruncateTable renders the script that would empty a table. Nothing is
// executed: this is the preview the explorer shows before it asks.
func (m *Manager) PlanTruncateTable(req models.TableOpRequest) (*models.DesignPlan, error) {
	return m.planTableOp(req, opEmptyTable)
}

// TruncateTable plans the emptying again and runs it. On SQLite the script is a
// DELETE FROM rather than a TRUNCATE, which is the engine's doing and not the
// caller's — the plan's warnings say so.
func (m *Manager) TruncateTable(req models.TableOpRequest) (*models.DesignResult, error) {
	return m.runTableOp(req, opEmptyTable)
}

// planTableOp reads the table a one-statement operation is about and renders it.
func (m *Manager) planTableOp(req models.TableOpRequest, op tableOp) (*models.DesignPlan, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Object) == "" {
		return nil, apperr.New(apperr.CodeInvalidConfig, "object name is required")
	}

	ctx, cancel := m.ctx(60 * time.Second)
	defer cancel()
	current, err := s.conn.Structure(ctx, req.Database, req.Schema, req.Object)
	if err != nil {
		return nil, err
	}

	plan, err := op.render(s.conn.Dialect(), current)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot %s table %s", op.verb, req.Object)
	}
	return &plan, nil
}

// runTableOp checks that the write is allowed, plans it and applies it, the way
// ApplyDesign and CopyTable do.
func (m *Manager) runTableOp(req models.TableOpRequest, op tableOp) (*models.DesignResult, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, err
	}
	if s.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}

	plan, err := m.planTableOp(req, op)
	if err != nil {
		return nil, err
	}
	// The statement is about the table it names, which is the table the entry
	// points at — unlike a copy's CREATE, there is no second object involved.
	return m.applyPlan(s, statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   models.ChangeSourceExplorer,
	}, plan), nil
}

// applyPlan runs an already-planned script, one statement at a time, and reports
// how far it got. A plan that is applied is always planned first, so this is the
// only place where a window executes DDL.
//
// Each statement is written to the change log as it runs, with the object the
// plan was for: this is the one path that executes statement by statement, so it
// is the one path that knows the outcome of every line it ran and can record a
// failure against the statement that caused it.
func (m *Manager) applyPlan(s *session, place statementPlace, plan *models.DesignPlan) *models.DesignResult {
	result := &models.DesignResult{Plan: *plan, Executed: []string{}, FailedIndex: -1}
	for i, statement := range plan.Statements {
		ctx, cancel := m.ctx(QueryTimeout(0))
		res, err := s.conn.Execute(ctx, drivers.ExecRequest{
			Database: place.database,
			SQL:      statement,
		})
		cancel()
		// The plan's statements come from the designer's own renderer, so they are
		// SQL whatever the engine calls it; a keyword that is not one still gets
		// the classification rather than being left blank. The count the engine
		// reported is how much data the statement changed — zero for DDL, which
		// has nothing to count.
		var affected int64
		if res != nil {
			affected = res.AffectedRows
		}
		m.logStatement(s, place, statement, statementKind(statement, sqlutil.KindOf(statement)), affected, err)
		if err != nil {
			result.FailedIndex = i
			result.Error = err.Error()
			return result
		}
		result.Executed = append(result.Executed, statement)
		if res != nil {
			result.Messages = append(result.Messages, res.Messages...)
		}
	}
	return result
}

// --- row edits -------------------------------------------------------------

// UpdateCell applies a single-cell edit from the data grid.
func (m *Manager) UpdateCell(req models.CellUpdate) (int64, error) {
	if strings.TrimSpace(req.Column) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "a column name is required")
	}
	return m.UpdateRow(models.RowUpdate{
		SessionID: req.SessionID,
		Database:  req.Database,
		Schema:    req.Schema,
		Object:    req.Object,
		Key:       req.Key,
		Values:    []models.KeyValue{{Column: req.Column, Value: req.Value}},
	})
}

// PlanRowUpdate renders the statement UpdateRow would run, for the row detail
// layer to show before anything is applied.
//
// It checks what the run checks — the session, read-only, the row identity — so
// a request that could not be applied does not get a preview that looks like it
// would. Nothing is executed, and nothing is logged: a statement that was only
// read is not a change.
func (m *Manager) PlanRowUpdate(req models.RowUpdate) (string, error) {
	s, err := m.editable(req.SessionID, req.Key)
	if err != nil {
		return "", err
	}
	return s.conn.PlanRowUpdate(req)
}

// PlanRowDelete renders the statement DeleteRow would run. It is what the grid
// shows before a selection is removed: one statement per row, in the order they
// would be run, which is the only honest way to say how much is about to go.
func (m *Manager) PlanRowDelete(req models.RowDelete) (string, error) {
	s, err := m.editable(req.SessionID, req.Key)
	if err != nil {
		return "", err
	}
	return s.conn.PlanRowDelete(req)
}

// UpdateRow applies an edit of one row made in the row detail layer.
//
// The statement is rendered first and executed second, and it is the rendered
// text — not a second rendering made for the log — that the change log keeps. So
// what the layer previewed, what ran and what the log says are the same string
// by construction. An edit the engine refuses still leaves that line behind,
// with the engine's message on it: the log answers "what was this database asked
// to do", and a statement that was sent and failed was asked for.
func (m *Manager) UpdateRow(req models.RowUpdate) (int64, error) {
	s, err := m.editable(req.SessionID, req.Key)
	if err != nil {
		return 0, err
	}

	statement, err := s.conn.PlanRowUpdate(req)
	if err != nil {
		return 0, err
	}

	ctx, cancel := m.ctx(QueryTimeout(0))
	defer cancel()
	affected, runErr := s.conn.UpdateRow(ctx, req)
	m.logStatement(s, statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   models.ChangeSourceGrid,
	}, statement, "update", affected, runErr)
	if runErr != nil {
		return 0, runErr
	}
	return affected, nil
}

// DeleteRow removes one row selected in the data grid.
func (m *Manager) DeleteRow(req models.RowDelete) (int64, error) {
	s, err := m.editable(req.SessionID, req.Key)
	if err != nil {
		return 0, err
	}

	statement, err := s.conn.PlanRowDelete(req)
	if err != nil {
		return 0, err
	}

	ctx, cancel := m.ctx(QueryTimeout(0))
	defer cancel()
	affected, runErr := s.conn.DeleteRow(ctx, req)
	m.logStatement(s, statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   models.ChangeSourceGrid,
	}, statement, "delete", affected, runErr)
	if runErr != nil {
		return 0, runErr
	}
	return affected, nil
}

// editable is the gate every row edit goes through: a live session on a
// connection that may write, and a row that can be identified again.
//
// A row without a primary key cannot be named in a WHERE clause without
// matching whatever else happens to look like it, so it is refused rather than
// guessed at — that refusal is the same error the drivers raise, so the preview
// and the run agree.
func (m *Manager) editable(sessionID string, key []models.KeyValue) (*session, error) {
	s, err := m.session(sessionID)
	if err != nil {
		return nil, err
	}
	if s.readOnly {
		return nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}
	if len(key) == 0 {
		return nil, apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
	}
	return s, nil
}

// InsertRows appends a batch of rows produced by the data generation window.
//
// What the window generates (placeholders, mock.js expressions, the Chinese
// name and address tables) is presentation and lives in the frontend. What is
// left for the backend is the part that decides what may reach the database:
// only identifiers are interpolated, every value is bound as a parameter, a
// whole JSON number is folded back into an integer (a `float64` bound to an
// `int4` column is read by PostgreSQL as double precision, which it refuses),
// the batch is width-checked and size-capped, and a statement the engine turns
// down comes back as a partial count rather than as an error. Whether such a
// refusal ends the batch or is counted over is the window's own decision and
// arrives in the request (`SkipErrors`); the service only carries it through.
func (m *Manager) InsertRows(req models.RowInsert) (models.RowInsertResult, error) {
	s, inserter, err := m.insertTarget(req)
	if err != nil {
		return models.RowInsertResult{}, err
	}
	if len(req.Rows) == 0 {
		return models.RowInsertResult{}, apperr.New(apperr.CodeInvalidConfig, "there is no row to insert")
	}
	if len(req.Rows) > maxInsertRows {
		return models.RowInsertResult{}, apperr.New(apperr.CodeInvalidConfig,
			"a batch may not carry more than %d rows, got %d", maxInsertRows, len(req.Rows))
	}
	rows, err := normalizeInsertRows(req.Rows, len(req.Columns))
	if err != nil {
		return models.RowInsertResult{}, err
	}
	req.Rows = rows

	// The statement is rendered before it runs and logged with its count after,
	// so the log line for a batch is the shape of what the engine was handed and
	// how many rows went in. It is logged even when part of the batch was refused:
	// the rows that arrived did arrive, and the count beside them says how many.
	statement, err := inserter.PlanRowInsert(req)
	if err != nil {
		return models.RowInsertResult{}, err
	}

	ctx, cancel := m.ctx(QueryTimeout(0))
	defer cancel()
	res, runErr := inserter.InsertRows(ctx, req)
	m.logStatement(s, statementPlace{
		database: req.Database,
		schema:   req.Schema,
		object:   req.Object,
		source:   models.ChangeSourceDataGen,
	}, statement, "insert", res.Inserted, runErr)
	return res, runErr
}

// PlanInsertRows renders the statement a batch of generated rows goes in as, for
// the data generation window to show next to the settings that produced it.
//
// The values are not there yet when the window asks — they are made up row by row
// while the run goes on — so what is rendered is the statement's shape: which
// table, which columns, and that the values arrive as parameters. It is the same
// renderer InsertRows runs through, so the preview is not a second guess at what
// the batch becomes.
func (m *Manager) PlanInsertRows(req models.RowInsert) (string, error) {
	_, inserter, err := m.insertTarget(req)
	if err != nil {
		return "", err
	}
	return inserter.PlanRowInsert(req)
}

// insertTarget is the gate InsertRows and PlanInsertRows share: a live session on
// a connection that may write, an engine that can take generated rows at all, and
// a table with at least one column to fill. A request that could not be applied
// must not get a preview that looks like it would.
func (m *Manager) insertTarget(req models.RowInsert) (*session, drivers.Inserter, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return nil, nil, err
	}
	if s.readOnly {
		return nil, nil, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}
	// Asking the connection rather than the driver's name is what lets a
	// document store answer honestly: it has no Inserter, so there is nothing
	// to offer and the window says so.
	inserter, ok := s.conn.(drivers.Inserter)
	if !ok {
		return nil, nil, apperr.New(apperr.CodeUnsupported,
			"%s cannot write generated rows", s.driver.Info().DisplayName)
	}
	if strings.TrimSpace(req.Object) == "" {
		return nil, nil, apperr.New(apperr.CodeInvalidConfig, "a table name is required")
	}
	if len(req.Columns) == 0 {
		return nil, nil, apperr.New(apperr.CodeInvalidConfig, "there is no column to fill")
	}
	return s, inserter, nil
}

// maxInsertRows caps one batch. The window sends small batches and reports
// progress between them, so this only guards against a caller that would build
// a statement no server would take (MySQL's max_allowed_packet is the tightest
// of the engines here).
const maxInsertRows = 500

// normalizeInsertRows checks every row's width once and folds its numbers into
// the shape the database expects.
func normalizeInsertRows(rows [][]any, width int) ([][]any, error) {
	out := make([][]any, len(rows))
	for i, row := range rows {
		if len(row) != width {
			return nil, apperr.New(apperr.CodeInvalidConfig,
				"row %d has %d value(s) for %d column(s)", i+1, len(row), width)
		}
		values := make([]any, len(row))
		for j, value := range row {
			values[j] = normalizeInsertValue(value)
		}
		out[i] = values
	}
	return out, nil
}

// normalizeInsertValue makes a whole JSON number an integer again.
//
// Everything the window sends over the bridge arrives as float64. Binding one
// to an integer column is what PostgreSQL refuses outright and what MySQL
// silently rounds, so a whole number becomes int64 — but only while the double
// was exact (below 2^53); a fractional one is left alone for the column to
// judge, exactly like a typed value from the data grid.
func normalizeInsertValue(value any) any {
	number, ok := value.(float64)
	if !ok {
		return value
	}
	if number != math.Trunc(number) || math.Abs(number) >= 1<<53 {
		return value
	}
	return int64(number)
}

// QueryTimeout normalises the per-request timeout.
func QueryTimeout(ms int) time.Duration {
	if ms <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(ms) * time.Millisecond
}

// --- UI state --------------------------------------------------------------

// SavedQueries lists the query favourites, sorted by name.
func (m *Manager) SavedQueries() ([]models.SavedQuery, error) {
	list, err := m.storeRef().LoadQueries()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "read saved queries")
	}
	sort.SliceStable(list, func(i, j int) bool {
		left, right := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if left == right {
			return list[i].ID < list[j].ID
		}
		return left < right
	})
	return list, nil
}

// SaveSavedQuery validates and persists a favourite.
//
// An empty ID means "create", an existing one means "update" (the window uses
// that to rename a snippet). Name and SQL are the only required fields; the
// database and driver are hints recorded for display.
func (m *Manager) SaveSavedQuery(query models.SavedQuery) (models.SavedQuery, error) {
	query.Name = strings.TrimSpace(query.Name)
	query.SQL = strings.TrimSpace(query.SQL)
	if query.Name == "" {
		return query, apperr.New(apperr.CodeInvalidConfig, "give the saved query a name")
	}
	if len([]rune(query.Name)) > 120 {
		return query, apperr.New(apperr.CodeInvalidConfig, "the saved query name is too long (120 characters max)")
	}
	if query.SQL == "" {
		return query, apperr.New(apperr.CodeInvalidConfig, "there is nothing to save: the editor is empty")
	}

	now := time.Now().UnixMilli()
	if query.ID == "" {
		query.ID = uuid.NewString()
		query.CreatedAt = now
	}
	if query.CreatedAt == 0 {
		query.CreatedAt = now
	}
	query.UpdatedAt = now

	stored, err := m.storeRef().UpsertQuery(query)
	if err != nil {
		return query, apperr.Wrap(apperr.CodeInternal, err, "save query favourite")
	}
	return stored, nil
}

// DeleteSavedQuery removes a favourite by id. Unknown ids are a no-op so a
// double click cannot fail.
func (m *Manager) DeleteSavedQuery(id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "query id is required")
	}
	if err := m.storeRef().DeleteQuery(id); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "delete saved query")
	}
	return nil
}

// LoadState returns the persisted UI preferences.
func (m *Manager) LoadState() (map[string]any, error) {
	state, err := m.storeRef().LoadState()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "read application state")
	}
	return state, nil
}

// SaveState persists the UI preferences.
func (m *Manager) SaveState(state map[string]any) error {
	if state == nil {
		state = map[string]any{}
	}
	if err := m.storeRef().SaveState(state); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "save application state")
	}
	return nil
}

// ErrSessionClosed is returned when a query races with a disconnect.
var ErrSessionClosed = errors.New("session closed")
