// Package mongodb implements the MongoDB driver.
//
// MongoDB is not a SQL engine, so this driver deliberately does not sit on
// sqlbase. It answers the same drivers.Conn contract with the server's own
// commands and maps documents onto the metadata model the rest of the app
// already understands:
//
//	database   -> catalog (the first level of the explorer tree)
//	collection -> object  (ObjectInfo.Kind is models.KindCollection)
//	field      -> column  (Structure infers fields by sampling documents)
//
// There is no SQL to send, so Execute runs a small shell-flavoured language
// that mirrors the mongosh methods one to one (db.users.find({...})); see
// shell.go for the grammar and the list of supported commands.
package mongodb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

const (
	defaultPort = 27017
	// defaultDatabase stands in for the session's current database and for
	// statements that do not name one. admin always exists, whereas the shell
	// default ("test") may not.
	defaultDatabase = "admin"

	connectTimeout   = 15 * time.Second
	selectionTimeout = 15 * time.Second
)

// Driver is the registered entry point.
type Driver struct{}

func init() { drivers.Register(Driver{}) }

// Info implements drivers.Driver.
func (Driver) Info() models.DriverInfo {
	return models.DriverInfo{
		Type:             models.DriverMongoDB,
		DisplayName:      "MongoDB",
		DefaultPort:      defaultPort,
		Implemented:      true,
		Relational:       false,
		SupportsDatabase: true,
		SupportsSchema:   false,
		DefaultDatabase:  defaultDatabase,
		SortOrder:        40,
		Notes:            "Document store: databases hold collections, collections hold documents. Fields are inferred from a document sample.",
	}
}

// Normalize implements drivers.Driver.
func (Driver) Normalize(cfg *models.ConnectionConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return apperr.New(apperr.CodeInvalidConfig, "port %d is out of range", cfg.Port)
	}
	if cfg.SSL.Mode == "" {
		cfg.SSL.Mode = models.SSLDisable
	}
	return nil
}

// Open implements drivers.Driver.
func (d Driver) Open(ctx context.Context, cfg models.ConnectionConfig) (drivers.Conn, error) {
	if err := d.Normalize(&cfg); err != nil {
		return nil, err
	}
	uri, err := buildURI(cfg)
	if err != nil {
		return nil, err
	}
	opts := options.Client().
		ApplyURI(uri).
		SetAppName("db-manager").
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(selectionTimeout)
	if tlsCfg, err := tlsConfig(cfg.SSL); err != nil {
		return nil, err
	} else if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeConnectionFail, err, "connect to MongoDB")
	}
	conn := &Conn{client: client, cfg: cfg}
	// Ping with primaryPreferred: connecting to a replica set secondary is a
	// legitimate thing to do, so requiring a primary would reject a working
	// session, but a primary is still what most commands need.
	if err := conn.client.Ping(ctx, readpref.PrimaryPreferred()); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// buildURI renders the connection string.
//
// The form fields cover the common cases: a single host, a replica set listed
// as comma separated hosts, and DNS seed lists through the "srv" parameter.
// Every other URI option goes through Params and is appended verbatim, so
// anything the official driver understands keeps working without a UI change.
//
// Credentials are embedded as userinfo rather than handed to SetAuth so that
// url.UserPassword escapes them: a password containing "@" or ":" must not
// break the URI. When a database is configured and no authSource is given, the
// database doubles as the authentication source — that is where an application
// user normally lives.
func buildURI(cfg models.ConnectionConfig) (string, error) {
	scheme := "mongodb"
	srv := isTruthy(cfg.Params["srv"])
	if srv {
		scheme = "mongodb+srv"
	}
	authority, err := buildAuthority(cfg, srv)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	for k, v := range cfg.Params {
		if k == "" || strings.EqualFold(k, "srv") {
			continue
		}
		query.Set(k, v)
	}
	if cfg.Username != "" && query.Get("authSource") == "" && cfg.Database != "" {
		query.Set("authSource", cfg.Database)
	}

	u := url.URL{Scheme: scheme, Host: authority, Path: "/"}
	if cfg.Username != "" || cfg.Password != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

// buildAuthority turns the host field into the URI authority. A comma
// separated list is how a replica set is spelled without a DNS seed list.
func buildAuthority(cfg models.ConnectionConfig, srv bool) (string, error) {
	parts := strings.Split(cfg.Host, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		host := strings.TrimSpace(part)
		if host == "" {
			continue
		}
		if srv {
			// mongodb+srv forbids a port: the SRV record supplies it.
			if _, _, err := net.SplitHostPort(host); err == nil {
				return "", apperr.New(apperr.CodeInvalidConfig, "mongodb+srv takes a host name without a port, got %q", host)
			}
			out = append(out, host)
			continue
		}
		out = append(out, withPort(host, cfg.Port))
	}
	if len(out) == 0 {
		return "", apperr.New(apperr.CodeInvalidConfig, "no host given")
	}
	if srv && len(out) > 1 {
		return "", apperr.New(apperr.CodeInvalidConfig, "mongodb+srv takes a single host name")
	}
	return strings.Join(out, ","), nil
}

// withPort appends the configured port to a host that does not carry one.
// IPv6 literals are bracketed by net.JoinHostPort.
func withPort(host string, port int) string {
	if net.ParseIP(host) != nil {
		// Bare IPv6 ("::1"): always bracket it.
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// tlsConfig maps our SSL settings onto a *tls.Config. The modes mean the same
// thing as they do for MySQL and PostgreSQL.
func tlsConfig(ssl models.SSLConfig) (*tls.Config, error) {
	mode := ssl.Mode
	if mode == "" {
		mode = models.SSLDisable
	}
	if mode == models.SSLDisable {
		return nil, nil
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch mode {
	case models.SSLRequire:
		// Encrypted transport without certificate verification.
		cfg.InsecureSkipVerify = true
	case models.SSLVerifyCA:
		// Chain is verified, host name is not.
		cfg.InsecureSkipVerify = true
		cfg.VerifyPeerCertificate = verifyChainOnly
	case models.SSLVerifyFull:
		// Library defaults already verify chain and host name.
	default:
		return nil, apperr.New(apperr.CodeInvalidConfig, "unknown SSL mode %q", ssl.Mode)
	}

	if ssl.CAFile != "" {
		pem, err := os.ReadFile(ssl.CAFile)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "read CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, apperr.New(apperr.CodeInvalidConfig, "CA file %s contains no usable certificate", ssl.CAFile)
		}
		cfg.RootCAs = pool
	}

	if ssl.CertFile != "" || ssl.KeyFile != "" {
		if ssl.CertFile == "" || ssl.KeyFile == "" {
			return nil, apperr.New(apperr.CodeInvalidConfig, "a client certificate needs both a certificate file and a key file")
		}
		pair, err := tls.LoadX509KeyPair(ssl.CertFile, ssl.KeyFile)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "load client certificate")
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

// verifyChainOnly validates the presented chain and skips the host name check,
// which is what verify-ca asks for. Same behaviour as the MySQL driver.
func verifyChainOnly(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	certs := make([]*x509.Certificate, 0, len(rawCerts))
	for _, raw := range rawCerts {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return fmt.Errorf("no certificate presented by server")
	}
	pool := x509.NewCertPool()
	for _, c := range certs[1:] {
		pool.AddCert(c)
	}
	_, err := certs[0].Verify(x509.VerifyOptions{Roots: pool, Intermediates: pool})
	return err
}

// describe renders a value as extended JSON: messages and generated scripts
// have to spell values the way the shell language reads them back ("events"
// with the quotes, {"a": 1} as a document).
//
// The extended JSON encoder only writes a document at the top level, so a
// scalar or an array is rendered inside one and the wrapper is trimmed off
// again — otherwise "["a", "b"]" would fall back to Go's %v and print text
// the shell cannot parse.
func describe(value any) string {
	wrapped := bson.D{{Key: describeWrapper, Value: value}}
	text, err := marshalExtJSON(wrapped)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	prefix := `{"` + describeWrapper + `":`
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, "}") {
		return text
	}
	return text[len(prefix) : len(text)-1]
}

// describeWrapper is the throwaway key describe wraps a value in.
const describeWrapper = "v"
