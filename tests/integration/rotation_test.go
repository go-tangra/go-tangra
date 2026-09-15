package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/internal/testutil"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func spiffeConfig(name, socket string) config.Config {
	cfg := config.Default()
	cfg.ServiceName, cfg.TrustDomain, cfg.Env = name, "example.org", "test"
	cfg.Identity.Provider = config.ProviderSPIFFE
	cfg.Identity.WorkloadSocket = socket
	cfg.Server.GRPCAddr, cfg.Admin.Addr = "127.0.0.1:0", "127.0.0.1:0"
	return cfg
}

func TestRotation(t *testing.T) {
	const ttl = 3 * time.Second // X.509 validity is second-granular; keep >= 2s effective
	wlInv := testutil.StartWorkloadAPI(t, "example.org", "inventory", ttl)
	wlOrd := testutil.StartWorkloadAPI(t, "example.org", "orders", ttl)
	wlOrd.ShareCA(wlInv)
	invLogs, ordLogs := &testutil.LogCapture{}, &testutil.LogCapture{}
	inv, err := freya.New(spiffeConfig("inventory", wlInv.Addr()), freya.WithAllowAllPolicy(), freya.WithLogger(invLogs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer inv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = inv.Run(ctx) }()
	ep, _ := inv.GRPC().Endpoint()
	ordCfg := spiffeConfig("orders", wlOrd.Addr())
	ordCfg.Discovery.Static = map[string][]string{"inventory": {ep.Host}}
	ord, err := freya.New(ordCfg, freya.WithAllowAllPolicy(), freya.WithLogger(ordLogs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()

	var calls, failures atomic.Int64
	stop := make(chan struct{})
	for w := 0; w < 4; w++ {
		go func() {
			tick := time.NewTicker(20 * time.Millisecond) // 4 workers × 50/s = 200 calls/s
			defer tick.Stop()
			for {
				select {
				case <-stop:
					return
				case <-tick.C:
					cctx, ccancel := context.WithTimeout(ctx, 3*time.Second)
					conn, err := ord.Client(cctx, "inventory")
					if err == nil {
						_, err = grpc_health_v1.NewHealthClient(conn).Check(cctx, &grpc_health_v1.HealthCheckRequest{})
					}
					ccancel()
					calls.Add(1)
					if err != nil {
						failures.Add(1)
						t.Logf("call failed: %v", err)
					}
				}
			}
		}()
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) && (invLogs.Count("type", "identity_renewed") < 10 || ordLogs.Count("type", "identity_renewed") < 10) {
		time.Sleep(200 * time.Millisecond)
	}
	close(stop)
	time.Sleep(100 * time.Millisecond)
	invR, ordR := invLogs.Count("type", "identity_renewed"), ordLogs.Count("type", "identity_renewed")
	t.Logf("calls=%d failures=%d renewals inventory=%d orders=%d", calls.Load(), failures.Load(), invR, ordR)
	if invR < 10 || ordR < 10 {
		t.Fatalf("expected >=10 renewals on each side, got %d/%d", invR, ordR)
	}
	if failures.Load() != 0 {
		t.Fatalf("%d of %d calls failed during rotation", failures.Load(), calls.Load())
	}
	if calls.Load() < 1000 {
		t.Fatalf("load generator too slow: %d calls", calls.Load())
	}
	// Serials changed on every renewal.
	seen := map[string]bool{}
	for _, l := range invLogs.Lines() {
		if l["type"] == "identity_renewed" {
			seen[l["attr_serial"].(string)] = true
		}
	}
	if len(seen) < 10 {
		t.Fatalf("expected distinct serials, got %d", len(seen))
	}
}
