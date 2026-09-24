package tlsconf

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
)

// fakeProvider is an in-memory identity.Provider + cred.Source.
type fakeProvider struct {
	mu     sync.Mutex
	crt    tls.Certificate
	roots  []*x509.Certificate
	td     string
	ver    uint64
	fail   error
	closed bool
}

func newFake(ca *testutil.CA, crt tls.Certificate) *fakeProvider {
	return &fakeProvider{crt: crt, roots: []*x509.Certificate{ca.Cert}, td: ca.TrustDomain, ver: 1}
}

type fakeIdentity struct{ leaf *x509.Certificate }

func (f fakeIdentity) ID() identity.SPIFFEID {
	id, _ := identity.ParseSPIFFEID(f.leaf.URIs[0].String())
	return id
}
func (f fakeIdentity) NotBefore() time.Time { return f.leaf.NotBefore }
func (f fakeIdentity) NotAfter() time.Time  { return f.leaf.NotAfter }
func (f fakeIdentity) Serial() string       { return f.leaf.SerialNumber.String() }

type fakeBundle struct {
	td    string
	roots []*x509.Certificate
	ver   uint64
}

func (b fakeBundle) TrustDomain() string        { return b.td }
func (b fakeBundle) Roots() []*x509.Certificate { return b.roots }
func (b fakeBundle) Version() uint64            { return b.ver }

func (p *fakeProvider) Current(context.Context) (identity.Identity, identity.Bundle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail != nil {
		return nil, nil, p.fail
	}
	return fakeIdentity{p.crt.Leaf}, fakeBundle{p.td, p.roots, p.ver}, nil
}
func (p *fakeProvider) Watch(context.Context) (<-chan identity.Update, error) {
	return make(chan identity.Update), nil
}
func (p *fakeProvider) Close() error { p.closed = true; return nil }
func (p *fakeProvider) Credential() (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail != nil {
		return nil, p.fail
	}
	c := p.crt
	return &c, nil
}
func (p *fakeProvider) rotate(crt tls.Certificate) { p.mu.Lock(); p.crt = crt; p.mu.Unlock() }
func (p *fakeProvider) setRoots(r ...*x509.Certificate) {
	p.mu.Lock()
	p.roots = r
	p.ver++
	p.mu.Unlock()
}

func TestServerConfigShape(t *testing.T) {
	ca := testutil.MustCA("example.org")
	p := newFake(ca, ca.MustIssue("inventory", testutil.IssueOptions{}))
	cfg, err := ServerConfig(p, Options{TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS13 || cfg.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("TLS version pin wrong: %d..%d", cfg.MinVersion, cfg.MaxVersion)
	}
	if cfg.ClientAuth != tls.RequireAnyClientCert || cfg.VerifyPeerCertificate == nil {
		t.Fatal("client certificates must be required and verified by Freya's verifier")
	}
	if cfg.GetCertificate == nil || cfg.Certificates != nil {
		t.Fatal("certificate must come from the provider callback only")
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "h2" {
		t.Fatalf("ALPN: %v", cfg.NextProtos)
	}
	if cfg.SessionTicketsDisabled != true {
		t.Fatal("session tickets must be disabled (no resumption across rotation)")
	}
	if _, err := ServerConfig(nil, Options{TrustDomain: "example.org"}); err == nil {
		t.Fatal("nil provider must be rejected")
	}
	if _, err := ServerConfig(p, Options{}); err == nil {
		t.Fatal("missing trust domain must be rejected")
	}
	if _, err := ServerConfig(p, Options{TrustDomain: "example.org", SkewTolerance: 20 * time.Minute}); err == nil {
		t.Fatal("skew above 15m must be rejected")
	}
	type notSource struct{ identity.Provider }
	if _, err := ServerConfig(notSource{p}, Options{TrustDomain: "example.org"}); err == nil {
		t.Fatal("provider without credential access must be rejected")
	}
}

func TestClientConfigShape(t *testing.T) {
	ca := testutil.MustCA("example.org")
	p := newFake(ca, ca.MustIssue("orders", testutil.IssueOptions{}))
	cfg, err := ClientConfig(p, identity.ForService("example.org", "inventory"), Options{TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS13 || cfg.MaxVersion != tls.VersionTLS13 {
		t.Fatal("TLS version pin wrong")
	}
	if !cfg.InsecureSkipVerify || cfg.VerifyPeerCertificate == nil {
		t.Fatal("client must bypass hostname verification and use the SPIFFE verifier")
	}
	if cfg.GetClientCertificate == nil {
		t.Fatal("client certificate callback missing")
	}
	if _, err := ClientConfig(p, identity.SPIFFEID{}, Options{TrustDomain: "example.org"}); err == nil {
		t.Fatal("zero expected peer must be rejected")
	}
	if _, err := ClientConfig(p, identity.ForService("other.org", "inventory"), Options{TrustDomain: "example.org"}); err == nil {
		t.Fatal("expected peer outside trust domain must be rejected")
	}
}

func TestCertificateCallbacksFollowRotation(t *testing.T) {
	ca := testutil.MustCA("example.org")
	first := ca.MustIssue("orders", testutil.IssueOptions{})
	p := newFake(ca, first)
	s, _ := ServerConfig(p, Options{TrustDomain: "example.org"})
	c, _ := ClientConfig(p, identity.ForService("example.org", "x"), Options{TrustDomain: "example.org"})
	got, err := s.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil || got.Leaf.SerialNumber.Cmp(first.Leaf.SerialNumber) != 0 {
		t.Fatalf("server cert: %v %v", got, err)
	}
	second := ca.MustIssue("orders", testutil.IssueOptions{})
	p.rotate(second)
	got, _ = s.GetCertificate(&tls.ClientHelloInfo{})
	if got.Leaf.SerialNumber.Cmp(second.Leaf.SerialNumber) != 0 {
		t.Fatal("server callback did not follow rotation")
	}
	got, _ = c.GetClientCertificate(&tls.CertificateRequestInfo{})
	if got.Leaf.SerialNumber.Cmp(second.Leaf.SerialNumber) != 0 {
		t.Fatal("client callback did not follow rotation")
	}
	p.fail = errors.New("provider down")
	if _, err := s.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("provider failure must fail the handshake (fail closed)")
	}
	if _, err := c.GetClientCertificate(&tls.CertificateRequestInfo{}); err == nil {
		t.Fatal("provider failure must fail the client handshake (fail closed)")
	}
}

func rawChain(crt tls.Certificate) [][]byte { return crt.Certificate }

func TestVerifyPeerMatrix(t *testing.T) {
	ca := testutil.MustCA("example.org")
	evil := testutil.MustCA("example.org")
	now := time.Now()
	p := newFake(ca, ca.MustIssue("inventory", testutil.IssueOptions{}))
	opts := Options{TrustDomain: "example.org", SkewTolerance: 5 * time.Minute, Now: func() time.Time { return now }}
	srv, _ := ServerConfig(p, opts)
	cli, _ := ClientConfig(p, identity.ForService("example.org", "inventory"), opts)

	cases := []struct {
		name   string
		crt    tls.Certificate
		reason identity.Reason
		client bool
	}{
		{"valid", ca.MustIssue("orders", testutil.IssueOptions{}), "", false},
		{"valid callee", ca.MustIssue("inventory", testutil.IssueOptions{}), "", true},
		{"untrusted ca", evil.MustIssue("orders", testutil.IssueOptions{}), identity.ReasonUntrusted, false},
		{"expired", ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-10 * time.Minute)}), identity.ReasonExpired, false},
		{"expired within skew ok", ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-2 * time.Minute)}), "", false},
		{"not yet valid", ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(10 * time.Minute), NotAfter: now.Add(time.Hour)}), identity.ReasonNotYetValid, false},
		{"not yet valid within skew ok", ca.MustIssue("orders", testutil.IssueOptions{NotBefore: now.Add(2 * time.Minute), NotAfter: now.Add(time.Hour)}), "", false},
		{"wrong trust domain", ca.MustIssue("orders", testutil.IssueOptions{TrustDomain: "evil.org"}), identity.ReasonUntrusted, false},
		{"no SAN", ca.MustIssue("orders", testutil.IssueOptions{NoSAN: true}), identity.ReasonUntrusted, false},
		{"two SANs", ca.MustIssue("orders", testutil.IssueOptions{URIs: []*url.URL{
			{Scheme: "spiffe", Host: "example.org", Path: "/svc/orders"},
			{Scheme: "spiffe", Host: "example.org", Path: "/svc/billing"},
		}}), identity.ReasonUntrusted, false},
		{"non-spiffe SAN", ca.MustIssue("orders", testutil.IssueOptions{URIs: []*url.URL{{Scheme: "https", Host: "example.org"}}}), identity.ReasonUntrusted, false},
		{"callee name mismatch", ca.MustIssue("billing", testutil.IssueOptions{}), identity.ReasonNameMismatch, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := srv
			if tc.client {
				cfg = cli
			}
			err := cfg.VerifyPeerCertificate(rawChain(tc.crt), nil)
			if tc.reason == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *identity.VerifyError
			if !errors.As(err, &ve) {
				t.Fatalf("expected VerifyError, got %T %v", err, err)
			}
			if ve.Reason != tc.reason {
				t.Fatalf("reason %q, want %q", ve.Reason, tc.reason)
			}
			if strings.Contains(err.Error(), "BEGIN") || len(err.Error()) > 300 {
				t.Fatalf("error must not carry certificate material: %q", err)
			}
		})
	}
	// Empty chain and garbage DER.
	if err := srv.VerifyPeerCertificate(nil, nil); err == nil {
		t.Fatal("empty chain must be refused")
	}
	if err := srv.VerifyPeerCertificate([][]byte{[]byte("garbage")}, nil); err == nil {
		t.Fatal("garbage DER must be refused")
	}
	// Bundle rotation is honoured without rebuilding the config.
	p.setRoots(evil.Cert)
	if err := srv.VerifyPeerCertificate(rawChain(evil.MustIssue("orders", testutil.IssueOptions{})), nil); err != nil {
		t.Fatalf("new root not honoured: %v", err)
	}
	if err := srv.VerifyPeerCertificate(rawChain(ca.MustIssue("orders", testutil.IssueOptions{})), nil); err == nil {
		t.Fatal("removed root still trusted")
	}
	p.setRoots()
	if err := srv.VerifyPeerCertificate(rawChain(evil.MustIssue("orders", testutil.IssueOptions{})), nil); err == nil {
		t.Fatal("empty bundle must fail closed")
	}
	p.fail = errors.New("down")
	if err := srv.VerifyPeerCertificate(rawChain(evil.MustIssue("orders", testutil.IssueOptions{})), nil); err == nil {
		t.Fatal("provider failure must fail closed")
	}
}

func TestPeerFromChains(t *testing.T) {
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	peer, err := PeerFromChain(crt.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	if peer.ID.String() != "spiffe://example.org/svc/orders" || peer.ServiceName != "orders" || peer.Serial == "" {
		t.Fatalf("bad peer %+v", peer)
	}
	if _, err := PeerFromChain(nil); err == nil {
		t.Fatal("nil leaf must fail")
	}
}

func TestEndToEndHandshakeAndDowngrade(t *testing.T) {
	ca := testutil.MustCA("example.org")
	srvP := newFake(ca, ca.MustIssue("inventory", testutil.IssueOptions{}))
	cliP := newFake(ca, ca.MustIssue("orders", testutil.IssueOptions{}))
	opts := Options{TrustDomain: "example.org"}
	scfg, _ := ServerConfig(srvP, opts)
	ccfg, _ := ClientConfig(cliP, identity.ForService("example.org", "inventory"), opts)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", scfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	results := make(chan error, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				results <- c.(*tls.Conn).Handshake()
			}(c)
		}
	}()
	dialAddr := func(addr string, cfg *tls.Config) error {
		c, err := tls.Dial("tcp", addr, cfg)
		if err != nil {
			return err
		}
		defer c.Close()
		st := c.ConnectionState()
		if st.Version != tls.VersionTLS13 || st.NegotiatedProtocol != "h2" {
			return errors.New("negotiated wrong version/protocol")
		}
		return nil
	}
	dial := func(cfg *tls.Config) error { return dialAddr(ln.Addr().String(), cfg) }
	if err := dial(ccfg); err != nil {
		t.Fatalf("valid mTLS handshake failed: %v", err)
	}
	if err := <-results; err != nil {
		t.Fatalf("server side: %v", err)
	}
	// TLS 1.2-only client is refused with a protocol version alert.
	old := ccfg.Clone()
	old.MinVersion, old.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
	if err := dial(old); err == nil || !strings.Contains(err.Error(), "protocol version") {
		t.Fatalf("expected protocol version refusal, got %v", err)
	}
	<-results
	// Client without certificate is refused.
	nocert := ccfg.Clone()
	nocert.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &tls.Certificate{}, nil }
	// In TLS 1.3 the client finishes its handshake before the server evaluates the
	// (missing) certificate, so the refusal is observed on the server side.
	_ = dial(nocert)
	if err := <-results; err == nil || Classify(err) != identity.ReasonNoIdentity {
		t.Fatalf("server should have refused the missing client certificate, got %v", err)
	}
	// Client refuses a rogue callee (valid cert, wrong name).
	rogue := newFake(ca, ca.MustIssue("billing", testutil.IssueOptions{}))
	rcfg, _ := ServerConfig(rogue, opts)
	rl, _ := tls.Listen("tcp", "127.0.0.1:0", rcfg)
	defer rl.Close()
	go func() {
		c, err := rl.Accept()
		if err == nil {
			_ = c.(*tls.Conn).Handshake()
			c.Close()
		}
	}()
	err = dialAddr(rl.Addr().String(), ccfg)
	var ve *identity.VerifyError
	if !errors.As(err, &ve) || ve.Reason != identity.ReasonNameMismatch {
		t.Fatalf("client must refuse rogue callee with name_mismatch, got %v", err)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]identity.Reason{
		"tls: client didn't provide a certificate":                                         identity.ReasonNoIdentity,
		"remote error: tls: certificate required":                                          identity.ReasonNoIdentity,
		"tls: client offered only unsupported versions: [303]":                             identity.ReasonDowngrade,
		"remote error: tls: protocol version not supported":                                identity.ReasonDowngrade,
		"tls: failed to verify certificate: x509: certificate signed by unknown authority": identity.ReasonUntrusted,
		"x509: certificate has expired or is not yet valid":                                identity.ReasonExpired,
		"EOF": identity.ReasonUntrusted,
	}
	for in, want := range cases {
		if got := Classify(errors.New(in)); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
	if got := Classify(&identity.VerifyError{Reason: identity.ReasonNameMismatch}); got != identity.ReasonNameMismatch {
		t.Errorf("VerifyError not passed through: %q", got)
	}
	if Classify(nil) != "" {
		t.Error("nil error must classify to empty")
	}
}
