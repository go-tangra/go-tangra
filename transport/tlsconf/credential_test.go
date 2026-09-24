package tlsconf

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
)

type rawCred struct {
	*testutil.MemProvider
	crt tls.Certificate
}

func (r rawCred) Credential() (*tls.Certificate, error) { c := r.crt; return &c, nil }

func TestCredentialParsesLeafAndRejectsNotYetValid(t *testing.T) {
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("a", testutil.IssueOptions{})
	noLeaf := crt
	noLeaf.Leaf = nil
	if got, err := credential(rawCred{testutil.NewMemProvider(ca, crt), noLeaf}); err != nil || got == nil {
		t.Fatalf("leaf parse: %v", err)
	}
	bad := crt
	bad.Leaf = nil
	bad.Certificate = [][]byte{[]byte("not-der")}
	if _, err := credential(rawCred{testutil.NewMemProvider(ca, crt), bad}); err == nil {
		t.Fatal("malformed credential must fail")
	}
	future := ca.MustIssue("a", testutil.IssueOptions{NotBefore: time.Now().Add(time.Hour), NotAfter: time.Now().Add(2 * time.Hour)})
	if _, err := credential(rawCred{testutil.NewMemProvider(ca, crt), future}); err == nil {
		t.Fatal("not-yet-valid credential must not be presented")
	}
}
