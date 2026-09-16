// Package service is the application layer sitting between the Wails bound
// methods and the database drivers. It owns the session registry, the
// connection profile store and the error/redaction policy.
package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/planned"
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
func (m *Manager) ConfigDir() string { return m.store.Dir() }

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
	list, err := m.store.Load()
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

	stored, err := m.store.Upsert(cfg)
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
	if err := m.store.Delete(id); err != nil {
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
	cfg, found, err := m.store.Find(req.ConnectionID)
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
	return s.conn.Execute(ctx, drivers.ExecRequest{
		Database:  req.Database,
		SQL:       req.SQL,
		MaxRows:   req.MaxRows,
		TimeoutMS: req.TimeoutMS,
		ReadOnly:  readOnly,
	})
}

// --- row edits -------------------------------------------------------------

// UpdateCell applies a single-cell edit from the data grid.
func (m *Manager) UpdateCell(req models.CellUpdate) (int64, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return 0, err
	}
	if s.readOnly {
		return 0, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}
	if strings.TrimSpace(req.Column) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "a column name is required")
	}
	if len(req.Key) == 0 {
		return 0, apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
	}

	ctx, cancel := m.ctx(QueryTimeout(0))
	defer cancel()
	return s.conn.UpdateCell(ctx, req)
}

// DeleteRow removes one row selected in the data grid.
func (m *Manager) DeleteRow(req models.RowDelete) (int64, error) {
	s, err := m.session(req.SessionID)
	if err != nil {
		return 0, err
	}
	if s.readOnly {
		return 0, apperr.New(apperr.CodeReadOnly, "this connection is read-only")
	}
	if len(req.Key) == 0 {
		return 0, apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
	}

	ctx, cancel := m.ctx(QueryTimeout(0))
	defer cancel()
	return s.conn.DeleteRow(ctx, req)
}

// QueryTimeout normalises the per-request timeout.
func QueryTimeout(ms int) time.Duration {
	if ms <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(ms) * time.Millisecond
}

// --- UI state --------------------------------------------------------------

// LoadState returns the persisted UI preferences.
func (m *Manager) LoadState() (map[string]any, error) {
	state, err := m.store.LoadState()
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
	if err := m.store.SaveState(state); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "save application state")
	}
	return nil
}

// ErrSessionClosed is returned when a query races with a disconnect.
var ErrSessionClosed = errors.New("session closed")
