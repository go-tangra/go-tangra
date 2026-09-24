// Package timescale is an audit.Sink that writes events to a TimescaleDB
// hypertable in asynchronous batches. The call path never blocks on the
// database: a full queue drops (and counts) events, which are still written to
// the primary slog sink by the framework.
package timescale

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-tangra/go-tangra/v4/audit"
)

// Inserter writes a batch of rows in column order of the audit_events table.
type Inserter interface {
	InsertBatch(ctx context.Context, rows [][]any) error
}

// Options tune batching.
type Options struct {
	QueueSize     int           // default 10000
	BatchSize     int           // default 500
	FlushInterval time.Duration // default 1s
	// OnError is called with the batch size when a write fails (rows are dropped).
	OnError func(err error, rows int)
}

func (o Options) withDefaults() Options {
	if o.QueueSize <= 0 {
		o.QueueSize = 10000
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 500
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = time.Second
	}
	if o.OnError == nil {
		o.OnError = func(error, int) {}
	}
	return o
}

// Batcher buffers events and writes them through an Inserter.
type Batcher struct {
	ins     Inserter
	opts    Options
	ch      chan audit.Event
	dropped atomic.Uint64
	written atomic.Uint64
	wg      sync.WaitGroup
	mu      sync.Mutex
	closed  bool
}

// NewBatcher starts the background writer.
func NewBatcher(ins Inserter, opts Options) *Batcher {
	b := &Batcher{ins: ins, opts: opts.withDefaults()}
	b.ch = make(chan audit.Event, b.opts.QueueSize)
	b.wg.Add(1)
	go b.run()
	return b
}

// Emit implements audit.Sink; it never blocks.
func (b *Batcher) Emit(_ context.Context, e audit.Event) {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		b.dropped.Add(1)
		return
	}
	select {
	case b.ch <- e:
	default:
		b.dropped.Add(1)
	}
}

// Dropped returns the number of events dropped because the queue was full.
func (b *Batcher) Dropped() uint64 { return b.dropped.Load() }

// Written returns the number of rows successfully written.
func (b *Batcher) Written() uint64 { return b.written.Load() }

func (b *Batcher) run() {
	defer b.wg.Done()
	t := time.NewTicker(b.opts.FlushInterval)
	defer t.Stop()
	buf := make([][]any, 0, b.opts.BatchSize)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := b.ins.InsertBatch(ctx, buf)
		cancel()
		if err != nil {
			b.opts.OnError(err, len(buf))
			b.dropped.Add(uint64(len(buf)))
		} else {
			b.written.Add(uint64(len(buf)))
		}
		buf = buf[:0]
	}
	for {
		select {
		case e, ok := <-b.ch:
			if !ok {
				flush()
				return
			}
			buf = append(buf, Row(e))
			if len(buf) >= b.opts.BatchSize {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}

// Close drains the queue and stops the writer.
func (b *Batcher) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	close(b.ch)
	b.mu.Unlock()
	b.wg.Wait()
}

// Columns is the column order used by Row and InsertBatch.
var Columns = []string{
	"ts", "event_type", "outcome", "reason", "local_id", "claimed_peer_id", "verified_peer_id",
	"operation", "correlation_id", "trace_id", "rule_id", "policy_version", "remote_addr", "attrs",
}

// Row converts an event to a row in Columns order.
func Row(e audit.Event) []any {
	attrs := e.Attrs
	if attrs == nil {
		attrs = map[string]string{}
	}
	return []any{
		e.Time.UTC(), string(e.Type), string(e.Outcome), string(e.Reason), e.LocalID,
		nullable(e.ClaimedPeerID), nullable(e.VerifiedPeerID), nullable(e.Operation), e.CorrelationID,
		nullable(e.TraceID), nullable(e.RuleID), nullable(e.PolicyVersion), nullable(e.RemoteAddr), attrs,
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
