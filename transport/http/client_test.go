package http

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"
)

func TestNewClientMutualTLSAndPinning(t *testing.T) {
	ca := testutil.MustCA("example.org")
	srvRT := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(srvRT, WithAddress("127.0.0.1:0"))
	srv.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		p, _ := authn.FromContext(r.Context())
		if r.TLS == nil || r.TLS.Version != tls.VersionTLS13 {
			http.Error(w, "not tls1.3", 500)
			return
		}
		_, _ = io.WriteString(w, p.ServiceName)
	})
	srv.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/", http.StatusFound)
	})
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()

	cliRT := testrt.New(t, ca, "orders")
	inventory, _ := identity.NewSPIFFEID("example.org", "inventory")
	c, err := NewClient(cliRT, inventory)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Get("https://" + ep.Host + "/whoami")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "orders" {
		t.Fatalf("%d %q: server must see the client SVID", resp.StatusCode, body)
	}
	// Redirects are never followed.
	resp, err = c.Get("https://" + ep.Host + "/redirect")
	if err != nil || resp.StatusCode != http.StatusFound {
		t.Fatalf("redirect followed: %v %v", resp, err)
	}
	_ = resp.Body.Close()

	// Pinned to the wrong identity: the handshake is refused and audited.
	billing, _ := identity.NewSPIFFEID("example.org", "billing")
	wrong, err := NewClient(cliRT, billing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Get("https://" + ep.Host + "/whoami"); err == nil {
		t.Fatal("connection to a peer with another identity must be refused")
	}
	ev := cliRT.Events.Next(t, 2*time.Second)
	if ev.Type != audit.TypeAuthnRefused || ev.Outcome != audit.OutcomeRefused || ev.ClaimedPeerID != inventory.String() || !strings.Contains(ev.Operation, "billing") {
		t.Fatalf("refusal not audited: %+v", ev)
	}
	// Foreign trust domain and zero identity are refused at construction.
	foreign, _ := identity.NewSPIFFEID("other.org", "inventory")
	if _, err := NewClient(cliRT, foreign); err == nil {
		t.Fatal("foreign trust domain accepted")
	}
	if _, err := NewClient(cliRT, identity.SPIFFEID{}); err == nil {
		t.Fatal("zero identity accepted")
	}
}

func TestNewClientTimeoutsAndLocalIdentity(t *testing.T) {
	ca := testutil.MustCA("example.org")
	srvRT := testrt.New(t, ca, "inventory")
	srv, _ := NewServer(srvRT, WithAddress("127.0.0.1:0"))
	srv.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	stop := testrt.StartServer(t, srv)
	defer stop()
	ep, _ := srv.Endpoint()
	cliRT := testrt.New(t, ca, "orders")
	cliRT.Lim.RequestTimeout = 100 * time.Millisecond
	inventory, _ := identity.NewSPIFFEID("example.org", "inventory")
	c, _ := NewClient(cliRT, inventory)
	if RequestTimeout(cliRT) != 100*time.Millisecond {
		t.Fatal("request timeout")
	}
	start := time.Now()
	if _, err := c.Get("https://" + ep.Host + "/slow"); err == nil || time.Since(start) > time.Second {
		t.Fatalf("response header timeout not applied: %v after %s", err, time.Since(start))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+ep.Host+"/slow", nil)
	if _, err := c.Do(req); !errors.Is(err, context.DeadlineExceeded) && err == nil {
		t.Fatal("context deadline ignored")
	}
	// Without a valid local identity no client is handed out.
	cliRT.Prov = testutil.NewMemProvider(ca, ca.MustIssue("orders", testutil.IssueOptions{NotAfter: time.Now().Add(-time.Hour)}))
	if _, err := NewClient(cliRT, inventory); err == nil {
		t.Fatal("client without local identity")
	}
}
