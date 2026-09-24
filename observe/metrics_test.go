package observe_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/observe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type hdr map[string][]string

func (h hdr) Get(k string) string {
	if v := h[strings.ToLower(k)]; len(v) > 0 {
		return v[0]
	}
	return ""
}
func (h hdr) Set(k, v string) { h[strings.ToLower(k)] = []string{v} }
func (h hdr) Add(k, v string) { h[strings.ToLower(k)] = append(h[strings.ToLower(k)], v) }
func (h hdr) Keys() []string {
	out := []string{}
	for k := range h {
		out = append(out, k)
	}
	return out
}
func (h hdr) Values(k string) []string { return h[strings.ToLower(k)] }

type tr struct {
	req, rep hdr
	kind     transport.Kind
}

func (t tr) Kind() transport.Kind            { return t.kind }
func (t tr) Endpoint() string                { return "grpc://x" }
func (t tr) Operation() string               { return "/a.B/C" }
func (t tr) RequestHeader() transport.Header { return t.req }
func (t tr) ReplyHeader() transport.Header   { return t.rep }

func peerFn(ctx context.Context) (string, string, bool) {
	p, ok := authn.FromContext(ctx)
	return p.ServiceName, p.ID.String(), ok
}

func TestMetricsInstrumentsAndExposition(t *testing.T) {
	m, err := observe.NewMetrics()
	if err != nil {
		t.Fatal(err)
	}
	mw := observe.Instrument(observe.InstrumentConfig{Metrics: m, ServiceName: "inventory", Peer: peerFn})
	peer := authn.PeerIdentity{ID: identity.ForService("example.org", "orders"), ServiceName: "orders"}
	call := func(h func(context.Context, any) (any, error), withPeer bool) {
		ctx := transport.NewServerContext(context.Background(), tr{req: hdr{}, rep: hdr{}, kind: transport.KindGRPC})
		_, _ = mw(func(ctx context.Context, req any) (any, error) {
			if withPeer {
				ctx = authn.WithPeer(ctx, peer)
				observe.RecordPeerContext(ctx)
			}
			return h(ctx, req)
		})(ctx, nil)
	}
	call(func(ctx context.Context, _ any) (any, error) { return "ok", nil }, true)
	call(func(ctx context.Context, _ any) (any, error) { return nil, kerrors.Unauthorized("untrusted", "") }, false)
	call(func(ctx context.Context, _ any) (any, error) { return nil, kerrors.Forbidden("denied", "") }, true)
	call(func(ctx context.Context, _ any) (any, error) { return nil, errors.New("boom") }, true)
	m.IdentityRenewal("ok")
	m.IdentityRenewal("failed")
	m.AuditDropped(3)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`freya_calls_total{outcome="ok",peer="orders"} 1`,
		`freya_calls_total{outcome="authn_refused",peer="unknown"} 1`,
		`freya_calls_total{outcome="authz_refused",peer="orders"} 1`,
		`freya_calls_total{outcome="error",peer="orders"} 1`,
		`freya_authn_refusals_total{reason="untrusted"} 1`,
		`freya_authz_refusals_total{operation="/a.B/C",peer="orders"} 1`,
		`freya_identity_renewals_total{outcome="failed"} 1`,
		`freya_identity_renewals_total{outcome="ok"} 1`,
		`freya_audit_dropped_total 3`,
		`freya_call_duration_seconds_count`,
		`freya_call_duration_seconds_bucket`,
		"# TYPE freya_calls_total counter",
		"# TYPE freya_call_duration_seconds histogram",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q\n%s", want, body)
		}
	}
	if rec.Header().Get("Content-Type") == "" {
		t.Fatal("content type missing")
	}
}

// A service's own instruments created on Meter are rendered by Handler.
func TestMetricsServiceMeter(t *testing.T) {
	m, err := observe.NewMetrics()
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Meter("example.org/svc").Int64Counter("svc.widgets", metric.WithDescription("Widgets made"))
	if err != nil {
		t.Fatal(err)
	}
	c.Add(context.Background(), 3, metric.WithAttributes(attribute.String("kind", "blue")))
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(rec.Body.String(), `svc_widgets_total{kind="blue"} 3`) {
		t.Fatalf("service counter missing:\n%s", rec.Body.String())
	}
}
