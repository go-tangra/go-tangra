// Package freya builds microservices whose every channel is mutually
// authenticated, encrypted, authorized, and audited by construction.
package freya

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/authz"
	_ "github.com/go-freya/freya/authz/file" // registers the file policy source
	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/discovery"
	"github.com/go-freya/freya/identity"
	idfile "github.com/go-freya/freya/identity/file"
	"github.com/go-freya/freya/identity/localdev"
	"github.com/go-freya/freya/identity/spiffe"
	"github.com/go-freya/freya/observe"
	"github.com/go-freya/freya/transport"
	tgrpc "github.com/go-freya/freya/transport/grpc"
	thttp "github.com/go-freya/freya/transport/http"
	"github.com/go-freya/freya/transport/tlsconf"
	"github.com/go-kratos/kratos/v3"
	klog "github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/registry"
	ktransport "github.com/go-kratos/kratos/v3/transport"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
)

// App is a configured Freya application. Obtain one with New.
type App struct {
	cfg       config.Config
	log       *slog.Logger
	emitter   *audit.Emitter
	provider  identity.Provider
	ownProv   bool
	authz     atomic.Pointer[authorizerBox]
	disc      registry.Discovery
	grpcSrv   *tgrpc.Server
	httpSrv   *thttp.Server
	pool      *tgrpc.Pool
	kapp      *kratos.App
	closed    atomic.Bool
	policyOK  atomic.Bool
	revoke    identity.RevocationChecker
	watchCtx  context.CancelFunc
	lifecycle *identity.Lifecycle
	admin     *observe.Admin
	metrics   *observe.Metrics
	tracer    trace.TracerProvider
	// lifecycleStarted flips on the first Valid event (initial issue, not a renewal).
	lifecycleStarted atomic.Bool
}

type authorizerBox struct{ a authz.Authorizer }

var _ transport.Runtime = (*App)(nil)

// New validates cfg, starts the identity provider, loads policy, and builds the
// servers. It returns an error — never a degraded app — when any security
// prerequisite is missing.
func New(cfg config.Config, opts ...Option) (*App, error) {
	var o options
	for _, f := range opts {
		f(&o)
	}
	if o.provider != nil || o.localDev {
		cfg.Identity.Provider = config.ProviderProvided
	}
	if o.policy != nil || o.allowAll {
		cfg.Authz.Source = config.AuthzProvided
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.IsProduction() && (o.localDev || o.allowAll) {
		return nil, errors.New("freya: WithInsecureLocalDev/WithAllowAllPolicy are refused when env is production")
	}
	if o.logger == nil {
		o.logger = slog.NewJSONHandler(os.Stderr, nil)
	}
	log := slog.New(audit.NewRedactingHandler(o.logger)).With("service", cfg.ServiceName)
	a := &App{cfg: cfg, log: log, revoke: o.revoke, tracer: o.tracer}
	if a.tracer == nil {
		a.tracer = noop.NewTracerProvider()
	}
	var err error
	if a.metrics, err = observe.NewMetrics(); err != nil {
		return nil, fmt.Errorf("freya: metrics: %w", err)
	}
	a.emitter = audit.NewEmitter(audit.NewSlogSink(log), 1024, o.sinks...)
	for _, w := range cfg.Warnings() {
		log.Warn("insecure configuration override accepted", "warning", w)
	}

	if err := a.initIdentity(o); err != nil {
		a.Close()
		return nil, err
	}
	if err := a.initPolicy(o); err != nil {
		a.Close()
		return nil, err
	}
	if err := a.initDiscovery(o); err != nil {
		a.Close()
		return nil, err
	}
	if a.grpcSrv, err = tgrpc.NewServer(a, tgrpc.WithAddress(cfg.Server.GRPCAddr)); err != nil {
		a.Close()
		return nil, err
	}
	if cfg.Server.HTTPAddr != "" {
		if a.httpSrv, err = thttp.NewServer(a, thttp.WithAddress(cfg.Server.HTTPAddr)); err != nil {
			a.Close()
			return nil, err
		}
	}
	a.pool = tgrpc.NewPool(a, a.disc)
	adminTLS, err := tlsconf.ServerConfig(a.provider, transport.TLSOptions(a))
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("freya: admin listener: %w", err)
	}
	if a.admin, err = observe.NewAdmin(cfg.Admin, a.Ready, a.metrics.Handler(), log, adminTLS); err != nil {
		a.Close()
		return nil, fmt.Errorf("freya: admin listener: %w", err)
	}
	// Bind listeners now: a busy port fails New, and Endpoint() can be read at
	// any time without racing the running server.
	for _, s := range a.servers() {
		if ep, ok := s.(ktransport.Endpointer); ok {
			if _, err := ep.Endpoint(); err != nil {
				a.Close()
				return nil, fmt.Errorf("freya: bind: %w", err)
			}
		}
	}
	return a, nil
}

func (a *App) servers() []ktransport.Server {
	srvs := []ktransport.Server{a.grpcSrv}
	if a.httpSrv != nil {
		srvs = append(srvs, a.httpSrv)
	}
	return srvs
}

func (a *App) initIdentity(o options) error {
	cfg := a.cfg
	switch {
	case o.provider != nil:
		a.provider = o.provider
	case o.localDev:
		p, err := localdev.New(cfg.TrustDomain, cfg.ServiceName, cfg.Identity.MaxLifetime)
		if err != nil {
			return err
		}
		a.provider, a.ownProv = p, true
		a.log.Warn("insecure local-dev identity in use: self-issued CA, not for deployment")
		a.audit(audit.Event{Type: audit.TypeInsecureModeEnabled, Outcome: audit.OutcomeOK, Reason: audit.ReasonLocalDev})
	case cfg.Identity.Provider == config.ProviderFile:
		p, err := idfile.New(idfile.Config{
			Cert: cfg.Identity.File.Cert, Key: cfg.Identity.File.Key, Bundle: cfg.Identity.File.Bundle,
			TrustDomain: cfg.TrustDomain,
		})
		if err != nil {
			return err
		}
		a.provider, a.ownProv = p, true
	default:
		p, err := spiffe.New(context.Background(), spiffe.Config{
			Socket: cfg.Identity.WorkloadSocket, TrustDomain: cfg.TrustDomain, StartupTimeout: cfg.Identity.StartupTimeout,
		})
		if err != nil {
			return err
		}
		a.provider, a.ownProv = p, true
	}
	id, bundle, err := a.provider.Current(context.Background())
	if err != nil {
		return fmt.Errorf("freya: identity unavailable: %w", err)
	}
	if id.ID().ServiceName() != cfg.ServiceName || id.ID().TrustDomain() != cfg.TrustDomain {
		return fmt.Errorf("freya: identity %s does not match configured service %s", id.ID(), cfg.LocalSPIFFEID())
	}
	if len(bundle.Roots()) == 0 {
		return errors.New("freya: trust bundle is empty")
	}
	a.log.Info("identity ready", "id", id.ID().String(), "not_after", id.NotAfter().UTC().Format(time.RFC3339), "serial", id.Serial())
	a.audit(audit.Event{Type: audit.TypeIdentityIssued, Outcome: audit.OutcomeOK, Reason: audit.ReasonIssued,
		Attrs: map[string]string{"serial": id.Serial(), "not_after": id.NotAfter().UTC().Format(time.RFC3339)}})
	return a.watchIdentity(id.Serial(), bundle.Version())
}

// watchIdentity runs the identity lifecycle: renewal scheduling, expiry
// detection, and audit events for every transition.
func (a *App) watchIdentity(_ string, _ uint64) error {
	ctx, cancel := context.WithCancel(context.Background())
	a.watchCtx = cancel
	a.lifecycle = identity.NewLifecycle(a.provider, identity.LifecycleConfig{
		RenewAt: a.cfg.Identity.RenewAt,
		OnEvent: a.onIdentityEvent,
	})
	if err := a.lifecycle.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("freya: identity lifecycle: %w", err)
	}
	return nil
}

func (a *App) onIdentityEvent(e identity.LifecycleEvent) {
	attrs := map[string]string{}
	if e.Identity != nil {
		attrs["serial"] = e.Identity.Serial()
		attrs["not_after"] = e.Identity.NotAfter().UTC().Format(time.RFC3339)
	}
	switch e.State {
	case identity.StateValid:
		if a.lifecycleStarted.Swap(true) {
			a.log.Info("identity renewed", "serial", attrs["serial"], "not_after", attrs["not_after"])
			a.metrics.IdentityRenewal("ok")
			a.audit(audit.Event{Type: audit.TypeIdentityRenewed, Outcome: audit.OutcomeOK, Reason: audit.ReasonRenewed, Attrs: attrs})
			if a.pool != nil {
				a.pool.Rotate()
			}
		}
	case identity.StateRenewing:
		a.log.Debug("identity renewal started")
	case identity.StateRenewFailed:
		msg := ""
		if e.Err != nil {
			msg = e.Err.Error()
		}
		a.log.Error("identity renewal failed; keeping last-known-good until expiry", "err", msg)
		a.metrics.IdentityRenewal("failed")
		a.audit(audit.Event{Type: audit.TypeIdentityRenewFailed, Outcome: audit.OutcomeFailed, Reason: audit.ReasonProviderUnavailable, Attrs: attrs})
	case identity.StateExpired:
		a.log.Error("identity expired and no renewal available: refusing all calls until renewed")
		a.audit(audit.Event{Type: audit.TypeIdentityExpired, Outcome: audit.OutcomeFailed, Reason: audit.ReasonIdentityExpired, Attrs: attrs})
	case identity.StateBundleUpdated:
		if e.Bundle != nil {
			attrs["bundle_version"] = fmt.Sprint(e.Bundle.Version())
			attrs["roots"] = fmt.Sprint(len(e.Bundle.Roots()))
		}
		a.log.Info("trust bundle updated", "bundle_version", attrs["bundle_version"], "roots", attrs["roots"])
		a.audit(audit.Event{Type: audit.TypeTrustBundleUpdated, Outcome: audit.OutcomeOK, Reason: audit.ReasonBundleUpdated, Attrs: attrs})
	}
}

func (a *App) initPolicy(o options) error {
	switch {
	case o.allowAll:
		a.setAuthorizer(authz.AllowAll{})
		a.log.Warn("allow-all authorization policy in use: every authenticated caller is permitted")
		a.audit(audit.Event{Type: audit.TypeInsecureModeEnabled, Outcome: audit.OutcomeOK, Reason: audit.ReasonAllowAll})
	case o.policy != nil:
		return a.startPolicySource(o.policy)
	default:
		src, err := authz.NewSourceFromConfig(a.cfg.Authz)
		if err != nil {
			return err
		}
		return a.startPolicySource(src)
	}
	return nil
}

func (a *App) startPolicySource(src authz.Source) error {
	ctx := context.Background()
	pol, err := src.Load(ctx)
	if err != nil {
		a.audit(audit.Event{Type: audit.TypePolicyLoadFailed, Outcome: audit.OutcomeFailed, Reason: audit.ReasonPolicyInvalid})
		return fmt.Errorf("freya: policy: %w", err)
	}
	cached := authz.NewCached(pol, 0)
	a.setAuthorizer(cached)
	a.audit(audit.Event{Type: audit.TypePolicyLoaded, Outcome: audit.OutcomeOK, Reason: audit.ReasonLoaded, PolicyVersion: pol.Version})
	a.log.Info("policy loaded", "version", pol.Version, "rules", len(pol.Rules))
	if len(pol.Rules) == 0 {
		a.log.Warn("policy has no rules: every call will be denied (deny by default)")
	}
	ch, err := src.Watch(ctx)
	if err != nil {
		return fmt.Errorf("freya: policy watch: %w", err)
	}
	go func() {
		for p := range ch {
			if p == nil {
				continue
			}
			cached.Swap(p)
			a.audit(audit.Event{Type: audit.TypePolicyLoaded, Outcome: audit.OutcomeOK, Reason: audit.ReasonLoaded, PolicyVersion: p.Version})
			a.log.Info("policy reloaded", "version", p.Version, "rules", len(p.Rules))
		}
	}()
	if es, ok := src.(interface{ Errors() <-chan error }); ok {
		go func() {
			for err := range es.Errors() {
				a.audit(audit.Event{Type: audit.TypePolicyLoadFailed, Outcome: audit.OutcomeFailed, Reason: audit.ReasonPolicyInvalid})
				a.log.Error("policy reload failed; keeping last-known-good", "err", err.Error())
			}
		}()
	}
	return nil
}

func (a *App) setAuthorizer(z authz.Authorizer) {
	a.authz.Store(&authorizerBox{a: z})
	a.policyOK.Store(z != nil)
}

func (a *App) initDiscovery(o options) error {
	if o.discovery != nil {
		a.disc = o.discovery
		return nil
	}
	if len(a.cfg.Discovery.Static) == 0 {
		return nil
	}
	s, err := discovery.NewStatic(a.cfg.Discovery.Static)
	if err != nil {
		return err
	}
	a.disc = s
	return nil
}

func (a *App) audit(ev audit.Event) {
	ev.LocalID = a.cfg.LocalSPIFFEID()
	if ev.CorrelationID == "" {
		ev.CorrelationID = observe.NewCorrelationID()
	}
	_ = a.emitter.Emit(context.Background(), ev)
}

// Run starts the servers and blocks until ctx is cancelled or a server fails.
func (a *App) Run(ctx context.Context) error {
	srvs := a.servers()
	klog.SetDefault(a.log)
	a.kapp = kratos.New(
		kratos.Name(a.cfg.ServiceName),
		kratos.Logger(a.log),
		kratos.Server(srvs...),
		kratos.Context(ctx),
		kratos.StopTimeout(10*time.Second),
	)
	errc := make(chan error, 1)
	go func() { errc <- a.kapp.Run() }()
	adminErr := make(chan error, 1)
	go func() { adminErr <- a.admin.Serve() }()
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.admin.Shutdown(sctx)
	}()
	select {
	case err := <-errc:
		return err
	case err := <-adminErr:
		_ = a.kapp.Stop()
		<-errc
		return fmt.Errorf("freya: admin listener: %w", err)
	case <-ctx.Done():
		if err := a.kapp.Stop(); err != nil {
			return err
		}
		return <-errc
	}
}

// AdminURL returns the base URL of the admin listener (health, readiness, metrics).
func (a *App) AdminURL() string { return a.admin.URL() }

// IdentityState reports the lifecycle state ("valid", "renewing", "expired", ...).
func (a *App) IdentityState() string {
	if a.lifecycle == nil {
		return string(identity.StatePending)
	}
	return string(a.lifecycle.State())
}

// Close releases the identity provider, audit queue, and client connections.
func (a *App) Close() {
	if a.closed.Swap(true) {
		return
	}
	if a.pool != nil {
		_ = a.pool.Close()
	}
	if a.watchCtx != nil {
		a.watchCtx()
	}
	if a.admin != nil {
		_ = a.admin.Shutdown(context.Background())
	}
	if a.ownProv && a.provider != nil {
		_ = a.provider.Close()
	}
	if a.emitter != nil {
		a.emitter.Close()
	}
}

// GRPC returns the mTLS gRPC server; register services on it.
func (a *App) GRPC() *tgrpc.Server { return a.grpcSrv }

// HTTP returns the mTLS HTTP server, or nil when server.http_addr is unset.
func (a *App) HTTP() *thttp.Server { return a.httpSrv }

// Client returns the pooled, mutually authenticated connection to service.
func (a *App) Client(ctx context.Context, service string) (*grpc.ClientConn, error) {
	return a.pool.Conn(ctx, service)
}

// Identity returns the current local identity (no key material).
func (a *App) Identity() identity.Identity {
	id, _, err := a.provider.Current(context.Background())
	if err != nil {
		return nil
	}
	return id
}

// Ready reports whether the identity is valid (per the lifecycle), the trust
// bundle is non-empty, and a policy is loaded.
func (a *App) Ready() bool {
	if a.closed.Load() || !a.policyOK.Load() {
		return false
	}
	if a.lifecycle != nil {
		if st := a.lifecycle.State(); st != identity.StateValid && st != identity.StateRenewing {
			return false
		}
	}
	id, bundle, err := a.provider.Current(context.Background())
	if err != nil || len(bundle.Roots()) == 0 {
		return false
	}
	now := time.Now()
	return !now.Before(id.NotBefore()) && now.Before(id.NotAfter())
}

// Runtime contract for the transports.

func (a *App) ServiceName() string                    { return a.cfg.ServiceName }
func (a *App) TrustDomain() string                    { return a.cfg.TrustDomain }
func (a *App) LocalID() string                        { return a.cfg.LocalSPIFFEID() }
func (a *App) Provider() identity.Provider            { return a.provider }
func (a *App) Limits() config.Limits                  { return a.cfg.Limits }
func (a *App) SkewTolerance() time.Duration           { return a.cfg.Identity.SkewTolerance }
func (a *App) Audit() *audit.Emitter                  { return a.emitter }
func (a *App) Logger() *slog.Logger                   { return a.log }
func (a *App) Revocation() identity.RevocationChecker { return a.revoke }
func (a *App) Metrics() *observe.Metrics              { return a.metrics }
func (a *App) Tracer() trace.TracerProvider           { return a.tracer }

// TracerProvider returns the provider used for spans (for application code).
func (a *App) TracerProvider() trace.TracerProvider { return a.tracer }
func (a *App) Authorizer() authz.Authorizer {
	if b := a.authz.Load(); b != nil {
		return b.a
	}
	return nil
}
