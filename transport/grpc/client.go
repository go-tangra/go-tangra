package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/registry"
	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/observe"
	"github.com/go-tangra/go-tangra/v4/transport"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
	"google.golang.org/grpc"
)

// Pool hands out one mutually authenticated connection per callee logical name.
// The callee is verified by SPIFFE ID, never by the address discovery returned.
//
// A *grpc.ClientConn the pool hands out stays valid until Close: callers keep
// it for the life of the process (registration leases, token verifiers,
// generated clients built once at startup), so the pool never closes one
// behind their back. Rotate re-verifies peers by dropping the network
// connections underneath instead (see Rotate).
type Pool struct {
	rt     transport.Runtime
	disc   registry.Discovery
	mu     sync.Mutex
	conns  map[string]*grpc.ClientConn
	closed bool
	links  *links
}

// NewPool creates a pool resolving names through disc (may be nil: every Conn
// then fails until discovery is configured).
func NewPool(rt transport.Runtime, disc registry.Discovery) *Pool {
	return &Pool{rt: rt, disc: disc, conns: map[string]*grpc.ClientConn{}, links: &links{set: map[*link]struct{}{}}}
}

// links tracks the pool's live network connections so Rotate can drop them
// without closing the *grpc.ClientConn that owns them.
type links struct {
	mu  sync.Mutex
	set map[*link]struct{}
}

type link struct {
	net.Conn
	owner *links
	once  sync.Once
}

func (l *link) Close() error {
	l.once.Do(func() {
		l.owner.mu.Lock()
		delete(l.owner.set, l)
		l.owner.mu.Unlock()
	})
	return l.Conn.Close()
}

func (ls *links) dial(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	l := &link{Conn: c, owner: ls}
	ls.mu.Lock()
	ls.set[l] = struct{}{}
	ls.mu.Unlock()
	return l, nil
}

// snapshot returns the connections open now.
func (ls *links) snapshot() []*link {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	out := make([]*link, 0, len(ls.set))
	for l := range ls.set {
		out = append(out, l)
	}
	return out
}

// Conn returns the pooled connection to service, dialing on first use.
func (p *Pool) Conn(ctx context.Context, service string) (*grpc.ClientConn, error) {
	expected, err := identity.NewSPIFFEID(p.rt.TrustDomain(), service)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	if p.disc == nil {
		return nil, errors.New("client: no discovery configured")
	}
	if !transport.LocalIdentityValid(p.rt) {
		return nil, transport.ErrLocalIdentityUnavailable
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("client: pool closed")
	}
	if c, ok := p.conns[service]; ok {
		return c, nil
	}
	if _, err := p.disc.GetService(ctx, service); err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	opts := transport.TLSOptions(p.rt)
	opts.OnRefusal = func(reason identity.Reason, claimed string) {
		_ = p.rt.Audit().Emit(ctx, audit.Event{
			Type: audit.TypeAuthnRefused, Outcome: audit.OutcomeRefused, Reason: audit.Reason(reason),
			LocalID: p.rt.LocalID(), ClaimedPeerID: claimed, Operation: "dial " + expected.String(),
			CorrelationID: observe.NewCorrelationID(),
		})
	}
	tlsCfg, err := tlsconf.ClientConfig(p.rt.Provider(), expected, opts)
	if err != nil {
		return nil, err
	}
	lim := p.rt.Limits()
	conn, err := kgrpc.NewClient(ctx,
		kgrpc.WithEndpoint("discovery:///"+service),
		kgrpc.WithDiscovery(p.disc),
		kgrpc.WithTLSConfig(tlsCfg),
		kgrpc.WithTimeout(lim.RequestTimeout),
		kgrpc.WithMiddleware(observe.ClientCorrelation(), observe.ClientTracing(p.rt.Tracer())),
		kgrpc.WithHealthCheck(false),
		kgrpc.WithOptions(grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(int(lim.MaxRequestBytes)),
			grpc.MaxCallSendMsgSize(int(lim.MaxRequestBytes)),
		), grpc.WithContextDialer(p.links.dial)),
	)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	p.conns[service] = conn
	return conn, nil
}

// Rotate makes every peer be re-verified against the current trust bundle. It
// is used ONLY when the trust bundle changes (a root added or removed). Leaf
// renewal must NOT call this: tlsconf serves the renewed cert per-handshake.
//
// The network connections open now are dropped after the request timeout (so
// in-flight calls finish); each *grpc.ClientConn stays open and reconnects
// with a fresh handshake, which tlsconf verifies against the bundle current
// at that moment. Closing the *grpc.ClientConn instead (as before) left every
// caller that kept it — gateway registration, the token verifier — retrying a
// closed connection ("the client connection is closing") until restarted.
// Long-lived streams on the old links end with Unavailable and their callers
// open them again on the same ClientConn.
func (p *Pool) Rotate() {
	old := p.links.snapshot()
	if len(old) == 0 {
		return
	}
	grace := p.rt.Limits().RequestTimeout
	time.AfterFunc(grace, func() {
		for _, l := range old {
			_ = l.Close()
		}
	})
}

// Close closes every connection; the pool cannot be reused.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	var first error
	for name, c := range p.conns {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
		delete(p.conns, name)
	}
	return first
}
