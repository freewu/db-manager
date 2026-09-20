package sqlbase

import (
	"context"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/format"
	"dbmanager/internal/models"
)

// Overview implements drivers.Overviewer for every database/sql engine.
//
// The session-level fields (id, name, version, connected-at) belong to the
// service, which knows the session; a driver only fills in what it had to ask
// the server for. See Spec.Overview.
func (c *Conn) Overview(ctx context.Context) (*models.ServerOverview, error) {
	if c.spec.Overview == nil {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s does not report runtime state", c.spec.Info.DisplayName)
	}
	db, err := c.DB(ctx, "")
	if err != nil {
		return nil, err
	}
	overview, err := c.spec.Overview(ctx, db, c.cfg)
	if err != nil {
		return nil, err
	}
	if overview == nil {
		return nil, apperr.New(apperr.CodeUnsupported,
			"%s does not report runtime state", c.spec.Info.DisplayName)
	}
	overview.Supported = true
	if overview.Warnings == nil {
		overview.Warnings = []string{}
	}
	return overview, nil
}

// --- shared formatting for the status pages ---------------------------------
//
// The helpers themselves live in internal/drivers/format, which the MongoDB
// driver uses too: "1.5 GiB" must look the same whichever engine produced it.
// These wrappers exist so the SQL drivers keep their familiar call sites
// (sqlbase.Metric, sqlbase.FormatBytes, ...).

// Metric builds one labelled value. Callers overwrite State when a number is
// worth colouring.
func Metric(label, value, hint string) models.OverviewMetric {
	return format.Metric(label, value, hint)
}

// Dash renders a value that could not be read as a dash, so a number the server
// never answered is never mistaken for a zero.
func Dash(value string) string { return format.Dash(value) }

// FormatBytes renders a byte count in binary units, e.g. "1.5 GiB".
func FormatBytes(n int64) string { return format.FormatBytes(n) }

// FormatCount renders an integer with thousands separators.
func FormatCount(n int64) string { return format.FormatCount(n) }

// FormatDuration renders a span of seconds the way uptime is read.
func FormatDuration(seconds float64) string { return format.FormatDuration(seconds) }

// FormatPercent renders part/total as a percentage.
func FormatPercent(part, total float64) string { return format.FormatPercent(part, total) }
