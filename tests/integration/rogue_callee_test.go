package integration

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/internal/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRogueCallee(t *testing.T) {
	f := newFixture(t, "billing", "orders")
	// A server that is legitimately "billing" sits where discovery says "inventory" is.
	_, addr, stop := f.startApp(t, f.config("billing"), freya.WithAllowAllPolicy())
	defer stop()
	logs := &testutil.LogCapture{}
	cfg := f.config("orders")
	cfg.Discovery.Static = map[string][]string{"inventory": {addr}}
	ord, err := freya.New(cfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if err := health(ctx, conn); status.Code(err) != codes.Unavailable {
		t.Fatalf("client must refuse the rogue callee, got %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for logs.Count("reason", "name_mismatch") == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	found := false
	for _, l := range logs.Lines() {
		if l["type"] == "authn_refused" {
			found = true
			if l["reason"] != "name_mismatch" || l["claimed_peer_id"] != "spiffe://example.org/svc/billing" || l["local_id"] != "spiffe://example.org/svc/orders" {
				t.Fatalf("unexpected refusal event: %v", l)
			}
		}
	}
	if !found {
		t.Fatalf("no client-side authn_refused event:\n%s", logs.String())
	}
}
