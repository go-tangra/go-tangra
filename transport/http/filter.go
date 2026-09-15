package http

import (
	"context"
	"net/http"

	"github.com/go-freya/freya/transport"
	"github.com/go-kratos/kratos/v3/middleware"
	ktransport "github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// securityFilter runs the Kratos middleware chain for every HTTP request before
// routing, so that plain HandleFunc routes are covered exactly like generated
// handlers. Errors are encoded by the Freya error encoder.
func securityFilter(rt transport.Runtime, chain []middleware.Middleware, encode khttp.EncodeErrorFunc) khttp.FilterFunc {
	mw := middleware.Chain(chain...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr := &preRouteTransport{r: r, reply: w.Header()}
			ctx := ktransport.NewServerContext(r.Context(), tr)
			h := mw(func(ctx context.Context, _ any) (any, error) {
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil, nil
			})
			if _, err := h(ctx, nil); err != nil {
				encode(w, r, err)
			}
		})
	}
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

// preRouteTransport is the transport visible to the security chain before the
// router has matched a path template.
type preRouteTransport struct {
	r     *http.Request
	reply http.Header
}

var _ khttp.Transporter = (*preRouteTransport)(nil)

func (t *preRouteTransport) Kind() ktransport.Kind            { return ktransport.KindHTTP }
func (t *preRouteTransport) Endpoint() string                 { return "https://" + t.r.Host }
func (t *preRouteTransport) Operation() string                { return t.r.URL.Path }
func (t *preRouteTransport) RequestHeader() ktransport.Header { return headerCarrier(t.r.Header) }
func (t *preRouteTransport) ReplyHeader() ktransport.Header   { return headerCarrier(t.reply) }
func (t *preRouteTransport) Request() *http.Request           { return t.r }
func (t *preRouteTransport) PathTemplate() string             { return t.r.URL.Path }
