package freya

import (
	"log/slog"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authz"
	"github.com/go-freya/freya/identity"
	"github.com/go-kratos/kratos/v3/registry"
	"go.opentelemetry.io/otel/trace"
)

// Option configures New.
type Option func(*options)

type options struct {
	provider  identity.Provider
	policy    authz.Source
	discovery registry.Discovery
	sinks     []audit.Sink
	logger    slog.Handler
	localDev  bool
	allowAll  bool
	revoke    identity.RevocationChecker
	tracer    trace.TracerProvider
}

// WithIdentityProvider supplies a custom identity.Provider (config.Identity is
// then ignored). The provider must expose credentials to the transport, which
// only providers in this module do.
func WithIdentityProvider(p identity.Provider) Option { return func(o *options) { o.provider = p } }

// WithPolicySource supplies the authorization policy source (config.Authz is
// then ignored).
func WithPolicySource(s authz.Source) Option { return func(o *options) { o.policy = s } }

// WithDiscovery supplies any Kratos registry.Discovery in place of static config.
func WithDiscovery(d registry.Discovery) Option { return func(o *options) { o.discovery = d } }

// WithAuditSink adds a sink; the redacting slog sink is always present.
func WithAuditSink(s audit.Sink) Option { return func(o *options) { o.sinks = append(o.sinks, s) } }

// WithLogger sets the log handler; it is wrapped by audit.NewRedactingHandler.
func WithLogger(h slog.Handler) Option { return func(o *options) { o.logger = h } }

// WithInsecureLocalDev uses a self-issued, in-memory CA for this process. It
// logs a warning, emits insecure_mode_enabled, and is refused in production.
func WithInsecureLocalDev() Option { return func(o *options) { o.localDev = true } }

// WithAllowAllPolicy permits every authenticated caller. It logs a warning,
// emits insecure_mode_enabled, and is refused in production.
func WithAllowAllPolicy() Option { return func(o *options) { o.allowAll = true } }

// WithRevocationChecker enables in-lifetime revocation (e.g. contrib/policy-valkey
// denylist). A checker error is treated as revoked (fail closed).
func WithRevocationChecker(c identity.RevocationChecker) Option {
	return func(o *options) { o.revoke = c }
}

// WithTracerProvider sets the OpenTelemetry tracer provider used for server and
// client spans. Without it spans propagate context but are not exported.
func WithTracerProvider(tp trace.TracerProvider) Option { return func(o *options) { o.tracer = tp } }
