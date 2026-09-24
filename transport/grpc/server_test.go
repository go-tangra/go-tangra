package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/observe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestServerDefaultsAndChain(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	var order []string
	mark := func(name string) middleware.Middleware {
		return func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req any) (any, error) {
				order = append(order, name)
				return next(ctx, req)
			}
		}
	}
	srv, err := NewServer(rt, WithAddress("127.0.0.1:0"), WithMiddleware(mark("user")), WithTimeout(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if srv.timeout != rt.Limits().RequestTimeout {
		t.Fatalf("timeout must be clamped to limits: %s", srv.timeout)
	}
	if srv.limits.MaxRequestBytes != 1<<20 || srv.limits.MaxHeaderBytes != 8<<10 || srv.limits.MaxConcurrentStreams != 100 {
		t.Fatalf("limits not applied: %+v", srv.limits)
	}
	if srv.reflection {
		t.Fatal("gRPC reflection must be off by default")
	}
	if ep, err := srv.Endpoint(); err != nil || ep.Scheme != "grpcs" {
		t.Fatalf("endpoint must advertise grpcs://, got %v %v", ep, err)
	}
	// Chain: the health service is registered by Kratos; calling it through mTLS
	// exercises correlation → authn → authz → user middleware in order.
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil || resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health: %v %v", resp, err)
	}
	if strings.Join(order, ",") != "user" {
		t.Fatalf("user middleware order: %v", order)
	}
	if rt.Logs.Count("msg", "audit") != 0 {
		t.Fatalf("no audit events expected on success: %s", rt.Logs.String())
	}
}

func TestServerPeerAndCorrelationReachHandler(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	var gotPeer authn.PeerIdentity
	var gotCID string
	srv, _ := NewServer(rt, WithAddress("127.0.0.1:0"), WithMiddleware(func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			gotPeer, _ = authn.FromContext(ctx)
			gotCID = observe.CorrelationID(ctx)
			return next(ctx, req)
		}
	}))
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if gotPeer.ServiceName != "orders" || gotPeer.ID.String() != "spiffe://example.org/svc/orders" {
		t.Fatalf("peer %+v", gotPeer)
	}
	if !observe.ValidCorrelationID(gotCID) {
		t.Fatalf("correlation id %q", gotCID)
	}
}

func TestServerRefusesPlaintextAndUntrusted(t *testing.T) {
	ca := testutil.MustCA("example.org")
	evil := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(rt, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Untrusted caller.
	conn := dial(t, evil, "orders", "inventory", srv)
	defer conn.Close()
	_, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("untrusted caller should fail the handshake, got %v", err)
	}
	ev := rt.Events.Next(t, 2*time.Second)
	if ev.Type != "authn_refused" || ev.Reason != "untrusted" || ev.RemoteAddr == "" {
		t.Fatalf("event %+v", ev)
	}
	// Plaintext client.
	ep, _ := srv.Endpoint()
	plain, err := grpc.NewClient(ep.Host, grpc.WithTransportCredentials(insecureCreds()))
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	_, err = grpc_health_v1.NewHealthClient(plain).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err == nil {
		t.Fatal("plaintext client must be refused")
	}
}

func TestOversizedRequestRefused(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	rt.Lim.MaxRequestBytes = 64
	srv, _ := NewServer(rt, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	_, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: strings.Repeat("x", 200)})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("want ResourceExhausted, got %v", err)
	}
}

func TestExpiredLocalIdentityRefusesCalls(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(rt, WithAddress("127.0.0.1:0"))
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	// The server's own identity expires while the connection stays open.
	rt.Prov.Rotate(ca.MustIssue("inventory", testutil.IssueOptions{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}))
	_, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expired local identity must refuse calls with Unavailable, got %v", err)
	}
	// Outbound is refused too.
	pool := NewPool(rt, nil)
	if _, err := pool.Conn(context.Background(), "orders"); err == nil {
		t.Fatal("pool must refuse to dial without a valid local identity")
	}
	rt.Prov.Rotate(ca.MustIssue("inventory", testutil.IssueOptions{}))
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("after renewal calls must succeed: %v", err)
	}
}
