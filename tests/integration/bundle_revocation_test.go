package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func TestBundleRevocation(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	logs := &testutil.LogCapture{}
	_, addr, stop := f.startApp(t, f.config("inventory"), freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	defer stop()
	ctx := context.Background()
	dial := func() error {
		p := testutil.NewMemProvider(f.ca, f.ca.MustIssue("orders", testutil.IssueOptions{}))
		cfg, _ := tlsconf.ClientConfig(p, identity.ForService("example.org", "inventory"), tlsconf.Options{TrustDomain: "example.org"})
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
		if err != nil {
			return err
		}
		defer conn.Close()
		return health(ctx, conn)
	}
	if err := dial(); err != nil {
		t.Fatalf("before rotation: %v", err)
	}
	// Replace the trust bundle on disk with a different CA: the old issuer is gone.
	other := testutil.MustCA("example.org")
	if err := os.WriteFile(filepath.Join(f.dir, "ca.pem"), other.BundlePEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := dial(); status.Code(err) == codes.Unavailable {
			if logs.Count("type", "trust_bundle_updated") == 0 {
				t.Fatalf("expected trust_bundle_updated event:\n%s", logs.String())
			}
			if logs.Count("reason", "untrusted") == 0 {
				t.Fatalf("expected untrusted refusal:\n%s", logs.String())
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("bundle change not applied within 10s")
}
