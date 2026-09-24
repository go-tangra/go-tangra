package grpc

import (
	"time"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/authz"
	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/observe"
	"github.com/go-tangra/go-tangra/v4/transport"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Server is a Kratos gRPC server that is always mutually authenticated. Register
// services on it exactly as on *grpc.Server.
type Server struct {
	*kgrpc.Server
	timeout    time.Duration
	limits     config.Limits
	reflection bool
}

// NewServer builds the server. Every call passes recover → local identity guard
// → correlation → tracing → instrumentation → authn → authz → (application
// middleware) → handler; nothing can remove the security
// stages. Reflection is disabled; the health service remains and is subject to
// the same policy as any other RPC.
func NewServer(rt transport.Runtime, opts ...ServerOption) (*Server, error) {
	lim := rt.Limits()
	o := serverOptions{network: "tcp", address: ":9443", timeout: lim.RequestTimeout}
	for _, f := range opts {
		f(&o)
	}
	if o.timeout <= 0 || o.timeout > lim.RequestTimeout {
		o.timeout = lim.RequestTimeout
	}
	tlsCfg, err := tlsconf.ServerConfig(rt.Provider(), transport.TLSOptions(rt))
	if err != nil {
		return nil, err
	}
	chain := append(SecurityChain(rt), o.middleware...)
	creds := auditedCreds{TransportCredentials: credentials.NewTLS(tlsCfg), rt: rt}
	ks := kgrpc.NewServer(
		kgrpc.Network(o.network),
		kgrpc.Address(o.address),
		kgrpc.Timeout(o.timeout),
		kgrpc.Middleware(chain...),
		kgrpc.DisableReflection(),
		// TLSConfig makes Kratos advertise grpcs:// (what discovery expects); the
		// audited credentials below are applied afterwards and take precedence.
		kgrpc.TLSConfig(tlsCfg),
		kgrpc.Options(
			grpc.Creds(creds),
			grpc.StatsHandler(limitStats{rt: rt}),
			grpc.MaxRecvMsgSize(int(lim.MaxRequestBytes)),
			grpc.MaxHeaderListSize(headerListSize(lim.MaxHeaderBytes)),
			grpc.MaxConcurrentStreams(lim.MaxConcurrentStreams),
			grpc.ConnectionTimeout(lim.HandshakeTimeout),
			grpc.KeepaliveParams(keepalive.ServerParameters{
				MaxConnectionIdle: lim.IdleTimeout, MaxConnectionAge: lim.MaxConnectionAge, MaxConnectionAgeGrace: lim.RequestTimeout,
			}),
		),
	)
	return &Server{Server: ks, timeout: o.timeout, limits: lim}, nil
}

// SecurityChain returns the fixed, ordered middleware every Freya server runs.
func SecurityChain(rt transport.Runtime) []middleware.Middleware {
	return []middleware.Middleware{
		recovery.Recovery(recovery.WithLogger(rt.Logger())),
		transport.LocalIdentityGuard(rt),
		observe.ServerCorrelation(observe.WithLogger(rt.Logger())),
		observe.ServerTracing(rt.Tracer()),
		observe.Instrument(observe.InstrumentConfig{Metrics: rt.Metrics(), ServiceName: rt.ServiceName(), Logger: rt.Logger(), Peer: transport.PeerFromContext}),
		authn.Middleware(authn.Config{
			TrustDomain: rt.TrustDomain(), LocalID: rt.LocalID(), Audit: rt.Audit(), Revocation: rt.Revocation(),
		}),
		authz.Middleware(authz.Config{
			Authorizer: rt.Authorizer(), LocalID: rt.LocalID(), ServiceName: rt.ServiceName(), Audit: rt.Audit(),
		}),
	}
}

// headerListSize converts the validated header limit to the gRPC option type,
// clamping to a sane maximum so the conversion can never overflow.
func headerListSize(n int) uint32 {
	const maxHeader = 1 << 20
	if n <= 0 {
		return 8 << 10
	}
	if n > maxHeader {
		return maxHeader
	}
	return uint32(n) // #nosec G115 -- clamped above
}
