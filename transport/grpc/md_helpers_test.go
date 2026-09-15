package grpc

import (
	"context"

	"github.com/go-freya/freya/observe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type metadataCarrier struct{ md *metadata.MD }

func (m *metadataCarrier) opt() grpc.CallOption {
	m.md = &metadata.MD{}
	return grpc.Header(m.md)
}

func (m *metadataCarrier) get(k string) string {
	if m.md == nil {
		return ""
	}
	if v := m.md.Get(k); len(v) > 0 {
		return v[0]
	}
	return ""
}

func withCID(ctx context.Context, id string) context.Context {
	return observe.WithCorrelationID(ctx, id)
}
