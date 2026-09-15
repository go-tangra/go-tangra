package authn

import (
	"net/http"

	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

type hdr http.Header

func (h hdr) Get(k string) string { return http.Header(h).Get(k) }
func (h hdr) Set(k, v string)     { http.Header(h).Set(k, v) }
func (h hdr) Add(k, v string)     { http.Header(h).Add(k, v) }
func (h hdr) Keys() []string {
	out := []string{}
	for k := range h {
		out = append(out, k)
	}
	return out
}
func (h hdr) Values(k string) []string { return http.Header(h).Values(k) }

type fakeHTTPTransport struct{ r *http.Request }

var _ khttp.Transporter = fakeHTTPTransport{}

func (f fakeHTTPTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f fakeHTTPTransport) Endpoint() string                { return "https://x" }
func (f fakeHTTPTransport) Operation() string               { return "GET /y" }
func (f fakeHTTPTransport) RequestHeader() transport.Header { return hdr(f.r.Header) }
func (f fakeHTTPTransport) ReplyHeader() transport.Header   { return hdr(http.Header{}) }
func (f fakeHTTPTransport) Request() *http.Request          { return f.r }
func (f fakeHTTPTransport) PathTemplate() string            { return "/y" }

// khttpTransport builds a Kratos HTTP transporter around r for tests.
func khttpTransport(r *http.Request) khttp.Transporter { return fakeHTTPTransport{r: r} }
