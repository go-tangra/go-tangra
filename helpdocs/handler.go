package helpdocs

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-tangra/go-tangra/v4/authn"
)

// BundlePath is where a module serves its bundle on its Freya HTTP server.
const BundlePath = "/docs/v1/bundle"

// DefaultGatewayService is the gateway's service name (spiffe://<td>/svc/gateway).
const DefaultGatewayService = "gateway"

// Authorizer decides whether the caller of r may fetch the bundle.
type Authorizer func(r *http.Request) bool

// AllowServices admits callers whose verified mTLS peer identity (placed in
// the request context by the Freya HTTP server's authn middleware, see
// authn.FromContext) has one of the given service names. The trust domain is
// already enforced by authn. A request without a verified peer is refused.
func AllowServices(names ...string) Authorizer {
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	return func(r *http.Request) bool {
		p, ok := authn.FromContext(r.Context())
		return ok && allowed[p.ID.ServiceName()]
	}
}

// NewHandler serves b as JSON for GET/HEAD with ETag "<digest>",
// Cache-Control: no-cache and If-None-Match -> 304. authorize gates every
// request; nil means AllowServices(DefaultGatewayService). A nil b serves an
// empty bundle (without an ETag).
//
// Mount it on the module's Freya HTTP server (app.HTTP()), whose security
// chain verifies the mTLS peer and applies the service-to-service policy
// before the handler runs; register it before any catch-all prefix route.
func NewHandler(b *Bundle, authorize Authorizer) http.Handler {
	if b == nil {
		b = &Bundle{Schema: Schema, Pages: []Page{}, Assets: []Asset{}}
	}
	if authorize == nil {
		authorize = AllowServices(DefaultGatewayService)
	}
	body, err := b.Marshal()
	etag := ""
	if b.Digest != "" {
		etag = `"` + b.Digest + `"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-cache")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorize(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, "bundle unavailable", http.StatusInternalServerError)
			return
		}
		if etag != "" {
			h.Set("ETag", etag)
			if etagMatch(r.Header.Values("If-None-Match"), etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		h.Set("Content-Type", "application/json")
		h.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
}

// etagMatch implements the weak comparison of If-None-Match.
func etagMatch(headers []string, etag string) bool {
	for _, h := range headers {
		for t := range strings.SplitSeq(h, ",") {
			t = strings.TrimSpace(t)
			if t == "*" || strings.TrimPrefix(t, "W/") == etag {
				return true
			}
		}
	}
	return false
}
