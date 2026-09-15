package authz

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/observe"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

type evSink struct{ evs []audit.Event }

func (s *evSink) Emit(_ context.Context, e audit.Event) { s.evs = append(s.evs, e) }

type grpcTr struct{ op string }

func (g grpcTr) Kind() transport.Kind            { return transport.KindGRPC }
func (g grpcTr) Endpoint() string                { return "" }
func (g grpcTr) Operation() string               { return g.op }
func (g grpcTr) RequestHeader() transport.Header { return nil }
func (g grpcTr) ReplyHeader() transport.Header   { return nil }

type httpTr struct {
	grpcTr
	r *http.Request
}

func (h httpTr) Request() *http.Request { return h.r }
func (h httpTr) PathTemplate() string   { return "/tpl" }

var _ khttp.Transporter = httpTr{}

func TestMiddlewareDecisions(t *testing.T) {
	p := mustLoad(t, `version: mw
rules:
  - {id: ok, from: ["spiffe://example.org/svc/orders"], to: ["inventory"], operations: ["/inv.V1/Reserve", "GET /items/*"], effect: allow}
`)
	sink := &evSink{}
	em := audit.NewEmitter(nil, 0, sink)
	mw := Middleware(Config{Authorizer: p, LocalID: "spiffe://example.org/svc/inventory", ServiceName: "inventory", Audit: em, SampleAllowed: true})
	ran := false
	h := mw(func(ctx context.Context, req any) (any, error) { ran = true; return "ok", nil })
	peer := authn.PeerIdentity{ID: identity.ForService("example.org", "orders"), ServiceName: "orders"}
	base := observe.WithCorrelationID(authn.WithPeer(context.Background(), peer), "cid-1")

	// gRPC allow.
	if _, err := h(transport.NewServerContext(base, grpcTr{"/inv.V1/Reserve"}), nil); err != nil || !ran {
		t.Fatalf("allow: %v", err)
	}
	if len(sink.evs) != 1 || sink.evs[0].Type != audit.TypeAuthzAllowed || sink.evs[0].RuleID != "ok" {
		t.Fatalf("allowed event: %+v", sink.evs)
	}
	// gRPC deny.
	ran = false
	_, err := h(transport.NewServerContext(base, grpcTr{"/inv.V1/Release"}), nil)
	if !kerrors.IsForbidden(err) || ran {
		t.Fatalf("deny: %v ran=%v", err, ran)
	}
	if kerrors.FromError(err).Reason != "denied" || kerrors.FromError(err).Message != "" {
		t.Fatalf("error must be opaque: %v", err)
	}
	ev := sink.evs[len(sink.evs)-1]
	if ev.Type != audit.TypeAuthzRefused || ev.Reason != "no_matching_rule" || ev.PolicyVersion != "mw" || ev.CorrelationID != "cid-1" || ev.Operation != "/inv.V1/Release" {
		t.Fatalf("refused event: %+v", ev)
	}
	// HTTP operation derivation.
	r, _ := http.NewRequest(http.MethodGet, "https://x/items/42", nil)
	if _, err := h(transport.NewServerContext(base, httpTr{grpcTr{"/tpl"}, r}), nil); err != nil {
		t.Fatalf("http allow: %v", err)
	}
	r2, _ := http.NewRequest(http.MethodPost, "https://x/items/42", nil)
	if _, err := h(transport.NewServerContext(base, httpTr{grpcTr{"/tpl"}, r2}), nil); !kerrors.IsForbidden(err) {
		t.Fatalf("http deny: %v", err)
	}
	// Missing peer → unauthenticated, never authorized.
	if _, err := h(transport.NewServerContext(context.Background(), grpcTr{"/inv.V1/Reserve"}), nil); !kerrors.IsUnauthorized(err) {
		t.Fatalf("no peer: %v", err)
	}
	// Nil authorizer → no_policy deny.
	nilMw := Middleware(Config{LocalID: "spiffe://example.org/svc/inventory", ServiceName: "inventory", Audit: em})
	_, err = nilMw(func(context.Context, any) (any, error) { return nil, nil })(transport.NewServerContext(base, grpcTr{"/x"}), nil)
	if !kerrors.IsForbidden(err) || sink.evs[len(sink.evs)-1].Reason != "no_policy" {
		t.Fatalf("nil authorizer: %v %+v", err, sink.evs[len(sink.evs)-1])
	}
	if Operation(context.Background()) != "" {
		t.Fatal("no transport → empty operation")
	}
	if !strings.HasPrefix(Operation(transport.NewServerContext(base, httpTr{grpcTr{"/tpl"}, nil})), "GET ") {
		t.Fatal("nil request falls back to GET + operation")
	}
}
