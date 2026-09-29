package edge

import (
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
)

func TestConnectSourcesDefaultUnchanged(t *testing.T) {
	h, nonce := cspOf(t, &Config{ConnectSources: nil})
	if got := h.Get("Content-Security-Policy"); got != v421CSP(nonce) {
		t.Fatalf("default CSP changed:\n got %q\nwant %q", got, v421CSP(nonce))
	}
}

func TestConnectSourcesEmitted(t *testing.T) {
	h, nonce := cspOf(t, &Config{
		ConnectSources: []string{"https://localhost:53952", "https://LOCALHOST:53953/"},
		FrameSources:   []string{"https://kvm.example.com"},
		CSPExtra:       "style-src-attr 'none'",
	})
	csp := h.Get("Content-Security-Policy")
	want := "font-src 'self'; connect-src 'self' https://localhost:53952 https://localhost:53953; frame-src 'self' https://kvm.example.com; frame-ancestors 'none'"
	if !strings.Contains(csp, want) {
		t.Fatalf("csp %q lacks %q", csp, want)
	}
	if !strings.HasSuffix(csp, "; style-src-attr 'none'") || !strings.Contains(csp, "script-src 'self' 'nonce-"+nonce+"'") ||
		strings.Contains(csp, "script-src 'self' 'nonce-"+nonce+"' https://localhost") {
		t.Fatalf("connect sources leaked into other directives or the nonce was lost: %q", csp)
	}
}

func TestConnectSourcesRefused(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "gateway")
	for _, bad := range []string{
		"", "*", "https://", "http://localhost:53952", "https://h.example/path", "https://h.example?q=1",
		"https://u@h.example", "https://h.example; script-src *", "https://h.example'", "wss://h.example", "%zz",
	} {
		_, err := NewServer(rt, Config{Addr: "127.0.0.1:0", Env: "test", ConnectSources: []string{bad}})
		if err == nil || !strings.Contains(err.Error(), "connect source") {
			t.Errorf("connect source %q: err = %v, want refusal", bad, err)
		}
	}
}
