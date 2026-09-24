package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/internal/cred"
)

// Config locates the PEM files.
type Config struct {
	Cert, Key, Bundle string
	// TrustDomain the identity must belong to.
	TrustDomain string
	// PollInterval for change detection (default 2s).
	PollInterval time.Duration
}

type ident struct {
	id     identity.SPIFFEID
	nb, na time.Time
	serial string
}

func (i ident) ID() identity.SPIFFEID { return i.id }
func (i ident) NotBefore() time.Time  { return i.nb }
func (i ident) NotAfter() time.Time   { return i.na }
func (i ident) Serial() string        { return i.serial }

type bundle struct {
	td    string
	roots []*x509.Certificate
	ver   uint64
}

func (b bundle) TrustDomain() string        { return b.td }
func (b bundle) Roots() []*x509.Certificate { return b.roots }
func (b bundle) Version() uint64            { return b.ver }

type state struct {
	crt        tls.Certificate
	id         ident
	bundle     bundle
	certHash   [32]byte
	bundleHash [32]byte
}

// Provider is the file-backed identity provider.
type Provider struct {
	cfg    Config
	cur    atomic.Pointer[state]
	ver    atomic.Uint64
	mu     sync.Mutex
	subs   []chan identity.Update
	stop   chan struct{}
	done   chan struct{}
	closed atomic.Bool
}

var (
	_ identity.Provider = (*Provider)(nil)
	_ cred.Source       = (*Provider)(nil)
)

// New loads the files, validates them, and starts change polling.
func New(cfg Config) (*Provider, error) {
	if !identity.ValidTrustDomain(cfg.TrustDomain) {
		return nil, errors.New("identity/file: trust domain is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	p := &Provider{cfg: cfg, stop: make(chan struct{}), done: make(chan struct{})}
	st, err := p.load(nil)
	if err != nil {
		return nil, err
	}
	p.cur.Store(st)
	go p.poll()
	return p, nil
}

func (p *Provider) load(prev *state) (*state, error) {
	certPEM, err := os.ReadFile(p.cfg.Cert)
	if err != nil {
		return nil, fmt.Errorf("identity/file: cert: %w", err)
	}
	keyPEM, err := os.ReadFile(p.cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("identity/file: key: %w", err)
	}
	bundlePEM, err := os.ReadFile(p.cfg.Bundle)
	if err != nil {
		return nil, fmt.Errorf("identity/file: bundle: %w", err)
	}
	st := &state{certHash: sha256.Sum256(append(append([]byte{}, certPEM...), keyPEM...)), bundleHash: sha256.Sum256(bundlePEM)}
	if prev != nil && prev.certHash == st.certHash && prev.bundleHash == st.bundleHash {
		return prev, nil
	}
	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("identity/file: cert/key: %w", err)
	}
	leaf, err := x509.ParseCertificate(crt.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("identity/file: cert: %w", err)
	}
	crt.Leaf = leaf
	if len(leaf.URIs) != 1 {
		return nil, fmt.Errorf("identity/file: cert must carry exactly one URI SAN, has %d", len(leaf.URIs))
	}
	id, err := identity.ParseSPIFFEID(leaf.URIs[0].String())
	if err != nil {
		return nil, fmt.Errorf("identity/file: cert SAN: %w", err)
	}
	if id.TrustDomain() != p.cfg.TrustDomain {
		return nil, fmt.Errorf("identity/file: cert trust domain %q does not match configured %q", id.TrustDomain(), p.cfg.TrustDomain)
	}
	roots, err := parseRoots(bundlePEM)
	if err != nil {
		return nil, err
	}
	st.crt = crt
	st.id = ident{id: id, nb: leaf.NotBefore, na: leaf.NotAfter, serial: leaf.SerialNumber.String()}
	ver := p.ver.Load()
	if prev == nil || prev.bundleHash != st.bundleHash {
		ver = p.ver.Add(1)
	}
	st.bundle = bundle{td: p.cfg.TrustDomain, roots: roots, ver: ver}
	return st, nil
}

func parseRoots(pemBytes []byte) ([]*x509.Certificate, error) {
	var roots []*x509.Certificate
	rest := bytes.TrimSpace(pemBytes)
	for len(rest) > 0 {
		var blk *pem.Block
		blk, rest = pem.Decode(rest)
		if blk == nil {
			break
		}
		if blk.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(blk.Bytes)
		if err != nil {
			return nil, fmt.Errorf("identity/file: bundle: %w", err)
		}
		roots = append(roots, c)
	}
	if len(roots) == 0 {
		return nil, errors.New("identity/file: bundle contains no certificates")
	}
	return roots, nil
}

func (p *Provider) poll() {
	defer close(p.done)
	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			prev := p.cur.Load()
			st, err := p.load(prev)
			if err != nil {
				p.notify(identity.Update{Identity: prev.id, Bundle: prev.bundle, Err: err})
				continue
			}
			if st == prev {
				continue
			}
			p.cur.Store(st)
			p.notify(identity.Update{Identity: st.id, Bundle: st.bundle})
		}
	}
}

func (p *Provider) notify(u identity.Update) {
	p.mu.Lock()
	subs := append([]chan identity.Update(nil), p.subs...)
	p.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- u:
		default: // a slow subscriber must not stall rotation
		}
	}
}

// Current implements identity.Provider.
func (p *Provider) Current(context.Context) (identity.Identity, identity.Bundle, error) {
	if p.closed.Load() {
		return nil, nil, errors.New("identity/file: provider closed")
	}
	st := p.cur.Load()
	return st.id, st.bundle, nil
}

// Watch implements identity.Provider.
func (p *Provider) Watch(ctx context.Context) (<-chan identity.Update, error) {
	if p.closed.Load() {
		return nil, errors.New("identity/file: provider closed")
	}
	ch := make(chan identity.Update, 8)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	p.mu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			p.unsubscribe(ch)
		case <-p.stop:
		}
	}()
	return ch, nil
}

func (p *Provider) unsubscribe(ch chan identity.Update) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.subs {
		if s == ch {
			p.subs = append(p.subs[:i], p.subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Credential implements cred.Source (module-internal).
func (p *Provider) Credential() (*tls.Certificate, error) {
	if p.closed.Load() {
		return nil, errors.New("identity/file: provider closed")
	}
	c := p.cur.Load().crt
	return &c, nil
}

// Close stops polling and closes all watch channels.
func (p *Provider) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	close(p.stop)
	<-p.done
	p.mu.Lock()
	for _, ch := range p.subs {
		close(ch)
	}
	p.subs = nil
	p.mu.Unlock()
	return nil
}
