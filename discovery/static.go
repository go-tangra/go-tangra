package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-freya/freya/identity"
	"github.com/go-kratos/kratos/v3/registry"
)

// Static resolves logical names from a fixed map. Endpoints are advertised with
// the grpcs:// scheme; the address is never trusted, only the peer's identity.
type Static struct {
	m map[string][]*registry.ServiceInstance
}

// NewStatic validates the name → host:port map.
func NewStatic(m map[string][]string) (*Static, error) {
	s := &Static{m: map[string][]*registry.ServiceInstance{}}
	for name, addrs := range m {
		if !identity.ValidServiceName(name) {
			return nil, fmt.Errorf("discovery: invalid service name %q", name)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("discovery: %s has no endpoints", name)
		}
		for i, a := range addrs {
			if strings.Contains(a, "://") {
				return nil, fmt.Errorf("discovery: %s endpoint %q must be host:port without scheme", name, a)
			}
			if _, _, err := net.SplitHostPort(a); err != nil {
				return nil, fmt.Errorf("discovery: %s endpoint %q: %w", name, a, err)
			}
			s.m[name] = append(s.m[name], &registry.ServiceInstance{
				ID: fmt.Sprintf("%s-%d", name, i), Name: name, Endpoints: []string{"grpcs://" + a},
			})
		}
	}
	return s, nil
}

// GetService implements registry.Discovery.
func (s *Static) GetService(_ context.Context, name string) ([]*registry.ServiceInstance, error) {
	inst, ok := s.m[name]
	if !ok {
		return nil, fmt.Errorf("discovery: unknown service %q", name)
	}
	return inst, nil
}

// Watch implements registry.Discovery: the first Next returns the instances,
// later calls block until the context is done or Stop is called.
func (s *Static) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	inst, err := s.GetService(ctx, name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &staticWatcher{inst: inst, ctx: ctx, cancel: cancel}, nil
}

type staticWatcher struct {
	inst   []*registry.ServiceInstance
	ctx    context.Context
	cancel context.CancelFunc
	sent   bool
}

func (w *staticWatcher) Next() ([]*registry.ServiceInstance, error) {
	if !w.sent {
		w.sent = true
		return w.inst, nil
	}
	<-w.ctx.Done()
	return nil, errors.New("discovery: watcher stopped")
}

func (w *staticWatcher) Stop() error { w.cancel(); return nil }
