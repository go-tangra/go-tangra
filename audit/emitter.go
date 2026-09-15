package audit

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Sink receives validated events. Emit MUST never block the caller for long;
// slow sinks must buffer internally.
type Sink interface {
	Emit(ctx context.Context, e Event)
}

// SlogSink writes events as one structured log line each with msg "audit".
type SlogSink struct{ Logger *slog.Logger }

// NewSlogSink returns a sink writing through the given logger (which should wrap
// a redacting handler).
func NewSlogSink(l *slog.Logger) *SlogSink { return &SlogSink{Logger: l} }

// Emit implements Sink.
func (s *SlogSink) Emit(ctx context.Context, e Event) {
	attrs := []slog.Attr{
		slog.String("audit_time", e.Time.UTC().Format(timeLayout)),
		slog.String("type", string(e.Type)),
		slog.String("outcome", string(e.Outcome)),
		slog.String("reason", string(e.Reason)),
		slog.String("local_id", e.LocalID),
		slog.String("correlation_id", e.CorrelationID),
	}
	opt := func(k, v string) {
		if v != "" {
			attrs = append(attrs, slog.String(k, v))
		}
	}
	opt("claimed_peer_id", e.ClaimedPeerID)
	opt("verified_peer_id", e.VerifiedPeerID)
	opt("operation", e.Operation)
	opt("trace_id", e.TraceID)
	opt("rule_id", e.RuleID)
	opt("policy_version", e.PolicyVersion)
	opt("remote_addr", e.RemoteAddr)
	for k, v := range e.Attrs {
		attrs = append(attrs, slog.String("attr_"+k, v))
	}
	s.Logger.LogAttrs(ctx, slog.LevelInfo, "audit", attrs...)
}

// Emitter validates events and fans them out to sinks through a bounded queue.
// The call path never blocks: when the queue is full the event is dropped and
// counted (and always still written to the primary slog sink synchronously).
type Emitter struct {
	primary Sink
	mu      sync.RWMutex
	sinks   []Sink
	ch      chan Event
	dropped atomic.Uint64
	invalid atomic.Uint64
	wg      sync.WaitGroup
	closeMu sync.RWMutex
	closed  bool
	now     func() time.Time
}

// NewEmitter creates an emitter. primary (normally the slog sink) is invoked
// synchronously so audit is never lost to queue pressure; extra sinks are fed
// asynchronously through a queue of the given size (0 = synchronous, for tests).
func NewEmitter(primary Sink, queue int, extra ...Sink) *Emitter {
	e := &Emitter{primary: primary, sinks: extra, now: time.Now}
	if queue > 0 {
		e.ch = make(chan Event, queue)
		e.wg.Add(1)
		go e.run()
	}
	return e
}

// AddSink registers an additional asynchronous sink.
func (e *Emitter) AddSink(s Sink) {
	e.mu.Lock()
	e.sinks = append(e.sinks, s)
	e.mu.Unlock()
}

// Emit validates and dispatches ev. A zero Time is filled in.
func (e *Emitter) Emit(ctx context.Context, ev Event) error {
	if ev.Time.IsZero() {
		ev.Time = e.now()
	}
	if err := ev.Validate(); err != nil {
		e.invalid.Add(1)
		return err
	}
	if e.primary != nil {
		e.primary.Emit(ctx, ev)
	}
	if e.ch == nil {
		e.fanout(ctx, ev)
		return nil
	}
	e.closeMu.RLock()
	defer e.closeMu.RUnlock()
	if e.closed {
		e.dropped.Add(1)
		return nil
	}
	select {
	case e.ch <- ev:
	default:
		e.dropped.Add(1)
	}
	return nil
}

func (e *Emitter) fanout(ctx context.Context, ev Event) {
	e.mu.RLock()
	sinks := e.sinks
	e.mu.RUnlock()
	for _, s := range sinks {
		s.Emit(ctx, ev)
	}
}

func (e *Emitter) run() {
	defer e.wg.Done()
	for ev := range e.ch {
		e.fanout(context.Background(), ev)
	}
}

// Dropped returns how many events were dropped because the queue was full.
func (e *Emitter) Dropped() uint64 { return e.dropped.Load() }

// Invalid returns how many events failed validation (a programming error).
func (e *Emitter) Invalid() uint64 { return e.invalid.Load() }

// Close drains the queue and waits for the worker.
func (e *Emitter) Close() {
	if e.ch == nil {
		return
	}
	e.closeMu.Lock()
	if e.closed {
		e.closeMu.Unlock()
		return
	}
	e.closed = true
	close(e.ch)
	e.closeMu.Unlock()
	e.wg.Wait()
}
