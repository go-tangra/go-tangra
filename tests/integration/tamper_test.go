package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/authn"
	inventoryv1 "github.com/go-tangra/go-tangra/v4/examples/two-services/api/inventory/v1"
)

type countingInventory struct {
	inventoryv1.UnimplementedInventoryServer
	calls atomic.Int32
}

func (s *countingInventory) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	s.calls.Add(1)
	p, _ := authn.FromContext(ctx)
	return &inventoryv1.ReserveResponse{ReservationId: "r1", ReservedBy: p.ServiceName}, nil
}

// twoServices starts inventory (allow-all) and returns an orders app whose
// discovery for "inventory" points at target.
func twoServices(t *testing.T, f *fixture, target func(invAddr string) string) (*countingInventory, *freya.App, func()) {
	t.Helper()
	svc := &countingInventory{}
	invCfg := f.config("inventory")
	inv, err := freya.New(invCfg, freya.WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	inventoryv1.RegisterInventoryServer(inv.GRPC(), svc)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = inv.Run(ctx) }()
	ep, _ := inv.GRPC().Endpoint()
	time.Sleep(100 * time.Millisecond)

	ordCfg := f.config("orders")
	ordCfg.Discovery.Static = map[string][]string{"inventory": {target(ep.Host)}}
	ord, err := freya.New(ordCfg, freya.WithAllowAllPolicy())
	if err != nil {
		t.Fatal(err)
	}
	return svc, ord, func() { ord.Close(); cancel(); inv.Close() }
}

func TestTamperProxy(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	var px *proxy
	svc, ord, stop := twoServices(t, f, func(inv string) string {
		px = newProxy(t, inv, true)
		return px.addr()
	})
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	_, err = inventoryv1.NewInventoryClient(conn).Reserve(ctx, &inventoryv1.ReserveRequest{Sku: "x", Quantity: 1})
	if err == nil {
		t.Fatal("tampered channel must fail the call")
	}
	if !px.flipped {
		t.Fatal("proxy never tampered")
	}
	if svc.calls.Load() != 0 {
		t.Fatal("handler ran despite tampering")
	}
}

func TestNoPlaintextOnWire(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	var px *proxy
	svc, ord, stop := twoServices(t, f, func(inv string) string {
		px = newProxy(t, inv, false)
		return px.addr()
	})
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	const marker = "PLAINTEXT-MARKER-7f3a9c"
	for i := 0; i < 20; i++ {
		resp, err := inventoryv1.NewInventoryClient(conn).Reserve(ctx, &inventoryv1.ReserveRequest{Sku: marker, Quantity: int32(i)})
		if err != nil || resp.GetReservedBy() != "orders" {
			t.Fatalf("call %d: %v %v", i, resp, err)
		}
	}
	if svc.calls.Load() != 20 {
		t.Fatalf("calls = %d", svc.calls.Load())
	}
	c2s, s2c := px.captured()
	if len(c2s) == 0 || len(s2c) == 0 {
		t.Fatal("proxy captured nothing")
	}
	for _, needle := range []string{marker, "inventory.v1", "Reserve", "reserved_by", "x-request-id"} {
		if contains(c2s, needle) || contains(s2c, needle) {
			t.Fatalf("plaintext %q visible on the wire", needle)
		}
	}
}

func contains(hay []byte, needle string) bool {
	return len(needle) > 0 && string(hay) != "" && indexOf(hay, []byte(needle)) >= 0
}

func indexOf(hay, needle []byte) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}
