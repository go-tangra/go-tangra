package identity

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"
)

// Identity is the read-only view of a service's own identity. It deliberately
// exposes no private key or certificate chain; only transport/tlsconf can obtain
// key material through the module-internal credential interface.
type Identity interface {
	ID() SPIFFEID
	NotBefore() time.Time
	NotAfter() time.Time
	Serial() string
}

// Bundle is the set of trust roots for one trust domain.
type Bundle interface {
	TrustDomain() string
	Roots() []*x509.Certificate
	// Version increases monotonically on every change.
	Version() uint64
}

// Update is delivered whenever the identity or bundle changes. Err is set when a
// renewal failed; Identity/Bundle then hold the last-known-good values (or nil).
type Update struct {
	Identity Identity
	Bundle   Bundle
	Err      error
}

// Provider supplies the local identity and trust bundle and reports changes.
// Implementations: identity/spiffe, identity/file, identity/localdev.
type Provider interface {
	// Current returns the current identity and bundle, or an error if none is valid.
	Current(ctx context.Context) (Identity, Bundle, error)
	// Watch delivers an Update on every change until ctx is done or Close is called.
	Watch(ctx context.Context) (<-chan Update, error)
	Close() error
}

// RevocationChecker answers whether a specific identity has been revoked inside
// its validity window. A checker error MUST be treated as revoked (fail closed).
type RevocationChecker interface {
	IsRevoked(ctx context.Context, id SPIFFEID, serial string) (bool, error)
}

// Reason is the closed vocabulary of identity verification outcomes. Values are
// identical to the audit event reason strings.
type Reason string

// Verification reasons.
const (
	ReasonNoIdentity          Reason = "no_identity"
	ReasonUntrusted           Reason = "untrusted"
	ReasonExpired             Reason = "identity_expired"
	ReasonNotYetValid         Reason = "identity_not_yet_valid"
	ReasonRevoked             Reason = "identity_revoked"
	ReasonNameMismatch        Reason = "name_mismatch"
	ReasonDowngrade           Reason = "downgrade_refused"
	ReasonProviderUnavailable Reason = "provider_unavailable"
	ReasonBundleEmpty         Reason = "bundle_empty"
)

// VerifyError is returned by peer verification. Its message is fixed-vocabulary
// and never contains certificate material.
type VerifyError struct {
	Reason Reason
	// Detail is a short, operator-facing hint (e.g. skew); never peer-supplied text.
	Detail string
	// Claimed is the SPIFFE ID the peer presented, if it parsed; audit-only.
	Claimed string
}

func (e *VerifyError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("identity: peer refused: %s", e.Reason)
	}
	return fmt.Sprintf("identity: peer refused: %s (%s)", e.Reason, e.Detail)
}
