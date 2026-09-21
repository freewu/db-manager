package sqlbase

import (
	"context"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// This is the shared half of drivers.DatabaseCreator: the two methods an engine
// inherits from the spec hooks. What the choices *are* stays with the engine —
// SHOW CHARACTER SET for the MySQL family, pg_database for PostgreSQL — and the
// statement is rendered by the engine too, so nothing here knows either syntax.

// DatabaseOptions implements drivers.DatabaseCreator.
func (c *Conn) DatabaseOptions(ctx context.Context) (*models.DatabaseOptions, error) {
	if c.spec.DatabaseOptions == nil {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s does not create databases", c.spec.Info.DisplayName)
	}
	db, err := c.DB(ctx, "")
	if err != nil {
		return nil, err
	}
	options, err := c.spec.DatabaseOptions(ctx, db)
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = &models.DatabaseOptions{}
	}
	if options.Charsets == nil {
		options.Charsets = []models.DatabaseCharset{}
	}
	return options, nil
}

// CreateDatabase implements drivers.DatabaseCreator.
func (c *Conn) CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	if c.spec.CreateDatabase == nil {
		return models.DatabasePlan{}, apperr.New(apperr.CodeUnsupported,
			"%s does not create databases", c.spec.Info.DisplayName)
	}
	name, err := sqlutil.ValidateDatabaseName(req.Name)
	if err != nil {
		return models.DatabasePlan{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot create the database")
	}
	req.Name = name
	plan, err := c.spec.CreateDatabase(req)
	if err != nil {
		return models.DatabasePlan{}, apperr.Wrap(apperr.CodeInvalidConfig, err, "cannot create database %s", name)
	}
	if plan.Warnings == nil {
		plan.Warnings = []string{}
	}
	return plan, nil
}
