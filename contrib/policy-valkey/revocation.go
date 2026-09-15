package valkey

import (
	"context"
	"fmt"

	"github.com/go-freya/freya/identity"
)

// RevocationChecker consults the freya:revoked:* denylist. Any lookup error is
// returned so that the authn middleware fails closed.
type RevocationChecker struct{ kv KV }

var _ identity.RevocationChecker = (*RevocationChecker)(nil)

// NewRevocationChecker wraps kv.
func NewRevocationChecker(kv KV) *RevocationChecker { return &RevocationChecker{kv: kv} }

// IsRevoked implements identity.RevocationChecker.
func (r *RevocationChecker) IsRevoked(ctx context.Context, id identity.SPIFFEID, serial string) (bool, error) {
	for _, k := range []string{revokedKey(id.String(), ""), revokedKey(id.String(), serial)} {
		ok, err := r.kv.Exists(ctx, k)
		if err != nil {
			return false, fmt.Errorf("policy-valkey: revocation lookup: %w", err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
