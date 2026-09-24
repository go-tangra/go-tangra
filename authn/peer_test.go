package authn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"testing"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func grpcCtx(leaf *x509.Certificate) context.Context {
	st := tls.ConnectionState{}
	if leaf != nil {
		st.PeerCertificates = []*x509.Certificate{leaf}
	}
	return peer.NewContext(context.Background(), &peer.Peer{
		Addr:     &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1234},
		AuthInfo: credentials.TLSInfo{State: st},
	})
}

func httpCtx(leaf *x509.Certificate) context.Context {
	r, _ := http.NewRequest(http.MethodGet, "https://x/y", nil)
	r.RemoteAddr = "10.0.0.2:4321"
	if leaf != nil {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
	}
	return transport.NewServerContext(context.Background(), khttpTransport(r))
}

func TestFromContextRoundTrip(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("empty context must not carry a peer")
	}
	p := PeerIdentity{ID: identity.ForService("example.org", "orders"), ServiceName: "orders", Serial: "1"}
	got, ok := FromContext(WithPeer(context.Background(), p))
	if !ok || got.ID != p.ID {
		t.Fatalf("round trip: %+v %v", got, ok)
	}
}

func TestMiddlewareExtractsVerifiedPeer(t *testing.T) {
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	em := audit.NewEmitter(nil, 0)
	mw := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory", Audit: em})
	var seen PeerIdentity
	h := mw(func(ctx context.Context, req any) (any, error) {
		p, ok := FromContext(ctx)
		if !ok {
			t.Fatal("peer missing in handler")
		}
		seen = p
		return "ok", nil
	})
	for name, ctx := range map[string]context.Context{"grpc": grpcCtx(crt.Leaf), "http": httpCtx(crt.Leaf)} {
		t.Run(name, func(t *testing.T) {
			out, err := h(ctx, nil)
			if err != nil || out != "ok" {
				t.Fatalf("handler: %v %v", out, err)
			}
			if seen.ID.String() != "spiffe://example.org/svc/orders" || seen.ServiceName != "orders" ||
				seen.Serial != crt.Leaf.SerialNumber.String() || time.Since(seen.VerifiedAt) > time.Minute {
				t.Fatalf("peer %+v", seen)
			}
		})
	}
}

func TestMiddlewareRefusesWithoutPeer(t *testing.T) {
	ca := testutil.MustCA("example.org")
	evil := testutil.MustCA("evil.org")
	em := audit.NewEmitter(nil, 0)
	mw := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory", Audit: em})
	called := false
	h := mw(func(ctx context.Context, req any) (any, error) { called = true; return nil, nil })
	cases := map[string]context.Context{
		"no transport":   context.Background(),
		"grpc no tls":    peer.NewContext(context.Background(), &peer.Peer{Addr: &net.TCPAddr{}}),
		"grpc no cert":   grpcCtx(nil),
		"http no tls":    httpCtx(nil),
		"foreign domain": grpcCtx(evil.MustIssue("orders", testutil.IssueOptions{}).Leaf),
		"no SAN":         grpcCtx(ca.MustIssue("orders", testutil.IssueOptions{NoSAN: true}).Leaf),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h(ctx, nil)
			if err == nil || called {
				t.Fatal("handler must not run without a verified peer")
			}
			if !kerrors.IsUnauthorized(err) {
				t.Fatalf("want Unauthorized, got %v", err)
			}
		})
	}
}
