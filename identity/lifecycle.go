package identity

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"
)

// State of the local identity, and the kinds of lifecycle events.
type State string

// States and event kinds.
const (
	StatePending       State = "pending"
	StateValid         State = "valid"
	StateRenewing      State = "renewing"
	StateRenewFailed   State = "renew_failed" // event only; state stays as it was
	StateExpired       State = "expired"
	StateRevoked       State = "revoked"
	StateBundleUpdated State = "bundle_updated" // event only
)

// Renewer is implemented by providers that renew on request (pull model).
// Push-model providers (Workload API) simply deliver updates via Watch.
type Renewer interface {
	Renew(ctx context.Context) error
}

// LifecycleEvent is delivered to LifecycleConfig.OnEvent.
type LifecycleEvent struct {
	State    State
	Identity Identity
	Bundle   Bundle
	Err      error
}

// LifecycleConfig tunes renewal scheduling.
type LifecycleConfig struct {
	// RenewAt is the fraction of the validity window at which renewal starts (0.3..0.8, default 0.5).
	RenewAt float64
	// Jitter is the relative random spread applied to the renewal time (default 0.1).
	Jitter float64
	// MinBackoff/MaxBackoff bound retries after a failed renewal (defaults 1s/1m).
	MinBackoff, MaxBackoff time.Duration
	Now                    func() time.Time
	OnEvent                func(LifecycleEvent)
}

// WithDefaults fills zero fields.
func (c LifecycleConfig) WithDefaults() LifecycleConfig {
	if c.RenewAt < 0.3 || c.RenewAt > 0.8 {
		c.RenewAt = 0.5
	}
	if c.Jitter == 0 {
		c.Jitter = 0.1
	}
	if c.MinBackoff <= 0 {
		c.MinBackoff = time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = time.Minute
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.OnEvent == nil {
		c.OnEvent = func(LifecycleEvent) {}
	}
	return c
}

// RenewTime returns the instant at which renewal should start.
func RenewTime(notBefore, notAfter time.Time, at, jitter float64) time.Time {
	window := notAfter.Sub(notBefore)
	point := time.Duration(float64(window) * at)
	if jitter > 0 {
		spread := float64(point) * jitter
		point += time.Duration((randomUnit()*2 - 1) * spread)
	}
	return notBefore.Add(point)
}

// randomUnit returns a uniformly distributed float in [0,1) from crypto/rand.
func randomUnit() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0.5
	}
	return float64(binary.BigEndian.Uint64(b[:])>>11) / (1 << 53)
}

// Lifecycle drives the identity state machine for one provider: it schedules
// renewal, retries with backoff, detects expiry, and reports every transition.
type Lifecycle struct {
	p      Provider
	cfg    LifecycleConfig
	mu     sync.RWMutex
	state  State
	serial string
	bver   uint64
}

// NewLifecycle creates a lifecycle for p; call Run to start it.
func NewLifecycle(p Provider, cfg LifecycleConfig) *Lifecycle {
	return &Lifecycle{p: p, cfg: cfg.WithDefaults(), state: StatePending}
}

// State returns the current state.
func (l *Lifecycle) State() State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

func (l *Lifecycle) set(s State) {
	l.mu.Lock()
	l.state = s
	l.mu.Unlock()
}

// Start subscribes to the provider, adopts the current identity synchronously
// (so State is meaningful when Start returns), and runs the state machine in
// the background until ctx is done.
func (l *Lifecycle) Start(ctx context.Context) error {
	ch, err := l.p.Watch(ctx)
	if err != nil {
		return err
	}
	rt := l.newRuntime()
	rt.initial(ctx)
	go rt.loop(ctx, ch)
	return nil
}

// Run is Start followed by blocking until ctx is done or the watch closes.
func (l *Lifecycle) Run(ctx context.Context) error {
	ch, err := l.p.Watch(ctx)
	if err != nil {
		return err
	}
	rt := l.newRuntime()
	rt.initial(ctx)
	rt.loop(ctx, ch)
	return nil
}

// lcRuntime holds the timers and backoff of one running lifecycle.
type lcRuntime struct {
	l         *Lifecycle
	renew     *time.Timer
	expire    *time.Timer
	backoff   time.Duration
	isRenewer bool
}

func (l *Lifecycle) newRuntime() *lcRuntime {
	rt := &lcRuntime{l: l, renew: time.NewTimer(time.Hour), expire: time.NewTimer(time.Hour), backoff: l.cfg.MinBackoff}
	rt.renew.Stop()
	rt.expire.Stop()
	_, rt.isRenewer = l.p.(Renewer)
	return rt
}

func (rt *lcRuntime) arm(id Identity) {
	l := rt.l
	now := l.cfg.Now()
	if !now.Before(id.NotAfter()) {
		l.set(StateExpired)
		l.cfg.OnEvent(LifecycleEvent{State: StateExpired, Identity: id})
		return
	}
	remaining := id.NotAfter().Sub(now)
	rt.expire.Reset(remaining)
	if rt.isRenewer {
		d := RenewTime(id.NotBefore(), id.NotAfter(), l.cfg.RenewAt, l.cfg.Jitter).Sub(now)
		// Certificates have one-second granularity; never spin on a renewal
		// point that already passed for a freshly issued identity.
		if d < remaining/4 {
			d = remaining / 4
		}
		rt.renew.Reset(d)
	}
}

func (rt *lcRuntime) adopt(id Identity, b Bundle) {
	l := rt.l
	l.mu.Lock()
	first := l.serial == ""
	newSerial := id.Serial() != l.serial
	newBundle := !first && b != nil && b.Version() != l.bver
	l.serial = id.Serial()
	if b != nil {
		l.bver = b.Version()
	}
	l.mu.Unlock()
	if newBundle {
		l.cfg.OnEvent(LifecycleEvent{State: StateBundleUpdated, Identity: id, Bundle: b})
	}
	if newSerial || l.State() != StateValid {
		rt.backoff = l.cfg.MinBackoff
		l.set(StateValid)
		l.cfg.OnEvent(LifecycleEvent{State: StateValid, Identity: id, Bundle: b})
		rt.arm(id)
	}
}

func (rt *lcRuntime) initial(ctx context.Context) {
	id, b, err := rt.l.p.Current(ctx)
	if err != nil {
		rt.l.cfg.OnEvent(LifecycleEvent{State: StateRenewFailed, Err: err})
		return
	}
	rt.adopt(id, b)
}

func (rt *lcRuntime) loop(ctx context.Context, ch <-chan Update) {
	l := rt.l
	defer rt.renew.Stop()
	defer rt.expire.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case u, ok := <-ch:
			if !ok {
				return
			}
			if u.Err != nil {
				l.cfg.OnEvent(LifecycleEvent{State: StateRenewFailed, Identity: u.Identity, Bundle: u.Bundle, Err: u.Err})
				continue
			}
			if u.Identity != nil {
				rt.adopt(u.Identity, u.Bundle)
			}
		case <-rt.renew.C:
			r := l.p.(Renewer)
			l.set(StateRenewing)
			l.cfg.OnEvent(LifecycleEvent{State: StateRenewing})
			if err := r.Renew(ctx); err != nil {
				l.cfg.OnEvent(LifecycleEvent{State: StateRenewFailed, Err: err})
				rt.renew.Reset(rt.backoff)
				rt.backoff *= 2
				if rt.backoff > l.cfg.MaxBackoff {
					rt.backoff = l.cfg.MaxBackoff
				}
				continue
			}
			// The provider reports the new identity through Watch; if it does not,
			// fall back to reading it directly.
			if id, b, err := l.p.Current(ctx); err == nil {
				rt.adopt(id, b)
			}
		case <-rt.expire.C:
			if id, _, err := l.p.Current(ctx); err == nil && l.cfg.Now().Before(id.NotAfter()) {
				// A newer identity arrived between the timer and now.
				continue
			}
			l.set(StateExpired)
			l.cfg.OnEvent(LifecycleEvent{State: StateExpired})
		}
	}
}
