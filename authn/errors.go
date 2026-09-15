package authn

import kerrors "github.com/go-kratos/kratos/v3/errors"

// Sentinel refusal errors. Reasons are the closed audit vocabulary; messages
// are intentionally empty so nothing internal reaches the peer.
var (
	ErrNoIdentity   = kerrors.Unauthorized("no_identity", "")
	ErrExpired      = kerrors.Unauthorized("identity_expired", "")
	ErrNotYetValid  = kerrors.Unauthorized("identity_not_yet_valid", "")
	ErrUntrusted    = kerrors.Unauthorized("untrusted", "")
	ErrRevoked      = kerrors.Unauthorized("identity_revoked", "")
	ErrNameMismatch = kerrors.Unauthorized("name_mismatch", "")
	ErrDowngrade    = kerrors.Unauthorized("downgrade_refused", "")
)
