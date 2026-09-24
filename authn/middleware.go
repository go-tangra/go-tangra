package authn

import (
	"context"
	"crypto/x509"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/observe"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// Config wires the middleware to the runtime.
type Config struct {
	TrustDomain string
	LocalID     string
	Audit       *audit.Emitter
	// Revocation may be nil. A checker error is treated as revoked.
	Revocation identity.RevocationChecker
	Now        func() time.Time
}

// Middleware re-derives the peer identity from the TLS layer on every call
// (defence in depth: it does not trust that the transport already did), checks
// revocation, audits refusals, and exposes the peer via FromContext.
func Middleware(cfg Config) middleware.Middleware {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			leaf, remote := peerLeaf(ctx)
			if leaf == nil {
				return nil, cfg.refuse(ctx, identity.ReasonNoIdentity, "", remote, ErrNoIdentity)
			}
			p, err := tlsconf.PeerFromChain(leaf)
			if err != nil {
				return nil, cfg.refuse(ctx, identity.ReasonUntrusted, "", remote, ErrUntrusted)
			}
			if p.ID.TrustDomain() != cfg.TrustDomain {
				return nil, cfg.refuse(ctx, identity.ReasonUntrusted, p.ID.String(), remote, ErrUntrusted)
			}
			if cfg.Revocation != nil {
				revoked, rerr := cfg.Revocation.IsRevoked(ctx, p.ID, p.Serial)
				if rerr != nil || revoked {
					return nil, cfg.refuse(ctx, identity.ReasonRevoked, p.ID.String(), remote, ErrRevoked)
				}
			}
			pid := PeerIdentity{ID: p.ID, ServiceName: p.ServiceName, Serial: p.Serial, VerifiedAt: cfg.Now()}
			ctx = WithPeer(ctx, pid)
			observe.AnnotatePeer(ctx, p.ServiceName, p.ID.String())
			observe.RecordPeerContext(ctx)
			return next(ctx, req)
		}
	}
}

func (cfg Config) refuse(ctx context.Context, reason identity.Reason, claimed, remote string, err *kerrors.Error) error {
	if cfg.Audit != nil {
		_ = cfg.Audit.Emit(ctx, audit.Event{
			Type: audit.TypeAuthnRefused, Outcome: audit.OutcomeRefused, Reason: audit.Reason(reason),
			LocalID: cfg.LocalID, ClaimedPeerID: claimed, RemoteAddr: remote,
			CorrelationID: correlation(ctx), TraceID: observe.TraceID(ctx),
		})
	}
	return err
}

func correlation(ctx context.Context) string {
	if id := observe.CorrelationID(ctx); id != "" {
		return id
	}
	return observe.NewCorrelationID()
}

// peerLeaf extracts the verified leaf certificate and remote address from a
// gRPC or HTTP server context.
func peerLeaf(ctx context.Context) (*x509.Certificate, string) {
	if p, ok := peer.FromContext(ctx); ok {
		remote := ""
		if p.Addr != nil {
			remote = p.Addr.String()
		}
		if ti, ok := p.AuthInfo.(credentials.TLSInfo); ok && len(ti.State.PeerCertificates) > 0 {
			return ti.State.PeerCertificates[0], remote
		}
		if !ok || p.AuthInfo == nil {
			// Fall through to HTTP: a gRPC peer without TLS never carries an identity,
			// but an HTTP request in the same context may.
		} else {
			return nil, remote
		}
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		if ht, ok := tr.(khttp.Transporter); ok {
			r := ht.Request()
			if r == nil {
				return nil, ""
			}
			if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
				return r.TLS.PeerCertificates[0], r.RemoteAddr
			}
			return nil, r.RemoteAddr
		}
	}
	return nil, ""
}
