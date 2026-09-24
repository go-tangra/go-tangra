package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func health(ctx context.Context, conn *grpc.ClientConn) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

func TestPolicy(t *testing.T) {
	f := newFixture(t, "a", "b", "c")
	policy := f.writePolicy("policy.yaml", "version: p1\nrules:\n  - {id: a-b, from: [\"spiffe://example.org/svc/a\"], to: [b], effect: allow}\n")
	logs := &testutil.LogCapture{}
	apps := map[string]*freya.App{}
	addrs := map[string]string{}
	var stops []func()
	defer func() {
		for _, s := range stops {
			s()
		}
	}()
	for _, name := range []string{"b", "c"} {
		cfg := f.config(name)
		cfg.Authz.Source, cfg.Authz.Path = config.AuthzFile, policy
		app, addr, stop := f.startApp(t, cfg, freya.WithLogger(logs.Handler()))
		apps[name], addrs[name] = app, addr
		stops = append(stops, stop)
	}
	// Callers a and b get static discovery to b and c. Callers use allow-all
	// because the policy under test is enforced by the callee.
	for _, name := range []string{"a"} {
		cfg := f.config(name)
		cfg.Discovery.Static = map[string][]string{"b": {addrs["b"]}, "c": {addrs["c"]}}
		app, err := freya.New(cfg, freya.WithAllowAllPolicy())
		if err != nil {
			t.Fatal(err)
		}
		apps[name] = app
		stops = append(stops, app.Close)
	}
	ctx := context.Background()
	connAB, _ := apps["a"].Client(ctx, "b")
	connAC, _ := apps["a"].Client(ctx, "c")
	if err := health(ctx, connAB); err != nil {
		t.Fatalf("a→b must be allowed: %v", err)
	}
	if err := health(ctx, connAC); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a→c must be denied: %v", err)
	}
	// b→a: b has policy p1 (no rule for itself as caller); a has allow-all but is
	// not a server here, so test b→c instead: denied by c's policy.
	bCfg := f.config("b")
	bCfg.Authz.Source, bCfg.Authz.Path = config.AuthzFile, policy
	bCfg.Discovery.Static = map[string][]string{"c": {addrs["c"]}}
	bCaller, err := freya.New(bCfg, freya.WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer bCaller.Close()
	connBC, _ := bCaller.Client(ctx, "c")
	if err := health(ctx, connBC); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("b→c must be denied: %v", err)
	}
	if logs.Count("reason", "no_matching_rule") < 2 {
		t.Fatalf("expected authz_refused events: %s", logs.String())
	}
	// Hot reload: allow a→c; must apply within 30s without restart.
	if err := os.WriteFile(policy, []byte("version: p2\nrules:\n  - {id: a-b, from: [\"spiffe://example.org/svc/a\"], to: [b], effect: allow}\n  - {id: a-c, from: [\"spiffe://example.org/svc/a\"], to: [c], effect: allow}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for time.Since(start) < 30*time.Second {
		if err := health(ctx, connAC); err == nil {
			t.Logf("policy applied after %s", time.Since(start))
			if logs.Count("policy_version", "p2") == 0 {
				t.Fatalf("expected policy_loaded p2: %s", logs.String())
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("policy change not applied within 30s")
}
