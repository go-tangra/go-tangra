package authz

import (
	"context"
	"strings"
	"testing"

	"github.com/go-freya/freya/identity"
)

func TestCacheHitMissAndBound(t *testing.T) {
	p := mustLoad(t, `version: c1
rules:
  - {id: a, from: ["spiffe://example.org/svc/*"], to: ["b"], effect: allow}
`)
	c := NewCached(p, 3)
	peer := identity.ForService("example.org", "x")
	d1 := c.Authorize(context.Background(), peer, "b", "op1")
	d2 := c.Authorize(context.Background(), peer, "b", "op1")
	if d1 != d2 || !d1.Allowed {
		t.Fatalf("%+v %+v", d1, d2)
	}
	if h, m := c.Stats(); h != 1 || m != 1 {
		t.Fatalf("stats %d/%d", h, m)
	}
	for _, op := range []string{"op2", "op3", "op4"} {
		c.Authorize(context.Background(), peer, "b", op)
	}
	if c.Len() > 3 {
		t.Fatalf("cache exceeded bound: %d", c.Len())
	}
	// Policy version change invalidates everything.
	p2 := mustLoad(t, "version: c2\nrules: []\n")
	c.Swap(p2)
	if c.Len() != 0 {
		t.Fatal("swap must clear the cache")
	}
	if d := c.Authorize(context.Background(), peer, "b", "op1"); d.Allowed || d.PolicyVersion != "c2" {
		t.Fatalf("after swap: %+v", d)
	}
	if c.Version() != "c2" {
		t.Fatal("version")
	}
	if NewCached(p, 0).cap != defaultCacheSize {
		t.Fatal("default cache size")
	}
	_ = strings.ToLower
}
