package integration

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRotationProviderDown(t *testing.T) {
	const ttl = 3 * time.Second
	wlInv := testutil.StartWorkloadAPI(t, "example.org", "inventory", ttl)
	wlOrd := testutil.StartWorkloadAPI(t, "example.org", "orders", time.Minute)
	wlOrd.ShareCA(wlInv)
	logs := &testutil.LogCapture{}
	invCfg := spiffeConfig("inventory", wlInv.Addr())
	invCfg.Admin.Addr = "127.0.0.1:0"
	inv, err := freya.New(invCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	defer inv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = inv.Run(ctx) }()
	ep, _ := inv.GRPC().Endpoint()
	adminURL := inv.AdminURL()
	ordCfg := spiffeConfig("orders", wlOrd.Addr())
	ordCfg.Discovery.Static = map[string][]string{"inventory": {ep.Host}}
	ord, err := freya.New(ordCfg, freya.WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if err := health(ctx, conn); err != nil {
		t.Fatalf("before outage: %v", err)
	}
	ready := func() int {
		resp, err := http.Get(adminURL + "/readyz")
		if err != nil {
			return -1
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if ready() != 200 {
		t.Fatal("expected ready before outage")
	}
	// Kill the identity provider; after the current SVID expires the service
	// must go unready and refuse calls, never fall back to plaintext.
	wlInv.Stop()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && ready() == 200 {
		time.Sleep(100 * time.Millisecond)
	}
	if ready() == 200 {
		t.Fatal("service stayed ready after its identity expired")
	}
	if logs.Count("type", "identity_expired") == 0 {
		t.Fatalf("expected identity_expired: %s", logs.String())
	}
	// New inbound handshakes are refused (server has no valid credential).
	raw, err := tls.Dial("tcp", ep.Host, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}) //nolint:gosec // probing only
	if err == nil {
		_ = raw.SetReadDeadline(time.Now().Add(time.Second))
		if _, rerr := raw.Read(make([]byte, 1)); rerr == nil {
			t.Fatal("server accepted a handshake without a valid identity")
		}
		raw.Close()
	}
	// The listener never speaks plaintext.
	c, err := net.Dial("tcp", ep.Host)
	if err == nil {
		_, _ = c.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		if n > 0 && buf[0] != 0x15 { // 0x15 = TLS alert record
			t.Fatalf("plaintext response from listener: %q", buf[:n])
		}
		c.Close()
	}
	if err := health(ctx, conn); status.Code(err) == codes.OK {
		t.Fatal("outbound calls must fail once the identity is gone")
	}
	// Recovery: provider returns, identity renews, service becomes ready again.
	wlInv.Start()
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && ready() != 200 {
		time.Sleep(200 * time.Millisecond)
	}
	if ready() != 200 {
		t.Fatalf("service did not recover: %s", logs.String())
	}
}
