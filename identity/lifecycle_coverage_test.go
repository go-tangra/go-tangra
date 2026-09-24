package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
)

type watchFailProvider struct{ *testutil.MemProvider }

func (watchFailProvider) Watch(context.Context) (<-chan identity.Update, error) {
	return nil, errors.New("watch broken")
}

func TestLifecycleEdges(t *testing.T) {
	ca := testutil.MustCA("example.org")
	valid := ca.MustIssue("svc", testutil.IssueOptions{})
	wf := watchFailProvider{testutil.NewMemProvider(ca, valid)}
	lc := identity.NewLifecycle(wf, identity.LifecycleConfig{})
	if err := lc.Start(context.Background()); err == nil {
		t.Fatal("Start must return the watch error")
	}
	if err := lc.Run(context.Background()); err == nil {
		t.Fatal("Run must return the watch error")
	}
	failing := testutil.NewMemProvider(ca, valid)
	failing.Fail(errors.New("down"))
	var events []identity.State
	lc2 := identity.NewLifecycle(failing, identity.LifecycleConfig{OnEvent: func(e identity.LifecycleEvent) { events = append(events, e.State) }})
	ctx, cancel := context.WithCancel(context.Background())
	if err := lc2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if lc2.State() != identity.StatePending || len(events) != 1 || events[0] != identity.StateRenewFailed {
		t.Fatalf("state %s events %v", lc2.State(), events)
	}
	cancel()
	expired := testutil.NewMemProvider(ca, ca.MustIssue("svc", testutil.IssueOptions{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}))
	lc3 := identity.NewLifecycle(expired, identity.LifecycleConfig{})
	ctx3, cancel3 := context.WithCancel(context.Background())
	defer cancel3()
	if err := lc3.Start(ctx3); err != nil {
		t.Fatal(err)
	}
	if lc3.State() != identity.StateExpired {
		t.Fatalf("state %s", lc3.State())
	}
	mp := testutil.NewMemProvider(ca, valid)
	got := make(chan identity.State, 32)
	lc4 := identity.NewLifecycle(mp, identity.LifecycleConfig{OnEvent: func(e identity.LifecycleEvent) { got <- e.State }})
	ctx4, cancel4 := context.WithCancel(context.Background())
	defer cancel4()
	done := make(chan struct{})
	go func() { _ = lc4.Run(ctx4); close(done) }()
	time.Sleep(20 * time.Millisecond)
	mp.SendUpdate(identity.Update{Err: errors.New("renewal failed upstream")})
	time.Sleep(20 * time.Millisecond)
	_ = mp.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the watch channel closed")
	}
	found := false
	for len(got) > 0 {
		if <-got == identity.StateRenewFailed {
			found = true
		}
	}
	if !found {
		t.Fatal("expected RenewFailed from an error update")
	}
	nb := time.Now().Truncate(time.Second)
	short := testutil.NewMemProvider(ca, ca.MustIssue("svc", testutil.IssueOptions{NotBefore: nb.Add(-time.Second), NotAfter: nb.Add(time.Second)}))
	lc5 := identity.NewLifecycle(short, identity.LifecycleConfig{OnEvent: func(identity.LifecycleEvent) {}})
	ctx5, cancel5 := context.WithCancel(context.Background())
	defer cancel5()
	if err := lc5.Start(ctx5); err != nil {
		t.Fatal(err)
	}
	short.SetCert(ca.MustIssue("svc", testutil.IssueOptions{}))
	time.Sleep(1500 * time.Millisecond)
	if lc5.State() != identity.StateValid {
		t.Fatalf("state %s, want valid (newer identity present at expiry)", lc5.State())
	}
}
