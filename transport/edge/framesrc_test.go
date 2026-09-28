package edge

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
)

// v421CSP is the policy every edge served before frame sources existed; with
// no frame sources it must stay byte-for-byte the same.
func v421CSP(nonce string) string {
	return "default-src 'self'; script-src 'self' 'nonce-" + nonce + "'; style-src 'self' 'nonce-" + nonce + "'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'self'"
}

func cspOf(t *testing.T, cfg Config) (http.Header, string) {
	t.Helper()
	srv, client, base := devServer(t, cfg)
	var nonce string
	srv.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) { nonce = Nonce(r.Context()) })
	resp, err := client.Get(base + "/x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.Header, nonce
}

func TestFrameSourcesDefaultUnchanged(t *testing.T) {
	h, nonce := cspOf(t, Config{})
	if got := h.Get("Content-Security-Policy"); got != v421CSP(nonce) {
		t.Fatalf("default CSP changed:\n got %q\nwant %q", got, v421CSP(nonce))
	}
}

func TestFrameSourcesEmitted(t *testing.T) {
	h, nonce := cspOf(t, Config{FrameSources: []string{"https://h.example:8444", "https://kvm.example.com/"}, CSPExtra: "style-src-attr 'none'"})
	csp := h.Get("Content-Security-Policy")
	want := "connect-src 'self'; frame-src 'self' https://h.example:8444 https://kvm.example.com; frame-ancestors 'none'"
	if !strings.Contains(csp, want) {
		t.Fatalf("csp %q lacks %q", csp, want)
	}
	if !strings.HasSuffix(csp, "; style-src-attr 'none'") || !strings.Contains(csp, "'nonce-"+nonce+"'") {
		t.Fatalf("extra/nonce lost: %q", csp)
	}
	if h.Get("X-Frame-Options") != "DENY" || h.Get("Cross-Origin-Opener-Policy") != "same-origin" || h.Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Fatalf("other headers changed: %v", h)
	}
}

func TestFrameSourcesRefused(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "gateway")
	for _, bad := range []string{
		"", "*", "https://", "http://h.example", "https://h.example/path", "https://h.example?q=1",
		"https://h.example#f", "https://u@h.example", "https://h.example; script-src *", "https://h.example'",
		"https://h.example, https://x", "https://h .example", "wss://h.example", "https://h.example:notaport", "https://h.example:", "%zz",
	} {
		_, err := NewServer(rt, Config{Addr: "127.0.0.1:0", Env: "test", FrameSources: []string{bad}})
		if err == nil || !strings.Contains(err.Error(), "frame source") {
			t.Errorf("frame source %q: err = %v, want refusal", bad, err)
		}
	}
}
