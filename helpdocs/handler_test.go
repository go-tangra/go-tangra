package helpdocs_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/helpdocs"
	"github.com/go-tangra/go-tangra/v4/identity"
	thttp "github.com/go-tangra/go-tangra/v4/transport/http"
)

func bundle(t *testing.T) *helpdocs.Bundle {
	t.Helper()
	b, err := helpdocs.Load(fstest.MapFS{
		"index.md": {Data: []byte("---\ntitle: Home\nkind: guide\n---\nHello.\n")},
	}, "ipam")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func as(service string, r *http.Request) *http.Request {
	if service == "" {
		return r
	}
	return r.WithContext(authn.WithPeer(r.Context(), authn.PeerIdentity{ID: identity.ForService("example.org", service), ServiceName: service}))
}

func TestHandler(t *testing.T) {
	b := bundle(t)
	etag := `"` + b.Digest + `"`
	h := helpdocs.NewHandler(b, nil)
	cases := []struct {
		name, caller, method, inm string
		status                    int
		body                      bool
	}{
		{"gateway get", "gateway", http.MethodGet, "", 200, true},
		{"gateway head", "gateway", http.MethodHead, "", 200, false},
		{"not modified", "gateway", http.MethodGet, etag, 304, false},
		{"not modified weak list", "gateway", http.MethodGet, `"x", W/` + etag, 304, false},
		{"not modified star", "gateway", http.MethodGet, "*", 304, false},
		{"stale etag", "gateway", http.MethodGet, `"old"`, 200, true},
		{"other service", "orders", http.MethodGet, "", 403, false},
		{"other service probing etag", "orders", http.MethodGet, etag, 403, false},
		{"no peer", "", http.MethodGet, "", 403, false},
		{"post", "gateway", http.MethodPost, "", 405, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := as(c.caller, httptest.NewRequestWithContext(context.Background(), c.method, helpdocs.BundlePath, http.NoBody))
			if c.inm != "" {
				r.Header.Set("If-None-Match", c.inm)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != c.status {
				t.Fatalf("status %d, want %d", w.Code, c.status)
			}
			if w.Header().Get("Cache-Control") != "no-cache" {
				t.Fatal("Cache-Control missing")
			}
			if c.status == 200 || c.status == 304 {
				if w.Header().Get("ETag") != etag {
					t.Fatalf("ETag %q", w.Header().Get("ETag"))
				}
			} else if w.Header().Get("ETag") != "" {
				t.Fatal("ETag leaked on refusal")
			}
			if c.body {
				got, err := helpdocs.Decode(w.Body.Bytes())
				if err != nil || got.Digest != b.Digest || w.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("body: %v", err)
				}
			} else if c.status != 403 && c.status != 405 && w.Body.Len() != 0 {
				t.Fatalf("unexpected body %q", w.Body.String())
			}
		})
	}
}

func TestHandlerCustomAuthorizerAndEmpty(t *testing.T) {
	h := helpdocs.NewHandler(nil, helpdocs.AllowServices("portal-gw"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, as("portal-gw", httptest.NewRequestWithContext(context.Background(), http.MethodGet, helpdocs.BundlePath, http.NoBody)))
	if w.Code != 200 || w.Header().Get("ETag") != "" {
		t.Fatalf("empty bundle: %d %v", w.Code, w.Header())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, as("gateway", httptest.NewRequestWithContext(context.Background(), http.MethodGet, helpdocs.BundlePath, http.NoBody)))
	if w.Code != 403 {
		t.Fatalf("gateway not in custom list: %d", w.Code)
	}
}

// TestHandlerOnFreyaServer mounts the handler the way a module does: on the
// Freya HTTP server, before the module's catch-all, and checks the mTLS peer
// identity reaches the authorizer.
func TestHandlerOnFreyaServer(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "ipam")
	srv, err := thttp.NewServer(rt, thttp.WithAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	b := bundle(t)
	srv.Handle(helpdocs.BundlePath, helpdocs.NewHandler(b, nil))
	srv.HandlePrefix("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "module api", http.StatusTeapot) }))
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()

	get := func(caller string) (int, string) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+ep.Host+helpdocs.BundlePath, http.NoBody)
		resp, err := testrt.HTTPClient(t, ca, caller, "ipam").Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, resp.Header.Get("ETag")
	}
	if code, etag := get("gateway"); code != 200 || etag != `"`+b.Digest+`"` {
		t.Fatalf("gateway: %d %q", code, etag)
	}
	if code, _ := get("orders"); code != 403 {
		t.Fatalf("orders: %d", code)
	}
}
