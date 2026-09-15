package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/transport"
	tgrpc "github.com/go-freya/freya/transport/grpc"
	"github.com/go-freya/freya/transport/tlsconf"
	"github.com/go-kratos/kratos/v3/middleware"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// Server is a Kratos HTTP server that is always mutually authenticated and
// never falls back to http.DefaultServeMux. It deliberately does not embed the
// Kratos/net/http server: only routing and lifecycle are exposed, so there is
// no ListenAndServe/ServeTLS path that could bypass the Freya listener.
type Server struct {
	ks      *khttp.Server
	timeout time.Duration
	limits  config.Limits
}

// Handle registers h for path (any method) behind the security chain.
func (s *Server) Handle(path string, h http.Handler) { s.ks.Handle(path, h) }

// HandleFunc registers h for path (any method) behind the security chain.
func (s *Server) HandleFunc(path string, h http.HandlerFunc) { s.ks.HandleFunc(path, h) }

// HandlePrefix registers h for every path under prefix.
func (s *Server) HandlePrefix(prefix string, h http.Handler) { s.ks.HandlePrefix(prefix, h) }

// Route returns a Kratos router for method-specific routes (GET/POST/...).
func (s *Server) Route(prefix string, filters ...khttp.FilterFunc) *khttp.Router {
	return s.ks.Route(prefix, filters...)
}

// Use adds application middleware for the given operation selector; it runs
// after the security chain and cannot remove it.
func (s *Server) Use(selector string, m ...middleware.Middleware) { s.ks.Use(selector, m...) }

// WalkRoute iterates registered routes.
func (s *Server) WalkRoute(fn khttp.WalkRouteFunc) error { return s.ks.WalkRoute(fn) }

// Endpoint returns the https:// endpoint.
func (s *Server) Endpoint() (*url.URL, error) { return s.ks.Endpoint() }

// Start serves until Stop.
func (s *Server) Start(ctx context.Context) error { return s.ks.Start(ctx) }

// Stop shuts the server down gracefully.
func (s *Server) Stop(ctx context.Context) error { return s.ks.Stop(ctx) }

// ServerOption tunes a Freya HTTP server. No option can alter TLS.
type ServerOption func(*serverOptions)

type serverOptions struct {
	network    string
	address    string
	timeout    time.Duration
	middleware []middleware.Middleware
}

// WithAddress sets the listen address (default ":8443").
func WithAddress(addr string) ServerOption { return func(o *serverOptions) { o.address = addr } }

// WithNetwork sets the listen network (default "tcp").
func WithNetwork(n string) ServerOption { return func(o *serverOptions) { o.network = n } }

// WithMiddleware appends application middleware after the security chain.
func WithMiddleware(m ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) { o.middleware = append(o.middleware, m...) }
}

// WithTimeout sets the per-request timeout; clamped to Limits.RequestTimeout.
func WithTimeout(d time.Duration) ServerOption { return func(o *serverOptions) { o.timeout = d } }

// NewServer binds the listener immediately (so a busy port fails fast) and
// installs: eager TLS 1.3 mTLS handshakes, explicit 404/405 handlers
// (mitigating GHSA-jj45-xvq5-rhh9), body/header limits, and the security chain
// as a router-wide filter.
func NewServer(rt transport.Runtime, opts ...ServerOption) (*Server, error) {
	lim := rt.Limits()
	o := serverOptions{network: "tcp", address: ":8443", timeout: lim.RequestTimeout}
	for _, f := range opts {
		f(&o)
	}
	if o.timeout <= 0 || o.timeout > lim.RequestTimeout {
		o.timeout = lim.RequestTimeout
	}
	tlsCfg, err := tlsconf.ServerConfig(rt.Provider(), transport.TLSOptions(rt))
	if err != nil {
		return nil, err
	}
	raw, err := net.Listen(o.network, o.address)
	if err != nil {
		return nil, err
	}
	lis := newTLSListener(raw, tlsCfg, lim.HandshakeTimeout, rt)
	chain := append(tgrpc.SecurityChain(rt), o.middleware...)
	ks := khttp.NewServer(
		khttp.Network(o.network),
		khttp.Address(o.address),
		khttp.Listener(lis),
		khttp.Endpoint(&url.URL{Scheme: "https", Host: transport.ResolveEndpoint(o.address, raw)}),
		khttp.Timeout(o.timeout),
		khttp.NotFoundHandler(http.NotFoundHandler()),
		khttp.MethodNotAllowedHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		})),
		khttp.ErrorEncoder(ErrorEncoder),
		khttp.Filter(limitBody(rt, lim.MaxRequestBytes), securityFilter(rt, chain, ErrorEncoder)),
	)
	// Kratos serves plain over our already-handshaken TLS conns; the http.Server
	// still needs the config for HTTP/2 negotiation and for introspection.
	ks.Server.TLSConfig = tlsCfg
	ks.Server.MaxHeaderBytes = lim.MaxHeaderBytes
	ks.Server.ReadHeaderTimeout = lim.HandshakeTimeout
	ks.Server.IdleTimeout = lim.IdleTimeout
	return &Server{ks: ks, timeout: o.timeout, limits: lim}, nil
}

func limitBody(rt transport.Runtime, n int64) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = &auditedBody{ReadCloser: http.MaxBytesReader(w, r.Body, n), onExceed: func() {
					transport.AuditLimitExceeded(rt, r.Context(), r.Method+" "+r.URL.Path, r.RemoteAddr, "request_body")
				}}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// auditedBody reports the first MaxBytesError exactly once.
type auditedBody struct {
	io.ReadCloser
	onExceed func()
	reported bool
}

func (b *auditedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var mbe *http.MaxBytesError
	if err != nil && !b.reported && errors.As(err, &mbe) {
		b.reported = true
		b.onExceed()
	}
	return n, err
}
