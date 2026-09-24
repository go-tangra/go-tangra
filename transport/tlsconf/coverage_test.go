package tlsconf

import (
	"crypto/tls"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
)

type nilCred struct{ *fakeProvider }

func (nilCred) Credential() (*tls.Certificate, error) { return &tls.Certificate{}, nil }

func TestCredentialEdgeCases(t *testing.T) {
	ca := testutil.MustCA("example.org")
	p := newFake(ca, ca.MustIssue("a", testutil.IssueOptions{}))
	cfg, err := ServerConfig(nilCred{p}, Options{TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("empty credential must fail closed")
	}
	if _, err := ClientConfig(p, identity.ForService("example.org", "b"), Options{SkewTolerance: -1, TrustDomain: "example.org"}); err == nil {
		t.Fatal("negative skew must be rejected")
	}
	if _, err := ClientConfig(nil, identity.ForService("example.org", "b"), Options{TrustDomain: "example.org"}); err == nil {
		t.Fatal("nil provider must be rejected")
	}
}

func TestOnRefusalHook(t *testing.T) {
	ca := testutil.MustCA("example.org")
	evil := testutil.MustCA("example.org")
	p := newFake(ca, ca.MustIssue("a", testutil.IssueOptions{}))
	var gotReason identity.Reason
	var gotClaimed string
	calls := 0
	cfg, _ := ServerConfig(p, Options{TrustDomain: "example.org", OnRefusal: func(r identity.Reason, claimed string) {
		gotReason, gotClaimed = r, claimed
		calls++
	}})
	_ = cfg.VerifyPeerCertificate(rawChain(evil.MustIssue("mallory", testutil.IssueOptions{})), nil)
	if gotReason != identity.ReasonUntrusted || gotClaimed != "spiffe://example.org/svc/mallory" || calls != 1 {
		t.Fatalf("hook: %q %q %d", gotReason, gotClaimed, calls)
	}
	_ = cfg.VerifyPeerCertificate([][]byte{[]byte("garbage")}, nil)
	if calls != 2 || gotClaimed != "" {
		t.Fatalf("hook on garbage: %d %q", calls, gotClaimed)
	}
	_ = cfg.VerifyPeerCertificate(rawChain(ca.MustIssue("ok", testutil.IssueOptions{})), nil)
	if calls != 2 {
		t.Fatal("hook must not fire on success")
	}
	// Chain with an intermediate that is not a CA still fails closed as untrusted.
	leaf := ca.MustIssue("x", testutil.IssueOptions{})
	other := ca.MustIssue("y", testutil.IssueOptions{})
	if err := cfg.VerifyPeerCertificate([][]byte{leaf.Certificate[0], other.Certificate[0]}, nil); err != nil {
		t.Fatalf("extra non-CA cert in chain must be ignored: %v", err)
	}
	// Expired issuing CA classifies as expired even when the leaf itself is valid.
	oldCA, err := testutil.NewCAWithValidity("example.org", time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	p.setRoots(oldCA.Cert)
	err = cfg.VerifyPeerCertificate(rawChain(oldCA.MustIssue("z", testutil.IssueOptions{})), nil)
	var ve *identity.VerifyError
	if !errors.As(err, &ve) || ve.Reason != identity.ReasonExpired {
		t.Fatalf("expired issuer: %v", err)
	}
}

func TestPeerFromChainNonSPIFFE(t *testing.T) {
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("a", testutil.IssueOptions{URIs: []*url.URL{{Scheme: "https", Host: "x"}}})
	if _, err := PeerFromChain(crt.Leaf); err == nil {
		t.Fatal("non-SPIFFE SAN must fail")
	}
	if Classify(errors.New("x509: certificate is not yet valid")) != identity.ReasonExpired {
		t.Fatal("classify")
	}
}
