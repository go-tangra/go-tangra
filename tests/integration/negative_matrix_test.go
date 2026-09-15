package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/transport/tlsconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestNegativeMatrix(t *testing.T) {
	f := newFixture(t, "inventory")
	logs := &testutil.LogCapture{}
	revoked := authn.NewMemoryRevocationChecker()
	inv, addr, stop := f.startApp(t, f.config("inventory"), freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()), freya.WithRevocationChecker(revoked))
	defer stop()
	_ = inv
	ca := f.ca
	evil := testutil.MustCA("example.org")
	now := time.Now()
	revokedCrt := ca.MustIssue("orders", testutil.IssueOptions{})
	revoked.Revoke(identity.ForService("example.org", "orders"), revokedCrt.Leaf.SerialNumber.String())

	// A rogue client presents whatever certificate it has, bypassing Freya's own
	// fail-closed credential checks; the server must still refuse it.
	clientCfg := func(crt tls.Certificate) *tls.Config {
		return &tls.Config{
			Certificates: []tls.Certificate{crt}, InsecureSkipVerify: true, //nolint:gosec // rogue-client simulation
			MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, NextProtos: []string{"h2"},
		}
	}
	freyaClient := func(crt tls.Certificate) *tls.Config {
		p := testutil.NewMemProvider(ca, crt)
		cfg, err := tlsconf.ClientConfig(p, identity.ForService("example.org", "inventory"), tlsconf.Options{TrustDomain: "example.org"})
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	valid := ca.MustIssue("orders", testutil.IssueOptions{})

	cases := []struct {
		name      string
		cfg       func() *tls.Config
		reason    string
		eventType string
		viaRPC    bool // refusal happens after the handshake (middleware)
	}{
		{"no client cert", func() *tls.Config {
			c := clientCfg(valid)
			c.Certificates = nil
			return c
		}, "no_identity", "authn_refused", false},
		{"expired", func() *tls.Config {
			return clientCfg(ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-20 * time.Minute)}))
		}, "identity_expired", "authn_refused", false},
		{"not yet valid", func() *tls.Config {
			return clientCfg(ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(20 * time.Minute), NotAfter: now.Add(time.Hour)}))
		}, "identity_not_yet_valid", "authn_refused", false},
		{"untrusted ca", func() *tls.Config { return clientCfg(evil.MustIssue("orders", testutil.IssueOptions{})) }, "untrusted", "authn_refused", false},
		{"wrong trust domain", func() *tls.Config {
			return clientCfg(ca.MustIssue("orders", testutil.IssueOptions{TrustDomain: "evil.org"}))
		}, "untrusted", "authn_refused", false},
		{"missing SAN", func() *tls.Config { return clientCfg(ca.MustIssue("orders", testutil.IssueOptions{NoSAN: true})) }, "untrusted", "authn_refused", false},
		{"two SANs", func() *tls.Config {
			return clientCfg(ca.MustIssue("orders", testutil.IssueOptions{URIs: []*url.URL{
				{Scheme: "spiffe", Host: "example.org", Path: "/svc/orders"}, {Scheme: "spiffe", Host: "example.org", Path: "/svc/billing"}}}))
		}, "untrusted", "authn_refused", false},
		{"tls 1.2 client", func() *tls.Config {
			c := clientCfg(valid)
			c.MinVersion, c.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
			return c
		}, "downgrade_refused", "channel_refused_downgrade", false},
		{"revoked", func() *tls.Config { return clientCfg(revokedCrt) }, "identity_revoked", "authn_refused", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := logs.Count("reason", tc.reason)
			cfg := tc.cfg()
			if tc.viaRPC {
				conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
				if status.Code(err) != codes.Unauthenticated {
					t.Fatalf("want Unauthenticated, got %v", err)
				}
			} else {
				c, err := tls.Dial("tcp", addr, cfg)
				if err == nil {
					// TLS 1.3 may complete the client side first; the server's alert
					// arrives on the first read.
					_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
					buf := make([]byte, 1)
					if _, rerr := c.Read(buf); rerr == nil {
						t.Fatal("server accepted an invalid peer")
					}
					c.Close()
				}
			}
			// Exactly one audit event with the expected reason and type.
			deadline := time.Now().Add(3 * time.Second)
			for logs.Count("reason", tc.reason) < before+1 && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			if n := logs.Count("reason", tc.reason) - before; n != 1 {
				t.Fatalf("expected exactly one %s event, got %d\n%s", tc.reason, n, logs.String())
			}
			if logs.Count("type", tc.eventType) == 0 {
				t.Fatalf("expected event type %s", tc.eventType)
			}
			if tc.reason == "identity_expired" || tc.reason == "identity_not_yet_valid" {
				found := false
				for _, l := range logs.Lines() {
					if l["reason"] == tc.reason && strings.Contains(fmt.Sprint(l["attr_detail"]), "clock skew tolerance") {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected the skew hint in attr_detail for %s:\n%s", tc.reason, logs.String())
				}
			}
		})
	}
	// Valid identity (through the real Freya client config) still works after the whole matrix.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(freyaClient(valid))))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("valid peer refused: %v", err)
	}
	for _, line := range logs.Lines() {
		if line["msg"] == "audit" && (line["type"] == "authn_refused" || line["type"] == "channel_refused_downgrade") {
			if _, ok := line["remote_addr"]; !ok {
				t.Fatalf("refusal event without remote_addr: %v", line)
			}
		}
	}
}
