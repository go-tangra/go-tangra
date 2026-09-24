package authn

import (
	"context"
	"sync"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// MemoryRevocationChecker is an in-memory identity.RevocationChecker: the
// reference implementation, used by tests and by contrib sources as a local cache.
type MemoryRevocationChecker struct {
	mu   sync.RWMutex
	ids  map[string]struct{} // whole identity revoked
	sers map[string]struct{} // id + "#" + serial revoked
}

// NewMemoryRevocationChecker returns an empty checker.
func NewMemoryRevocationChecker() *MemoryRevocationChecker {
	return &MemoryRevocationChecker{ids: map[string]struct{}{}, sers: map[string]struct{}{}}
}

// Revoke marks id (all serials when serial is empty) as revoked.
func (m *MemoryRevocationChecker) Revoke(id identity.SPIFFEID, serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if serial == "" {
		m.ids[id.String()] = struct{}{}
		return
	}
	m.sers[id.String()+"#"+serial] = struct{}{}
}

// Unrevoke reverses Revoke.
func (m *MemoryRevocationChecker) Unrevoke(id identity.SPIFFEID, serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if serial == "" {
		delete(m.ids, id.String())
		return
	}
	delete(m.sers, id.String()+"#"+serial)
}

// IsRevoked implements identity.RevocationChecker.
func (m *MemoryRevocationChecker) IsRevoked(_ context.Context, id identity.SPIFFEID, serial string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.ids[id.String()]; ok {
		return true, nil
	}
	_, ok := m.sers[id.String()+"#"+serial]
	return ok, nil
}
