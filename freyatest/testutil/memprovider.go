package testutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// MemProvider is an in-memory identity.Provider (plus credential source) for tests.
type MemProvider struct {
	mu     sync.Mutex
	crt    tls.Certificate
	roots  []*x509.Certificate
	td     string
	ver    uint64
	fail   error
	subs   []chan identity.Update
	closed bool
}

// NewMemProvider returns a provider presenting crt, trusting ca.
func NewMemProvider(ca *CA, crt tls.Certificate) *MemProvider {
	return &MemProvider{crt: crt, roots: []*x509.Certificate{ca.Cert}, td: ca.TrustDomain, ver: 1}
}

type memIdentity struct{ leaf *x509.Certificate }

func (m memIdentity) ID() identity.SPIFFEID {
	id, _ := identity.ParseSPIFFEID(m.leaf.URIs[0].String())
	return id
}
func (m memIdentity) NotBefore() time.Time { return m.leaf.NotBefore }
func (m memIdentity) NotAfter() time.Time  { return m.leaf.NotAfter }
func (m memIdentity) Serial() string       { return m.leaf.SerialNumber.String() }

type memBundle struct {
	td    string
	roots []*x509.Certificate
	ver   uint64
}

func (b memBundle) TrustDomain() string        { return b.td }
func (b memBundle) Roots() []*x509.Certificate { return b.roots }
func (b memBundle) Version() uint64            { return b.ver }

// Current implements identity.Provider.
func (p *MemProvider) Current(context.Context) (identity.Identity, identity.Bundle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail != nil {
		return nil, nil, p.fail
	}
	if p.closed {
		return nil, nil, errors.New("closed")
	}
	return memIdentity{p.crt.Leaf}, memBundle{p.td, p.roots, p.ver}, nil
}

// Watch implements identity.Provider.
func (p *MemProvider) Watch(ctx context.Context) (<-chan identity.Update, error) {
	ch := make(chan identity.Update, 8)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	p.mu.Unlock()
	return ch, nil
}

// Close implements identity.Provider.
func (p *MemProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	for _, ch := range p.subs {
		close(ch)
	}
	p.subs = nil
	return nil
}

// Credential implements the module-internal credential source.
func (p *MemProvider) Credential() (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail != nil {
		return nil, p.fail
	}
	c := p.crt
	return &c, nil
}

// Rotate swaps the presented certificate and notifies watchers.
func (p *MemProvider) Rotate(crt tls.Certificate) {
	p.mu.Lock()
	p.crt = crt
	subs := append([]chan identity.Update(nil), p.subs...)
	u := identity.Update{Identity: memIdentity{crt.Leaf}, Bundle: memBundle{p.td, p.roots, p.ver}}
	p.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- u:
		default:
		}
	}
}

// SetRoots replaces the trust bundle and bumps its version.
func (p *MemProvider) SetRoots(roots ...*x509.Certificate) {
	p.mu.Lock()
	p.roots = roots
	p.ver++
	p.mu.Unlock()
}

// Fail makes every call return err (nil restores service).
func (p *MemProvider) Fail(err error) {
	p.mu.Lock()
	p.fail = err
	p.mu.Unlock()
}

// SendUpdate delivers an arbitrary update to watchers (tests of error paths).
func (p *MemProvider) SendUpdate(u identity.Update) {
	p.mu.Lock()
	subs := append([]chan identity.Update(nil), p.subs...)
	p.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- u:
		default:
		}
	}
}

// SetCert replaces the certificate without notifying watchers.
func (p *MemProvider) SetCert(crt tls.Certificate) {
	p.mu.Lock()
	p.crt = crt
	p.mu.Unlock()
}
