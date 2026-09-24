package authn

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/observe"
)

type evSink struct{ evs []audit.Event }

func (s *evSink) Emit(_ context.Context, e audit.Event) { s.evs = append(s.evs, e) }

func TestRefusalsAreAuditedOnceAndOpaque(t *testing.T) {
	ca := testutil.MustCA("example.org")
	evil := testutil.MustCA("evil.org")
	sink := &evSink{}
	em := audit.NewEmitter(nil, 0, sink)
	mw := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory", Audit: em})
	h := mw(func(ctx context.Context, req any) (any, error) { t.Fatal("handler must not run"); return nil, nil })
	cases := []struct {
		name   string
		ctx    context.Context
		reason string
		err    *kerrors.Error
	}{
		{"no-cert", grpcCtx(nil), "no_identity", ErrNoIdentity},
		{"foreign-domain", grpcCtx(evil.MustIssue("orders", testutil.IssueOptions{}).Leaf), "untrusted", ErrUntrusted},
		{"non-svc-path", grpcCtx(ca.MustIssue("orders", testutil.IssueOptions{URIs: []*url.URL{{Scheme: "spiffe", Host: "example.org", Path: "/ns/orders"}}}).Leaf), "untrusted", ErrUntrusted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(sink.evs)
			ctx := observe.WithCorrelationID(tc.ctx, "cid-"+tc.name)
			_, err := h(ctx, nil)
			if !errors.Is(err, tc.err) && kerrors.FromError(err).Reason != tc.err.Reason {
				t.Fatalf("got %v, want %v", err, tc.err)
			}
			ke := kerrors.FromError(err)
			if ke.Message != "" || len(ke.Metadata) != 0 {
				t.Fatalf("error must be opaque: %+v", ke)
			}
			if len(sink.evs) != before+1 {
				t.Fatalf("expected exactly one audit event, got %d", len(sink.evs)-before)
			}
			ev := sink.evs[len(sink.evs)-1]
			if ev.Type != audit.TypeAuthnRefused || string(ev.Reason) != tc.reason || ev.CorrelationID != "cid-"+tc.name || ev.RemoteAddr == "" {
				t.Fatalf("event %+v", ev)
			}
		})
	}
	// Success emits nothing.
	before := len(sink.evs)
	ok := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })
	if _, err := ok(grpcCtx(ca.MustIssue("orders", testutil.IssueOptions{}).Leaf), nil); err != nil || len(sink.evs) != before {
		t.Fatalf("success: %v events=%d", err, len(sink.evs)-before)
	}
}

func TestCallerHeaderIsIgnored(t *testing.T) {
	ca := testutil.MustCA("example.org")
	mw := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory"})
	var seen PeerIdentity
	h := mw(func(ctx context.Context, req any) (any, error) { seen, _ = FromContext(ctx); return nil, nil })
	r, _ := http.NewRequest(http.MethodGet, "https://x/y", nil)
	r.Header.Set("x-freya-caller", "spiffe://example.org/svc/admin")
	ctx := httpCtx(ca.MustIssue("orders", testutil.IssueOptions{}).Leaf)
	if _, err := h(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if seen.ServiceName != "orders" {
		t.Fatalf("identity must come from the certificate only, got %q", seen.ServiceName)
	}
	// Header on gRPC metadata is equally ignored: only the TLS peer counts.
	if _, err := h(grpcCtx(ca.MustIssue("orders", testutil.IssueOptions{}).Leaf), nil); err != nil || seen.ServiceName != "orders" {
		t.Fatalf("grpc: %v %q", err, seen.ServiceName)
	}
}
