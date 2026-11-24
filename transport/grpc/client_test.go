package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/discovery"
	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestPoolReusesAndVerifiesName(t *testing.T) {
	ca := testutil.MustCA("example.org")
	inv := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(inv, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()

	orders := testrt.New(t, ca, "orders")
	disc, _ := discovery.NewStatic(map[string][]string{"inventory": {ep.Host}, "billing": {ep.Host}})
	pool := NewPool(orders, disc)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c1, err := pool.Conn(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := pool.Conn(ctx, "inventory")
	if c1 != c2 {
		t.Fatal("connection must be reused per callee")
	}
	if _, err := grpc_health_v1.NewHealthClient(c1).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("call via pool: %v", err)
	}
	// Same address, wrong logical name → identity mismatch, never address trust.
	c3, err := pool.Conn(ctx, "billing")
	if err != nil {
		t.Fatal(err)
	}
	_, err = grpc_health_v1.NewHealthClient(c3).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("rogue callee must be refused by the client, got %v", err)
	}
	if _, err := pool.Conn(ctx, "unknown"); err == nil {
		t.Fatal("unknown service must fail")
	}
	if _, err := pool.Conn(ctx, "Bad Name"); err == nil {
		t.Fatal("invalid service name must fail")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Conn(ctx, "inventory"); err == nil {
		t.Fatal("Conn after Close must fail")
	}
}

func TestPoolPropagatesCorrelation(t *testing.T) {
	ca := testutil.MustCA("example.org")
	inv := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(inv, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()
	orders := testrt.New(t, ca, "orders")
	disc, _ := discovery.NewStatic(map[string][]string{"inventory": {ep.Host}})
	pool := NewPool(orders, disc)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pool.Conn(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	var hdr metadataCarrier
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(withCID(ctx, "trace-me-1"), &grpc_health_v1.HealthCheckRequest{}, hdr.opt()); err != nil {
		t.Fatal(err)
	}
	if got := hdr.get("x-request-id"); got != "trace-me-1" {
		t.Fatalf("reply header x-request-id = %q", got)
	}
}

// TestPoolConnSurvivesLeafRenewal is the core rotation-safety guarantee that
// freya.go relies on: when the local leaf is renewed, tlsconf serves the new
// cert per-handshake, so an already-established pooled connection keeps working
// WITHOUT draining the pool. onIdentityEvent must not call Rotate on renewal.
func TestPoolConnSurvivesLeafRenewal(t *testing.T) {
	ca := testutil.MustCA("example.org")
	inv := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(inv, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()

	orders := testrt.New(t, ca, "orders")
	disc, _ := discovery.NewStatic(map[string][]string{"inventory": {ep.Host}})
	pool := NewPool(orders, disc)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c1, err := pool.Conn(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := grpc_health_v1.NewHealthClient(c1).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("establish call: %v", err)
	}

	// Renew the client's own leaf (a new valid SVID under the same root).
	orders.Prov.Rotate(ca.MustIssue("orders", testutil.IssueOptions{}))

	// The pool must still hand back the SAME connection (not drained)...
	c2, err := pool.Conn(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Fatal("leaf renewal must not replace the pooled connection")
	}
	// ...and it must still work end to end after the renewal.
	if _, err := grpc_health_v1.NewHealthClient(c2).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("call after leaf renewal must succeed on the surviving conn: %v", err)
	}
}

// rotatePool starts inventory, dials it through a pool owned by orders with a
// short grace, and makes one call so a network connection is open.
func rotatePool(t *testing.T) (*testrt.Runtime, *Pool, *grpc.ClientConn) {
	t.Helper()
	ca := testutil.MustCA("example.org")
	inv := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(inv, WithAddress("127.0.0.1:0"))
	t.Cleanup(testrt.StartServer(t, srv))
	ep, _ := srv.Endpoint()

	orders := testrt.New(t, ca, "orders")
	orders.Lim.RequestTimeout = 200 * time.Millisecond
	disc, _ := discovery.NewStatic(map[string][]string{"inventory": {ep.Host}})
	pool := NewPool(orders, disc)
	t.Cleanup(func() { _ = pool.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pool.Conn(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("establish call: %v", err)
	}
	return orders, pool, conn
}

// checkUntil calls Check on conn until want(err) holds or the deadline passes.
func checkUntil(t *testing.T, conn *grpc.ClientConn, want func(error) bool) error {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		cancel()
		if want(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestPoolRotateKeepsConnAndReconnects is the trust-bundle change path. The
// *grpc.ClientConn a caller already holds (gateway registration, the token
// verifier) must keep working: Rotate drops the network connection under it
// and the same ClientConn reconnects with a fresh, re-verified handshake.
// Closing the ClientConn left such callers failing with "the client
// connection is closing" until the process restarted.
func TestPoolRotateKeepsConnAndReconnects(t *testing.T) {
	_, pool, c1 := rotatePool(t)
	before := pool.links.snapshot()
	if len(before) != 1 {
		t.Fatalf("want 1 open link, got %d", len(before))
	}

	pool.Rotate()
	time.Sleep(400 * time.Millisecond) // past the grace

	c2, err := pool.Conn(context.Background(), "inventory")
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Fatal("Rotate must keep the pooled ClientConn: callers hold it")
	}
	if err := checkUntil(t, c1, func(err error) bool { return err == nil }); err != nil {
		t.Fatalf("held conn must work again after Rotate: %v", err)
	}
	after := pool.links.snapshot()
	if len(after) != 1 || after[0] == before[0] {
		t.Fatal("Rotate must replace the network connection (a fresh handshake)")
	}
}

// TestPoolRotateRefusesRootRotatedOut proves the reconnect is verified against
// the new bundle: once the peer's root is no longer trusted, the held conn is
// refused instead of riding the old handshake.
func TestPoolRotateRefusesRootRotatedOut(t *testing.T) {
	orders, pool, c1 := rotatePool(t)
	orders.Prov.SetRoots(testutil.MustCA("example.org").Cert)

	pool.Rotate()
	time.Sleep(400 * time.Millisecond)

	err := checkUntil(t, c1, func(err error) bool { return status.Code(err) == codes.Unavailable })
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("peer under a rotated-out root must be refused, got %v", err)
	}
}
