package tlsconf

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
)

func TestRotationKeepsExistingConnectionsAndRefusesExpiredLocal(t *testing.T) {
	ca := testutil.MustCA("example.org")
	srvP := testutil.NewMemProvider(ca, ca.MustIssue("inventory", testutil.IssueOptions{}))
	cliP := testutil.NewMemProvider(ca, ca.MustIssue("orders", testutil.IssueOptions{}))
	opts := Options{TrustDomain: "example.org"}
	scfg, _ := ServerConfig(srvP, opts)
	ccfg, _ := ClientConfig(cliP, identity.ForService("example.org", "inventory"), opts)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", scfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				tc := c.(*tls.Conn)
				if tc.Handshake() == nil {
					buf := make([]byte, 4)
					for {
						if _, err := tc.Read(buf); err != nil {
							break
						}
						_, _ = tc.Write(buf)
					}
				}
				c.Close()
			}(c)
		}
	}()
	first, err := tls.Dial("tcp", ln.Addr().String(), ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	echo := func(c *tls.Conn) error {
		if _, err := c.Write([]byte("ping")); err != nil {
			return err
		}
		buf := make([]byte, 4)
		_, err := c.Read(buf)
		return err
	}
	if err := echo(first); err != nil {
		t.Fatal(err)
	}
	// Rotate both sides: the existing connection keeps working; a new one uses the new serial.
	newSrv := ca.MustIssue("inventory", testutil.IssueOptions{})
	srvP.Rotate(newSrv)
	cliP.Rotate(ca.MustIssue("orders", testutil.IssueOptions{}))
	if err := echo(first); err != nil {
		t.Fatalf("existing connection broken by rotation: %v", err)
	}
	second, err := tls.Dial("tcp", ln.Addr().String(), ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if got := second.ConnectionState().PeerCertificates[0].SerialNumber; got.Cmp(newSrv.Leaf.SerialNumber) != 0 {
		t.Fatalf("new connection did not get the rotated server certificate")
	}
	// An expired local credential is never presented: the handshake fails closed.
	cliP.Rotate(ca.MustIssue("orders", testutil.IssueOptions{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}))
	if _, err := ccfg.GetClientCertificate(&tls.CertificateRequestInfo{}); err == nil {
		t.Fatal("expired local credential must not be presented")
	}
	srvP.Rotate(ca.MustIssue("inventory", testutil.IssueOptions{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}))
	if _, err := scfg.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("expired local server credential must not be presented")
	}
}
