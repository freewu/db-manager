package mongodb

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

func TestNormalize(t *testing.T) {
	cfg := models.ConnectionConfig{}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != defaultPort {
		t.Errorf("defaults = %s:%d", cfg.Host, cfg.Port)
	}
	if cfg.SSL.Mode != models.SSLDisable {
		t.Errorf("ssl mode = %q", cfg.SSL.Mode)
	}

	cfg = models.ConnectionConfig{Host: "db.internal", Port: 27018, SSL: models.SSLConfig{Mode: models.SSLRequire}}
	if err := (Driver{}).Normalize(&cfg); err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if cfg.Host != "db.internal" || cfg.Port != 27018 || cfg.SSL.Mode != models.SSLRequire {
		t.Errorf("explicit values were overwritten: %+v", cfg)
	}

	if err := (Driver{}).Normalize(&models.ConnectionConfig{Port: 70000}); err == nil {
		t.Error("an out of range port must be refused")
	}
}

func TestInfo(t *testing.T) {
	info := (Driver{}).Info()
	if info.Type != models.DriverMongoDB || !info.Implemented || info.Relational {
		t.Errorf("info = %+v", info)
	}
	if info.SupportsSchema || !info.SupportsDatabase {
		t.Errorf("a database is a catalog and there is no schema level: %+v", info)
	}
}

func TestBuildURI(t *testing.T) {
	cases := []struct {
		name string
		cfg  models.ConnectionConfig
		want string
	}{
		{
			name: "plain host",
			cfg:  models.ConnectionConfig{Host: "127.0.0.1", Port: 27017},
			want: "mongodb://127.0.0.1:27017/",
		},
		{
			name: "credentials and database",
			cfg:  models.ConnectionConfig{Host: "db", Port: 27017, Username: "app", Password: "s3cret", Database: "shop"},
			want: "mongodb://app:s3cret@db:27017/?authSource=shop",
		},
		{
			name: "a password with reserved characters stays encoded",
			cfg:  models.ConnectionConfig{Host: "db", Port: 27017, Username: "app", Password: "p@ss:w/rd", Database: "shop"},
			want: "mongodb://app:p%40ss%3Aw%2Frd@db:27017/?authSource=shop",
		},
		{
			name: "an explicit auth source wins",
			cfg: models.ConnectionConfig{
				Host: "db", Port: 27017, Username: "app", Password: "x", Database: "shop",
				Params: map[string]string{"authSource": "admin"},
			},
			want: "mongodb://app:x@db:27017/?authSource=admin",
		},
		{
			name: "replica set members as a list",
			cfg:  models.ConnectionConfig{Host: "a:27017, b:27018", Port: 27017},
			want: "mongodb://a:27017,b:27018/",
		},
		{
			name: "srv lookup",
			cfg: models.ConnectionConfig{
				Host: "cluster.example.com", Port: 27017,
				Params: map[string]string{"srv": "true", "retryWrites": "false"},
			},
			want: "mongodb+srv://cluster.example.com/?retryWrites=false",
		},
		{
			name: "ipv6 is bracketed",
			cfg:  models.ConnectionConfig{Host: "::1", Port: 27017},
			want: "mongodb://[::1]:27017/",
		},
		{
			name: "an explicit port in the host wins",
			cfg:  models.ConnectionConfig{Host: "db:9999", Port: 27017},
			want: "mongodb://db:9999/",
		},
		{
			name: "parameters are passed through",
			cfg: models.ConnectionConfig{
				Host: "db", Port: 27017,
				Params: map[string]string{"replicaSet": "rs0", "readPreference": "secondaryPreferred"},
			},
			want: "mongodb://db:27017/?readPreference=secondaryPreferred&replicaSet=rs0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildURI(tc.cfg)
			if err != nil {
				t.Fatalf("buildURI failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("uri = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildURIErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  models.ConnectionConfig
		want string
	}{
		{
			name: "srv takes no port",
			cfg:  models.ConnectionConfig{Host: "db:27017", Params: map[string]string{"srv": "1"}},
			want: "without a port",
		},
		{
			name: "srv takes one host",
			cfg:  models.ConnectionConfig{Host: "a,b", Params: map[string]string{"srv": "yes"}},
			want: "single host",
		},
		{
			name: "an empty host",
			cfg:  models.ConnectionConfig{Host: " , "},
			want: "no host given",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildURI(tc.cfg)
			if err == nil {
				t.Fatal("buildURI succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestWithPort(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"db", "db:27017"},
		{"db:27018", "db:27018"},
		{"127.0.0.1", "127.0.0.1:27017"},
		{"::1", "[::1]:27017"},
		{"[::1]:27018", "[::1]:27018"},
	}
	for _, tc := range cases {
		if got := withPort(tc.host, 27017); got != tc.want {
			t.Errorf("withPort(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestTLSConfig(t *testing.T) {
	if cfg, err := tlsConfig(models.SSLConfig{Mode: models.SSLDisable}); err != nil || cfg != nil {
		t.Errorf("disabled ssl must mean no tls config, got %v / %v", cfg, err)
	}

	cfg, err := tlsConfig(models.SSLConfig{Mode: models.SSLRequire})
	if err != nil || cfg == nil || !cfg.InsecureSkipVerify {
		t.Errorf("require must encrypt without verifying, got %+v / %v", cfg, err)
	}

	cfg, err = tlsConfig(models.SSLConfig{Mode: models.SSLVerifyCA})
	if err != nil || cfg == nil || cfg.VerifyPeerCertificate == nil {
		t.Errorf("verify-ca must verify the chain by hand, got %+v / %v", cfg, err)
	}

	cfg, err = tlsConfig(models.SSLConfig{Mode: models.SSLVerifyFull})
	if err != nil || cfg == nil || cfg.InsecureSkipVerify || cfg.VerifyPeerCertificate != nil {
		t.Errorf("verify-full must use the library defaults, got %+v / %v", cfg, err)
	}

	if _, err := tlsConfig(models.SSLConfig{Mode: "sometimes"}); err == nil {
		t.Error("an unknown ssl mode must be refused")
	}
	if _, err := tlsConfig(models.SSLConfig{Mode: models.SSLRequire, CertFile: "client.pem"}); err == nil {
		t.Error("a client certificate without a key must be refused")
	}
	if _, err := tlsConfig(models.SSLConfig{Mode: models.SSLRequire, CAFile: "does-not-exist.pem"}); err == nil {
		t.Error("a missing ca file must be refused")
	}
}

func TestIndexInfos(t *testing.T) {
	docs := []bson.D{
		doc(t, `{"name": "_id_", "key": {"_id": 1}}`),
		doc(t, `{"name": "sku_1", "key": {"sku": 1}, "unique": true}`),
		doc(t, `{"name": "text_idx", "key": {"title": "text"}}`),
		doc(t, `{"name": "geo_idx", "key": {"loc": "2dsphere"}}`),
		doc(t, `{"name": "hidden_idx", "key": {"a": -1}, "hidden": true}`),
	}
	infos := indexInfos(docs)
	if len(infos) != 5 {
		t.Fatalf("got %d indexes", len(infos))
	}
	if !infos[0].Primary || infos[0].Name != "_id_" {
		t.Errorf("the primary index must sort first: %+v", infos[0])
	}
	byName := map[string]models.IndexInfo{}
	for _, info := range infos {
		byName[info.Name] = info
	}
	if got := byName["sku_1"]; !got.Unique || got.Columns[0] != "sku ASC" || got.Method != "btree" {
		t.Errorf("sku_1 = %+v", got)
	}
	if got := byName["text_idx"]; got.Method != "text" || got.Columns[0] != "title text" {
		t.Errorf("text_idx = %+v", got)
	}
	if got := byName["geo_idx"]; got.Method != "geo" {
		t.Errorf("geo_idx = %+v", got)
	}
	if got := byName["hidden_idx"]; got.Columns[0] != "a DESC" || got.Comment != "hidden" {
		t.Errorf("hidden_idx = %+v", got)
	}
}

func TestIndexKeyAndOptionsRoundTrip(t *testing.T) {
	idx := models.IndexInfo{Name: "sku_1", Columns: []string{"sku ASC", "brand DESC"}, Unique: true, Comment: "hidden"}
	if got, want := describe(indexKey(idx)), `{"sku":1,"brand":-1}`; got != want {
		t.Errorf("key = %s, want %s", got, want)
	}
	if got, want := describe(indexOptions(idx)), `{"name":"sku_1","unique":true,"hidden":true}`; got != want {
		t.Errorf("options = %s, want %s", got, want)
	}

	// A text or geo index keeps its special key value instead of a direction.
	text := models.IndexInfo{Name: "t", Columns: []string{"title text"}}
	if got, want := describe(indexKey(text)), `{"title":"text"}`; got != want {
		t.Errorf("text key = %s, want %s", got, want)
	}
}

func TestDescribeFetch(t *testing.T) {
	cases := []struct {
		name   string
		req    string
		filter string
		want   string
	}{
		{
			name:   "no filter",
			req:    "orders",
			filter: `{}`,
			want:   `db.getCollection("orders").find({}).sort({"_id":1}).limit(200)`,
		},
		{
			name:   "a filter and an offset",
			req:    "orders",
			filter: `{"state": "paid"}`,
			want:   `db.getCollection("orders").find({"state":"paid"}).sort({"_id":1}).skip(400).limit(200)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var filter bson.D
			if err := bson.UnmarshalExtJSON([]byte(tc.filter), false, &filter); err != nil {
				t.Fatalf("bad filter: %v", err)
			}
			offset := 0
			if strings.Contains(tc.want, "skip") {
				offset = 400
			}
			got := describeFetch(drivers.FetchRequest{Object: tc.req}, filter, bson.D{{Key: idField, Value: 1}}, 200, offset)
			if got != tc.want {
				t.Errorf("describeFetch =\n %s\nwant\n %s", got, tc.want)
			}
		})
	}
}

func TestResolveDatabase(t *testing.T) {
	cases := []struct {
		name     string
		profile  string
		explicit string
		want     string
	}{
		{"explicit wins", "shop", "reporting", "reporting"},
		{"the profile is the default", "shop", "", "shop"},
		{"admin is the fallback", "", "", defaultDatabase},
		{"whitespace is not a database", "shop", "  ", "shop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Conn{cfg: models.ConnectionConfig{Database: tc.profile}}
			if got := c.resolveDatabase(tc.explicit); got != tc.want {
				t.Errorf("resolveDatabase(%q) = %q, want %q", tc.explicit, got, tc.want)
			}
		})
	}
}

// TestRegistered checks the driver is wired into the registry: without it the
// connection dialog would never offer MongoDB.
func TestRegistered(t *testing.T) {
	registered, ok := drivers.Get(models.DriverMongoDB)
	if !ok {
		t.Fatal("the MongoDB driver is not registered")
	}
	if registered.Info().Implemented {
		return
	}
	t.Error("the registered driver must be marked as implemented")
}

func TestDialect(t *testing.T) {
	d := Dialect{}
	if d.SupportsLimitOffset() {
		t.Error("MongoDB has no LIMIT/OFFSET clause; paging is skip/limit")
	}
	if got := d.LimitOffset(10, 20); got != "" {
		t.Errorf("LimitOffset = %q, want nothing", got)
	}
	if got := d.Qualify("shop", "", "orders"); got != "shop.orders" {
		t.Errorf("Qualify = %q", got)
	}
	if got := d.Qualify("", "", "orders"); got != "orders" {
		t.Errorf("Qualify without a database = %q", got)
	}
	if got := d.Quote("some.field"); got != "some.field" {
		t.Errorf("Quote must not touch dotted paths, got %q", got)
	}
	if got := d.Placeholder(1); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if d.Name() != models.DriverMongoDB {
		t.Error("the dialect must name its driver")
	}
}
