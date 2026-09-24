package grpc

import (
	"context"
	"net"

	"github.com/go-tangra/go-tangra/v4/transport"
	"google.golang.org/grpc/credentials"
)

// auditedCreds wraps TLS credentials so that every failed server handshake is
// audited with its classified reason and remote address.
type auditedCreds struct {
	credentials.TransportCredentials
	rt transport.Runtime
}

func (c auditedCreds) ServerHandshake(raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := c.TransportCredentials.ServerHandshake(raw)
	if err != nil {
		transport.AuditHandshakeRefusal(c.rt, err, raw.RemoteAddr().String())
	}
	return conn, info, err
}

func (c auditedCreds) ClientHandshake(ctx context.Context, authority string, raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return c.TransportCredentials.ClientHandshake(ctx, authority, raw)
}

func (c auditedCreds) Clone() credentials.TransportCredentials {
	return auditedCreds{TransportCredentials: c.TransportCredentials.Clone(), rt: c.rt}
}
