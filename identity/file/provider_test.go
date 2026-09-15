package file

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/cred"
	"github.com/go-freya/freya/internal/testutil"
)

func setup(t *testing.T) (*testutil.CA, string, string, string) {
	t.Helper()
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	c, k, b, err := ca.WriteSVID(t.TempDir(), "orders", crt)
	if err != nil {
		t.Fatal(err)
	}
	return ca, c, k, b
}

func TestLoadsValidIdentity(t *testing.T) {
	_, c, k, b := setup(t)
	p, err := New(Config{Cert: c, Key: k, Bundle: b, TrustDomain: "example.org"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	id, bundle, err := p.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id.ID().String() != "spiffe://example.org/svc/orders" || id.Serial() == "" {
		t.Fatalf("unexpected identity %v", id.ID())
	}
	if id.NotAfter().Before(id.NotBefore()) {
		t.Fatal("bad validity")
	}
	if bundle.TrustDomain() != "example.org" || len(bundle.Roots()) != 1 || bundle.Version() == 0 {
		t.Fatalf("unexpected bundle %+v", bundle)
	}
	// Key material is reachable only through the internal credential interface.
	var src cred.Source = p
	crt, err := src.Credential()
	if err != nil || crt == nil || crt.PrivateKey == nil {
		t.Fatalf("credential: %v %v", crt, err)
	}
	var _ identity.Provider = p
}

func TestRejectsBrokenInputs(t *testing.T) {
	ca, c, k, b := setup(t)
	other := testutil.MustCA("example.org")
	otherCrt := other.MustIssue("orders", testutil.IssueOptions{})
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	noSAN := ca.MustIssue("orders", testutil.IssueOptions{NoSAN: true})
	twoSAN := ca.MustIssue("orders", testutil.IssueOptions{URIs: []*url.URL{
		{Scheme: "spiffe", Host: "example.org", Path: "/svc/orders"},
		{Scheme: "spiffe", Host: "example.org", Path: "/svc/billing"},
	}})
	wrongTD := ca.MustIssue("orders", testutil.IssueOptions{TrustDomain: "evil.org"})
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"missing cert", Config{Cert: filepath.Join(dir, "nope.pem"), Key: k, Bundle: b, TrustDomain: "example.org"}, "cert"},
		{"missing key", Config{Cert: c, Key: filepath.Join(dir, "nope.key"), Bundle: b, TrustDomain: "example.org"}, "key"},
		{"missing bundle", Config{Cert: c, Key: k, Bundle: filepath.Join(dir, "nope.pem"), TrustDomain: "example.org"}, "bundle"},
		{"key mismatch", Config{Cert: c, Key: write("other.key", testutil.KeyPEM(otherCrt)), Bundle: b, TrustDomain: "example.org"}, "private key"},
		{"no SAN", Config{Cert: write("nosan.pem", testutil.CertPEM(noSAN)), Key: write("nosan.key", testutil.KeyPEM(noSAN)), Bundle: b, TrustDomain: "example.org"}, "SAN"},
		{"two SANs", Config{Cert: write("two.pem", testutil.CertPEM(twoSAN)), Key: write("two.key", testutil.KeyPEM(twoSAN)), Bundle: b, TrustDomain: "example.org"}, "SAN"},
		{"wrong trust domain", Config{Cert: write("td.pem", testutil.CertPEM(wrongTD)), Key: write("td.key", testutil.KeyPEM(wrongTD)), Bundle: b, TrustDomain: "example.org"}, "trust domain"},
		{"empty bundle", Config{Cert: c, Key: k, Bundle: write("empty.pem", []byte("")), TrustDomain: "example.org"}, "bundle"},
		{"garbage cert", Config{Cert: write("g.pem", []byte("garbage")), Key: k, Bundle: b, TrustDomain: "example.org"}, "cert"},
		{"missing trust domain", Config{Cert: c, Key: k, Bundle: b}, "trust domain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.cfg)
			if err == nil {
				p.Close()
				t.Fatal("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestWatchDetectsRotation(t *testing.T) {
	ca, c, k, b := setup(t)
	p, err := New(Config{Cert: c, Key: k, Bundle: b, TrustDomain: "example.org", PollInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := p.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := p.Current(ctx)
	time.Sleep(30 * time.Millisecond)
	next := ca.MustIssue("orders", testutil.IssueOptions{})
	if _, _, _, err := ca.WriteSVID(filepath.Dir(c), "orders", next); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-ch:
		if u.Err != nil {
			t.Fatal(u.Err)
		}
		if u.Identity.Serial() == first.Serial() {
			t.Fatal("update carried the old serial")
		}
	case <-ctx.Done():
		t.Fatal("no update observed")
	}
	cur, _, _ := p.Current(ctx)
	if cur.Serial() == first.Serial() {
		t.Fatal("Current not swapped")
	}
	// A broken rewrite must not replace the last good identity; it surfaces as Update.Err.
	if err := os.WriteFile(c, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-ch:
		if u.Err == nil {
			t.Fatal("expected error update")
		}
	case <-ctx.Done():
		t.Fatal("no error update observed")
	}
	still, _, err := p.Current(ctx)
	if err != nil || still.Serial() != cur.Serial() {
		t.Fatalf("last-known-good lost: %v %v", still, err)
	}
	// Restore a valid certificate, then change the bundle: the version increments.
	if _, _, _, err := ca.WriteSVID(filepath.Dir(c), "orders", next); err != nil {
		t.Fatal(err)
	}
	_, bnd, _ := p.Current(ctx)
	v := bnd.Version()
	if err := os.WriteFile(b, append(ca.BundlePEM(), testutil.MustCA("example.org").BundlePEM()...), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ch:
		default:
		}
		_, bnd, _ = p.Current(ctx)
		if bnd.Version() > v && len(bnd.Roots()) == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("bundle not reloaded: version %d roots %d", bnd.Version(), len(bnd.Roots()))
}

func TestCloseStopsWatch(t *testing.T) {
	_, c, k, b := setup(t)
	p, err := New(Config{Cert: c, Key: k, Bundle: b, TrustDomain: "example.org", PollInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := p.Watch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, open := <-ch:
		if open {
			t.Fatal("expected closed channel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch channel not closed")
	}
	if _, _, err := p.Current(context.Background()); err == nil {
		t.Fatal("Current after Close must fail")
	}
}
