package freya

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func baseConfig(t *testing.T, name string) (config.Config, *testutil.CA) {
	t.Helper()
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue(name, testutil.IssueOptions{})
	c, k, b, err := ca.WriteSVID(t.TempDir(), name, crt)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ServiceName = name
	cfg.TrustDomain = "example.org"
	cfg.Identity.Provider = config.ProviderFile
	cfg.Identity.File = config.FileIdentity{Cert: c, Key: k, Bundle: b}
	cfg.Server.GRPCAddr = "127.0.0.1:0"
	cfg.Admin.Addr = "127.0.0.1:0"
	return cfg, ca
}

func TestNewRefusesBrokenSetups(t *testing.T) {
	cfg, _ := baseConfig(t, "orders")
	cases := []struct {
		name string
		mut  func(*config.Config)
		opts []Option
		want string
	}{
		{"invalid config", func(c *config.Config) { c.ServiceName = "" }, []Option{WithAllowAllPolicy()}, "service_name"},
		{"no policy source", func(*config.Config) {}, nil, "authz.path"},
		{"identity name mismatch", func(c *config.Config) { c.ServiceName = "billing" }, []Option{WithAllowAllPolicy()}, "does not match"},
		{"provider unavailable", func(c *config.Config) { c.Identity.File.Cert = "/nonexistent.pem" }, []Option{WithAllowAllPolicy()}, "identity"},
		{"localdev in production", func(c *config.Config) { c.Env = "production" }, []Option{WithAllowAllPolicy(), WithInsecureLocalDev()}, "production"},
		{"allow-all in production", func(c *config.Config) { c.Env = "production" }, []Option{WithAllowAllPolicy()}, "production"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := cfg
			tc.mut(&c)
			app, err := New(c, tc.opts...)
			if err == nil {
				app.Close()
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestNewAllowAllWarnsAndRuns(t *testing.T) {
	cfg, _ := baseConfig(t, "orders")
	logs := &testutil.LogCapture{}
	app, err := New(cfg, WithAllowAllPolicy(), WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.Identity().ID().String() != "spiffe://example.org/svc/orders" {
		t.Fatalf("identity %v", app.Identity().ID())
	}
	if !app.Ready() {
		t.Fatal("app with valid identity and allow-all policy must be ready")
	}
	out := logs.String()
	if !strings.Contains(out, "allow-all") || logs.Count("type", "insecure_mode_enabled") != 1 {
		t.Fatalf("expected WARN + insecure_mode_enabled event, got %s", out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop on ctx cancel")
	}
}

func TestLocalDevWarns(t *testing.T) {
	cfg, _ := baseConfig(t, "orders")
	cfg.Identity = config.Default().Identity // no files: localdev supplies identity
	logs := &testutil.LogCapture{}
	app, err := New(cfg, WithAllowAllPolicy(), WithInsecureLocalDev(), WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.Identity().ID().ServiceName() != "orders" {
		t.Fatalf("identity %v", app.Identity().ID())
	}
	if logs.Count("type", "insecure_mode_enabled") != 2 || !strings.Contains(logs.String(), "local-dev") {
		t.Fatalf("expected two insecure_mode_enabled events (local_dev, allow_all): %s", logs.String())
	}
}

func TestClientRoundTrip(t *testing.T) {
	invCfg, ca := baseConfig(t, "inventory")
	inv, err := New(invCfg, WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer inv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = inv.Run(ctx) }()
	ep := waitEndpoint(t, inv)

	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	c, k, b, _ := ca.WriteSVID(t.TempDir(), "orders", crt)
	ordCfg := invCfg
	ordCfg.ServiceName = "orders"
	ordCfg.Identity.File = config.FileIdentity{Cert: c, Key: k, Bundle: b}
	ordCfg.Discovery.Static = map[string][]string{"inventory": {ep}}
	ord, err := New(ordCfg, WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if err := healthCheck(ctx, conn); err != nil {
		t.Fatalf("secure call failed: %v", err)
	}
}

func TestNoPolicyDeniesAndWarns(t *testing.T) {
	cfg, ca := baseConfig(t, "inventory")
	logs := &testutil.LogCapture{}
	pol := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(pol, []byte("version: empty\nrules: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Authz.Source, cfg.Authz.Path = config.AuthzFile, pol
	inv, err := New(cfg, WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer inv.Close()
	if !inv.Ready() || logs.Count("type", "policy_loaded") != 1 {
		t.Fatalf("policy_loaded expected: %s", logs.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = inv.Run(ctx) }()
	ep := waitEndpoint(t, inv)
	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	c, k, b, _ := ca.WriteSVID(t.TempDir(), "orders", crt)
	ordCfg := cfg
	ordCfg.ServiceName = "orders"
	ordCfg.Identity.File = config.FileIdentity{Cert: c, Key: k, Bundle: b}
	ordCfg.Discovery.Static = map[string][]string{"inventory": {ep}}
	ord, err := New(ordCfg, WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if err := healthCheck(ctx, conn); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("empty policy must deny with PermissionDenied, got %v", err)
	}
	if logs.Count("reason", "no_matching_rule") != 1 {
		t.Fatalf("expected one authz_refused: %s", logs.String())
	}
	// Hot reload: allow orders → inventory health checks.
	allow := "version: v2\nrules:\n  - {id: h, from: [\"spiffe://example.org/svc/orders\"], to: [inventory], effect: allow}\n"
	if err := os.WriteFile(pol, []byte(allow), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := healthCheck(ctx, conn); err == nil {
			if logs.Count("policy_version", "v2") == 0 {
				t.Fatalf("expected policy_loaded v2: %s", logs.String())
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("policy reload not applied within 10s")
}

func TestReadinessFollowsIdentity(t *testing.T) {
	ca := testutil.MustCA("example.org")
	nb := time.Now().Truncate(time.Second)
	short := ca.MustIssue("orders", testutil.IssueOptions{NotBefore: nb.Add(-time.Second), NotAfter: nb.Add(2 * time.Second)})
	prov := testutil.NewMemProvider(ca, short)
	cfg := config.Default()
	cfg.ServiceName, cfg.TrustDomain = "orders", "example.org"
	cfg.Server.GRPCAddr, cfg.Admin.Addr = "127.0.0.1:0", "127.0.0.1:0"
	logs := &testutil.LogCapture{}
	app, err := New(cfg, WithIdentityProvider(prov), WithAllowAllPolicy(), WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if !app.Ready() {
		t.Fatal("ready with a valid identity")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && app.Ready() {
		time.Sleep(20 * time.Millisecond)
	}
	if app.Ready() {
		t.Fatal("must become unready when the identity expires")
	}
	for time.Now().Before(deadline) && logs.Count("type", "identity_expired") == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if logs.Count("type", "identity_expired") != 1 {
		t.Fatalf("expected identity_expired event: %s", logs.String())
	}
	prov.Rotate(ca.MustIssue("orders", testutil.IssueOptions{}))
	for time.Now().Before(deadline) && !app.Ready() {
		time.Sleep(20 * time.Millisecond)
	}
	if !app.Ready() || logs.Count("type", "identity_renewed") == 0 {
		t.Fatalf("must recover after renewal: %s", logs.String())
	}
	if app.IdentityState() != "valid" {
		t.Fatalf("state %s", app.IdentityState())
	}
}
