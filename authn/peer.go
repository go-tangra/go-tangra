package authn

import (
	"context"
	"time"

	"github.com/go-freya/freya/identity"
)

// PeerIdentity is the read-only, verified identity of the remote party.
type PeerIdentity struct {
	ID          identity.SPIFFEID
	ServiceName string
	Serial      string
	VerifiedAt  time.Time
}

type peerKey struct{}

// FromContext returns the verified peer placed by the authn middleware. In a
// running Freya server a handler is never reached without one.
func FromContext(ctx context.Context) (PeerIdentity, bool) {
	p, ok := ctx.Value(peerKey{}).(PeerIdentity)
	return p, ok
}

// WithPeer stores p in ctx. It exists for tests of handlers; the middleware is
// the only production writer.
func WithPeer(ctx context.Context, p PeerIdentity) context.Context {
	return context.WithValue(ctx, peerKey{}, p)
}
