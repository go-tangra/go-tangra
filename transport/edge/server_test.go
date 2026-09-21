package edge

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"
)

// devServer starts an edge server with a generated dev certificate (env != production).
var lastRT *testrt.Runtime

func devServer(t *testing.T, cfg Config, opts ...ServerOption) (*Server, *http.Client, string) {
	t.Helper()
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "auth")
	lastRT = rt
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	cfg.Env = "test"
	srv, err := NewServer(rt, cfg, opts...)
	if err != nil {
		t.Fatal(err)
	}
	stop := testrt.StartServer(t, srv)
	t.Cleanup(stop)
	ep, _ := srv.Endpoint()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}, //nolint:gosec // dev cert
		ForceAttemptHTTP2: true,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return srv, client, "https://" + ep.Host
}

func TestServerTLS13OnlyAndDevCertGate(t *testing.T) {
	srv, client, base := devServer(t, Config{})
	srv.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "pong") })
	resp, err := client.Get(base + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "pong" || resp.ProtoMajor != 2 || resp.TLS.Version != tls.VersionTLS13 {
		t.Fatalf("status %d body %q proto %s tls %x", resp.StatusCode, body, resp.Proto, resp.TLS.Version)
	}
	// TLS 1.2 client refused.
	old := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12} //nolint:gosec // probe
	if _, err := tls.Dial("tcp", strings.TrimPrefix(base, "https://"), old); err == nil {
		t.Fatal("TLS 1.2 must be refused")
	}
	// Plaintext refused.
	c, _ := net.Dial("tcp", strings.TrimPrefix(base, "https://"))
	_, _ = c.Write([]byte("GET /ping HTTP/1.1\r\nHost: x\r\n\r\n"))
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 16)
	n, _ := c.Read(buf)
	c.Close()
	if n > 0 && buf[0] != 0x15 {
		t.Fatalf("plaintext answered: %q", buf[:n])
	}
	// Dev certificate is refused in production.
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "auth")
	if _, err := NewServer(rt, Config{Addr: "127.0.0.1:0", Env: "production"}); err == nil {
		t.Fatal("production without a certificate must fail")
	}
	if rt.Logs.Count("reason", "local_dev") != 0 {
		t.Fatal("no dev-cert event expected for the refused server")
	}
}

func TestServerCertFilesAndHotReload(t *testing.T) {
	ca := testutil.MustCA("example.org")
	dir := t.TempDir()
	first := ca.MustIssue("edge", testutil.IssueOptions{})
	certPath, keyPath := filepath.Join(dir, "edge.pem"), filepath.Join(dir, "edge.key")
	_ = os.WriteFile(certPath, testutil.CertPEM(first), 0o600)
	_ = os.WriteFile(keyPath, testutil.KeyPEM(first), 0o600)
	rt := testrt.New(t, ca, "auth")
	srv, err := NewServer(rt, Config{Addr: "127.0.0.1:0", Env: "production", CertFile: certPath, KeyFile: keyPath, ReloadInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()
	serial := func() string {
		c, err := tls.Dial("tcp", ep.Host, &tls.Config{RootCAs: ca.Pool(), ServerName: "edge", MinVersion: tls.VersionTLS13, InsecureSkipVerify: true}) //nolint:gosec // serial probe
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		return c.ConnectionState().PeerCertificates[0].SerialNumber.String()
	}
	s1 := serial()
	second := ca.MustIssue("edge", testutil.IssueOptions{})
	time.Sleep(30 * time.Millisecond)
	_ = os.WriteFile(certPath, testutil.CertPEM(second), 0o600)
	_ = os.WriteFile(keyPath, testutil.KeyPEM(second), 0o600)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if serial() != s1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("certificate not reloaded")
}

func TestSecurityHeaders(t *testing.T) {
	srv, client, base := devServer(t, Config{})
	srv.HandleFunc("/api/v1/x", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "{}") })
	srv.HandleFunc("/console/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<script nonce=\""+Nonce(r.Context())+"\"></script>")
	})
	for _, path := range []string{"/api/v1/x", "/console/", "/nope"} {
		resp, err := client.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		h := resp.Header
		want := map[string]string{
			"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
			"X-Content-Type-Options":    "nosniff",
			"Referrer-Policy":           "no-referrer",
			"X-Frame-Options":           "DENY",
		}
		for k, v := range want {
			if h.Get(k) != v {
				t.Errorf("%s: header %s=%q want %q", path, k, h.Get(k), v)
			}
		}
		csp := h.Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "object-src 'none'") {
			t.Errorf("%s: csp %q", path, csp)
		}
		if h.Get("Permissions-Policy") == "" {
			t.Errorf("%s: permissions policy missing", path)
		}
		if strings.HasPrefix(path, "/api/") && h.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: cache-control %q", path, h.Get("Cache-Control"))
		}
		if path == "/console/" {
			nonce := strings.TrimSuffix(strings.TrimPrefix(string(body), "<script nonce=\""), "\"></script>")
			if len(nonce) < 16 || !strings.Contains(csp, "'nonce-"+nonce+"'") {
				t.Errorf("nonce not reflected in CSP: body %q csp %q", body, csp)
			}
		}
	}
}

func TestCSRFMatrix(t *testing.T) {
	srv, client, base := devServer(t, Config{AllowedOrigins: []string{"https://console.example.org"}})
	srv.HandleFunc("/api/v1/change", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	srv.HandleFunc("/api/v1/read", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	token := "0123456789abcdef0123456789abcdef0123456789ab"
	do := func(method, path string, cookie, header, origin, fetchSite string) int {
		req, _ := http.NewRequest(method, base+path, strings.NewReader("{}"))
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: cookie})
		}
		if header != "" {
			req.Header.Set(CSRFHeader, header)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 403 && !strings.Contains(string(body), `"reason":"csrf"`) {
			t.Fatalf("403 body %q", body)
		}
		return resp.StatusCode
	}
	good := "https://console.example.org"
	cases := []struct {
		name                         string
		method, path                 string
		cookie, header, origin, site string
		want                         int
	}{
		{"ok", "POST", "/api/v1/change", token, token, good, "same-origin", 204},
		{"safe method exempt", "GET", "/api/v1/read", "", "", "", "", 200},
		{"missing cookie", "POST", "/api/v1/change", "", token, good, "same-origin", 403},
		{"missing header", "POST", "/api/v1/change", token, "", good, "same-origin", 403},
		{"mismatch", "POST", "/api/v1/change", token, "x" + token[1:], good, "same-origin", 403},
		{"bad origin", "POST", "/api/v1/change", token, token, "https://evil.example", "same-origin", 403},
		{"cross-site", "POST", "/api/v1/change", token, token, good, "cross-site", 403},
		{"no origin no fetch-site", "POST", "/api/v1/change", token, token, "", "", 403},
		{"put", "PUT", "/api/v1/change", token, token, good, "", 204},
	}
	for _, c := range cases {
		if got := do(c.method, c.path, c.cookie, c.header, c.origin, c.site); got != c.want {
			t.Errorf("%s: status %d want %d", c.name, got, c.want)
		}
	}
	if n := lastRT.Logs.Count("attr_detail", "missing_cookie"); n == 0 {
		t.Fatal("csrf refusals must be audited")
	}
	// Issuing a CSRF cookie.
	rec := newRecorder()
	IssueCSRFCookie(rec)
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].Name != CSRFCookie || !c[0].Secure || c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode || len(c[0].Value) < 32 {
		t.Fatalf("csrf cookie %+v", c)
	}
}

func TestRateLimit(t *testing.T) {
	srv, client, base := devServer(t, Config{RateLimit: RateLimit{PerSecond: 5, Burst: 5, Routes: map[string]RateLimit{"/api/v1/signin": {PerSecond: 1, Burst: 2}}}})
	srv.HandleFunc("/api/v1/read", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	srv.HandleFunc("/api/v1/signin", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	count := func(path string, n int) (ok, limited int) {
		for i := 0; i < n; i++ {
			resp, err := client.Get(base + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch resp.StatusCode {
			case 200:
				ok++
			case 429:
				limited++
				if !strings.Contains(string(body), "rate_limited") {
					t.Fatalf("429 body %q", body)
				}
			}
		}
		return
	}
	if ok, limited := count("/api/v1/read", 10); ok != 5 || limited != 5 {
		t.Fatalf("global: ok=%d limited=%d", ok, limited)
	}
	if ok, limited := count("/api/v1/signin", 5); ok != 2 || limited != 3 {
		t.Fatalf("route: ok=%d limited=%d", ok, limited)
	}
	if lastRT.Logs.Count("type", "limit_exceeded") == 0 {
		t.Fatal("rate limit refusals must be audited as limit_exceeded")
	}
	// X-Forwarded-For is ignored from untrusted sources: same bucket.
	req, _ := http.NewRequest("GET", base+"/api/v1/read", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, _ := client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Fatalf("spoofed XFF must not reset the bucket: %d", resp.StatusCode)
	}
	req.RemoteAddr = "127.0.0.1:1234"
	if got := clientIP(req, nil); got != "127.0.0.1" {
		t.Fatalf("clientIP without trusted proxies = %q", got)
	}
	_, cidr, _ := net.ParseCIDR("127.0.0.0/8")
	if got := clientIP(req, []*net.IPNet{cidr}); got != "203.0.113.9" {
		t.Fatalf("clientIP with trusted proxy = %q", got)
	}
}

func TestOptionsAndLimits(t *testing.T) {
	srv, client, base := devServer(t, Config{}, WithMiddleware())
	srv.HandleFunc("/api/v1/echo", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = w.Write(b)
	})
	req, _ := http.NewRequest("POST", base+"/api/v1/echo", strings.NewReader(strings.Repeat("b", 2<<20)))
	token := strings.Repeat("c", 43)
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: token})
	req.Header.Set(CSRFHeader, token)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("body limit: %d", resp.StatusCode)
	}
	if lastRT.Logs.Count("attr_limit", "request_body") == 0 {
		t.Fatal("body overflow must be audited")
	}
	if _, err := NewServer(nil, Config{}); err == nil {
		t.Fatal("nil runtime must fail")
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = os.Getenv
}
