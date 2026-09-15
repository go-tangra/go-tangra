//go:build bench

package integration

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/go-freya/freya"
	inventoryv1 "github.com/go-freya/freya/examples/two-services/api/inventory/v1"
	"github.com/go-freya/freya/observe"
	klog "github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type echoInventory struct {
	inventoryv1.UnimplementedInventoryServer
}

func (echoInventory) Reserve(_ context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	return &inventoryv1.ReserveResponse{ReservationId: req.GetSku(), ReservedBy: "b"}, nil
}

var benchReq = &inventoryv1.ReserveRequest{Sku: "widget-benchmark-payload-0123456789-0123456789-0123456789", Quantity: 1}

// plaintextBaseline is a Kratos gRPC server running the same non-security
// middleware Freya runs (recover, correlation, tracing, instrumentation) over
// an insecure transport: the SC-006 baseline isolates TLS + identity
// verification + authorization.
func plaintextBaseline(b *testing.B) (*grpc.ClientConn, func()) {
	b.Helper()
	m, _ := observe.NewMetrics()
	quiet := slog.New(slog.NewJSONHandler(io.Discard, nil))
	klog.SetDefault(quiet)
	srv := kgrpc.NewServer(
		kgrpc.Address("127.0.0.1:0"),
		kgrpc.Middleware(
			recovery.Recovery(),
			observe.ServerCorrelation(observe.WithLogger(quiet)),
			observe.ServerTracing(noop.NewTracerProvider()),
			observe.Instrument(observe.InstrumentConfig{Metrics: m, ServiceName: "inventory", Logger: quiet}),
		),
		kgrpc.DisableReflection(),
	)
	inventoryv1.RegisterInventoryServer(srv, echoInventory{})
	ep, err := srv.Endpoint()
	if err != nil {
		b.Fatal(err)
	}
	go func() { _ = srv.Start(context.Background()) }()
	conn, err := kgrpc.NewClient(context.Background(), kgrpc.WithEndpoint(ep.Host),
		kgrpc.WithMiddleware(observe.ClientCorrelation(), observe.ClientTracing(noop.NewTracerProvider())))
	if err != nil {
		b.Fatal(err)
	}
	_ = insecure.NewCredentials
	return conn, func() { conn.Close(); _ = srv.Stop(context.Background()) }
}

func freyaChannel(b *testing.B) (*grpc.ClientConn, func()) {
	b.Helper()
	t := &testing.T{}
	f := newFixture(t, "inventory", "orders")
	quiet := freya.WithLogger(slog.NewJSONHandler(io.Discard, nil))
	inv, err := freya.New(f.config("inventory"), freya.WithAllowAllPolicy(), quiet)
	if err != nil {
		b.Fatal(err)
	}
	inventoryv1.RegisterInventoryServer(inv.GRPC(), echoInventory{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = inv.Run(ctx) }()
	ep, _ := inv.GRPC().Endpoint()
	ordCfg := f.config("orders")
	ordCfg.Discovery.Static = map[string][]string{"inventory": {ep.Host}}
	ord, err := freya.New(ordCfg, freya.WithAllowAllPolicy(), quiet)
	if err != nil {
		b.Fatal(err)
	}
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		b.Fatal(err)
	}
	return conn, func() { ord.Close(); cancel(); inv.Close() }
}

func runSequential(b *testing.B, conn *grpc.ClientConn) {
	c := inventoryv1.NewInventoryClient(conn)
	ctx := context.Background()
	if _, err := c.Reserve(ctx, benchReq); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Reserve(ctx, benchReq); err != nil {
			b.Fatal(err)
		}
	}
}

func runParallel(b *testing.B, conn *grpc.ClientConn) {
	c := inventoryv1.NewInventoryClient(conn)
	if _, err := c.Reserve(context.Background(), benchReq); err != nil {
		b.Fatal(err)
	}
	b.SetParallelism(8)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := c.Reserve(context.Background(), benchReq); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkCall: sequential latency, plaintext baseline vs the Freya channel.
func BenchmarkCall(b *testing.B) {
	b.Run("plaintext", func(b *testing.B) { conn, stop := plaintextBaseline(b); defer stop(); runSequential(b, conn) })
	b.Run("mtls", func(b *testing.B) { conn, stop := freyaChannel(b); defer stop(); runSequential(b, conn) })
}

// BenchmarkThroughput: 8 concurrent callers over one pooled connection.
func BenchmarkThroughput(b *testing.B) {
	b.Run("plaintext", func(b *testing.B) { conn, stop := plaintextBaseline(b); defer stop(); runParallel(b, conn) })
	b.Run("mtls", func(b *testing.B) { conn, stop := freyaChannel(b); defer stop(); runParallel(b, conn) })
}
