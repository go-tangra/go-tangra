package observe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"regexp"
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// HeaderCorrelationID is the metadata key / HTTP header carrying the correlation ID.
const HeaderCorrelationID = "x-request-id"

var correlationRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type ctxKey struct{}

// CorrelationID returns the correlation ID stored in ctx, or "".
func CorrelationID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// WithCorrelationID stores id in ctx.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// ValidCorrelationID reports whether s satisfies ^[A-Za-z0-9._-]{1,128}$.
func ValidCorrelationID(s string) bool { return correlationRE.MatchString(s) }

// NewCorrelationID returns a UUIDv7 string (time-ordered, random tail).
func NewCorrelationID() string {
	var b [16]byte
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(time.Now().UnixMilli())) // #nosec G115 -- UnixMilli is positive
	copy(b[0:6], ts[2:8])                                             // 48-bit millisecond timestamp
	if _, err := rand.Read(b[6:]); err != nil {
		panic("observe: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Option configures the correlation middleware.
type Option func(*options)

type options struct{ log *slog.Logger }

// WithLogger sets the logger used to report replaced (invalid) incoming IDs.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.log = l } }

// ServerCorrelation returns middleware that accepts a valid incoming
// x-request-id, replaces an invalid one, generates one when absent, stores it in
// the context and echoes it in the reply header.
func ServerCorrelation(opts ...Option) middleware.Middleware {
	o := options{log: slog.Default()}
	for _, f := range opts {
		f(&o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id := ""
			tr, ok := transport.FromServerContext(ctx)
			if ok {
				id = tr.RequestHeader().Get(HeaderCorrelationID)
			}
			if id != "" && !ValidCorrelationID(id) {
				o.log.DebugContext(ctx, "correlation id replaced: invalid format", "length", len(id))
				id = ""
			}
			if id == "" {
				id = NewCorrelationID()
			}
			if ok {
				tr.ReplyHeader().Set(HeaderCorrelationID, id)
			}
			return next(WithCorrelationID(ctx, id), req)
		}
	}
}

// ClientCorrelation returns middleware that injects the context's correlation ID
// (generating one if absent) into the outgoing request header.
func ClientCorrelation() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id := CorrelationID(ctx)
			if id == "" {
				id = NewCorrelationID()
				ctx = WithCorrelationID(ctx, id)
			}
			if tr, ok := transport.FromClientContext(ctx); ok {
				tr.RequestHeader().Set(HeaderCorrelationID, id)
			}
			return next(ctx, req)
		}
	}
}
