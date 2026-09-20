package sqlbase

import "testing"

// The status pages show these strings to users, so the units and the rounding
// have to be right: a "1.5 GiB" that is really 1.46 GiB is fine, a "100%" cache
// hit rate that hides a thousand disk reads is not.

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{-1, "—"},
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
		{200 * 1024 * 1024, "200 MiB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.0 PiB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
	}
	for _, c := range cases {
		if got := FormatCount(c.in); got != c.want {
			t.Errorf("FormatCount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{-1, "—"},
		{0, "0s"},
		{42, "42s"},
		{90, "1m 30s"},
		{3600, "1h 0m"},
		{3*3600 + 4*60, "3h 4m"},
		{3*86400 + 4*3600, "3d 4h"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.in); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	cases := []struct {
		part, total float64
		want        string
	}{
		{0, 0, "—"},
		{1, 0, "—"},
		{50, 100, "50.00%"},
		{1, 3, "33.33%"},
		{99999, 100000, "100%"},
		// Just below the 100% cut-off the difference must survive.
		{9999, 10000, "99.99%"},
		{1, 10000, "0.01%"},
	}
	for _, c := range cases {
		if got := FormatPercent(c.part, c.total); got != c.want {
			t.Errorf("FormatPercent(%v, %v) = %q, want %q", c.part, c.total, got, c.want)
		}
	}
}
