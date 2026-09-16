package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
	"dbmanager/internal/service"
)

// Version is overridden at build time:
//
//	wails build -ldflags "-X main.Version=1.2.3"
var Version = "0.1.0-dev"

// App is the object bound to the frontend. Every exported method becomes a
// callable function on window.go.main.App in the webview.
type App struct {
	ctx     context.Context
	manager *service.Manager
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
// query cancellation work.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetContext(ctx)
}

// shutdown releases every database pool.
func (a *App) shutdown(context.Context) {
	a.manager.Shutdown()
}

// --- application -----------------------------------------------------------

// AppInfo returns build metadata for the welcome screen.
func (a *App) AppInfo() models.AppInfo {
	return models.AppInfo{
		Name:       "DB Manager",
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

// --- metadata --------------------------------------------------------------

// ListDatabases returns the catalogs of a session.
func (a *App) ListDatabases(sessionID string) ([]string, error) {
	return a.manager.Databases(sessionID)
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

// FormatError normalises an error for display; kept for completeness so the
// frontend has a single place to sanitise backend messages.
func (a *App) FormatError(message string) string {
	return apperr.Sanitize(message)
}
