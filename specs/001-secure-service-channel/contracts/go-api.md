# Contract: Public Go API

**Module**: `github.com/go-freya/freya` | **Stability**: v0.x — additive changes only
within a MINOR; everything listed here is exercised by `tests/contract` and the
package tests. Regenerated 2026-09-15 from the implemented surface.

Design rule (Principles I/II): **no exported constructor can produce a plaintext server
or client, no exported type exposes key material, and no option can remove or reorder
the security middleware.** `tests/contract/api_surface_test.go` walks every exported
symbol of `freya`, `transport/grpc` and `transport/http` to enforce the first rule.

## Package `freya`

```go
// New validates cfg, starts the identity provider (waiting up to
// identity.startup_timeout), loads policy, builds and binds the servers and the
// admin listener. It returns an error — never a degraded app — when any
// security prerequisite is missing.
func New(cfg config.Config, opts ...Option) (*App, error)

type App struct{ /* unexported */ }
func (a *App) Run(ctx context.Context) error                  // serves until ctx is cancelled
func (a *App) Close()                                         // releases provider, pool, audit queue, admin listener
func (a *App) GRPC() *tgrpc.Server                            // register services here
func (a *App) HTTP() *thttp.Server                            // nil unless server.http_addr is set
func (a *App) Client(ctx context.Context, service string) (*grpc.ClientConn, error) // pooled, mTLS, callee verified by SPIFFE ID
func (a *App) Identity() identity.Identity                    // no key material
func (a *App) Ready() bool                                    // identity valid + bundle non-empty + policy loaded
func (a *App) IdentityState() string                          // pending | valid | renewing | expired
func (a *App) AdminURL() string                               // http://127.0.0.1:port, or https:// when non-loopback (mTLS)
func (a *App) TracerProvider() trace.TracerProvider

// Options
func WithIdentityProvider(p identity.Provider) Option         // provider must be from this module (credential access is internal)
func WithPolicySource(s authz.Source) Option
func WithRevocationChecker(c identity.RevocationChecker) Option // checker error ⇒ revoked (fail closed)
func WithDiscovery(d registry.Discovery) Option               // any Kratos registry; static config otherwise
func WithAuditSink(s audit.Sink) Option                       // additional async sink; the redacting slog sink is always present
func WithLogger(h slog.Handler) Option                        // wrapped by audit.NewRedactingHandler
func WithTracerProvider(tp trace.TracerProvider) Option       // default: no-op provider (context still propagates)
func WithInsecureLocalDev() Option                            // self-issued CA; WARN + insecure_mode_enabled; refused when env=production
func WithAllowAllPolicy() Option                              // WARN + insecure_mode_enabled; refused when env=production
```

`App` also satisfies `transport.Runtime` (below); application code never needs it.

## Package `config`

```go
func Default() Config                          // secure defaults (constitution limits)
func Load(path string) (Config, error)         // YAML over Default(); unknown fields rejected; not yet validated
func (c Config) Validate() error               // field-level errors name the field in snake_case
func (c Config) Warnings() []string            // accepted deviations from secure defaults (logged at startup)
func (c Config) IsProduction() bool
func (c Config) LocalSPIFFEID() string
// Identity providers settable from config: ProviderSPIFFE, ProviderFile (ProviderProvided = supplied by option).
// Policy sources settable from config: AuthzFile, AuthzValkey (AuthzProvided = supplied by option).
```

Fields: see `docs/configuration.md`.

## Package `identity`

```go
type SPIFFEID struct{ /* unexported */ }
func ParseSPIFFEID(s string) (SPIFFEID, error)                 // canonical form only; fuzzed
func NewSPIFFEID(trustDomain, serviceName string) (SPIFFEID, error)
func ForService(trustDomain, serviceName string) SPIFFEID      // panics on invalid input
func ValidTrustDomain(td string) bool; func ValidServiceName(name string) bool
func (id SPIFFEID) String() string; TrustDomain() string; ServiceName() string; IsZero() bool; Equal(SPIFFEID) bool

type Identity interface { ID() SPIFFEID; NotBefore() time.Time; NotAfter() time.Time; Serial() string }
type Bundle   interface { TrustDomain() string; Roots() []*x509.Certificate; Version() uint64 }
type Update   struct { Identity Identity; Bundle Bundle; Err error }
type Provider interface {
    Current(ctx context.Context) (Identity, Bundle, error)
    Watch(ctx context.Context) (<-chan Update, error)
    Close() error
}
type Renewer interface { Renew(ctx context.Context) error }     // optional; pull-model providers
type RevocationChecker interface { IsRevoked(ctx context.Context, id SPIFFEID, serial string) (bool, error) }

type Reason string   // closed vocabulary: no_identity, untrusted, identity_expired, identity_not_yet_valid,
                     // identity_revoked, name_mismatch, downgrade_refused, provider_unavailable, bundle_empty
type VerifyError struct { Reason Reason; Detail string; Claimed string }  // Error() never contains certificate material

// Lifecycle drives renewal, backoff and expiry for one provider.
type State string    // pending, valid, renewing, expired, revoked; event-only: renew_failed, bundle_updated
type LifecycleEvent struct { State State; Identity Identity; Bundle Bundle; Err error }
type LifecycleConfig struct { RenewAt, Jitter float64; MinBackoff, MaxBackoff time.Duration; Now func() time.Time; OnEvent func(LifecycleEvent) }
func NewLifecycle(p Provider, cfg LifecycleConfig) *Lifecycle
func (l *Lifecycle) Start(ctx context.Context) error   // adopts the current identity synchronously, then runs in the background
func (l *Lifecycle) Run(ctx context.Context) error     // Start + block
func (l *Lifecycle) State() State
func RenewTime(notBefore, notAfter time.Time, at, jitter float64) time.Time
```

Providers: `identity/spiffe.New(ctx, Config{Socket, TrustDomain, StartupTimeout})`,
`identity/file.New(Config{Cert, Key, Bundle, TrustDomain, PollInterval})`,
`identity/localdev.New(trustDomain, name, ttl)` (only reachable via `WithInsecureLocalDev`).
Key material is obtainable solely through the module-internal `internal/cred.Source`.

## Package `authn`

```go
type PeerIdentity struct { ID identity.SPIFFEID; ServiceName, Serial string; VerifiedAt time.Time }
func FromContext(ctx context.Context) (PeerIdentity, bool)   // always ok inside a Freya handler
func WithPeer(ctx context.Context, p PeerIdentity) context.Context // tests only
type Config struct { TrustDomain, LocalID string; Audit *audit.Emitter; Revocation identity.RevocationChecker; Now func() time.Time }
func Middleware(cfg Config) middleware.Middleware
type MemoryRevocationChecker; func NewMemoryRevocationChecker() *MemoryRevocationChecker
func (m *MemoryRevocationChecker) Revoke(id identity.SPIFFEID, serial string); Unrevoke(...); IsRevoked(...)
// Sentinel Kratos errors (401 / UNAUTHENTICATED; empty message): ErrNoIdentity, ErrExpired, ErrNotYetValid,
// ErrUntrusted, ErrRevoked, ErrNameMismatch, ErrDowngrade
```

## Package `authz`

```go
type Rule struct { ID string; From, To, Operations []string; Effect Effect }   // Effect: Allow | Deny
type Policy struct { Version string; Rules []Rule; Source string; LoadedAt time.Time /* + compiled matcher */ }
func Load(r io.Reader) (*Policy, error)                       // YAML/JSON, schema-validated, fuzzed
func NewPolicy(version string, rules []Rule) (*Policy, error)
func (p *Policy) Authorize(ctx, peer identity.SPIFFEID, callee, operation string) Decision // deny wins; nil policy ⇒ no_policy

type Decision struct { Allowed bool; Reason Reason; RuleID, PolicyVersion string }
type Reason string   // explicit_allow, explicit_deny, no_matching_rule, no_policy
type Authorizer interface { Authorize(ctx, peer identity.SPIFFEID, callee, operation string) Decision }
type AllowAll struct{}; type DenyAll struct{}
type Cached; func NewCached(p *Policy, size int) *Cached      // bounded LRU; Swap(p) invalidates; Version(); Policy(); Len(); Stats()

type Source interface { Load(ctx) (*Policy, error); Watch(ctx) (<-chan *Policy, error) }  // invalid updates never delivered
func NewSourceFromConfig(cfg config.Authz) (Source, error)    // "file" via authz/file (linked by import); "valkey" via contrib
type Config struct { Authorizer Authorizer; LocalID, ServiceName string; Audit *audit.Emitter; SampleAllowed bool }
func Middleware(cfg Config) middleware.Middleware             // nil Authorizer ⇒ deny (no_policy)
func Operation(ctx context.Context) string                    // "/pkg.Svc/Method" or "METHOD /path"
var ErrDenied error                                           // 403 / PERMISSION_DENIED, reason "denied"
```

`authz/file.New(path, WithPollInterval(d))` implements `Source` with `Errors() <-chan error`.

## Package `audit`

```go
type Event struct { /* fields exactly as audit-event.schema.json; JSON time is RFC 3339 nano UTC */ }
func (e Event) Validate() error;  func AllTypes() []Type;  func AllReasons() []Reason
type Sink interface { Emit(ctx context.Context, e Event) }    // must not block
func NewSlogSink(l *slog.Logger) *SlogSink
type Emitter; func NewEmitter(primary Sink, queue int, extra ...Sink) *Emitter
func (e *Emitter) Emit(ctx, ev Event) error /* invalid events rejected */; AddSink(Sink); Dropped() uint64; Invalid() uint64; Close()
func NewRedactingHandler(inner slog.Handler) slog.Handler     // idempotent; Redacted = "[REDACTED]"
```

## Package `observe`

```go
const HeaderCorrelationID = "x-request-id"
func CorrelationID(ctx) string; func WithCorrelationID(ctx, id) context.Context
func NewCorrelationID() string /* UUIDv7 */; func ValidCorrelationID(s string) bool
func ServerCorrelation(opts ...Option) middleware.Middleware; func ClientCorrelation() middleware.Middleware
func ServerTracing(tp trace.TracerProvider) middleware.Middleware; func ClientTracing(tp) middleware.Middleware
func AnnotatePeer(ctx, service, spiffeID string); func TraceID(ctx) string
type Metrics; func NewMetrics() (*Metrics, error); (*Metrics).Handler() http.Handler /* OpenMetrics text */
func (m *Metrics) IdentityRenewal(outcome string); AuditDropped(n int64)
type InstrumentConfig struct { Metrics *Metrics; ServiceName string; Logger *slog.Logger; Peer func(ctx) (service, id string, ok bool) }
func Instrument(cfg InstrumentConfig) middleware.Middleware   // per-call metrics + "request" log line
type Admin; func NewAdmin(cfg config.Admin, ready func() bool, metrics http.Handler, log *slog.Logger, tlsCfg *tls.Config) (*Admin, error)
func (a *Admin) URL() string; Secure() bool; Serve() error; Shutdown(ctx) error
```

## Package `transport`

```go
type Runtime interface {   // implemented by *freya.App
    ServiceName() string; TrustDomain() string; LocalID() string
    Provider() identity.Provider; Limits() config.Limits; SkewTolerance() time.Duration
    Audit() *audit.Emitter; Logger() *slog.Logger; Authorizer() authz.Authorizer
    Revocation() identity.RevocationChecker; Metrics() *observe.Metrics; Tracer() trace.TracerProvider
}
func LocalIdentityGuard(rt Runtime) middleware.Middleware     // 503 identity_expired while the local identity is invalid
func LocalIdentityValid(rt Runtime) bool
func AuditHandshakeRefusal(rt Runtime, err error, remote string)   // authn_refused / channel_refused_downgrade / limit_exceeded (timeout)
func AuditLimitExceeded(rt Runtime, ctx context.Context, operation, remote, limit string)
var ErrLocalIdentityUnavailable error
```

### `transport/tlsconf`

```go
type Options struct { TrustDomain string; SkewTolerance time.Duration; Now func() time.Time; OnRefusal func(identity.Reason, claimed string) }
func ServerConfig(p identity.Provider, o Options) (*tls.Config, error)   // TLS 1.3 only, client cert required, verifier + VerifyConnection
func ClientConfig(p identity.Provider, expected identity.SPIFFEID, o Options) (*tls.Config, error)
type Peer struct { ID identity.SPIFFEID; ServiceName, Serial string; NotAfter time.Time }
func PeerFromChain(leaf *x509.Certificate) (Peer, error)
func Classify(err error) identity.Reason
```

### `transport/grpc`

```go
type Server struct { *kgrpc.Server /* RegisterService etc. */ }
func NewServer(rt transport.Runtime, opts ...ServerOption) (*Server, error) // chain: recover → guard → correlation → tracing → instrument → authn → authz → user
func SecurityChain(rt transport.Runtime) []middleware.Middleware
type Pool; func NewPool(rt transport.Runtime, disc registry.Discovery) *Pool
func (p *Pool) Conn(ctx, service string) (*grpc.ClientConn, error); Rotate(); Close() error
// ServerOption: WithAddress, WithNetwork, WithMiddleware (appended after the chain), WithTimeout (clamped to limits)
```

### `transport/http`

```go
type Server struct{ /* wraps *khttp.Server; no ListenAndServe/ServeTLS exposed */ }
func NewServer(rt transport.Runtime, opts ...ServerOption) (*Server, error) // eager mTLS listener, 404/405 handlers, body limit, security filter before routing
func (s *Server) Handle / HandleFunc / HandlePrefix / Route / Use / WalkRoute / Endpoint / Start / Stop
func ErrorEncoder(w http.ResponseWriter, r *http.Request, err error)      // {"reason": ...} only
// ServerOption: WithAddress, WithNetwork, WithMiddleware, WithTimeout
```

### `discovery`

```go
func NewStatic(m map[string][]string) (*Static, error)   // registry.Discovery advertising grpcs://host:port
```

## Contract tests (tests/contract)

1. `TestNoPlaintextConstructor` — reflection walk: no exported signature in `freya`, `transport/grpc`, `transport/http` mentions `*tls.Config`, `tls.Certificate`, `grpc.DialOption` or `credentials.TransportCredentials`; returned types expose no `TLS*` methods.
2. `TestHTTPServerNeverServesDefaultMux` — with `net/http/pprof` imported, `/debug/pprof/` → 404, wrong method → 405.
3. `TestIdentityHasNoKeyAccessor` — `identity.Identity`, `identity.Provider`, `authn.PeerIdentity`, `freya.App` expose no key/certificate/byte-slice accessors.
4. Production refusal of insecure options — `TestNewRefusesBrokenSetups` (package `freya`).
5. Middleware order and local-identity guard — `transport/grpc` `TestServerDefaultsAndChain`, `TestExpiredLocalIdentityRefusesCalls`.
