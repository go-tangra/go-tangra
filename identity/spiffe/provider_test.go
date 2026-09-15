package spiffe

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya/internal/cred"
	"github.com/go-freya/freya/internal/testutil"
)

func TestProviderFetchesAndRotates(t *testing.T) {
	wl := testutil.StartWorkloadAPI(t, "example.org", "orders", 600*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := New(ctx, Config{Socket: wl.Addr(), TrustDomain: "example.org", StartupTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	id, bundle, err := p.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id.ID().String() != "spiffe://example.org/svc/orders" || len(bundle.Roots()) != 1 || bundle.Version() == 0 {
		t.Fatalf("identity %v bundle %+v", id.ID(), bundle)
	}
	var src cred.Source = p
	crt, err := src.Credential()
	if err != nil || crt.PrivateKey == nil || crt.Leaf == nil {
		t.Fatalf("credential %v %v", crt, err)
	}
	ch, err := p.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-ch:
		if u.Err != nil || u.Identity.Serial() == id.Serial() {
			t.Fatalf("update %+v", u)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no rotation observed")
	}
	cur, _, _ := p.Current(ctx)
	if cur.Serial() == id.Serial() {
		t.Fatal("Current not updated after rotation")
	}
	// Bundle change bumps the version.
	v := bundle.Version()
	wl.RotateCA()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, b, _ := p.Current(ctx)
		if b.Version() > v {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("bundle version did not change after CA rotation")
}

func TestProviderFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := New(ctx, Config{Socket: "unix:///nonexistent/agent.sock", TrustDomain: "example.org", StartupTimeout: 500 * time.Millisecond}); err == nil {
		t.Fatal("unavailable socket must fail")
	}
	if _, err := New(ctx, Config{Socket: "", TrustDomain: "example.org"}); err == nil {
		t.Fatal("empty socket must fail")
	}
	if _, err := New(ctx, Config{Socket: "unix:///x", TrustDomain: "Bad Domain"}); err == nil {
		t.Fatal("bad trust domain must fail")
	}
	// SVID for another trust domain is rejected.
	wl := testutil.StartWorkloadAPI(t, "other.org", "orders", time.Minute)
	if _, err := New(ctx, Config{Socket: wl.Addr(), TrustDomain: "example.org", StartupTimeout: 3 * time.Second}); err == nil {
		t.Fatal("foreign trust domain SVID must be rejected")
	}
	// Close stops Watch and Current.
	wl2 := testutil.StartWorkloadAPI(t, "example.org", "orders", time.Minute)
	p, err := New(ctx, Config{Socket: wl2.Addr(), TrustDomain: "example.org", StartupTimeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := p.Watch(ctx)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, open := <-ch; open {
		t.Fatal("watch channel must close on Close")
	}
	if _, _, err := p.Current(ctx); err == nil {
		t.Fatal("Current after Close must fail")
	}
	_ = p.Close()
}
