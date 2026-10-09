package preflight

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run1(c Check) Result { return Run(context.Background(), []Check{c})[0] }

func expectResult(t *testing.T, r Result, status Status, detail string) {
	t.Helper()
	if r.Status != status || !strings.Contains(r.Detail, detail) {
		t.Fatalf("got %s %q, want %s containing %q (fix %q)", r.Status, r.Detail, status, detail, r.Fix)
	}
}

func isRoot() bool { return os.Geteuid() == 0 }

func TestFileReadable(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok")
	empty := filepath.Join(dir, "empty")
	locked := filepath.Join(dir, "locked")
	for p, body := range map[string]string{ok: "secret-value", empty: "", locked: "x"} {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		status     Status
		detail     string
	}{
		{"readable", ok, Pass, "readable (12 bytes)"},
		{"unset", "", Fail, "not configured"},
		{"missing", filepath.Join(dir, "absent"), Fail, "does not exist"},
		{"directory", dir, Fail, "is a directory"},
		{"empty", empty, Fail, "is empty"},
	}
	if !isRoot() {
		cases = append(cases, struct {
			name, path string
			status     Status
			detail     string
		}{"unreadable", locked, Fail, "not readable by uid"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := run1(FileReadable("f", tc.path))
			expectResult(t, r, tc.status, tc.detail)
			if strings.Contains(r.Detail, "secret-value") {
				t.Fatal("content must never be reported")
			}
		})
	}
}

func TestDirWritable(t *testing.T) {
	dir := t.TempDir()
	expectResult(t, run1(DirWritable("d", dir)), Pass, "writable")
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("probe file left behind: %v", entries)
	}
	expectResult(t, run1(DirWritable("d", "")), Fail, "not configured")
	expectResult(t, run1(DirWritable("d", filepath.Join(dir, "absent"))), Fail, "does not exist")
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectResult(t, run1(DirWritable("d", f)), Fail, "not a directory")
	if !isRoot() {
		ro := filepath.Join(dir, "ro")
		if err := os.Mkdir(ro, 0o500); err != nil {
			t.Fatal(err)
		}
		expectResult(t, run1(DirWritable("d", ro)), Fail, "not writable")
	}
}

// selfSigned writes a self-signed server certificate for 127.0.0.1 valid
// within [nb, na] and returns its cert/key files and TLS certificate.
func selfSigned(t *testing.T, nb, na time.Time) (certFile, keyFile string, crt tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "probe"}, DNSNames: []string{"probe.test"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: nb, NotAfter: na,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kder, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kder})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	crt, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile, crt
}

func TestKeyPair(t *testing.T) {
	now := func() time.Time { return clock }
	good, goodKey, _ := selfSigned(t, clock.Add(-time.Hour), clock.Add(90*24*time.Hour))
	soon, soonKey, _ := selfSigned(t, clock.Add(-time.Hour), clock.Add(3*24*time.Hour))
	old, oldKey, _ := selfSigned(t, clock.Add(-48*time.Hour), clock.Add(-24*time.Hour))
	future, futureKey, _ := selfSigned(t, clock.Add(time.Hour), clock.Add(48*time.Hour))
	cases := []struct {
		name, cert, key string
		status          Status
		detail          string
	}{
		{"valid", good, goodKey, Pass, "probe.test valid until"},
		{"expiring", soon, soonKey, Warn, "expires in 3 days"},
		{"expired", old, oldKey, Fail, "expired 24 h ago"},
		{"not yet valid", future, futureKey, Fail, "not valid before"},
		{"mismatched key", good, soonKey, Fail, "tls.crt"},
		{"missing key", good, filepath.Join(t.TempDir(), "nokey"), Fail, "does not exist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectResult(t, run1(KeyPair("kp", tc.cert, tc.key, now)), tc.status, tc.detail)
		})
	}
}

// closedAddr returns a loopback address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func TestTCPDial(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	expectResult(t, run1(TCPDial("lcm", l.Addr().String(), 0)), Pass, "reachable")
	r := run1(TCPDial("lcm", closedAddr(t), time.Second))
	expectResult(t, r, Fail, "connection refused")
	if r.Fix == "" {
		t.Fatal("refusal needs a fix hint")
	}
	expectResult(t, run1(TCPDial("lcm", "no-port", 0)), Fail, "not host:port")
	expectResult(t, run1(TCPDial("lcm", "host.invalid:9945", time.Second)), Fail, "cannot resolve host host.invalid")
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestDialFailureClassifies(t *testing.T) {
	expectResult(t, DialFailure("a:1", &net.OpError{Op: "dial", Err: timeoutErr{}}, 3*time.Second), Fail, "no answer within 3s")
	expectResult(t, DialFailure("a:1", context.DeadlineExceeded, time.Second), Fail, "no answer")
	expectResult(t, DialFailure("a:1", errors.New("weird\nfailure"), time.Second), Fail, "a:1: weird failure")
}

func TestHTTPSProbe(t *testing.T) {
	quiet := log.New(io.Discard, "", 0) // refused handshakes are expected
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) }))
	srv.Config.ErrorLog = quiet
	srv.StartTLS()
	defer srv.Close()
	plain := httptest.NewServer(http.NotFoundHandler())
	defer plain.Close()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	trusted := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "https://"))
	_, _, expired := selfSigned(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	old := httptest.NewUnstartedServer(http.NotFoundHandler())
	old.TLS = &tls.Config{Certificates: []tls.Certificate{expired}, MinVersion: tls.VersionTLS12}
	old.Config.ErrorLog = quiet
	old.StartTLS()
	defer old.Close()
	oldRoots := x509.NewCertPool()
	oldRoots.AddCert(expired.Leaf)
	cases := []struct {
		name, url string
		cfg       *tls.Config
		status    Status
		detail    string
	}{
		{"verified", srv.URL + "/api/lcm/v1/enroll", trusted, Pass, "answered HTTP 405 over TLS 1.3"},
		{"system roots do not know the server", srv.URL, nil, Fail, "TLS: certificate signed by an authority this host does not trust"},
		{"host name mismatch", "https://localhost:" + port, trusted, Fail, "TLS: certificate is not valid for localhost"},
		{"expired certificate", old.URL, &tls.Config{RootCAs: oldRoots, MinVersion: tls.VersionTLS12}, Fail, "TLS: certificate expired"},
		{"plain http port", "https://" + strings.TrimPrefix(plain.URL, "http://"), trusted, Fail, "did not answer with TLS"},
		{"insecure", srv.URL, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, Warn, "NOT verified"}, // #nosec G402 -- test
		{"custom verification refuses", srv.URL, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, // #nosec G402 -- test
			VerifyConnection: func(tls.ConnectionState) error { return errors.New("wrong SPIFFE ID") }}, Fail, "TLS: wrong SPIFFE ID"},
		{"refused", "https://" + closedAddr(t), trusted, Fail, "connection refused"},
		{"not https", "http://example.org", nil, Fail, "not an https URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectResult(t, run1(HTTPSProbe("enroll", tc.url, tc.cfg, time.Second)), tc.status, tc.detail)
		})
	}
}

func TestTLSFailureClassifies(t *testing.T) {
	expectResult(t, TLSFailure("a:1", tls.AlertError(40)), Fail, "refused the handshake")
	expectResult(t, TLSFailure("a:1", context.DeadlineExceeded), Fail, "handshake timed out")
	expectResult(t, TLSFailure("a:1", x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign}), Fail, "invalid certificate")
	expectResult(t, TLSFailure("a:1", x509.UnknownAuthorityError{}), Fail, "unknown issuer")
	expectResult(t, TLSFailure("a:1", x509.HostnameError{Host: "h"}), Fail, "it names nothing")
}
