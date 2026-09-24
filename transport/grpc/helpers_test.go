package grpc

import (
	"testing"

	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func dial(t *testing.T, ca *testutil.CA, caller, callee string, srv *Server) *grpc.ClientConn {
	t.Helper()
	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(ep.Host, grpc.WithTransportCredentials(credentials.NewTLS(testrt.ClientTLS(t, ca, caller, callee))))
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func insecureCreds() credentials.TransportCredentials { return insecure.NewCredentials() }
