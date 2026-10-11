package grpc

import (
	"context"

	"github.com/go-kratos/kratos/v3/middleware"
	"google.golang.org/grpc"
)

// streamSecurity runs the security chain (recovery, local identity guard,
// correlation, tracing, instrumentation, authn, authz) once per streaming
// call, before the handler, and hands the handler a stream whose context
// carries what the chain established (the verified peer, the correlation
// id). A refusal ends the call before the handler runs. Application
// middleware is unary only: it is written against request messages, which a
// stream does not have when it opens.
func streamSecurity(chain []middleware.Middleware) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		var herr error
		run := middleware.Chain(chain...)(func(ctx context.Context, _ any) (any, error) {
			herr = handler(srv, &securedStream{ServerStream: ss, ctx: ctx})
			return nil, herr
		})
		if _, err := run(ss.Context(), nil); err != nil {
			return err
		}
		return herr
	}
}

// securedStream is a server stream whose context is the one the security
// chain produced.
type securedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *securedStream) Context() context.Context { return s.ctx }
