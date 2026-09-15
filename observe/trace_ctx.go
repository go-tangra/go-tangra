package observe

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceID returns the W3C trace ID of the span in ctx, or "" when none.
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}
