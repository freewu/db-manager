package sqlbase

import (
	"context"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// MaxExplainRows caps how many rows of a plan are read back.
//
// PostgreSQL answers a multi-line text plan with one row per line and a deeply
// nested query can run to hundreds of them, so the cap is generous; it exists
// to keep a runaway plan from filling the window with rows nobody asked for.
const MaxExplainRows = 2000

// explainNote is the sentence every plan carries.
//
// It is the same for every engine on purpose: none of the statements behind it
// run the query (`EXPLAIN QUERY PLAN` and MySQL's `EXPLAIN` only plan; only
// EXPLAIN ANALYZE executes, and that is never emitted). Saying so next to the
// plan is what stops "rows" from being read as "rows this query touched".
const explainNote = "Estimated plan — the statement was not run. Row counts and costs are the engine's guesses."

// Explain implements drivers.Explainer for engines that can be asked about a
// statement in one line of SQL. Which line is the engine's business and lives
// in Spec.ExplainSQL; a spec without it does not get this capability.
func (c *Conn) Explain(ctx context.Context, req drivers.ExplainRequest) (*models.ExplainResult, error) {
	if c.spec.ExplainSQL == nil {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s cannot explain a statement", c.spec.Info.DisplayName)
	}

	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return nil, err
	}

	ctx, cancel := withTimeout(ctx, req.TimeoutMS)
	defer cancel()

	statement := c.spec.ExplainSQL(req.SQL)
	started := time.Now()
	res, err := scanQuery(ctx, db, statement, nil, MaxExplainRows)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "explain the statement")
	}

	return &models.ExplainResult{
		Statement:  statement,
		SQL:        req.SQL,
		Columns:    res.Columns,
		Rows:       res.Rows,
		Notes:      []string{explainNote},
		DurationMS: time.Since(started).Milliseconds(),
		Truncated:  res.Truncated,
	}, nil
}
