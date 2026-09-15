package authz

import "context"

// Source loads the policy and reports changes (hot reload). Implementations:
// authz/file (this module) and contrib/policy-valkey.
type Source interface {
	Load(ctx context.Context) (*Policy, error)
	// Watch yields a new *Policy on every successful reload. Invalid updates are
	// never delivered; sources keep the last-known-good policy.
	Watch(ctx context.Context) (<-chan *Policy, error)
}
