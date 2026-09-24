package tlsconf

import (
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
)

const enrollTD = "example.org"

// svidServer runs a server-auth-only TLS 1.3 listener presenting crt, like
// lcm's keyless enroll listener (an SVID: URI SAN only, mesh root issuer).
func svidServer(t *testing.T, crt *tls.Certificate) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion: tls.VersionTLS13, ClientAuth: tls.NoClientCert, Certificates: []tls.Certificate{*crt},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_ = c.(*tls.Conn).Handshake()
			}(c)
		}
	}()
	return ln.Addr().String()
}

// dial performs a client handshake the way an HTTP client to https://lcm:9947
// would (ServerName is the URL host, which the SVID does not carry).
func dial(t *testing.T, addr string, cfg *tls.Config) error {
	t.Helper()
	cfg = cfg.Clone()
	cfg.ServerName = "lcm"
	c, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, cfg)
	if err != nil {
		return err
	}
	return c.Close()
}

func lcmID(t *testing.T) identity.SPIFFEID {
	t.Helper()
	return identity.ForService(enrollTD, "lcm")
}

func TestEnrollClientConfigMeshVerified(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	addr := svidServer(t, ptr(ca.MustIssue("lcm", testutil.IssueOptions{})))
	cfg, err := EnrollClientConfig(EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion < tls.VersionTLS12 {
		t.Fatalf("min version %x", cfg.MinVersion)
	}
	if err := dial(t, addr, cfg); err != nil {
		t.Fatalf("verified enroll handshake failed: %v", err)
	}
}

func TestEnrollClientConfigRefusals(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	other := testutil.MustCA(enrollTD)
	now := time.Now()
	cases := []struct {
		name   string
		leaf   tls.Certificate
		opts   EnrollOptions
		reason identity.Reason
	}{
		{name: "wrong spiffe id", leaf: ca.MustIssue("auth", testutil.IssueOptions{}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonNameMismatch},
		{name: "wrong ca", leaf: other.MustIssue("lcm", testutil.IssueOptions{}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonUntrusted},
		{name: "foreign trust domain", leaf: ca.MustIssue("lcm", testutil.IssueOptions{TrustDomain: "evil.org"}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonUntrusted},
		{name: "no uri san", leaf: ca.MustIssue("lcm", testutil.IssueOptions{NoSAN: true}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonUntrusted},
		{name: "two uri sans", leaf: ca.MustIssue("lcm", testutil.IssueOptions{URIs: []*url.URL{
			{Scheme: "spiffe", Host: enrollTD, Path: "/svc/lcm"}, {Scheme: "spiffe", Host: enrollTD, Path: "/svc/auth"}}}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonUntrusted},
		{name: "expired", leaf: ca.MustIssue("lcm", testutil.IssueOptions{NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-time.Hour)}),
			opts: EnrollOptions{CAPEM: ca.BundlePEM(), ServerID: lcmID(t)}, reason: identity.ReasonExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := svidServer(t, &tc.leaf)
			cfg, err := EnrollClientConfig(tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			err = dial(t, addr, cfg)
			if err == nil {
				t.Fatal("handshake accepted")
			}
			var ve *identity.VerifyError
			if !errors.As(err, &ve) || ve.Reason != tc.reason {
				t.Fatalf("err = %v (%s), want reason %s", err, Classify(err), tc.reason)
			}
		})
	}
}

// Without a mesh trust bundle the dial falls back to public verification
// (system roots + host name), which an SVID can never satisfy: the gateway
// needs enroll.ca_file (or, in development only, insecure) to reach lcm.
func TestEnrollClientConfigPublicRefusesSVID(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	addr := svidServer(t, ptr(ca.MustIssue("lcm", testutil.IssueOptions{})))
	cfg, err := EnrollClientConfig(EnrollOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InsecureSkipVerify || cfg.RootCAs != nil || cfg.VerifyConnection != nil || cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("public config is not the standard verification: %+v", cfg)
	}
	if err := dial(t, addr, cfg); err == nil {
		t.Fatal("public verification accepted an SVID")
	}
}

func TestEnrollClientConfigInsecure(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	addr := svidServer(t, ptr(ca.MustIssue("anything", testutil.IssueOptions{})))
	cfg, err := EnrollClientConfig(EnrollOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("insecure did not skip verification")
	}
	if err := dial(t, addr, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestEnrollClientConfigOptionErrors(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	cases := map[string]EnrollOptions{
		"insecure with ca":   {Insecure: true, CAPEM: ca.BundlePEM(), ServerID: lcmID(t)},
		"ca without id":      {CAPEM: ca.BundlePEM()},
		"id without ca":      {ServerID: lcmID(t)},
		"ca without certs":   {CAPEM: []byte("not pem"), ServerID: lcmID(t)},
		"insecure with id":   {Insecure: true, ServerID: lcmID(t)},
		"only a key block":   {CAPEM: []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"), ServerID: lcmID(t)},
		"malformed ca block": {CAPEM: []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"), ServerID: lcmID(t)},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := EnrollClientConfig(o); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestLoadEnrollClientConfig(t *testing.T) {
	ca := testutil.MustCA(enrollTD)
	addr := svidServer(t, ptr(ca.MustIssue("lcm", testutil.IssueOptions{})))
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, ca.BundlePEM(), 0o600); err != nil {
		t.Fatal(err)
	}

	// Default expected server: spiffe://<td>/svc/lcm.
	cfg, err := LoadEnrollClientConfig(config.EnrollTLS{CAFile: caFile}, enrollTD)
	if err != nil {
		t.Fatal(err)
	}
	if err := dial(t, addr, cfg); err != nil {
		t.Fatalf("default server id: %v", err)
	}

	// An explicit, different expected ID refuses lcm's SVID.
	cfg, err = LoadEnrollClientConfig(config.EnrollTLS{CAFile: caFile, ServerSPIFFEID: "spiffe://example.org/svc/auth"}, enrollTD)
	if err != nil {
		t.Fatal(err)
	}
	if err := dial(t, addr, cfg); Classify(err) != identity.ReasonNameMismatch {
		t.Fatalf("err = %v", err)
	}

	// Public and insecure modes need no file.
	if cfg, err = LoadEnrollClientConfig(config.EnrollTLS{}, enrollTD); err != nil || cfg.InsecureSkipVerify {
		t.Fatalf("public: %v", err)
	}
	if cfg, err = LoadEnrollClientConfig(config.EnrollTLS{Insecure: true}, enrollTD); err != nil || !cfg.InsecureSkipVerify {
		t.Fatalf("insecure: %v", err)
	}

	// A missing file and invalid settings are errors, never a silent fallback.
	if _, err = LoadEnrollClientConfig(config.EnrollTLS{CAFile: filepath.Join(t.TempDir(), "missing.pem")}, enrollTD); err == nil ||
		!strings.Contains(err.Error(), "enroll.ca_file") {
		t.Fatalf("missing file: %v", err)
	}
	if _, err = LoadEnrollClientConfig(config.EnrollTLS{CAFile: caFile, ServerSPIFFEID: "spiffe://other.org/svc/lcm"}, enrollTD); err == nil {
		t.Fatal("foreign server id accepted")
	}
}

func ptr[T any](v T) *T { return &v }
