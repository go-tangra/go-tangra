package localdev

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/internal/cred"
)

// Provider is a self-issued identity provider for a single workstation. It is
// only constructible through freya.WithInsecureLocalDev(); it is never a default.
type Provider struct {
	mu     sync.Mutex
	ca     *testutil.CA
	name   string
	crt    tls.Certificate
	ttl    time.Duration
	subs   []chan identity.Update
	closed bool
	ver    uint64
}

var (
	_ identity.Provider = (*Provider)(nil)
	_ cred.Source       = (*Provider)(nil)
)

// New creates a throw-away CA for trustDomain and issues an SVID for name.
func New(trustDomain, name string, ttl time.Duration) (*Provider, error) {
	if !identity.ValidServiceName(name) || !identity.ValidTrustDomain(trustDomain) {
		return nil, errors.New("localdev: invalid trust domain or service name")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	ca, err := testutil.NewCA(trustDomain)
	if err != nil {
		return nil, err
	}
	p := &Provider{ca: ca, name: name, ttl: ttl, ver: 1}
	if err := p.issue(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) issue() error {
	crt, err := p.ca.Issue(p.name, testutil.IssueOptions{NotAfter: time.Now().Add(p.ttl)})
	if err != nil {
		return err
	}
	p.crt = crt
	return nil
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
	td   string
	root *x509.Certificate
	ver  uint64
}

func (b bundle) TrustDomain() string        { return b.td }
func (b bundle) Roots() []*x509.Certificate { return []*x509.Certificate{b.root} }
func (b bundle) Version() uint64            { return b.ver }

// Current re-issues automatically when less than half the lifetime remains.
func (p *Provider) Current(context.Context) (identity.Identity, identity.Bundle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, nil, errors.New("localdev: provider closed")
	}
	if time.Until(p.crt.Leaf.NotAfter) < p.ttl/2 {
		if err := p.issue(); err != nil {
			return nil, nil, err
		}
		u := identity.Update{Identity: ident{p.crt.Leaf}, Bundle: bundle{p.ca.TrustDomain, p.ca.Cert, p.ver}}
		for _, ch := range p.subs {
			select {
			case ch <- u:
			default:
			}
		}
	}
	return ident{p.crt.Leaf}, bundle{p.ca.TrustDomain, p.ca.Cert, p.ver}, nil
}

// Watch implements identity.Provider.
func (p *Provider) Watch(context.Context) (<-chan identity.Update, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("localdev: provider closed")
	}
	ch := make(chan identity.Update, 4)
	p.subs = append(p.subs, ch)
	return ch, nil
}

// Credential implements cred.Source.
func (p *Provider) Credential() (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("localdev: provider closed")
	}
	c := p.crt
	return &c, nil
}

// Close implements identity.Provider.
func (p *Provider) Close() error {
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

// TrustBundlePEM exposes the dev CA so a second local service can trust it.
func (p *Provider) TrustBundlePEM() []byte { return p.ca.BundlePEM() }
