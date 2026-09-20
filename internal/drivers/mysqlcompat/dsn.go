package mysqlcompat

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// DSNOptions are the per-engine knobs of the shared DSN builder.
type DSNOptions struct {
	// InterpolateParams lets the driver expand placeholders client-side, which
	// avoids a server round trip per statement. Doris supports it; MySQL
	// deliberately does not (its prepared-statement protocol is the faster and
	// safer path there anyway).
	InterpolateParams bool
}

// DSN builds the connection string for one database. Engines pass their own
// options through DSNWith.
func DSN(cfg models.ConnectionConfig, database string) (string, error) {
	return DSNWith(DSNOptions{})(cfg, database)
}

// DSNWith returns the DSN builder of an engine that needs its own options.
func DSNWith(opts DSNOptions) func(models.ConnectionConfig, string) (string, error) {
	return func(cfg models.ConnectionConfig, database string) (string, error) {
		return BuildDSN(cfg, database, opts)
	}
}

// BuildDSN renders the go-sql-driver connection string for a MySQL-compatible
// server.
func BuildDSN(cfg models.ConnectionConfig, database string, opts DSNOptions) (string, error) {
	mc := driver.NewConfig()
	mc.User = cfg.Username
	mc.Passwd = cfg.Password
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mc.DBName = database
	mc.ParseTime = true
	mc.Loc = time.Local
	mc.Timeout = 15 * time.Second
	mc.AllowNativePasswords = true
	mc.InterpolateParams = opts.InterpolateParams
	mc.Params = map[string]string{"charset": "utf8mb4"}

	for k, v := range cfg.Params {
		if k == "" || isReservedParam(k) {
			continue
		}
		mc.Params[k] = v
	}

	tlsName, err := TLSConfigName(cfg.SSL)
	if err != nil {
		return "", err
	}
	mc.TLSConfig = tlsName

	// FormatDSN never includes the password when it is empty, but when set it
	// is embedded; callers must treat the result as a secret.
	return mc.FormatDSN(), nil
}

// isReservedParam keeps the extra-parameter table from overriding what the
// builder above decided (a typo in a hand-written DSN is a connection that
// silently ignores TLS).
func isReservedParam(k string) bool {
	switch strings.ToLower(k) {
	case "tls", "charset", "parsetime", "loc", "timeout", "readtimeout", "writetimeout",
		"interpolateparams":
		return true
	}
	return false
}

// TLSConfigName maps our SSL settings onto go-sql-driver's TLS registry.
func TLSConfigName(ssl models.SSLConfig) (string, error) {
	switch ssl.Mode {
	case "", models.SSLDisable:
		return "false", nil
	case models.SSLRequire:
		// Encrypted transport, no certificate verification: the historical
		// meaning of "require" in MySQL.
		return "skip-verify", nil
	case models.SSLVerifyCA, models.SSLVerifyFull:
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if ssl.Mode == models.SSLVerifyFull {
			tlsCfg.ServerName = ""
		} else {
			// verify-ca: verify the chain but not the hostname.
			tlsCfg.InsecureSkipVerify = true
			tlsCfg.VerifyPeerCertificate = VerifyChainOnly
		}
		if ssl.CAFile != "" {
			pem, err := os.ReadFile(ssl.CAFile)
			if err != nil {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "read CA file")
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return "", apperr.New(apperr.CodeInvalidConfig, "CA file %s contains no usable certificate", ssl.CAFile)
			}
			tlsCfg.RootCAs = pool
		}
		if ssl.CertFile != "" && ssl.KeyFile != "" {
			pair, err := tls.LoadX509KeyPair(ssl.CertFile, ssl.KeyFile)
			if err != nil {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "load client certificate")
			}
			tlsCfg.Certificates = []tls.Certificate{pair}
		}

		// Registering under a deterministic name keeps repeated connects from
		// leaking registry entries.
		sum := sha1.Sum([]byte(string(ssl.Mode) + "|" + ssl.CAFile + "|" + ssl.CertFile + "|" + ssl.KeyFile))
		name := "dbm-" + hex.EncodeToString(sum[:8])
		if err := driver.RegisterTLSConfig(name, tlsCfg); err != nil {
			// A duplicate registration is fine: the config is equivalent.
			if !strings.Contains(err.Error(), "already registered") {
				return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "register TLS config")
			}
		}
		return name, nil
	default:
		return "", apperr.New(apperr.CodeInvalidConfig, "unsupported SSL mode %q", ssl.Mode)
	}
}

// VerifyChainOnly checks a presented certificate chain without checking the
// hostname, which is what "verify-ca" means.
func VerifyChainOnly(rawCerts [][]byte, _ [][]*x509.Certificate) error {
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
