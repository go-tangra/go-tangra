// Package testrt provides a ready-made transport.Runtime and mTLS clients for
// tests of the transport packages and the App.
package testrt

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authz"
	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/observe"
	"github.com/go-freya/freya/transport"
	"github.com/go-freya/freya/transport/tlsconf"
	ktransport "github.com/go-kratos/kratos/v3/transport"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Runtime is a concrete transport.Runtime for tests.
type Runtime struct {
	Name   string
	TD     string
	Prov   *testutil.MemProvider
	Lim    config.Limits
	Skew   time.Duration
	Aud    *audit.Emitter
	Log    *slog.Logger
	Logs   *testutil.LogCapture
	Authz  authz.Authorizer
	Events *EventSink
	Met    *observe.Metrics
	TP     trace.TracerProvider
}

// EventSink records audit events for assertions.
type EventSink struct {
	ch chan audit.Event
}

// Emit implements audit.Sink.
func (s *EventSink) Emit(_ context.Context, e audit.Event) {
	select {
	case s.ch <- e:
	default:
	}
}

// Next returns the next event or fails the test after d.
func (s *EventSink) Next(t *testing.T, d time.Duration) audit.Event {
	t.Helper()
	select {
	case e := <-s.ch:
		return e
	case <-time.After(d):
		t.Fatal("no audit event")
		return audit.Event{}
	}
}

// Drain returns all buffered events.
func (s *EventSink) Drain() []audit.Event {
	var out []audit.Event
	for {
		select {
		case e := <-s.ch:
			out = append(out, e)
		default:
			return out
		}
	}
}

var _ transport.Runtime = (*Runtime)(nil)

// New builds a runtime for service name using ca, with an allow-all authorizer.
func New(t *testing.T, ca *testutil.CA, name string) *Runtime {
	t.Helper()
	crt := ca.MustIssue(name, testutil.IssueOptions{})
	logs := &testutil.LogCapture{}
	log := slog.New(audit.NewRedactingHandler(logs.Handler()))
	ev := &EventSink{ch: make(chan audit.Event, 256)}
	em := audit.NewEmitter(audit.NewSlogSink(log), 0, ev)
	met, _ := observe.NewMetrics()
	return &Runtime{
		Name: name, TD: ca.TrustDomain, Prov: testutil.NewMemProvider(ca, crt),
		Lim: config.Default().Limits, Skew: 5 * time.Minute, Aud: em, Log: log, Logs: logs,
		Authz: authz.AllowAll{}, Events: ev, Met: met, TP: noop.NewTracerProvider(),
	}
}

func (r *Runtime) ServiceName() string                    { return r.Name }
func (r *Runtime) TrustDomain() string                    { return r.TD }
func (r *Runtime) LocalID() string                        { return "spiffe://" + r.TD + "/svc/" + r.Name }
func (r *Runtime) Provider() identity.Provider            { return r.Prov }
func (r *Runtime) Limits() config.Limits                  { return r.Lim }
func (r *Runtime) SkewTolerance() time.Duration           { return r.Skew }
func (r *Runtime) Audit() *audit.Emitter                  { return r.Aud }
func (r *Runtime) Logger() *slog.Logger                   { return r.Log }
func (r *Runtime) Authorizer() authz.Authorizer           { return r.Authz }
func (r *Runtime) Revocation() identity.RevocationChecker { return nil }
func (r *Runtime) Metrics() *observe.Metrics              { return r.Met }
func (r *Runtime) Tracer() trace.TracerProvider           { return r.TP }

// StartServer starts a Kratos transport server and returns a stop func.
func StartServer(t *testing.T, s ktransport.Server) func() {
	t.Helper()
	// Bind first so that Endpoint() and Start() never race on the listener.
	if ep, ok := s.(ktransport.Endpointer); ok {
		if _, err := ep.Endpoint(); err != nil {
			t.Fatalf("bind: %v", err)
		}
	}
	errc := make(chan error, 1)
	go func() { errc <- s.Start(context.Background()) }()
	select {
	case err := <-errc:
		t.Fatalf("server exited early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	return func() { _ = s.Stop(context.Background()) }
}

// ClientTLS returns a client TLS config presenting caller and expecting callee.
func ClientTLS(t *testing.T, ca *testutil.CA, caller, callee string) *tls.Config {
	t.Helper()
	p := testutil.NewMemProvider(ca, ca.MustIssue(caller, testutil.IssueOptions{}))
	cfg, err := tlsconf.ClientConfig(p, identity.ForService(ca.TrustDomain, callee), tlsconf.Options{TrustDomain: ca.TrustDomain})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// HTTPClient returns an mTLS HTTP/2 client presenting caller and expecting callee.
func HTTPClient(t *testing.T, ca *testutil.CA, caller, callee string) *http.Client {
	t.Helper()
	return &http.Client{Transport: &http.Transport{TLSClientConfig: ClientTLS(t, ca, caller, callee), ForceAttemptHTTP2: true}, Timeout: 5 * time.Second}
}
