package integration

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/config"
	inventoryv1 "github.com/go-freya/freya/examples/two-services/api/inventory/v1"
	"github.com/go-freya/freya/identity"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/observe"
	"github.com/go-freya/freya/transport/tlsconf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type slowInventory struct {
	inventoryv1.UnimplementedInventoryServer
	hold chan struct{}
}

func (s *slowInventory) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if s.hold != nil {
		select {
		case <-s.hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &inventoryv1.ReserveResponse{ReservationId: "r", ReservedBy: "x"}, nil
}

func TestLimits(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	logs := &testutil.LogCapture{}
	cfg := f.config("inventory")
	cfg.Server.HTTPAddr = "127.0.0.1:0"
	cfg.Limits.MaxConcurrentStreams = 4
	cfg.Limits.RequestTimeout = 2 * time.Second
	cfg.Limits.HandshakeTimeout = time.Second
	inv, err := freya.New(cfg, freya.WithAllowAllPolicy(), freya.WithLogger(logs.Handler()))
	if err != nil {
		t.Fatal(err)
	}
	svc := &slowInventory{hold: make(chan struct{})}
	inventoryv1.RegisterInventoryServer(inv.GRPC(), svc)
	inv.HTTP().HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = w.Write(b)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = inv.Run(ctx) }()
	defer inv.Close()
	grpcEp, _ := inv.GRPC().Endpoint()
	httpEp, _ := inv.HTTP().Endpoint()
	time.Sleep(150 * time.Millisecond)

	clientTLS := func() *tls.Config {
		p := testutil.NewMemProvider(f.ca, f.ca.MustIssue("orders", testutil.IssueOptions{}))
		c, _ := tlsconf.ClientConfig(p, identity.ForService("example.org", "inventory"), tlsconf.Options{TrustDomain: "example.org"})
		return c
	}
	conn, err := grpc.NewClient(grpcEp.Host, grpc.WithTransportCredentials(credentials.NewTLS(clientTLS())))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := inventoryv1.NewInventoryClient(conn)

	limitEvents := func() int { return logs.Count("type", "limit_exceeded") }
	waitEvents := func(t *testing.T, want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) && limitEvents() < want {
			time.Sleep(20 * time.Millisecond)
		}
		if got := limitEvents(); got != want {
			t.Fatalf("limit_exceeded events = %d, want %d\n%s", got, want, logs.String())
		}
	}
	t.Run("2MiB body refused", func(t *testing.T) {
		before := limitEvents()
		big := &inventoryv1.ReserveRequest{Sku: strings.Repeat("x", 2<<20)}
		_, err := client.Reserve(ctx, big, grpc.MaxCallSendMsgSize(4<<20))
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("want ResourceExhausted, got %v", err)
		}
		waitEvents(t, before+1)
		if logs.Count("attr_limit", "message_size") == 0 {
			t.Fatal("expected attr limit=message_size")
		}
	})
	t.Run("oversized correlation id replaced", func(t *testing.T) {
		md := metadata.Pairs(observe.HeaderCorrelationID, strings.Repeat("a", 200))
		var hdr metadata.MD
		go func() { time.Sleep(50 * time.Millisecond); svc.hold <- struct{}{} }()
		_, err := client.Reserve(metadata.NewOutgoingContext(ctx, md), &inventoryv1.ReserveRequest{Sku: "s"}, grpc.Header(&hdr))
		if err != nil {
			t.Fatal(err)
		}
		got := hdr.Get(observe.HeaderCorrelationID)
		if len(got) != 1 || len(got[0]) > 128 || got[0] == strings.Repeat("a", 200) {
			t.Fatalf("reply x-request-id %v", got)
		}
	})
	t.Run("9KiB header refused", func(t *testing.T) {
		md := metadata.Pairs("x-big", strings.Repeat("h", 9<<10))
		_, err := client.Reserve(metadata.NewOutgoingContext(ctx, md), &inventoryv1.ReserveRequest{Sku: "s"})
		if err == nil {
			t.Fatal("oversized header must be refused")
		}
	})
	t.Run("request timeout", func(t *testing.T) {
		before := limitEvents()
		start := time.Now()
		_, err := client.Reserve(ctx, &inventoryv1.ReserveRequest{Sku: "slow"})
		if status.Code(err) != codes.DeadlineExceeded || time.Since(start) > 4*time.Second {
			t.Fatalf("want DeadlineExceeded within the limit, got %v after %s", err, time.Since(start))
		}
		waitEvents(t, before+1)
	})
	t.Run("concurrent streams capped", func(t *testing.T) {
		// Hold 4 streams open; the 5th must wait (HTTP/2 flow control), so with a
		// short deadline it times out client-side rather than being served.
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = client.Reserve(ctx, &inventoryv1.ReserveRequest{Sku: "hold"})
			}()
		}
		time.Sleep(100 * time.Millisecond)
		cctx, ccancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer ccancel()
		_, err := client.Reserve(cctx, &inventoryv1.ReserveRequest{Sku: "fifth"})
		if status.Code(err) != codes.DeadlineExceeded {
			t.Fatalf("5th stream should queue behind the cap, got %v", err)
		}
		for i := 0; i < 4; i++ {
			svc.hold <- struct{}{}
		}
		wg.Wait()
	})
	t.Run("http body limit and 413", func(t *testing.T) {
		before := limitEvents()
		hc := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS(), ForceAttemptHTTP2: true}, Timeout: 5 * time.Second}
		resp, err := hc.Post("https://"+httpEp.Host+"/echo", "text/plain", strings.NewReader(strings.Repeat("b", 2<<20)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status %d", resp.StatusCode)
		}
		waitEvents(t, before+1)
		if logs.Count("attr_limit", "request_body") == 0 {
			t.Fatal("expected attr limit=request_body")
		}
	})
	t.Run("slow-loris handshake timeout", func(t *testing.T) {
		// A client that connects and never completes the TLS handshake is dropped
		// after limits.handshake_timeout on both listeners.
		before := limitEvents()
		for _, addr := range []string{httpEp.Host, grpcEp.Host} {
			raw, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			_ = raw.SetReadDeadline(time.Now().Add(cfg.Limits.HandshakeTimeout + 5*time.Second))
			start := time.Now()
			_, rerr := raw.Read(make([]byte, 1))
			raw.Close()
			if rerr == nil || time.Since(start) > cfg.Limits.HandshakeTimeout+4*time.Second {
				t.Fatalf("%s: server did not drop the idle client in time: %v after %s", addr, rerr, time.Since(start))
			}
		}
		waitEvents(t, before+2)
		if logs.Count("attr_limit", "handshake_timeout") < 2 {
			t.Fatalf("expected two handshake_timeout events: %s", logs.String())
		}
	})
	_ = config.Default
}
