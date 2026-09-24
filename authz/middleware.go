package authz

import (
	"context"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/observe"
)

// ErrDenied is returned to callers for every refused authorization. The reason
// string is fixed so that error bodies never reveal policy details.
var ErrDenied = kerrors.Forbidden("denied", "")

// Config wires the middleware to the runtime.
type Config struct {
	// Authorizer decides calls; nil denies everything with no_policy.
	Authorizer Authorizer
	// LocalID and ServiceName identify this (callee) service.
	LocalID     string
	ServiceName string
	Audit       *audit.Emitter
	// SampleAllowed emits authz_allowed events (off by default; high volume).
	SampleAllowed bool
}

// Middleware evaluates the policy for every call after authn has placed the
// verified peer in the context. It refuses (fail closed) when the peer is
// missing, the authorizer is nil, or the decision is deny.
func Middleware(cfg Config) middleware.Middleware {
	authorizer := cfg.Authorizer
	if authorizer == nil {
		authorizer = DenyAll{}
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			peer, ok := authn.FromContext(ctx)
			if !ok {
				return nil, authn.ErrNoIdentity
			}
			op := Operation(ctx)
			d := authorizer.Authorize(ctx, peer.ID, cfg.ServiceName, op)
			if d.Allowed {
				if cfg.SampleAllowed {
					cfg.emit(ctx, audit.TypeAuthzAllowed, audit.OutcomeOK, d, peer, op)
				}
				return next(ctx, req)
			}
			cfg.emit(ctx, audit.TypeAuthzRefused, audit.OutcomeRefused, d, peer, op)
			return nil, ErrDenied
		}
	}
}

func (cfg Config) emit(ctx context.Context, t audit.Type, o audit.Outcome, d Decision, peer authn.PeerIdentity, op string) {
	if cfg.Audit == nil {
		return
	}
	_ = cfg.Audit.Emit(ctx, audit.Event{
		Type: t, Outcome: o, Reason: audit.Reason(d.Reason),
		LocalID: cfg.LocalID, ClaimedPeerID: peer.ID.String(), VerifiedPeerID: peer.ID.String(),
		Operation: op, CorrelationID: observe.CorrelationID(ctx),
		RuleID: d.RuleID, PolicyVersion: d.PolicyVersion, TraceID: observe.TraceID(ctx),
	})
}

// Operation derives the policy operation string for the current call:
// "/pkg.Service/Method" for gRPC, "METHOD /path" for HTTP.
func Operation(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	if ht, ok := tr.(khttp.Transporter); ok {
		r := ht.Request()
		if r != nil {
			return r.Method + " " + r.URL.Path
		}
		return http.MethodGet + " " + tr.Operation()
	}
	return tr.Operation()
}
