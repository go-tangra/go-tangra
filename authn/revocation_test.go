package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
	kerrors "github.com/go-kratos/kratos/v3/errors"
)

type failingChecker struct{}

func (failingChecker) IsRevoked(context.Context, identity.SPIFFEID, string) (bool, error) {
	return false, errors.New("valkey down")
}

func TestRevocation(t *testing.T) {
	ca := testutil.MustCA("example.org")
	crt := ca.MustIssue("orders", testutil.IssueOptions{})
	sink := &evSink{}
	em := audit.NewEmitter(nil, 0, sink)
	mem := NewMemoryRevocationChecker()
	mw := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory", Audit: em, Revocation: mem})
	ran := 0
	h := mw(func(ctx context.Context, req any) (any, error) { ran++; return nil, nil })
	if _, err := h(grpcCtx(crt.Leaf), nil); err != nil || ran != 1 {
		t.Fatalf("not revoked: %v", err)
	}
	// Revoke by serial.
	mem.Revoke(identity.ForService("example.org", "orders"), crt.Leaf.SerialNumber.String())
	_, err := h(grpcCtx(crt.Leaf), nil)
	if !kerrors.IsUnauthorized(err) || kerrors.FromError(err).Reason != "identity_revoked" || ran != 1 {
		t.Fatalf("revoked by serial: %v ran=%d", err, ran)
	}
	if ev := sink.evs[len(sink.evs)-1]; ev.Reason != audit.ReasonIdentityRevoked || ev.ClaimedPeerID != "spiffe://example.org/svc/orders" {
		t.Fatalf("event %+v", ev)
	}
	// A fresh serial of the same identity is fine until the whole ID is revoked.
	crt2 := ca.MustIssue("orders", testutil.IssueOptions{})
	if _, err := h(grpcCtx(crt2.Leaf), nil); err != nil {
		t.Fatalf("new serial: %v", err)
	}
	mem.Revoke(identity.ForService("example.org", "orders"), "")
	if _, err := h(grpcCtx(crt2.Leaf), nil); !kerrors.IsUnauthorized(err) {
		t.Fatalf("revoked by id: %v", err)
	}
	mem.Unrevoke(identity.ForService("example.org", "orders"), "")
	mem.Unrevoke(identity.ForService("example.org", "orders"), crt.Leaf.SerialNumber.String())
	if _, err := h(grpcCtx(crt.Leaf), nil); err != nil {
		t.Fatalf("unrevoked: %v", err)
	}
	// Checker failure fails closed.
	failing := Middleware(Config{TrustDomain: "example.org", LocalID: "spiffe://example.org/svc/inventory", Audit: em, Revocation: failingChecker{}})
	_, err = failing(func(context.Context, any) (any, error) { return nil, nil })(grpcCtx(crt.Leaf), nil)
	if !kerrors.IsUnauthorized(err) || kerrors.FromError(err).Reason != "identity_revoked" {
		t.Fatalf("checker error must fail closed: %v", err)
	}
	if ok, err := mem.IsRevoked(context.Background(), identity.ForService("example.org", "x"), "1"); ok || err != nil {
		t.Fatal("unknown id must not be revoked")
	}
}
