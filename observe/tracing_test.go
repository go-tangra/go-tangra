package observe_test

import (
	"context"
	"testing"

	"github.com/go-freya/freya/observe"
	"github.com/go-kratos/kratos/v3/transport"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestServerTracingExtractsAndAnnotates(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	mw := observe.ServerTracing(tp)
	var innerTrace string
	h := mw(func(ctx context.Context, _ any) (any, error) {
		observe.AnnotatePeer(ctx, "orders", "spiffe://example.org/svc/orders") // what authn does after verification
		innerTrace = observe.TraceID(ctx)
		return nil, nil
	})
	incoming := hdr{"traceparent": {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}}
	ctx := transport.NewServerContext(context.Background(), tr{req: incoming, rep: hdr{}, kind: transport.KindGRPC})
	if _, err := h(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if innerTrace != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("trace not continued: %q", innerTrace)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].SpanKind != trace.SpanKindServer || spans[0].Name != "/a.B/C" {
		t.Fatalf("spans %+v", spans)
	}
	found := map[string]string{}
	for _, a := range spans[0].Attributes {
		found[string(a.Key)] = a.Value.String()
	}
	if found["peer.service"] != "orders" || found["peer.spiffe_id"] != "spiffe://example.org/svc/orders" || found["rpc.system"] != "grpc" {
		t.Fatalf("attributes %v", found)
	}
	// Without an incoming header a new root trace is started.
	exp.Reset()
	_, _ = h(transport.NewServerContext(context.Background(), tr{req: hdr{}, rep: hdr{}}), nil)
	if len(exp.GetSpans()) != 1 || !exp.GetSpans()[0].SpanContext.HasTraceID() {
		t.Fatal("root span expected")
	}
	if observe.TraceID(context.Background()) != "" {
		t.Fatal("no trace id without span")
	}
}

func TestClientTracingInjects(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	out := hdr{}
	h := observe.ClientTracing(tp)(func(ctx context.Context, _ any) (any, error) { return nil, nil })
	if _, err := h(transport.NewClientContext(ctx, tr{req: out, rep: hdr{}}), nil); err != nil {
		t.Fatal(err)
	}
	span.End()
	tpar := out.Get("traceparent")
	if tpar == "" || tpar[3:35] != span.SpanContext().TraceID().String() {
		t.Fatalf("traceparent not injected: %q", tpar)
	}
	if n := len(exp.GetSpans()); n != 2 {
		t.Fatalf("expected client+parent spans, got %d", n)
	}
	// No transport in context: passthrough.
	if _, err := h(ctx, nil); err != nil {
		t.Fatal(err)
	}
}
