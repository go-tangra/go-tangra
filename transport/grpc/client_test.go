package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya/discovery"
	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"
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
