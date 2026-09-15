package observe

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/transport/tlsconf"
)

func serve(t *testing.T, a *Admin) {
	t.Helper()
	go func() { _ = a.Serve() }()
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestAdminEndpoints(t *testing.T) {
	ready := false
	m, _ := NewMetrics()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	a, err := NewAdmin(config.Admin{Addr: "127.0.0.1:0"}, func() bool { return ready }, m.Handler(), log, nil)
	if err != nil {
		t.Fatal(err)
	}
	serve(t, a)
	if !strings.HasPrefix(a.URL(), "http://127.0.0.1:") {
		t.Fatalf("url %q", a.URL())
	}
	if code, body := get(t, a.URL()+"/healthz"); code != 200 || body != "ok\n" {
		t.Fatalf("healthz %d %q", code, body)
	}
	if code, _ := get(t, a.URL()+"/readyz"); code != 503 {
		t.Fatalf("readyz before ready: %d", code)
	}
	ready = true
	if code, _ := get(t, a.URL()+"/readyz"); code != 200 {
		t.Fatalf("readyz after ready: %d", code)
	}
	if code, body := get(t, a.URL()+"/metrics"); code != 200 || !strings.Contains(body, "freya_") {
		t.Fatalf("metrics %d %q", code, body)
	}
	if code, _ := get(t, a.URL()+"/debug/pprof/"); code != 404 {
		t.Fatalf("pprof must be off by default: %d", code)
	}
	if strings.Contains(buf.String(), "WARN") {
		t.Fatalf("no warnings expected for loopback defaults: %s", buf.String())
	}
}

func TestAdminPprofAndNonLoopback(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	if _, err := NewAdmin(config.Admin{Addr: "0.0.0.0:0"}, nil, nil, log, nil); err == nil {
		t.Fatal("non-loopback without opt-in must be refused")
	}
	// Opted in but without an mTLS config: still refused (never plaintext off-loopback).
	if _, err := NewAdmin(config.Admin{Addr: "0.0.0.0:0", AllowNonLoopback: true}, nil, nil, log, nil); err == nil {
		t.Fatal("non-loopback without TLS must be refused")
	}
	ca := testutil.MustCA("example.org")
	srvP := testutil.NewMemProvider(ca, ca.MustIssue("inventory", testutil.IssueOptions{}))
	tlsCfg, err := tlsconf.ServerConfig(srvP, tlsconf.Options{TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAdmin(config.Admin{Addr: "0.0.0.0:0", AllowNonLoopback: true, EnablePprof: true}, nil, nil, log, tlsCfg)
	if err != nil {
		t.Fatal(err)
	}
	serve(t, a)
	if !a.Secure() || !strings.HasPrefix(a.URL(), "https://") {
		t.Fatalf("non-loopback admin must be mTLS: %q", a.URL())
	}
	// Plain HTTP gets net/http's fixed "HTTPS server" 400 and never any admin
	// content; TLS without a client certificate is refused at the handshake.
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get("http://" + strings.TrimPrefix(a.URL(), "https://") + "/healthz")
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || strings.Contains(string(body), "ok") {
			t.Fatalf("plaintext request served admin content: %d %q", resp.StatusCode, body)
		}
	}
	anon := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}}} //nolint:gosec // probing only
	if _, err := anon.Get(a.URL() + "/healthz"); err == nil {
		t.Fatal("a client without identity must be refused")
	}
	// An mTLS client with a SPIFFE identity is served.
	cliP := testutil.NewMemProvider(ca, ca.MustIssue("scraper", testutil.IssueOptions{}))
	ccfg, err := tlsconf.ClientConfig(cliP, identity.ForService("example.org", "inventory"), tlsconf.Options{TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	mtls := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: ccfg, ForceAttemptHTTP2: true}}
	for path, want := range map[string]int{"/debug/pprof/": 200, "/readyz": 503, "/metrics": 404} {
		resp, err := mtls.Get(a.URL() + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s: status %d want %d", path, resp.StatusCode, want)
		}
	}
	if !strings.Contains(buf.String(), "pprof enabled") || !strings.Contains(buf.String(), "mTLS only") {
		t.Fatalf("expected warnings: %s", buf.String())
	}
	if _, err := NewAdmin(config.Admin{Addr: "nonsense"}, nil, nil, log, nil); err == nil {
		t.Fatal("bad addr must fail")
	}
}
