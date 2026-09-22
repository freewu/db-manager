// The service side of the saved query files: validation, the connection check,
// and the error codes the tree shows when something is wrong.
//
// The storage rules live in internal/config/queryfiles.go; what happens here is
// everything that needs to know about connections and users. There is
// deliberately no SQL involved: the bytes of a script are never parsed, only
// carried, so a query file can hold whatever the editor was given (including a
// mongosh snippet on a document database).
package service

import (
	"os"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// maxQueryNameRunes mirrors the cap on query favourites. It is a UI limit as
// much as a filesystem one: a name has to fit in the tree without being cut off.
const maxQueryNameRunes = 120

// ListQueryFiles returns the scripts saved for one connection and database.
//
// An empty database name is refused rather than answered with an empty list: the
// path is per database, so "all databases" does not name a folder, and answering
// would hide a bug in the caller.
func (m *Manager) ListQueryFiles(connectionID, database string) ([]models.QueryFile, error) {
	if err := m.checkQueryScope(connectionID, database); err != nil {
		return nil, err
	}
	list, err := m.storeRef().ListQueryFiles(connectionID, database)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "read saved queries")
	}
	return list, nil
}

// ReadQueryFile returns one script.
func (m *Manager) ReadQueryFile(connectionID, database, name string) (models.QueryFile, error) {
	if err := m.checkQueryScope(connectionID, database); err != nil {
		return models.QueryFile{}, err
	}
	if strings.TrimSpace(name) == "" {
		return models.QueryFile{}, apperr.New(apperr.CodeInvalidConfig, "query name is required")
	}
	file, err := m.storeRef().ReadQueryFile(connectionID, database, name)
	if err != nil {
		return models.QueryFile{}, queryFileError(err, name)
	}
	return file, nil
}

// SaveQueryFile writes one script, creating it when it does not exist yet.
//
// An empty script is allowed on purpose: a query file is a place to write, and
// "new query" creates one before anything has been typed into it. Saving writes
// the name that was asked for and nothing else — renaming is RenameQueryFile, so
// a save can never move a script the user did not point at.
func (m *Manager) SaveQueryFile(save models.QueryFileSave) (models.QueryFile, error) {
	if err := m.checkQueryScope(save.ConnectionID, save.Database); err != nil {
		return models.QueryFile{}, err
	}
	name, err := m.queryName(save.Name)
	if err != nil {
		return models.QueryFile{}, err
	}
	save.Name = name

	file, err := m.storeRef().SaveQueryFile(save)
	if err != nil {
		return models.QueryFile{}, queryFileError(err, name)
	}
	// The write went through, so what is on disk is exactly the script that was
	// sent; the caller gets it back without reading the file a second time.
	file.SQL = save.SQL
	return file, nil
}

// RenameQueryFile moves one script to another name without touching its
// contents, and returns the file at its new name.
func (m *Manager) RenameQueryFile(rename models.QueryFileRename) (models.QueryFile, error) {
	if err := m.checkQueryScope(rename.ConnectionID, rename.Database); err != nil {
		return models.QueryFile{}, err
	}
	from, err := m.queryName(rename.From)
	if err != nil {
		return models.QueryFile{}, err
	}
	to, err := m.queryName(rename.To)
	if err != nil {
		return models.QueryFile{}, err
	}
	if from == to {
		// Nothing to do, and saying so is better than a message about a write
		// that never happened.
		return m.ReadQueryFile(rename.ConnectionID, rename.Database, from)
	}
	// The name has to be free: the alternative is replacing (and so deleting) a
	// script the user never opened.
	if taken, err := m.queryExists(rename.ConnectionID, rename.Database, to); err != nil {
		return models.QueryFile{}, err
	} else if taken && !strings.EqualFold(from, to) {
		return models.QueryFile{}, apperr.New(apperr.CodeInvalidConfig,
			"a query called %q already exists in this database", to)
	}

	file, err := m.storeRef().RenameQueryFile(models.QueryFileRename{
		ConnectionID: rename.ConnectionID,
		Database:     rename.Database,
		From:         from,
		To:           to,
	})
	if err != nil {
		return models.QueryFile{}, queryFileError(err, from)
	}
	return file, nil
}

// CreateQueryFile creates a script holding the given text and returns it, so a
// new query window opens onto a file that really exists (and shows up in the tree
// immediately).
//
// The text is what the caller already has: naming an unsaved script and writing
// it are one step, since a file created empty and filled in by a second call can
// be left empty by a failure in between.
//
// The name has to be free: overwriting an existing script is the exact data loss
// this check is here to prevent.
func (m *Manager) CreateQueryFile(connectionID, database, name, sql string) (models.QueryFile, error) {
	if err := m.checkQueryScope(connectionID, database); err != nil {
		return models.QueryFile{}, err
	}
	clean, err := m.queryName(name)
	if err != nil {
		return models.QueryFile{}, err
	}
	if taken, err := m.queryExists(connectionID, database, clean); err != nil {
		return models.QueryFile{}, err
	} else if taken {
		return models.QueryFile{}, apperr.New(apperr.CodeInvalidConfig,
			"a query called %q already exists in this database", clean)
	}
	file, err := m.storeRef().SaveQueryFile(models.QueryFileSave{
		ConnectionID: connectionID,
		Database:     database,
		Name:         clean,
		SQL:          sql,
	})
	if err != nil {
		return models.QueryFile{}, queryFileError(err, clean)
	}
	// The write went through, so what is on disk is exactly the text that was
	// sent; the caller gets it back without reading the file a second time.
	file.SQL = sql
	return file, nil
}

// DeleteQueryFile removes one script.
func (m *Manager) DeleteQueryFile(connectionID, database, name string) error {
	if err := m.checkQueryScope(connectionID, database); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "query name is required")
	}
	if err := m.storeRef().DeleteQueryFile(connectionID, database, name); err != nil {
		return queryFileError(err, name)
	}
	return nil
}

// checkQueryScope rejects a request that cannot name a folder: an empty
// connection or database, or a connection that is not in the profile file any
// more (the tree is drawn from profiles, so a folder for an unknown id would be
// invisible and never cleaned up).
func (m *Manager) checkQueryScope(connectionID, database string) error {
	if strings.TrimSpace(connectionID) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "a saved query belongs to a connection: this window has none")
	}
	if strings.TrimSpace(database) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "a saved query belongs to a database: open it from a database node")
	}
	if _, found, err := m.storeRef().Find(connectionID); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "read connection profile")
	} else if !found {
		return apperr.New(apperr.CodeNotFound, "connection profile not found")
	}
	return nil
}

// queryName trims and checks one query name.
func (m *Manager) queryName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "give the query a name")
	}
	if len([]rune(clean)) > maxQueryNameRunes {
		return "", apperr.New(apperr.CodeInvalidConfig,
			"the query name is too long (%d characters max)", maxQueryNameRunes)
	}
	// A name that is nothing but dots would be a path, not a file name. The
	// escape step turns it into a safe segment, but such a name is a mistake
	// worth naming.
	if strings.Trim(clean, ".") == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "a query name cannot be only dots")
	}
	return clean, nil
}

// queryExists reports whether a script of that name (ignoring case, which is how
// the filesystems this runs on treat it) is already saved.
func (m *Manager) queryExists(connectionID, database, name string) (bool, error) {
	list, err := m.storeRef().ListQueryFiles(connectionID, database)
	if err != nil {
		return false, apperr.Wrap(apperr.CodeInternal, err, "read saved queries")
	}
	for _, file := range list {
		if strings.EqualFold(file.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

// queryFileError maps a storage error onto the code the UI shows. A missing file
// is not an internal failure: another window may have renamed or deleted it, and
// saying so is more useful than "internal error".
func queryFileError(err error, name string) error {
	if os.IsNotExist(err) {
		return apperr.New(apperr.CodeNotFound, "the query %q is no longer there", name)
	}
	return apperr.Wrap(apperr.CodeInternal, err, "query file %s", name)
}
