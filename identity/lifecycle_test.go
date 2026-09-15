package identity_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
)

// renewingProvider wraps MemProvider with a Renew that may fail N times.
type renewingProvider struct {
	*testutil.MemProvider
	ca       *testutil.CA
	name     string
	ttl      time.Duration
	mu       sync.Mutex
	failures int
	renewals int
}

func (r *renewingProvider) Renew(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failures > 0 {
		r.failures--
		return errors.New("issuer busy")
	}
	r.renewals++
	r.Rotate(r.ca.MustIssue(r.name, window(r.ttl)))
	return nil
}

// window returns a validity window aligned to whole seconds (X.509 granularity).
func window(ttl time.Duration) testutil.IssueOptions {
	nb := time.Now().Truncate(time.Second)
	return testutil.IssueOptions{NotBefore: nb, NotAfter: nb.Add(ttl)}
}

func (r *renewingProvider) renewed() int { r.mu.Lock(); defer r.mu.Unlock(); return r.renewals }

func newRenewing(ca *testutil.CA, ttl time.Duration, failures int) *renewingProvider {
	crt := ca.MustIssue("svc", window(ttl))
	return &renewingProvider{MemProvider: testutil.NewMemProvider(ca, crt), ca: ca, name: "svc", ttl: ttl, failures: failures}
}

type recorder struct {
	mu  sync.Mutex
	evs []identity.LifecycleEvent
}

func (r *recorder) on(e identity.LifecycleEvent) {
	r.mu.Lock()
	r.evs = append(r.evs, e)
	r.mu.Unlock()
}
func (r *recorder) states() []identity.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]identity.State, len(r.evs))
	for i, e := range r.evs {
		out[i] = e.State
	}
	return out
}
func (r *recorder) count(s identity.State) int {
	n := 0
	for _, x := range r.states() {
		if x == s {
			n++
		}
	}
	return n
}

func TestLifecycleRenewsAtFraction(t *testing.T) {
	ca := testutil.MustCA("example.org")
	p := newRenewing(ca, 2*time.Second, 0)
	rec := &recorder{}
	lc := identity.NewLifecycle(p, identity.LifecycleConfig{RenewAt: 0.5, Jitter: 0, OnEvent: rec.on})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() { _ = lc.Run(ctx) }()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && rec.count(identity.StateRenewing) < 3 {
		time.Sleep(20 * time.Millisecond)
	}
	if rec.count(identity.StateRenewing) < 3 || rec.count(identity.StateExpired) != 0 {
		t.Fatalf("states %v", rec.states())
	}
	if lc.State() != identity.StateValid && lc.State() != identity.StateRenewing {
		t.Fatalf("state %s", lc.State())
	}
	if n := p.renewed(); n < 3 {
		t.Fatalf("renewals %d", n)
	}
}

func TestLifecycleBackoffAndExpiry(t *testing.T) {
	ca := testutil.MustCA("example.org")
	p := newRenewing(ca, 2*time.Second, 2)
	rec := &recorder{}
	lc := identity.NewLifecycle(p, identity.LifecycleConfig{RenewAt: 0.3, Jitter: 0, MinBackoff: 20 * time.Millisecond, MaxBackoff: 40 * time.Millisecond, OnEvent: rec.on})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() { _ = lc.Run(ctx) }()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) && rec.count(identity.StateRenewFailed) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if rec.count(identity.StateRenewFailed) < 2 {
		t.Fatalf("expected two failures: %v", rec.states())
	}
	for time.Now().Before(deadline) && p.renewed() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if p.renewed() == 0 {
		t.Fatalf("renewal never succeeded after backoff: %v", rec.states())
	}
	// A provider that cannot renew at all reaches Expired, then recovers on update.
	static := testutil.NewMemProvider(ca, ca.MustIssue("svc", window(time.Second)))
	rec2 := &recorder{}
	lc2 := identity.NewLifecycle(static, identity.LifecycleConfig{RenewAt: 0.5, OnEvent: rec2.on})
	go func() { _ = lc2.Run(ctx) }()
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && lc2.State() != identity.StateExpired {
		time.Sleep(10 * time.Millisecond)
	}
	if lc2.State() != identity.StateExpired {
		t.Fatalf("expected Expired, got %s (%v)", lc2.State(), rec2.states())
	}
	static.Rotate(ca.MustIssue("svc", testutil.IssueOptions{}))
	for time.Now().Before(deadline) && lc2.State() != identity.StateValid {
		time.Sleep(10 * time.Millisecond)
	}
	if lc2.State() != identity.StateValid {
		t.Fatalf("expected recovery to Valid, got %s", lc2.State())
	}
	// A bundle change is reported as BundleUpdated.
	static.SetRoots(ca.Cert, testutil.MustCA("example.org").Cert)
	static.Rotate(ca.MustIssue("svc", testutil.IssueOptions{}))
	for time.Now().Before(deadline) && rec2.count(identity.StateBundleUpdated) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if rec2.count(identity.StateBundleUpdated) == 0 {
		t.Fatalf("expected BundleUpdated: %v", rec2.states())
	}
}

func TestLifecycleConfigDefaultsAndBounds(t *testing.T) {
	cfg := identity.LifecycleConfig{RenewAt: 0}
	cfg = cfg.WithDefaults()
	if cfg.RenewAt != 0.5 || cfg.Jitter != 0.1 || cfg.MinBackoff == 0 || cfg.MaxBackoff == 0 || cfg.Now == nil {
		t.Fatalf("defaults %+v", cfg)
	}
	if identity.RenewTime(time.Unix(0, 0), time.Unix(100, 0), 0.5, 0) != time.Unix(50, 0) {
		t.Fatal("renew time")
	}
	rt := identity.RenewTime(time.Unix(0, 0), time.Unix(100, 0), 0.5, 0.2)
	if rt.Before(time.Unix(40, 0)) || rt.After(time.Unix(60, 0)) {
		t.Fatalf("jittered renew time out of bounds: %v", rt)
	}
}
