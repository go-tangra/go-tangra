package grpc

import (
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
)

// ServerOption tunes a Freya gRPC server. There is deliberately no option to
// supply TLS settings, transport credentials, or to reorder/remove the security
// middleware.
type ServerOption func(*serverOptions)

type serverOptions struct {
	network    string
	address    string
	timeout    time.Duration
	middleware []middleware.Middleware
}

// WithAddress sets the listen address (default ":9443").
func WithAddress(addr string) ServerOption { return func(o *serverOptions) { o.address = addr } }

// WithNetwork sets the listen network (default "tcp").
func WithNetwork(n string) ServerOption { return func(o *serverOptions) { o.network = n } }

// WithMiddleware appends application middleware after the security chain.
func WithMiddleware(m ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) { o.middleware = append(o.middleware, m...) }
}

// WithTimeout sets the per-call timeout; it is clamped to Limits.RequestTimeout.
func WithTimeout(d time.Duration) ServerOption { return func(o *serverOptions) { o.timeout = d } }
