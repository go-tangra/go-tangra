package integration

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
)

// secretMarkers returns strings that must never appear in any output.
func secretMarkers(f *fixture, crts ...tls.Certificate) []string {
	m := []string{"-----BEGIN", "PRIVATE KEY", "Bearer "}
	for _, c := range crts {
		if fp := testutil.KeyFingerprint(c); fp != "" {
			m = append(m, fp)
		}
	}
	for _, name := range []string{"inventory", "orders"} {
		if b, err := os.ReadFile(filepath.Join(f.dir, name+".key")); err == nil {
			// The base64 body of the key file, minus PEM armor.
			body := strings.ReplaceAll(strings.ReplaceAll(string(b), "-----BEGIN PRIVATE KEY-----", ""), "-----END PRIVATE KEY-----", "")
			for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
				if len(line) > 20 {
					m = append(m, line)
				}
			}
		}
	}
	return m
}

func TestRedactionScan(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	logs := &testutil.LogCapture{}
	invCfg := f.config("inventory")
	invCfg.Server.HTTPAddr = "127.0.0.1:0"
	inv, addr, stop := f.startApp(t, invCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	defer stop()
	inv.HTTP().HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal detail with -----BEGIN PRIVATE KEY----- leak", 500)
	})
	httpEp, _ := inv.HTTP().Endpoint()

	// Exercise: valid call, refused handshakes, HTTP error bodies, admin endpoints.
	ordCfg := f.config("orders")
	ordCfg.Discovery.Static = map[string][]string{"inventory": {addr}}
	ord, err := freya.New(ordCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	ctx := context.Background()
	conn, _ := ord.Client(ctx, "inventory")
	_ = health(ctx, conn)
	evil := testutil.MustCA("example.org")
	evilCrt := evil.MustIssue("orders", testutil.IssueOptions{})
	p := testutil.NewMemProvider(evil, evilCrt)
	cfg, _ := tlsconf.ClientConfig(p, identity.ForService("example.org", "inventory"), tlsconf.Options{TrustDomain: "example.org"})
	if c, err := tls.Dial("tcp", addr, cfg); err == nil {
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		_, _ = c.Read(make([]byte, 1))
		c.Close()
	}
	var bodies strings.Builder
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}, Timeout: 3 * time.Second}
	if resp, err := client.Get("https://" + httpEp.Host + "/boom"); err == nil {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodies.Write(b)
	}
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		_, b := httpGet(t, inv.AdminURL()+path)
		bodies.WriteString(b)
	}
	time.Sleep(200 * time.Millisecond)

	output := logs.String() + bodies.String()
	if len(output) < 500 {
		t.Fatalf("captured too little output: %d bytes", len(output))
	}
	for _, marker := range secretMarkers(f, evilCrt) {
		if strings.Contains(output, marker) {
			t.Fatalf("secret material leaked into output: %.40q...", marker)
		}
	}
	// Serials and SPIFFE IDs are fine to log; private material is not.
	if !strings.Contains(output, "spiffe://example.org/svc/inventory") {
		t.Fatal("sanity: expected identities in logs")
	}
	if dir := os.Getenv("FREYA_CAPTURE_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "redaction_scan.log"), []byte(output), 0o600)
	}
}
