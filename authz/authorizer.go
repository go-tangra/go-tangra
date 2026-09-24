package authz

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// Reason is the closed vocabulary of authorization decisions.
type Reason string

// Decision reasons (identical to the audit vocabulary).
const (
	ReasonExplicitAllow  Reason = "explicit_allow"
	ReasonExplicitDeny   Reason = "explicit_deny"
	ReasonNoMatchingRule Reason = "no_matching_rule"
	ReasonNoPolicy       Reason = "no_policy"
)

// Decision is the outcome of evaluating one call.
type Decision struct {
	Allowed       bool
	Reason        Reason
	RuleID        string
	PolicyVersion string
}

// Authorizer decides whether peer may invoke operation on callee.
type Authorizer interface {
	Authorize(ctx context.Context, peer identity.SPIFFEID, callee, operation string) Decision
}

// AllowAll permits everything. It exists only for explicitly opted-in
// development use (freya.WithAllowAllPolicy) and is never a default.
type AllowAll struct{}

// Authorize implements Authorizer.
func (AllowAll) Authorize(context.Context, identity.SPIFFEID, string, string) Decision {
	return Decision{Allowed: true, Reason: ReasonExplicitAllow, RuleID: "allow-all", PolicyVersion: "allow-all"}
}

// DenyAll is the deny-by-default authorizer used when no policy is loaded.
type DenyAll struct{}

// Authorize implements Authorizer.
func (DenyAll) Authorize(context.Context, identity.SPIFFEID, string, string) Decision {
	return Decision{Allowed: false, Reason: ReasonNoPolicy}
}
