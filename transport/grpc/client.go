package grpc

import (
	"context"
	"errors"
	"fmt"
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
type Pool struct {
	rt     transport.Runtime
	disc   registry.Discovery
	mu     sync.Mutex
	conns  map[string]*grpc.ClientConn
	closed bool
}

// NewPool creates a pool resolving names through disc (may be nil: every Conn
// then fails until discovery is configured).
func NewPool(rt transport.Runtime, disc registry.Discovery) *Pool {
	return &Pool{rt: rt, disc: disc, conns: map[string]*grpc.ClientConn{}}
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
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	p.conns[service] = conn
	return conn, nil
}

// Rotate drains the pool: existing connections are closed after the request
// timeout (so in-flight calls finish) and the next Conn dials fresh. It is
// used ONLY when the trust bundle changes (a root added or removed) so peers
// are re-verified against the new root set. Leaf renewal must NOT call this:
// tlsconf serves the renewed cert per-handshake, so closing live connections
// would needlessly drop long-lived streams on every rotation.
func (p *Pool) Rotate() {
	p.mu.Lock()
	old := p.conns
	p.conns = map[string]*grpc.ClientConn{}
	p.mu.Unlock()
	if len(old) == 0 {
		return
	}
	grace := p.rt.Limits().RequestTimeout
	time.AfterFunc(grace, func() {
		for _, c := range old {
			_ = c.Close()
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
