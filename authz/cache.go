package authz

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"

	"github.com/go-freya/freya/identity"
)

const defaultCacheSize = 10000

type cacheKey struct{ peer, callee, op string }

// Cached wraps a Policy with a bounded LRU decision cache. Swap replaces the
// policy and invalidates every entry.
type Cached struct {
	mu     sync.Mutex
	policy *Policy
	cap    int
	ll     *list.List
	items  map[cacheKey]*list.Element
	hits   atomic.Uint64
	misses atomic.Uint64
}

type entry struct {
	key cacheKey
	d   Decision
}

// NewCached creates a cache of at most size entries (0 = 10000).
func NewCached(p *Policy, size int) *Cached {
	if size <= 0 {
		size = defaultCacheSize
	}
	return &Cached{policy: p, cap: size, ll: list.New(), items: map[cacheKey]*list.Element{}}
}

// Authorize implements Authorizer.
func (c *Cached) Authorize(ctx context.Context, peer identity.SPIFFEID, callee, op string) Decision {
	k := cacheKey{peer.String(), callee, op}
	c.mu.Lock()
	if el, ok := c.items[k]; ok {
		c.ll.MoveToFront(el)
		d := el.Value.(*entry).d
		c.mu.Unlock()
		c.hits.Add(1)
		return d
	}
	p := c.policy
	c.mu.Unlock()
	c.misses.Add(1)
	d := p.Authorize(ctx, peer, callee, op)
	c.mu.Lock()
	if c.policy == p { // don't cache a decision from a policy swapped meanwhile
		if _, ok := c.items[k]; !ok {
			el := c.ll.PushFront(&entry{key: k, d: d})
			c.items[k] = el
			for c.ll.Len() > c.cap {
				last := c.ll.Back()
				c.ll.Remove(last)
				delete(c.items, last.Value.(*entry).key)
			}
		}
	}
	c.mu.Unlock()
	return d
}

// Swap installs p and clears the cache.
func (c *Cached) Swap(p *Policy) {
	c.mu.Lock()
	c.policy = p
	c.ll.Init()
	c.items = map[cacheKey]*list.Element{}
	c.mu.Unlock()
}

// Version returns the active policy version.
func (c *Cached) Version() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy == nil {
		return ""
	}
	return c.policy.Version
}

// Policy returns the active policy.
func (c *Cached) Policy() *Policy {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.policy
}

// Len returns the number of cached decisions.
func (c *Cached) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// Stats returns hit and miss counters.
func (c *Cached) Stats() (hits, misses uint64) { return c.hits.Load(), c.misses.Load() }
