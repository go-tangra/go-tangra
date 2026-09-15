package authn

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v3/transport"
)

func TestPeerLeafNilRequest(t *testing.T) {
	ctx := transport.NewServerContext(context.Background(), fakeHTTPTransport{r: nil})
	if leaf, remote := peerLeaf(ctx); leaf != nil || remote != "" {
		t.Fatal("nil request must yield no peer")
	}
}
