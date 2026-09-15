package grpc

import (
	"context"

	"github.com/go-freya/freya/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// limitStats observes every RPC's final status — including RPCs that gRPC
// rejects while decoding the request (message too large), which never reach
// the interceptor chain — and audits limit violations.
type limitStats struct{ rt transport.Runtime }

type rpcInfoKey struct{}

type rpcInfo struct {
	method string
	remote string
}

func (l limitStats) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	return context.WithValue(ctx, rpcInfoKey{}, &rpcInfo{method: info.FullMethodName})
}

func (l limitStats) HandleRPC(ctx context.Context, s stats.RPCStats) {
	ri, _ := ctx.Value(rpcInfoKey{}).(*rpcInfo)
	switch st := s.(type) {
	case *stats.InHeader:
		if ri != nil && st.RemoteAddr != nil {
			ri.remote = st.RemoteAddr.String()
		}
	case *stats.End:
		if st.Error == nil || ri == nil {
			return
		}
		switch status.Code(st.Error) {
		case codes.ResourceExhausted:
			transport.AuditLimitExceeded(l.rt, ctx, ri.method, ri.remote, "message_size")
		case codes.DeadlineExceeded:
			transport.AuditLimitExceeded(l.rt, ctx, ri.method, ri.remote, "request_timeout")
		}
	}
}

func (l limitStats) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context { return ctx }
func (l limitStats) HandleConn(context.Context, stats.ConnStats)                       {}
