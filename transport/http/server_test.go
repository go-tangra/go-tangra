package http

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"
)

func TestServerLimitsAndOverrides(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	srv, err := NewServer(rt, WithAddress("127.0.0.1:0"), WithTimeout(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if srv.ks.Server.MaxHeaderBytes != 8<<10 || srv.ks.Server.ReadHeaderTimeout != rt.Limits().HandshakeTimeout ||
		srv.ks.Server.IdleTimeout != rt.Limits().IdleTimeout {
		t.Fatalf("limits not applied: %+v", srv.ks.Server)
	}
	if srv.timeout != rt.Limits().RequestTimeout {
		t.Fatalf("timeout must be clamped: %s", srv.timeout)
	}
	if srv.ks.Server.TLSConfig == nil || srv.ks.Server.TLSConfig.MinVersion != 0x0304 {
		t.Fatal("server must be TLS 1.3")
	}
}

func TestServerServesVerifiedPeerAndRejectsOversize(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	rt.Lim.MaxRequestBytes = 32
	srv, _ := NewServer(rt, WithAddress("127.0.0.1:0"))
	srv.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.FromContext(r.Context())
		if !ok {
			http.Error(w, "no peer", 500)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too big", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = io.WriteString(w, p.ServiceName+":"+string(body))
	})
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()
	client := testrt.HTTPClient(t, ca, "orders", "inventory")
	resp, err := client.Post("https://"+ep.Host+"/whoami", "text/plain", strings.NewReader("hi"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "orders:hi" || resp.Header.Get("x-request-id") == "" {
		t.Fatalf("status %d body %q headers %v", resp.StatusCode, body, resp.Header)
	}
	if resp.ProtoMajor != 2 {
		t.Fatalf("expected HTTP/2, got %s", resp.Proto)
	}
	resp, err = client.Post("https://"+ep.Host+"/whoami", "text/plain", strings.NewReader(strings.Repeat("x", 100)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize body: status %d", resp.StatusCode)
	}
	// Plain HTTP client is refused at the TLS layer.
	if _, err := (&http.Client{Timeout: time.Second}).Get("http://" + ep.Host + "/whoami"); err == nil {
		t.Fatal("plaintext must be refused")
	}
	// Admin-style paths never exist on the service listener.
	for _, p := range []string{"/metrics", "/healthz", "/readyz"} {
		resp, err := client.Get("https://" + ep.Host + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s served on the service listener: %d", p, resp.StatusCode)
		}
	}
}
