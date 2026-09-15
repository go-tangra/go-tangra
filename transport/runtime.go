// Package transport defines the runtime contract shared by the Freya gRPC and
// HTTP transports, plus helpers for auditing handshake refusals.
package transport

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/authz"
	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/observe"
	"github.com/go-freya/freya/transport/tlsconf"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"go.opentelemetry.io/otel/trace"
)

// Runtime is what a transport needs from the application. freya.App implements it.
type Runtime interface {
	ServiceName() string
	TrustDomain() string
	// LocalID is the SPIFFE ID string this service presents.
	LocalID() string
	Provider() identity.Provider
	Limits() config.Limits
	SkewTolerance() time.Duration
	Audit() *audit.Emitter
	Logger() *slog.Logger
	// Authorizer decides every inbound call; nil means deny everything (no_policy).
	Authorizer() authz.Authorizer
	// Revocation may be nil.
	Revocation() identity.RevocationChecker
	// Metrics may be nil (no instrumentation).
	Metrics() *observe.Metrics
	// Tracer provides spans; never nil (use a no-op provider).
	Tracer() trace.TracerProvider
}

// PeerFromContext adapts authn.FromContext for observe.InstrumentConfig.
func PeerFromContext(ctx context.Context) (string, string, bool) {
	p, ok := authn.FromContext(ctx)
	return p.ServiceName, p.ID.String(), ok
}

// AuditLimitExceeded emits one limit_exceeded event (FR-017: limit violations
// are recorded, not just enforced).
func AuditLimitExceeded(rt Runtime, ctx context.Context, operation, remote, limit string) {
	cid := observe.CorrelationID(ctx)
	if cid == "" {
		cid = observe.NewCorrelationID()
	}
	_ = rt.Audit().Emit(ctx, audit.Event{
		Type: audit.TypeLimitExceeded, Outcome: audit.OutcomeRefused, Reason: audit.ReasonLimitExceeded,
		LocalID: rt.LocalID(), Operation: operation, RemoteAddr: remote, CorrelationID: cid,
		TraceID: observe.TraceID(ctx), Attrs: map[string]string{"limit": limit},
	})
}

// AuditHandshakeRefusal emits exactly one authn_refused (or
// channel_refused_downgrade, or limit_exceeded for a handshake that timed out)
// event for a failed TLS handshake.
func AuditHandshakeRefusal(rt Runtime, err error, remote string) {
	if isTimeout(err) {
		AuditLimitExceeded(rt, context.Background(), "tls handshake", remote, "handshake_timeout")
		return
	}
	reason := tlsconf.Classify(err)
	if reason == "" {
		return
	}
	claimed, detail := "", ""
	var ve *identity.VerifyError
	if errors.As(err, &ve) {
		claimed, detail = ve.Claimed, ve.Detail
	}
	ev := audit.Event{
		Type:          audit.TypeAuthnRefused,
		Outcome:       audit.OutcomeRefused,
		Reason:        audit.Reason(reason),
		LocalID:       rt.LocalID(),
		ClaimedPeerID: claimed,
		RemoteAddr:    remote,
		CorrelationID: "handshake-" + shortID(),
	}
	if detail != "" {
		// Fixed-vocabulary operator hint (e.g. "beyond clock skew tolerance 5m0s");
		// never peer-supplied text.
		ev.Attrs = map[string]string{"detail": detail}
	}
	if reason == identity.ReasonDowngrade {
		ev.Type = audit.TypeChannelRefusedDowngr
	}
	_ = rt.Audit().Emit(context.Background(), ev)
}

// TLSOptions builds the verification options for a runtime.
func TLSOptions(rt Runtime) tlsconf.Options {
	return tlsconf.Options{TrustDomain: rt.TrustDomain(), SkewTolerance: rt.SkewTolerance()}
}

// ResolveEndpoint returns host:port for a bound listener, substituting a routable
// interface address when the configured host is unspecified.
func ResolveEndpoint(configured string, lis net.Listener) string {
	host, _, err := net.SplitHostPort(configured)
	_, port, _ := net.SplitHostPort(lis.Addr().String())
	if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
		if ip := firstNonLoopbackIPv4(); ip != "" {
			return net.JoinHostPort(ip, port)
		}
		return net.JoinHostPort("127.0.0.1", port)
	}
	return net.JoinHostPort(host, port)
}

func firstNonLoopbackIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			return ipn.IP.String()
		}
	}
	return ""
}

// ErrLocalIdentityUnavailable is returned to callers while this service has no
// valid identity of its own (FR-012, SR-003): it neither accepts nor makes calls.
var ErrLocalIdentityUnavailable = kerrors.ServiceUnavailable("identity_expired", "")

// LocalIdentityValid reports whether the runtime currently holds a valid identity.
func LocalIdentityValid(rt Runtime) bool {
	id, bundle, err := rt.Provider().Current(context.Background())
	if err != nil || len(bundle.Roots()) == 0 {
		return false
	}
	now := time.Now()
	return !now.Before(id.NotBefore().Add(-rt.SkewTolerance())) && now.Before(id.NotAfter())
}

// LocalIdentityGuard refuses every call while the local identity is missing or
// expired, regardless of connection state.
func LocalIdentityGuard(rt Runtime) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if !LocalIdentityValid(rt) {
				return nil, ErrLocalIdentityUnavailable
			}
			return next(ctx, req)
		}
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
