package spiffe

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/internal/cred"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// Config for the Workload API provider.
type Config struct {
	// Socket is the Workload API address, e.g. unix:///run/spire/sockets/agent.sock.
	Socket      string
	TrustDomain string
	// StartupTimeout bounds the wait for the first SVID (default 30s).
	StartupTimeout time.Duration
}

// Provider is the Workload API-backed identity provider. The agent pushes
// rotated SVIDs and bundle updates; nothing here ever writes key material to
// disk. It watches through workloadapi.Client directly (not X509Source) so
// that every read of the current context is mutex-guarded.
type Provider struct {
	client *workloadapi.Client
	td     spiffeid.TrustDomain
	tdStr  string

	mu       sync.RWMutex
	crt      tls.Certificate
	leaf     *x509.Certificate
	roots    []*x509.Certificate
	ver      uint64
	hash     [32]byte
	haveCtx  bool
	firstErr error

	first  chan struct{}
	once   sync.Once
	subs   []chan identity.Update
	closed atomic.Bool
	cancel context.CancelFunc
	done   chan struct{}
}

var (
	_ identity.Provider              = (*Provider)(nil)
	_ cred.Source                    = (*Provider)(nil)
	_ workloadapi.X509ContextWatcher = (*Provider)(nil)
)

// New connects to the Workload API and waits (up to StartupTimeout) for the
// first SVID. It fails closed if the agent is unreachable or issues an identity
// outside the configured trust domain.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.Socket == "" {
		return nil, errors.New("identity/spiffe: workload socket is required")
	}
	if !identity.ValidTrustDomain(cfg.TrustDomain) {
		return nil, errors.New("identity/spiffe: invalid trust domain")
	}
	td, err := spiffeid.TrustDomainFromString(cfg.TrustDomain)
	if err != nil {
		return nil, fmt.Errorf("identity/spiffe: %w", err)
	}
	if cfg.StartupTimeout <= 0 {
		cfg.StartupTimeout = 30 * time.Second
	}
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(cfg.Socket))
	if err != nil {
		return nil, fmt.Errorf("identity/spiffe: workload api: %w", err)
	}
	wctx, cancel := context.WithCancel(context.Background())
	p := &Provider{client: client, td: td, tdStr: cfg.TrustDomain, first: make(chan struct{}), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		_ = client.WatchX509Context(wctx, p)
	}()
	select {
	case <-p.first:
	case <-time.After(cfg.StartupTimeout):
		_ = p.Close()
		return nil, errors.New("identity/spiffe: no identity from the workload api within the startup timeout")
	case <-ctx.Done():
		_ = p.Close()
		return nil, ctx.Err()
	}
	p.mu.RLock()
	err = p.firstErr
	p.mu.RUnlock()
	if err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

// OnX509ContextUpdate implements workloadapi.X509ContextWatcher.
func (p *Provider) OnX509ContextUpdate(c *workloadapi.X509Context) {
	u, err := p.adopt(c)
	if err != nil {
		p.mu.Lock()
		if !p.haveCtx {
			p.firstErr = err
		}
		p.mu.Unlock()
		p.once.Do(func() { close(p.first) })
		p.notify(identity.Update{Err: err})
		return
	}
	p.once.Do(func() { close(p.first) })
	p.notify(u)
}

// OnX509ContextWatchError implements workloadapi.X509ContextWatcher.
func (p *Provider) OnX509ContextWatchError(err error) {
	if errors.Is(err, context.Canceled) || p.closed.Load() {
		return
	}
	p.mu.RLock()
	have := p.haveCtx
	p.mu.RUnlock()
	if have {
		p.notify(identity.Update{Err: fmt.Errorf("identity/spiffe: workload api: %w", err)})
	}
}

func (p *Provider) adopt(c *workloadapi.X509Context) (identity.Update, error) {
	svid := c.DefaultSVID()
	if svid == nil || len(svid.Certificates) == 0 {
		return identity.Update{}, errors.New("identity/spiffe: empty SVID")
	}
	leaf := svid.Certificates[0]
	if len(leaf.URIs) != 1 {
		return identity.Update{}, errors.New("identity/spiffe: SVID must carry exactly one URI SAN")
	}
	id, err := identity.ParseSPIFFEID(leaf.URIs[0].String())
	if err != nil {
		return identity.Update{}, fmt.Errorf("identity/spiffe: SVID id: %w", err)
	}
	if id.TrustDomain() != p.tdStr {
		return identity.Update{}, fmt.Errorf("identity/spiffe: SVID trust domain %q does not match %q", id.TrustDomain(), p.tdStr)
	}
	b, err := c.Bundles.GetX509BundleForTrustDomain(p.td)
	if err != nil {
		return identity.Update{}, fmt.Errorf("identity/spiffe: bundle: %w", err)
	}
	roots := b.X509Authorities()
	if len(roots) == 0 {
		return identity.Update{}, errors.New("identity/spiffe: empty trust bundle")
	}
	h := sha256.New()
	for _, r := range roots {
		h.Write(r.Raw)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	crt := tls.Certificate{PrivateKey: svid.PrivateKey, Leaf: leaf}
	for _, cert := range svid.Certificates {
		crt.Certificate = append(crt.Certificate, cert.Raw)
	}
	p.mu.Lock()
	if !p.haveCtx || sum != p.hash {
		p.hash = sum
		p.ver++
	}
	p.crt, p.leaf, p.roots, p.haveCtx = crt, leaf, roots, true
	u := identity.Update{Identity: ident{leaf}, Bundle: bundle{td: p.tdStr, roots: roots, ver: p.ver}}
	p.mu.Unlock()
	return u, nil
}

func (p *Provider) notify(u identity.Update) {
	p.mu.RLock()
	subs := append([]chan identity.Update(nil), p.subs...)
	p.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- u:
		default:
		}
	}
}

type ident struct{ leaf *x509.Certificate }

func (i ident) ID() identity.SPIFFEID {
	id, _ := identity.ParseSPIFFEID(i.leaf.URIs[0].String())
	return id
}
func (i ident) NotBefore() time.Time { return i.leaf.NotBefore }
func (i ident) NotAfter() time.Time  { return i.leaf.NotAfter }
func (i ident) Serial() string       { return i.leaf.SerialNumber.String() }

type bundle struct {
	td    string
	roots []*x509.Certificate
	ver   uint64
}

func (b bundle) TrustDomain() string        { return b.td }
func (b bundle) Roots() []*x509.Certificate { return b.roots }
func (b bundle) Version() uint64            { return b.ver }

// Current implements identity.Provider.
func (p *Provider) Current(context.Context) (identity.Identity, identity.Bundle, error) {
	if p.closed.Load() {
		return nil, nil, errors.New("identity/spiffe: provider closed")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.haveCtx {
		return nil, nil, errors.New("identity/spiffe: no identity yet")
	}
	return ident{p.leaf}, bundle{td: p.tdStr, roots: p.roots, ver: p.ver}, nil
}

// Watch implements identity.Provider.
func (p *Provider) Watch(ctx context.Context) (<-chan identity.Update, error) {
	if p.closed.Load() {
		return nil, errors.New("identity/spiffe: provider closed")
	}
	ch := make(chan identity.Update, 8)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	p.mu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			p.mu.Lock()
			for i, c := range p.subs {
				if c == ch {
					p.subs = append(p.subs[:i], p.subs[i+1:]...)
					close(ch)
					break
				}
			}
			p.mu.Unlock()
		case <-p.done:
		}
	}()
	return ch, nil
}

// Credential implements cred.Source (module-internal).
func (p *Provider) Credential() (*tls.Certificate, error) {
	if p.closed.Load() {
		return nil, errors.New("identity/spiffe: provider closed")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.haveCtx {
		return nil, errors.New("identity/spiffe: no identity yet")
	}
	c := p.crt
	return &c, nil
}

// Close implements identity.Provider.
func (p *Provider) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	p.cancel()
	<-p.done
	p.mu.Lock()
	for _, ch := range p.subs {
		close(ch)
	}
	p.subs = nil
	p.mu.Unlock()
	return p.client.Close()
}
