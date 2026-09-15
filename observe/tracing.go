package observe

import (
	"context"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/go-freya/freya"

var propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// headerCarrier adapts a Kratos transport header to OpenTelemetry propagation.
type headerCarrier struct{ h transport.Header }

func (c headerCarrier) Get(k string) string { return c.h.Get(k) }
func (c headerCarrier) Set(k, v string)     { c.h.Set(k, v) }
func (c headerCarrier) Keys() []string      { return c.h.Keys() }

// ServerTracing starts a server span per call, continuing an incoming W3C
// trace context. The peer attributes are added by the authn middleware via
// AnnotatePeer once the identity is verified.
func ServerTracing(tp trace.TracerProvider) middleware.Middleware {
	tracer := tp.Tracer(tracerName)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			name, system := "call", "unknown"
			if tr, ok := transport.FromServerContext(ctx); ok {
				ctx = propagator.Extract(ctx, headerCarrier{tr.RequestHeader()})
				name = tr.Operation()
				system = string(tr.Kind())
			}
			ctx, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String("rpc.system", system), attribute.String("rpc.method", name)))
			defer span.End()
			out, err := next(ctx, req)
			if err != nil {
				span.SetStatus(codes.Error, "refused or failed")
			}
			return out, err
		}
	}
}

// ClientTracing starts a client span and injects the trace context into the
// outgoing request headers.
func ClientTracing(tp trace.TracerProvider) middleware.Middleware {
	tracer := tp.Tracer(tracerName)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromClientContext(ctx)
			if !ok {
				return next(ctx, req)
			}
			ctx, span := tracer.Start(ctx, tr.Operation(), trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attribute.String("rpc.system", string(tr.Kind())), attribute.String("peer.endpoint", tr.Endpoint())))
			defer span.End()
			propagator.Inject(ctx, headerCarrier{tr.RequestHeader()})
			out, err := next(ctx, req)
			if err != nil {
				span.SetStatus(codes.Error, "failed")
			}
			return out, err
		}
	}
}

// AnnotatePeer records the verified peer on the current span.
func AnnotatePeer(ctx context.Context, service, spiffeID string) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("peer.service", service), attribute.String("peer.spiffe_id", spiffeID))
}
