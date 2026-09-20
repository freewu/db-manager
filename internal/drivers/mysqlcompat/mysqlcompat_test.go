package mysqlcompat

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

func TestFilterDatabasesHidesTheSchemasTheServerShipsWith(t *testing.T) {
	tests := []struct {
		name   string
		system map[string]bool
		in     []string
		want   []string
	}{
		{
			name: "user databases hide the system ones",
			in:   []string{"book", "information_schema", "mysql", "performance_schema", "sys"},
			want: []string{"book"},
		},
		{
			// A freshly installed server must not look like a broken connection.
			name: "fresh server keeps the system schemas",
			in:   []string{"information_schema", "mysql", "performance_schema", "sys"},
			want: []string{"information_schema", "mysql", "performance_schema", "sys"},
		},
		{
			name: "no databases at all stays empty",
			in:   []string{},
			want: []string{},
		},
		{
			name: "case insensitive filter",
			in:   []string{"MySQL", "SYS"},
			want: []string{"MySQL", "SYS"},
		},
		{
			// TiDB and Doris ship different schemas; the map travels with the
			// introspector so one list does not hide the other engine's.
			name:   "an engine brings its own list",
			system: map[string]bool{"__internal_schema": true},
			in:     []string{"demo", "information_schema", "__internal_schema"},
			want:   []string{"demo", "information_schema"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Introspector{SystemSchemas: tt.system}.FilterDatabases(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FilterDatabases(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// The listings report what the catalog knows; nil must stay nil so the UI can
// tell "no value" from "zero".
func TestNullableInt(t *testing.T) {
	if got := NullableInt(sql.NullInt64{}); got != nil {
		t.Errorf("NullableInt(NULL) = %v, want nil", *got)
	}
	got := NullableInt(sql.NullInt64{Int64: 12, Valid: true})
	if got == nil || *got != 12 {
		t.Errorf("NullableInt(12) = %v, want 12", got)
	}
}

func TestKindFromTableType(t *testing.T) {
	tests := map[string]models.ObjectKind{
		"BASE TABLE":  models.KindTable,
		"base table":  models.KindTable,
		"VIEW":        models.KindView,
		"SYSTEM VIEW": models.KindTable,
	}
	for in, want := range tests {
		if got := KindFromTableType(in); got != want {
			t.Errorf("KindFromTableType(%q) = %q, want %q", in, got, want)
		}
	}
}

// Status variables are read by name and parsed defensively: a server that does
// not have a counter still renders the page.
func TestNumberReadsWhatTheServerReported(t *testing.T) {
	values := map[string]string{"threads_connected": "12", "uptime": " not a number "}
	tests := []struct {
		name string
		key  string
		want float64
	}{
		{"reported counter", "Threads_connected", 12},
		{"missing counter", "questions", 0},
		{"unparseable counter", "Uptime", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Number(values, tt.key); got != tt.want {
				t.Fatalf("Number(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestStringOrFallsBackOnBlankValues(t *testing.T) {
	values := map[string]string{"hostname": "db-1", "datadir": "  "}
	if got := StringOr(values, "hostname", "—"); got != "db-1" {
		t.Errorf("StringOr(hostname) = %q, want db-1", got)
	}
	if got := StringOr(values, "datadir", "—"); got != "—" {
		t.Errorf("StringOr(blank) = %q, want the fallback", got)
	}
	if got := StringOr(values, "version", "—"); got != "—" {
		t.Errorf("StringOr(missing) = %q, want the fallback", got)
	}
}

func TestPeakConnectionsShowsTheCeiling(t *testing.T) {
	status := map[string]string{"max_used_connections": "37"}
	if got := PeakConnections(status, 151); got != "37 / 151" {
		t.Errorf("PeakConnections() = %q, want \"37 / 151\"", got)
	}
	if got := PeakConnections(map[string]string{}, 0); got != "0" {
		t.Errorf("PeakConnections() = %q, want \"0\"", got)
	}
}

func TestWithStateColoursAMetric(t *testing.T) {
	metric := WithState(sqlbase.Metric("Aborted connects", "3", "Aborted_connects"), "warn")
	if metric.State != "warn" {
		t.Fatalf("WithState() left State empty: %+v", metric)
	}
}

// DSN building is shared by every engine in the family, so the parts the UI can
// break are pinned here: the extra-parameter table, the SSL modes and the
// options Doris needs.
func TestBuildDSN(c *testing.T) {
	base := models.ConnectionConfig{
		Host: "127.0.0.1", Port: 9030, Username: "root", Password: "",
	}

	c.Run("keeps user parameters", func(t *testing.T) {
		cfg := base
		cfg.Params = map[string]string{"connect_timeout": "5", "foo": "bar"}
		dsn, err := BuildDSN(cfg, "demo", DSNOptions{})
		if err != nil {
			t.Fatalf("BuildDSN() error = %v", err)
		}
		for _, want := range []string{"foo=bar", "connect_timeout=5"} {
			if !strings.Contains(dsn, want) {
				t.Errorf("DSN = %q, want it to contain %q", dsn, want)
			}
		}
	})

	c.Run("reserved parameters cannot be overridden", func(t *testing.T) {
		cfg := base
		cfg.Params = map[string]string{"tls": "skip-verify", "charset": "latin1", "parseTime": "false"}
		dsn, err := BuildDSN(cfg, "demo", DSNOptions{})
		if err != nil {
			t.Fatalf("BuildDSN() error = %v", err)
		}
		if strings.Contains(dsn, "latin1") || strings.Contains(dsn, "skip-verify") {
			t.Errorf("DSN = %q, want the reserved parameters ignored", dsn)
		}
	})

	c.Run("interpolateParams is off unless asked for", func(t *testing.T) {
		plain, err := BuildDSN(base, "demo", DSNOptions{})
		if err != nil {
			t.Fatalf("BuildDSN() error = %v", err)
		}
		if strings.Contains(plain, "interpolateParams") {
			t.Errorf("DSN = %q, want no interpolateParams for MySQL", plain)
		}
		interpolated, err := BuildDSN(base, "demo", DSNOptions{InterpolateParams: true})
		if err != nil {
			t.Fatalf("BuildDSN() error = %v", err)
		}
		if !strings.Contains(interpolated, "interpolateParams=true") {
			t.Errorf("DSN = %q, want interpolateParams=true", interpolated)
		}
	})
}

func TestTLSConfigNameMapsOurModes(t *testing.T) {
	tests := []struct {
		name string
		ssl  models.SSLConfig
		want string
	}{
		{"unset means no TLS", models.SSLConfig{}, "false"},
		{"disable", models.SSLConfig{Mode: models.SSLDisable}, "false"},
		{"require encrypts without verifying", models.SSLConfig{Mode: models.SSLRequire}, "skip-verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TLSConfigName(tt.ssl)
			if err != nil {
				t.Fatalf("TLSConfigName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("TLSConfigName() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("verify modes register a config", func(t *testing.T) {
		name, err := TLSConfigName(models.SSLConfig{Mode: models.SSLVerifyFull})
		if err != nil {
			t.Fatalf("TLSConfigName() error = %v", err)
		}
		if name == "" || name == "false" || name == "skip-verify" {
			t.Fatalf("TLSConfigName() = %q, want a registered config name", name)
		}
		// The name is derived from the settings, so a second call reuses it
		// instead of growing the driver's registry.
		again, err := TLSConfigName(models.SSLConfig{Mode: models.SSLVerifyFull})
		if err != nil || again != name {
			t.Fatalf("TLSConfigName() = %q, %v; want %q", again, err, name)
		}
	})

	t.Run("a missing CA file is reported", func(t *testing.T) {
		_, err := TLSConfigName(models.SSLConfig{Mode: models.SSLVerifyCA, CAFile: "/does/not/exist.pem"})
		if err == nil {
			t.Fatal("TLSConfigName() accepted a CA file that does not exist")
		}
	})

	t.Run("an unknown mode is reported", func(t *testing.T) {
		_, err := TLSConfigName(models.SSLConfig{Mode: "sometimes"})
		if err == nil {
			t.Fatal("TLSConfigName() accepted an unknown mode")
		}
	})
}
