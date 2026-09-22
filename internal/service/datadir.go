// Data-directory handling: where profiles, favourites and the password key are
// kept, and how the settings page moves them somewhere else.
package service

import (
	"dbmanager/internal/apperr"
	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// DataDir reports the directory the running app keeps its data in, what is in
// it, and where the default directory is.
func (m *Manager) DataDir() (models.DataDirInfo, error) {
	info, err := config.DescribeDataDir(m.storeRef().Dir())
	if err != nil {
		return models.DataDirInfo{}, apperr.Wrap(apperr.CodeInternal, err, "inspect the data directory")
	}
	return info, nil
}

// MoveDataDir moves the app's data into target and switches the running app over
// to it. An empty target means the default directory, which is how the settings
// page offers "use the default again".
//
// What moves is every file the store owns — the profiles, the query favourites,
// the explorer arrangement, the UI state and the key the saved passwords are
// sealed with — because a move that left the key behind would strand every
// password in the profiles it just copied.
//
// Live sessions are deliberately untouched: they are connections to database
// servers, and where the JSON files sit has nothing to do with them. What the
// move changes is where the *next* write lands, so the store is rebuilt against
// the new directory before this returns.
func (m *Manager) MoveDataDir(target string) (models.DataDirMoveResult, error) {
	// Held for the whole move: a profile save landing in the old directory while
	// the files are being copied would be lost by the delete that follows.
	m.mu.Lock()
	defer m.mu.Unlock()

	result, err := config.MoveData(target)
	if err != nil {
		return models.DataDirMoveResult{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "move the data directory")
	}

	store, err := config.New()
	if err != nil {
		// The files are in the new directory and the pointer names it, so a
		// restart finds them; keeping the old store would only make the running
		// app write into the directory it just emptied. Say that plainly instead
		// of reporting a move that did happen as a failure.
		return result, apperr.New(apperr.CodeInternal,
			"data moved to %s, but the running app could not switch over (%v) — restart DB Manager",
			result.Info.Path, err)
	}
	m.store = store
	return result, nil
}
