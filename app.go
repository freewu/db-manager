package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
	"dbmanager/internal/service"
)

// appName is what the window title, the notification-area tooltip and the
// project-info pane call this program.
const appName = "DB Manager"

// Version is overridden at build time:
//
//	wails build -ldflags "-X main.Version=1.2.3"
var Version = "0.1.0-dev"

// App is the object bound to the frontend. Every exported method becomes a
// callable function on window.go.main.App in the webview.
type App struct {
	ctx     context.Context
	manager *service.Manager

	// tray is the notification-area icon; nil until startup, and a no-op stub on
	// platforms that have no notification area.
	tray *tray

	// quitting is what the tray's Quit sets, so the close hook can tell "the user
	// closed the window" (hide it, stay running) from "the user is done" (go
	// down).
	quitting atomic.Bool
}

// NewApp wires the application layer.
func NewApp() (*App, error) {
	manager, err := service.New()
	if err != nil {
		return nil, err
	}
	return &App{manager: manager}, nil
}

// startup stores the Wails context so runtime helpers (dialogs, events) and
// query cancellation work, and puts the app in the notification area.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetContext(ctx)
	a.tray = newTray(a.trayActions())
}

// trayActions is what the notification-area menu can ask of the app.
func (a *App) trayActions() trayActions {
	return trayActions{
		showWindow: a.showWindow,
		openRepo:   func() { a.openURL(repoURL) },
		openIssue:  func() { a.openURL(issuesURL) },
		quit:       a.quit,
	}
}

// shutdown releases every database pool and takes the tray icon down.
func (a *App) shutdown(context.Context) {
	if a.tray != nil {
		a.tray.close()
	}
	a.manager.Shutdown()
}

// --- window and tray -------------------------------------------------------

// showWindow brings the main window back to the front — the tray's "Show window",
// and a click on the icon itself. It may be hidden (the close button put it away)
// or minimised (the taskbar button did), and both have to end with the window in
// front of the user, so it is un-minimised before it is shown.
func (a *App) showWindow() {
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
}

// openURL hands a link to the user's browser instead of navigating the webview
// away from the app.
func (a *App) openURL(url string) {
	wruntime.BrowserOpenURL(a.ctx, url)
}

// quit leaves through the front door: the flag stops beforeClose from putting
// the window away again, the icon goes back out of the notification area, and
// Wails unwinds — running OnShutdown, so the connection pools are closed.
func (a *App) quit() {
	a.quitting.Store(true)
	if a.tray != nil {
		a.tray.close()
	}
	wruntime.Quit(a.ctx)
}

// beforeClose is Wails' close hook. While the tray icon is up, closing the
// window means "put it away", not "stop": the icon is the way back, and the
// menu's Quit is the way out — which is what a desktop app with a tray is
// expected to do.
//
// The check on the icon is what keeps that a promise we can keep: if the icon
// never made it into the notification area there would be no way back to the
// window, so the close goes through and the app exits as it did before the tray
// existed.
func (a *App) beforeClose(context.Context) bool {
	if a.quitting.Load() || a.tray == nil || !a.tray.running() {
		return false
	}
	wruntime.WindowHide(a.ctx)
	a.tray.noticeHidden()
	return true
}

// --- application -----------------------------------------------------------

// AppInfo returns build metadata for the welcome screen.
func (a *App) AppInfo() models.AppInfo {
	return models.AppInfo{
		Name:       appName,
		Version:    Version,
		GoVersion:  runtime.Version(),
		ConfigPath: a.manager.ConfigDir(),
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// ListDrivers returns the implemented drivers plus the roadmap entries.
func (a *App) ListDrivers() []models.DriverInfo { return a.manager.Drivers() }

// LoadState returns persisted UI preferences.
func (a *App) LoadState() (map[string]any, error) { return a.manager.LoadState() }

// SaveState persists UI preferences.
func (a *App) SaveState(state map[string]any) error { return a.manager.SaveState(state) }

// --- connection profiles ---------------------------------------------------

// ListConnections returns every saved profile with passwords removed.
func (a *App) ListConnections() ([]models.ConnectionConfig, error) {
	return a.manager.Connections()
}

// SaveConnection creates or updates a profile.
func (a *App) SaveConnection(cfg models.ConnectionConfig) (models.ConnectionConfig, error) {
	return a.manager.SaveConnection(cfg)
}

// DeleteConnection removes a profile.
func (a *App) DeleteConnection(id string) error { return a.manager.DeleteConnection(id) }

// ListConnectionLayout returns the explorer arrangement: the groups, and every
// profile with its group and position.
func (a *App) ListConnectionLayout() (models.ConnectionLayout, error) {
	return a.manager.ConnectionLayout()
}

// SaveConnectionLayout stores the arrangement the explorer was left in.
func (a *App) SaveConnectionLayout(layout models.ConnectionLayout) (models.ConnectionLayout, error) {
	return a.manager.SaveConnectionLayout(layout)
}

// SaveConnectionGroup creates or renames a connection group.
func (a *App) SaveConnectionGroup(group models.ConnectionGroup) (models.ConnectionGroup, error) {
	return a.manager.SaveConnectionGroup(group)
}

// DeleteConnectionGroup removes a group; the connections in it move back to the
// top level.
func (a *App) DeleteConnectionGroup(id string) error {
	return a.manager.DeleteConnectionGroup(id)
}

// TestConnection validates a profile without opening a session.
func (a *App) TestConnection(cfg models.ConnectionConfig) models.TestResult {
	return a.manager.TestConnection(cfg)
}

// OpenConnection opens (or reuses) a session.
func (a *App) OpenConnection(req models.OpenRequest) (models.SessionInfo, error) {
	return a.manager.Open(req)
}

// CloseConnection tears a session down.
func (a *App) CloseConnection(sessionID string) error { return a.manager.Close(sessionID) }

// ListSessions returns the live sessions.
func (a *App) ListSessions() []models.SessionInfo { return a.manager.Sessions() }

// --- query favourites ------------------------------------------------------

// ListSavedQueries returns every saved SQL snippet.
func (a *App) ListSavedQueries() ([]models.SavedQuery, error) {
	return a.manager.SavedQueries()
}

// SaveSavedQuery creates or renames a saved SQL snippet.
func (a *App) SaveSavedQuery(query models.SavedQuery) (models.SavedQuery, error) {
	return a.manager.SaveSavedQuery(query)
}

// DeleteSavedQuery removes a saved SQL snippet.
func (a *App) DeleteSavedQuery(id string) error { return a.manager.DeleteSavedQuery(id) }

// --- metadata --------------------------------------------------------------

// ListDatabases returns the catalogs of a session.
func (a *App) ListDatabases(sessionID string) ([]string, error) {
	return a.manager.Databases(sessionID)
}

// DatabaseOptions returns what a session's server accepts for a new database
// (character sets and collations, or encodings and locales).
func (a *App) DatabaseOptions(sessionID string) (*models.DatabaseOptions, error) {
	return a.manager.DatabaseOptions(sessionID)
}

// PlanCreateDatabase returns the statement that would create a database,
// without running it.
func (a *App) PlanCreateDatabase(sessionID string, req models.CreateDatabaseRequest) (*models.DatabasePlan, error) {
	return a.manager.PlanCreateDatabase(sessionID, req)
}

// ListSchemas returns the schemas of a database.
func (a *App) ListSchemas(sessionID, database string) ([]string, error) {
	return a.manager.Schemas(sessionID, database)
}

// ListObjects returns the tables and views of a namespace.
func (a *App) ListObjects(sessionID, database, schema string) ([]models.ObjectInfo, error) {
	return a.manager.Objects(sessionID, database, schema)
}

// GetStructure returns the full description of an object, including DDL.
func (a *App) GetStructure(sessionID, database, schema, object string) (*models.TableStructure, error) {
	return a.manager.Structure(sessionID, database, schema, object)
}

// ListIndexes returns every index of a namespace.
func (a *App) ListIndexes(sessionID, database, schema string) ([]models.IndexEntry, error) {
	return a.manager.Indexes(sessionID, database, schema)
}

// --- ER diagram ------------------------------------------------------------

// GetSchemaGraph returns the objects, columns and foreign keys of a namespace.
func (a *App) GetSchemaGraph(sessionID, database, schema string) (*models.SchemaGraph, error) {
	return a.manager.Graph(sessionID, database, schema)
}

// --- runtime overview ------------------------------------------------------

// GetServerOverview reports the live state of one connection (uptime,
// connections, cache hit rates, running queries). The payload is engine
// specific: see models.ServerOverview.
func (a *App) GetServerOverview(sessionID string) (*models.ServerOverview, error) {
	return a.manager.Overview(sessionID)
}

// --- scripts (DDL editor) --------------------------------------------------

// AnalyzeSQL inspects a script without running it (statement kinds + warnings).
func (a *App) AnalyzeSQL(sessionID, sql string) (*models.ScriptAnalysis, error) {
	return a.manager.AnalyzeScript(sessionID, sql)
}

// --- table designer --------------------------------------------------------

// PlanTableDesign returns the script that would bring a table in line with a
// design, without running it.
func (a *App) PlanTableDesign(design models.TableDesign) (*models.DesignPlan, error) {
	return a.manager.PlanDesign(design)
}

// ApplyTableDesign applies a design and reports how far it got.
func (a *App) ApplyTableDesign(design models.TableDesign) (*models.DesignResult, error) {
	return a.manager.ApplyDesign(design)
}

// PlanCreateTable returns the script that would create a table from a design,
// without running it.
func (a *App) PlanCreateTable(design models.TableDesign) (*models.DesignPlan, error) {
	return a.manager.PlanCreateDesign(design)
}

// ApplyCreateTable creates a table from a design and reports how far it got.
func (a *App) ApplyCreateTable(design models.TableDesign) (*models.DesignResult, error) {
	return a.manager.ApplyCreateDesign(design)
}

// --- data ------------------------------------------------------------------

// FetchRows returns one page of rows for the data grid.
func (a *App) FetchRows(req models.FetchRequest) (*models.FetchResult, error) {
	return a.manager.Fetch(req)
}

// ExecuteSQL runs a script and returns the last result set.
func (a *App) ExecuteSQL(req models.ExecRequest) (*models.QueryResult, error) {
	return a.manager.Execute(req)
}

// UpdateCell applies an inline edit from the data grid.
func (a *App) UpdateCell(req models.CellUpdate) (int64, error) {
	return a.manager.UpdateCell(req)
}

// DeleteRow removes one row from the data grid.
func (a *App) DeleteRow(req models.RowDelete) (int64, error) {
	return a.manager.DeleteRow(req)
}

// --- file helpers ----------------------------------------------------------

// SaveTextFile prompts for a location and writes text content there. The
// frontend serialises the export (CSV/JSON/SQL) so no data has to travel back
// through the bridge.
func (a *App) SaveTextFile(req models.SaveFileRequest) (string, error) {
	if strings.TrimSpace(req.DefaultFilename) == "" {
		req.DefaultFilename = "export.txt"
	}

	filters := make([]wruntime.FileFilter, 0, len(req.Filters))
	for _, f := range req.Filters {
		filters = append(filters, wruntime.FileFilter{DisplayName: f.DisplayName, Pattern: f.Pattern})
	}
	if len(filters) == 0 {
		filters = []wruntime.FileFilter{{DisplayName: "All files", Pattern: "*.*"}}
	}

	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: req.DefaultFilename,
		Title:           "Export",
		Filters:         filters,
	})
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "open save dialog")
	}
	if path == "" {
		// User cancelled: not an error.
		return "", nil
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "write %s", filepath.Base(path))
	}
	return path, nil
}

// PickFile opens a native file chooser (used for SQLite files and TLS
// material).
func (a *App) PickFile(title string, patterns []string) (string, error) {
	filters := make([]wruntime.FileFilter, 0, len(patterns))
	for _, p := range patterns {
		filters = append(filters, wruntime.FileFilter{DisplayName: p, Pattern: p})
	}
	if len(filters) == 0 {
		filters = []wruntime.FileFilter{{DisplayName: "All files", Pattern: "*.*"}}
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "open file dialog")
	}
	return path, nil
}

// RevealInExplorer opens the config directory in the OS file manager.
func (a *App) RevealInExplorer(path string) error {
	if path == "" {
		path = a.manager.ConfigDir()
	}
	return revealPath(path)
}

// --- settings --------------------------------------------------------------

// GetDataDir reports where the app keeps its data (profiles, query favourites,
// UI state and the key saved passwords are sealed with), what is in it, and
// where the default directory is.
func (a *App) GetDataDir() (models.DataDirInfo, error) {
	return a.manager.DataDir()
}

// PickDataDirectory opens a native folder chooser for the new data directory.
// An empty string means the user cancelled — the caller decides what to say,
// because nothing was asked of the backend.
func (a *App) PickDataDirectory(title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		title = "Choose where DB Manager keeps its data"
	}
	// DefaultDirectory is where the chooser opens: the directory in use, so the
	// dialog starts from something the user recognises.
	path, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: a.manager.ConfigDir(),
	})
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "open folder dialog")
	}
	return path, nil
}

// MoveDataDirectory moves the app's data into dir and switches the running app
// over to it. An empty dir means "use the default directory again".
func (a *App) MoveDataDirectory(dir string) (models.DataDirMoveResult, error) {
	return a.manager.MoveDataDir(dir)
}

// FormatError normalises an error for display; kept for completeness so the
// frontend has a single place to sanitise backend messages.
func (a *App) FormatError(message string) string {
	return apperr.Sanitize(message)
}
