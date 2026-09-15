package integration

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/authn"
	inventoryv1 "github.com/go-freya/freya/examples/two-services/api/inventory/v1"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/observe"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// relay forwards Reserve to the next hop using the same request context, so
// correlation ID and trace context must propagate.
type relay struct {
	inventoryv1.UnimplementedInventoryServer
	app  *freya.App
	next string
}

func (r *relay) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if r.next == "" {
		p, _ := authn.FromContext(ctx)
		return &inventoryv1.ReserveResponse{ReservationId: observe.CorrelationID(ctx), ReservedBy: p.ServiceName}, nil
	}
	conn, err := r.app.Client(ctx, r.next)
	if err != nil {
		return nil, err
	}
	return inventoryv1.NewInventoryClient(conn).Reserve(ctx, req)
}

func TestMultiHopTrace(t *testing.T) {
	f := newFixture(t, "a", "b", "c")
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	logs := &testutil.LogCapture{}
	// Start c, then b (→c), then a (→b).
	cCfg := f.config("c")
	cApp, err := freya.New(cCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()), freya.WithTracerProvider(tp))
	if err != nil {
		t.Fatal(err)
	}
	inventoryv1.RegisterInventoryServer(cApp.GRPC(), &relay{app: cApp})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = cApp.Run(ctx) }()
	defer cApp.Close()
	cEp, _ := cApp.GRPC().Endpoint()

	bCfg := f.config("b")
	bCfg.Discovery.Static = map[string][]string{"c": {cEp.Host}}
	bApp, err := freya.New(bCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()), freya.WithTracerProvider(tp))
	if err != nil {
		t.Fatal(err)
	}
	inventoryv1.RegisterInventoryServer(bApp.GRPC(), &relay{app: bApp, next: "c"})
	go func() { _ = bApp.Run(ctx) }()
	defer bApp.Close()
	bEp, _ := bApp.GRPC().Endpoint()

	aCfg := f.config("a")
	aCfg.Discovery.Static = map[string][]string{"b": {bEp.Host}}
	aApp, err := freya.New(aCfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()), freya.WithTracerProvider(tp))
	if err != nil {
		t.Fatal(err)
	}
	defer aApp.Close()
	time.Sleep(150 * time.Millisecond)

	const cid = "multihop-cid-1"
	callCtx := observe.WithCorrelationID(ctx, cid)
	conn, err := aApp.Client(callCtx, "b")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := inventoryv1.NewInventoryClient(conn).Reserve(callCtx, &inventoryv1.ReserveRequest{Sku: "x", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetReservationId() != cid || resp.GetReservedBy() != "b" {
		t.Fatalf("resp %+v", resp)
	}
	// One trace ID across all spans (2 server spans on b and c, 2 client spans on a and b).
	spans := exp.GetSpans()
	if len(spans) < 4 {
		t.Fatalf("expected >=4 spans, got %d", len(spans))
	}
	traceID := spans[0].SpanContext.TraceID()
	for _, s := range spans {
		if s.SpanContext.TraceID() != traceID {
			t.Fatalf("trace split: %v vs %v", s.SpanContext.TraceID(), traceID)
		}
	}
	// Request log lines: one hop line on b and one on c, queryable by correlation ID.
	hops := 0
	for _, l := range logs.Lines() {
		if l["msg"] == "request" && l["correlation_id"] == cid {
			hops++
			if l["caller"] == "" || l["callee"] == "" || l["outcome"] != "ok" || l["duration_ms"] == nil || l["trace_id"] != traceID.String() || l["operation"] != "/inventory.v1.Inventory/Reserve" {
				t.Fatalf("hop line %v", l)
			}
		}
	}
	if hops != 2 {
		t.Fatalf("expected 2 hop lines for %s, got %d\n%s", cid, hops, logs.String())
	}
	// Metrics on the admin listener count the calls per peer.
	code, body := httpGet(t, cApp.AdminURL()+"/metrics")
	if code != 200 || !containsAll(body, `freya_calls_total{outcome="ok",peer="b"} 1`) {
		t.Fatalf("metrics: %d\n%s", code, body)
	}
}
