package observe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Metrics holds the framework instruments. They are exposed on the admin
// listener in OpenMetrics text format without a Prometheus client dependency.
type Metrics struct {
	reader   *sdkmetric.ManualReader
	provider *sdkmetric.MeterProvider
	calls    metric.Int64Counter
	authn    metric.Int64Counter
	authz    metric.Int64Counter
	renewals metric.Int64Counter
	dropped  metric.Int64Counter
	duration metric.Float64Histogram
}

// NewMetrics creates the instruments on a private meter provider.
func NewMetrics() (*Metrics, error) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("github.com/go-freya/freya")
	m := &Metrics{reader: reader, provider: mp}
	var err error
	if m.calls, err = meter.Int64Counter("freya.calls", metric.WithDescription("Calls handled, by peer and outcome")); err != nil {
		return nil, err
	}
	if m.authn, err = meter.Int64Counter("freya.authn.refusals", metric.WithDescription("Authentication refusals by reason")); err != nil {
		return nil, err
	}
	if m.authz, err = meter.Int64Counter("freya.authz.refusals", metric.WithDescription("Authorization refusals by peer and operation")); err != nil {
		return nil, err
	}
	if m.renewals, err = meter.Int64Counter("freya.identity.renewals", metric.WithDescription("Identity renewals by outcome")); err != nil {
		return nil, err
	}
	if m.dropped, err = meter.Int64Counter("freya.audit.dropped", metric.WithDescription("Audit events dropped because the queue was full")); err != nil {
		return nil, err
	}
	if m.duration, err = meter.Float64Histogram("freya.call.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)); err != nil {
		return nil, err
	}
	return m, nil
}

// IdentityRenewal records a renewal outcome ("ok" or "failed").
func (m *Metrics) IdentityRenewal(outcome string) {
	m.renewals.Add(context.Background(), 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// AuditDropped adds to the dropped-audit counter.
func (m *Metrics) AuditDropped(n int64) { m.dropped.Add(context.Background(), n) }

// InstrumentConfig wires the per-call instrumentation middleware.
type InstrumentConfig struct {
	Metrics     *Metrics
	ServiceName string
	// Logger receives one "request" line per call; nil disables the request log.
	Logger *slog.Logger
	// Peer extracts the verified peer from the context (supplied by the transport
	// package to avoid an import cycle with authn). nil = unknown.
	Peer func(ctx context.Context) (service, spiffeID string, ok bool)
}

// Instrument records duration, outcome and refusal counters for every call and
// writes the per-hop request log line (caller, callee, operation, outcome,
// duration, correlation ID, trace ID). It runs before authn so refusals are
// counted; the peer is read after the handler chain has run.
func Instrument(cfg InstrumentConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			start := time.Now()
			var peerService, peerID string
			// The peer is set by authn inside next; capture it through a mutable holder.
			holder := &peerHolder{}
			ctx = context.WithValue(ctx, peerHolderKey{}, holder)
			out, err := next(ctx, req)
			dur := time.Since(start)
			if cfg.Peer != nil {
				peerService, peerID, _ = cfg.Peer(holder.ctx())
			}
			if peerService == "" {
				peerService = "unknown"
			}
			op := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				op = tr.Operation()
			}
			outcome := classify(err)
			if cfg.Metrics != nil {
				m := cfg.Metrics
				m.calls.Add(ctx, 1, metric.WithAttributes(attribute.String("peer", peerService), attribute.String("outcome", outcome)))
				m.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(attribute.String("peer", peerService), attribute.String("outcome", outcome)))
				switch outcome {
				case "authn_refused":
					m.authn.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", kerrors.FromError(err).Reason)))
				case "authz_refused":
					m.authz.Add(ctx, 1, metric.WithAttributes(attribute.String("peer", peerService), attribute.String("operation", op)))
				}
			}
			if cfg.Logger != nil {
				cfg.Logger.LogAttrs(ctx, slog.LevelInfo, "request",
					slog.String("caller", peerService), slog.String("caller_id", peerID), slog.String("callee", cfg.ServiceName),
					slog.String("operation", op), slog.String("outcome", outcome), slog.Float64("duration_ms", float64(dur)/1e6),
					slog.String("correlation_id", CorrelationID(ctx)), slog.String("trace_id", TraceID(ctx)))
			}
			return out, err
		}
	}
}

// peerHolder lets the instrumentation middleware observe the context that the
// authn middleware produced further down the chain.
type peerHolder struct{ c context.Context }
type peerHolderKey struct{}

func (h *peerHolder) ctx() context.Context {
	if h.c == nil {
		return context.Background()
	}
	return h.c
}

// RecordPeerContext is called by the authn middleware after it stored the peer,
// so that instrumentation can read it.
func RecordPeerContext(ctx context.Context) {
	if h, ok := ctx.Value(peerHolderKey{}).(*peerHolder); ok {
		h.c = ctx
	}
}

func classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case kerrors.IsUnauthorized(err):
		return "authn_refused"
	case kerrors.IsForbidden(err):
		return "authz_refused"
	case kerrors.IsServiceUnavailable(err):
		return "unavailable"
	case kerrors.IsGatewayTimeout(err):
		return "timeout"
	default:
		return "error"
	}
}

// Handler renders all instruments in OpenMetrics/Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rm metricdata.ResourceMetrics
		if err := m.reader.Collect(r.Context(), &rm); err != nil {
			http.Error(w, "collect failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var sb strings.Builder
		sb.WriteString("# HELP freya_up Always 1 while the admin listener serves.\n# TYPE freya_up gauge\nfreya_up 1\n")
		for _, sm := range rm.ScopeMetrics {
			for _, mt := range sm.Metrics {
				name := promName(mt.Name)
				switch data := mt.Data.(type) {
				case metricdata.Sum[int64]:
					if data.IsMonotonic {
						name += "_total"
					}
					fmt.Fprintf(&sb, "# HELP %s %s\n# TYPE %s counter\n", name, mt.Description, name)
					for _, dp := range sortedInt(data.DataPoints) {
						fmt.Fprintf(&sb, "%s%s %d\n", name, labels(dp.Attributes), dp.Value)
					}
				case metricdata.Histogram[float64]:
					name += "_seconds"
					fmt.Fprintf(&sb, "# HELP %s %s\n# TYPE %s histogram\n", name, mt.Description, name)
					for _, dp := range data.DataPoints {
						var cum uint64
						for i, bound := range dp.Bounds {
							cum += dp.BucketCounts[i]
							fmt.Fprintf(&sb, "%s_bucket%s %d\n", name, labelsWith(dp.Attributes, "le", fmt.Sprintf("%g", bound)), cum)
						}
						cum += dp.BucketCounts[len(dp.Bounds)]
						fmt.Fprintf(&sb, "%s_bucket%s %d\n", name, labelsWith(dp.Attributes, "le", "+Inf"), cum)
						fmt.Fprintf(&sb, "%s_sum%s %g\n%s_count%s %d\n", name, labels(dp.Attributes), dp.Sum, name, labels(dp.Attributes), dp.Count)
					}
				}
			}
		}
		_, _ = w.Write([]byte(sb.String()))
	})
}

func promName(n string) string { return strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(n) }

func sortedInt(dps []metricdata.DataPoint[int64]) []metricdata.DataPoint[int64] {
	out := append([]metricdata.DataPoint[int64](nil), dps...)
	sort.Slice(out, func(i, j int) bool { return labels(out[i].Attributes) < labels(out[j].Attributes) })
	return out
}

func labels(set attribute.Set) string { return labelsWith(set, "", "") }

func labelsWith(set attribute.Set, extraKey, extraVal string) string {
	var parts []string
	iter := set.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		parts = append(parts, fmt.Sprintf(`%s="%s"`, string(kv.Key), sanitizeLabel(kv.Value.String())))
	}
	if extraKey != "" {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, extraKey, extraVal))
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ",") + "}"
}

func sanitizeLabel(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
