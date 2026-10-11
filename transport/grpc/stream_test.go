package grpc

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/authz"
	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"github.com/go-tangra/go-tangra/v4/observe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// peerStream is a server-streaming test service that answers with the
// verified peer and the correlation id its stream context carries.
var peerStreamDesc = grpc.ServiceDesc{
	ServiceName: "test.v1.Peer",
	HandlerType: (*any)(nil),
	Streams: []grpc.StreamDesc{{StreamName: "Who", ServerStreams: true, Handler: func(_ any, ss grpc.ServerStream) error {
		if err := ss.RecvMsg(&emptypb.Empty{}); err != nil {
			return err
		}
		p, ok := authn.FromContext(ss.Context())
		if !ok {
			return status.Error(codes.Internal, "no peer in the stream context")
		}
		if err := ss.SendMsg(wrapperspb.String(p.ID.String())); err != nil {
			return err
		}
		return ss.SendMsg(wrapperspb.String(observe.CorrelationID(ss.Context())))
	}}},
}

func callWho(t *testing.T, conn *grpc.ClientConn) ([]string, error) {
	t.Helper()
	st, err := conn.NewStream(context.Background(), &peerStreamDesc.Streams[0], "/test.v1.Peer/Who")
	if err != nil {
		return nil, err
	}
	if err := st.SendMsg(&emptypb.Empty{}); err != nil {
		return nil, err
	}
	if err := st.CloseSend(); err != nil {
		return nil, err
	}
	var out []string
	for {
		m := &wrapperspb.StringValue{}
		if err := st.RecvMsg(m); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return out, err
		}
		out = append(out, m.GetValue())
	}
}

func TestStreamCallsRunTheSecurityChain(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	srv, err := NewServer(rt, WithAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	srv.RegisterService(&peerStreamDesc, nil)
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	got, err := callWho(t, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "spiffe://example.org/svc/orders" || !observe.ValidCorrelationID(got[1]) {
		t.Fatalf("stream context: %q", got)
	}
	if rt.Logs.Count("operation", "/test.v1.Peer/Who") == 0 {
		t.Fatalf("stream call not logged: %s", rt.Logs.String())
	}
}

func TestStreamCallsRefusedByPolicy(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	rt.Authz = authz.DenyAll{}
	srv, err := NewServer(rt, WithAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	desc := peerStreamDesc
	desc.Streams = []grpc.StreamDesc{{StreamName: "Who", ServerStreams: true, Handler: func(any, grpc.ServerStream) error {
		ran = true
		return nil
	}}}
	srv.RegisterService(&desc, nil)
	stop := testrt.StartServer(t, srv)
	defer stop()
	conn := dial(t, ca, "orders", "inventory", srv)
	defer conn.Close()
	_, err = callWho(t, conn)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("denied stream: %v", err)
	}
	if ran {
		t.Fatal("handler ran for a denied stream")
	}
	if rt.Logs.Count("msg", "audit") == 0 {
		t.Fatalf("refusal not audited: %s", rt.Logs.String())
	}
}
