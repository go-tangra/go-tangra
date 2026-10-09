package preflight

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const platformJWKS = `{"keys":[{"kty":"OKP","crv":"Ed25519","kid":"k1","x":"J-dVacDYm0zmy2-X1K6XQWwsCnsjz5FQQ7K8U3wqHPE","use":"sig","alg":"EdDSA"}]}`

func TestIssuerJWKS(t *testing.T) {
	quiet := log.New(io.Discard, "", 0)
	answers := map[string]string{
		"/.well-known/jwks.json":       platformJWKS,
		"/rsa/.well-known/jwks.json":   `{"keys":[{"kty":"RSA","n":"x","e":"AQAB"}]}`,
		"/html/.well-known/jwks.json":  `<html>portal</html>`,
		"/empty/.well-known/jwks.json": `{"keys":[]}`,
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := answers[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	srv.Config.ErrorLog = quiet
	srv.StartTLS()
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	trusted := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}

	cases := []struct {
		name, issuer string
		cfg          *tls.Config
		status       Status
		detail       string
	}{
		{"platform origin", srv.URL, trusted, Pass, "publishes 1 platform signing key"},
		{"trailing slash", srv.URL + "/", trusted, Pass, "publishes 1 platform signing key"},
		{"no keys at that path", srv.URL + "/elsewhere", trusted, Fail, "answered HTTP 404"},
		{"not a key set", srv.URL + "/html", trusted, Fail, "did not answer with a key set"},
		{"no Ed25519 key", srv.URL + "/rsa", trusted, Fail, "has no Ed25519 signing keys"},
		{"empty key set", srv.URL + "/empty", trusted, Fail, "has no Ed25519 signing keys"},
		{"nothing listens (localhost:8443 case)", "https://" + closedAddr(t), trusted, Fail, "connection refused"},
		{"untrusted certificate", srv.URL, nil, Fail, "authority this host does not trust"},
		{"insecure", srv.URL, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, Warn, "NOT verified"}, // #nosec G402 -- test
		{"placeholder", "@@GATEWAY_ISSUER@@", nil, Fail, "not an https origin"},
		{"plain http", "http://portal.example.com", nil, Fail, "not an https origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := run1(IssuerJWKS("issuer", tc.issuer, tc.cfg, time.Second))
			expectResult(t, r, tc.status, tc.detail)
			if r.Status == Fail && !strings.Contains(r.Fix, "portal's public origin") {
				t.Fatalf("a failure must say what the issuer should be: fix %q", r.Fix)
			}
		})
	}
}

func TestIssuerOrigin(t *testing.T) {
	const enroll = "https://portal.example.com:8443/api/lcm/v1/enroll"
	cases := []struct {
		issuer, ref string
		status      Status
		detail      string
	}{
		{"https://portal.example.com:8443", enroll, Pass, "is the enrolment URL's origin"},
		{"https://Portal.Example.com:8443/", enroll, Pass, "is the enrolment URL's origin"},
		{"https://portal.example.com", "https://portal.example.com:443/x", Pass, "origin"},
		{"https://localhost:8443", enroll, Warn, "differs from the enrolment URL's origin https://portal.example.com:8443"},
		{"https://portal.example.com", enroll, Warn, "https://portal.example.com:443 differs"},
		{"@@GATEWAY_ISSUER@@", enroll, Fail, "not an https origin"},
		{"https://portal.example.com:8443", "", Skip, "not a URL"},
	}
	for _, tc := range cases {
		expectResult(t, run1(IssuerOrigin("issuer origin", tc.issuer, "enrolment URL", tc.ref)), tc.status, tc.detail)
	}
}
