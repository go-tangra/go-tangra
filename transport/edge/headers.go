package edge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

type nonceKey struct{}

// Nonce returns the per-request CSP nonce for inline scripts/styles.
func Nonce(ctx context.Context) string {
	n, _ := ctx.Value(nonceKey{}).(string)
	return n
}

// WithNonce attaches a CSP nonce to ctx (tests, and servers behind an edge
// that relays the edge's nonce).
func WithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, nonceKey{}, nonce)
}

func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawStdEncoding.EncodeToString(b[:])
}

func (s *Server) headersFilter() khttp.FilterFunc {
	extra := strings.TrimSpace(s.cfg.CSPExtra)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := newNonce()
			h := w.Header()
			csp := "default-src 'self'; script-src 'self' 'nonce-" + nonce + "'; style-src 'self' 'nonce-" + nonce + "'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'self'"
			if extra != "" {
				csp += "; " + extra
			}
			h.Set("Content-Security-Policy", csp)
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/.well-known/") {
				h.Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce)))
		})
	}
}

type clientIPKey struct{}

// WithClientIP records the proxy-aware client address for handlers.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIP returns the address the rate limiter attributed the request to
// ("" outside an edge request).
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
