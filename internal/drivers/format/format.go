// Package format holds the display helpers every status page needs.
//
// It lives outside sqlbase because formatting has nothing to do with SQL: the
// MongoDB driver renders "1.5 GiB" and "3d 4h" exactly like the SQL engines do,
// and a DBA reading two status pages should not have to notice which driver
// produced them.
package format

import (
	"fmt"
	"strconv"
	"strings"

	"dbmanager/internal/models"
)

// Metric builds one labelled value. Callers overwrite State when a number is
// worth colouring.
func Metric(label, value, hint string) models.OverviewMetric {
	return models.OverviewMetric{Label: label, Value: value, Hint: hint}
}

// Dash renders a value that could not be read as a dash, so a number the server
// never answered is never mistaken for a zero.
func Dash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

// FormatBytes renders a byte count in binary units, e.g. "1.5 GiB". Negative
// counts (used for "not applicable") render as a dash.
func FormatBytes(n int64) string {
	if n < 0 {
		return "—"
	}
	const step = 1024
	if n < step {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := float64(n)
	for i, unit := range units {
		value /= step
		if value < step || i == len(units)-1 {
			if value >= 100 {
				return fmt.Sprintf("%.0f %s", value, unit)
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return "—"
}

// FormatCount renders an integer with thousands separators ("1,204,331"),
// because a nine-digit counter is unreadable without them.
func FormatCount(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := strconv.FormatInt(n, 10)
	out := make([]byte, 0, len(digits)+len(digits)/3)
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, digits[i])
	}
	return sign + string(out)
}

// FormatDuration renders a span of seconds the way uptime is read: "3d 4h",
// "5h 12m", "42s".
func FormatDuration(seconds float64) string {
	if seconds < 0 {
		return "—"
	}
	total := int64(seconds + 0.5)
	switch {
	case total >= 24*3600:
		return fmt.Sprintf("%dd %dh", total/(24*3600), (total%(24*3600))/3600)
	case total >= 3600:
		return fmt.Sprintf("%dh %dm", total/3600, (total%3600)/60)
	case total >= 60:
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	default:
		return fmt.Sprintf("%ds", total)
	}
}

// FormatPercent renders part/total as a percentage. A zero total has no
// meaningful percentage, so it renders as a dash rather than "0%" or "NaN".
func FormatPercent(part, total float64) string {
	if total <= 0 {
		return "—"
	}
	ratio := part / total * 100
	if ratio >= 99.995 {
		return "100%"
	}
	return fmt.Sprintf("%.2f%%", ratio)
}
