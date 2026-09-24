package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/registry"
	"github.com/go-tangra/go-tangra/v4"
	inventoryv1 "github.com/go-tangra/go-tangra/v4/examples/two-services/api/inventory/v1"
)

// fakeRegistry is a mutable registry.Discovery: SetEndpoint moves a service to
// a new address and wakes every watcher, like a real registry would.
type fakeRegistry struct {
	mu       sync.Mutex
	addr     map[string]string
	watchers []chan struct{}
}

func (r *fakeRegistry) instances(name string) []*registry.ServiceInstance {
	if a, ok := r.addr[name]; ok {
		return []*registry.ServiceInstance{{ID: name + "-" + a, Name: name, Endpoints: []string{"grpcs://" + a}}}
	}
	return nil
}

func (r *fakeRegistry) GetService(_ context.Context, name string) ([]*registry.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.instances(name), nil
}

func (r *fakeRegistry) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.watchers = append(r.watchers, ch)
	r.mu.Unlock()
	return &fakeWatcher{r: r, name: name, ch: ch, ctx: ctx, first: true}, nil
}

func (r *fakeRegistry) SetEndpoint(name, addr string) {
	r.mu.Lock()
	r.addr[name] = addr
	ws := append([]chan struct{}(nil), r.watchers...)
	r.mu.Unlock()
	for _, w := range ws {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

type fakeWatcher struct {
	r     *fakeRegistry
	name  string
	ch    chan struct{}
	ctx   context.Context
	first bool
}

func (w *fakeWatcher) Next() ([]*registry.ServiceInstance, error) {
	if !w.first {
		select {
		case <-w.ch:
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		}
	}
	w.first = false
	w.r.mu.Lock()
	defer w.r.mu.Unlock()
	return w.r.instances(w.name), nil
}

func (w *fakeWatcher) Stop() error { return nil }

func TestCalleeAddressChange(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	startInventory := func() (string, func()) {
		app, err := freya.New(f.config("inventory"), freya.WithAllowAllPolicy())
		if err != nil {
			t.Fatal(err)
		}
		inventoryv1.RegisterInventoryServer(app.GRPC(), echoingInventory{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() { _ = app.Run(ctx) }()
		ep, _ := app.GRPC().Endpoint()
		time.Sleep(100 * time.Millisecond)
		return ep.Host, func() { cancel(); app.Close() }
	}
	addr1, stop1 := startInventory()
	reg := &fakeRegistry{addr: map[string]string{"inventory": addr1}}
	ord, err := freya.New(f.config("orders"), freya.WithAllowAllPolicy(), freya.WithDiscovery(reg))
	if err != nil {
		t.Fatal(err)
	}
	defer ord.Close()
	ctx := context.Background()
	conn, err := ord.Client(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	call := func() error {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := inventoryv1.NewInventoryClient(conn).Reserve(cctx, &inventoryv1.ReserveRequest{Sku: "x"})
		return err
	}
	if err := call(); err != nil {
		t.Fatalf("first address: %v", err)
	}
	// The callee restarts elsewhere: same identity, new address.
	stop1()
	addr2, stop2 := startInventory()
	defer stop2()
	if addr2 == addr1 {
		t.Skip("port reused; cannot observe an address change")
	}
	reg.SetEndpoint("inventory", addr2)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := call(); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("calls did not recover after the callee moved from %s to %s", addr1, addr2)
}

type echoingInventory struct {
	inventoryv1.UnimplementedInventoryServer
}

func (echoingInventory) Reserve(_ context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	return &inventoryv1.ReserveResponse{ReservationId: req.GetSku(), ReservedBy: "inventory"}, nil
}
