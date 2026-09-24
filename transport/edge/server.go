// Package edge is the browser-facing listener for Freya services: server-
// authenticated TLS 1.3 (browsers cannot present SPIFFE identities), strict
// security headers, CSRF protection and rate limits. It never speaks plaintext
// and never replaces the mTLS service listeners.
package edge

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	ktransport "github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/observe"
	"github.com/go-tangra/go-tangra/v4/transport"
	thttp "github.com/go-tangra/go-tangra/v4/transport/http"
)

// Config for the edge listener.
type Config struct {
	Addr string // default ":8443"
	// Env is the deployment environment; a generated self-signed certificate is
	// only permitted when Env != "production".
	Env string
	// CertFile/KeyFile hold the public certificate chain and key (PEM); they are
	// re-read every ReloadInterval (default 1m) and swapped without restart.
	CertFile, KeyFile string
	ReloadInterval    time.Duration
	// AllowedOrigins are the browser origins accepted for state-changing requests.
	AllowedOrigins []string
	RateLimit      RateLimit
	// TrustedProxies are CIDRs allowed to set X-Forwarded-For (default: none).
	TrustedProxies []string
	// CSPExtra is appended to the default Content-Security-Policy (e.g. style
	// sources); the per-request nonce is always added for script-src.
	CSPExtra string
	// CSRFExempt, if set, is consulted for state-changing requests; returning
	// true skips the double-submit check. Only exempt requests that carry no
	// cookie credential (for example bearer-token API clients): CSRF exists to
	// protect ambient cookies, and a bearer header cannot be sent cross-site
	// without a CORS preflight the browser refuses.
	CSRFExempt func(r *http.Request) bool
}

// ServerOption tunes the server. No option can alter TLS.
type ServerOption func(*serverOptions)

type serverOptions struct{ middleware []middleware.Middleware }

// WithMiddleware appends application middleware after the built-in chain.
func WithMiddleware(m ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) { o.middleware = append(o.middleware, m...) }
}

// Server is the edge listener.
type Server struct {
	ks      *khttp.Server
	rt      transport.Runtime
	cfg     Config
	cert    *certLoader
	limiter *limiter
}

// NewServer binds the listener. Without CertFile/KeyFile a self-signed
// certificate is generated when cfg.Env != "production" (audited as
// insecure_mode_enabled/local_dev); in production a certificate is required.
func NewServer(rt transport.Runtime, cfg Config, opts ...ServerOption) (*Server, error) {
	if rt == nil {
		return nil, errors.New("edge: runtime is required")
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8443"
	}
	if cfg.ReloadInterval <= 0 {
		cfg.ReloadInterval = time.Minute
	}
	var o serverOptions
	for _, f := range opts {
		f(&o)
	}
	loader, err := newCertLoader(cfg, rt)
	if err != nil {
		return nil, err
	}
	proxies, err := parseCIDRs(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}
	lim := rt.Limits()
	tlsCfg := &tls.Config{
		MinVersion:     tls.VersionTLS13,
		MaxVersion:     tls.VersionTLS13,
		NextProtos:     []string{"h2", "http/1.1"},
		GetCertificate: loader.get,
	}
	raw, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	limiter := newLimiter(cfg.RateLimit)
	chain := append([]middleware.Middleware{
		recovery.Recovery(recovery.WithLogger(rt.Logger())),
		observe.ServerCorrelation(observe.WithLogger(rt.Logger())),
		observe.ServerTracing(rt.Tracer()),
		observe.Instrument(observe.InstrumentConfig{Metrics: rt.Metrics(), ServiceName: rt.ServiceName(), Logger: rt.Logger(), Peer: func(context.Context) (string, string, bool) { return "browser", "", true }}),
	}, o.middleware...)
	s := &Server{rt: rt, cfg: cfg, cert: loader, limiter: limiter}
	ks := khttp.NewServer(
		khttp.Address(cfg.Addr),
		khttp.Listener(thttp.NewTLSListener(raw, tlsCfg, lim.HandshakeTimeout, rt)),
		khttp.Endpoint(&url.URL{Scheme: "https", Host: transport.ResolveEndpoint(cfg.Addr, raw)}),
		khttp.Timeout(lim.RequestTimeout),
		khttp.NotFoundHandler(http.NotFoundHandler()),
		khttp.MethodNotAllowedHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		})),
		khttp.ErrorEncoder(thttp.ErrorEncoder),
		khttp.Filter(
			s.rateLimitFilter(proxies),
			s.headersFilter(),
			s.csrfFilter(),
			bodyLimitFilter(rt, lim.MaxRequestBytes),
			chainFilter(chain),
		),
	)
	ks.Server.TLSConfig = tlsCfg
	ks.Server.MaxHeaderBytes = lim.MaxHeaderBytes
	ks.Server.ReadHeaderTimeout = lim.HandshakeTimeout
	ks.Server.IdleTimeout = lim.IdleTimeout
	s.ks = ks
	return s, nil
}

// Handle / HandleFunc / HandlePrefix / Route / Use / Endpoint / Start / Stop mirror transport/http.
func (s *Server) Handle(path string, h http.Handler)         { s.ks.Handle(path, h) }
func (s *Server) HandleFunc(path string, h http.HandlerFunc) { s.ks.HandleFunc(path, h) }
func (s *Server) HandlePrefix(prefix string, h http.Handler) { s.ks.HandlePrefix(prefix, h) }
func (s *Server) Route(prefix string, f ...khttp.FilterFunc) *khttp.Router {
	return s.ks.Route(prefix, f...)
}
func (s *Server) Use(selector string, m ...middleware.Middleware) { s.ks.Use(selector, m...) }
func (s *Server) Endpoint() (*url.URL, error)                     { return s.ks.Endpoint() }
func (s *Server) Start(ctx context.Context) error {
	s.cert.start(s.cfg.ReloadInterval)
	return s.ks.Start(ctx)
}
func (s *Server) Stop(ctx context.Context) error {
	s.cert.stop()
	return s.ks.Stop(ctx)
}

// chainFilter runs Kratos middleware for every request (as transport/http does).
func chainFilter(chain []middleware.Middleware) khttp.FilterFunc {
	mw := middleware.Chain(chain...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr := &edgeTransport{r: r, reply: w.Header()}
			ctx := ktransport.NewServerContext(r.Context(), tr)
			h := mw(func(ctx context.Context, _ any) (any, error) {
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil, nil
			})
			if _, err := h(ctx, nil); err != nil {
				thttp.ErrorEncoder(w, r, err)
			}
		})
	}
}

func bodyLimitFilter(rt transport.Runtime, n int64) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = &auditedBody{readCloser: http.MaxBytesReader(w, r.Body, n), onExceed: func() {
					transport.AuditLimitExceeded(rt, r.Context(), r.Method+" "+r.URL.Path, r.RemoteAddr, "request_body")
				}}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type auditedBody struct {
	readCloser
	onExceed func()
	reported bool
}

type readCloser interface {
	Read([]byte) (int, error)
	Close() error
}

func (b *auditedBody) Read(p []byte) (int, error) {
	n, err := b.readCloser.Read(p)
	var mbe *http.MaxBytesError
	if err != nil && !b.reported && errors.As(err, &mbe) {
		b.reported = true
		b.onExceed()
	}
	return n, err
}

func parseCIDRs(list []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(list))
	for _, c := range list {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil {
			return nil, fmt.Errorf("edge: trusted proxy %q: %w", c, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// clientIP returns the peer address, honouring X-Forwarded-For only from trusted proxies.
func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(trusted) == 0 {
		return host
	}
	ip := net.ParseIP(host)
	for _, n := range trusted {
		if ip != nil && n.Contains(ip) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return host
}

func auditRefusal(rt transport.Runtime, r *http.Request, reason audit.Reason, attrs map[string]string) {
	cid := observe.CorrelationID(r.Context())
	if cid == "" {
		cid = observe.NewCorrelationID()
	}
	_ = rt.Audit().Emit(r.Context(), audit.Event{
		Type: audit.TypeAuthnRefused, Outcome: audit.OutcomeRefused, Reason: reason,
		LocalID: rt.LocalID(), Operation: r.Method + " " + r.URL.Path, RemoteAddr: r.RemoteAddr,
		CorrelationID: cid, Attrs: attrs,
	})
}

// edgeTransport is the Kratos transporter visible to the middleware chain.
type edgeTransport struct {
	r     *http.Request
	reply http.Header
}

type headerCarrier http.Header

func (h headerCarrier) Get(k string) string      { return http.Header(h).Get(k) }
func (h headerCarrier) Set(k, v string)          { http.Header(h).Set(k, v) }
func (h headerCarrier) Add(k, v string)          { http.Header(h).Add(k, v) }
func (h headerCarrier) Values(k string) []string { return http.Header(h).Values(k) }
func (h headerCarrier) Keys() []string {
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	return out
}

func (t *edgeTransport) Kind() ktransport.Kind            { return ktransport.KindHTTP }
func (t *edgeTransport) Endpoint() string                 { return "https://" + t.r.Host }
func (t *edgeTransport) Operation() string                { return t.r.Method + " " + t.r.URL.Path }
func (t *edgeTransport) RequestHeader() ktransport.Header { return headerCarrier(t.r.Header) }
func (t *edgeTransport) ReplyHeader() ktransport.Header   { return headerCarrier(t.reply) }
func (t *edgeTransport) Request() *http.Request           { return t.r }
func (t *edgeTransport) PathTemplate() string             { return t.r.URL.Path }
