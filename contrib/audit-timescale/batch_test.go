package timescale

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-freya/freya/audit"
)

type fakeInserter struct {
	mu      sync.Mutex
	batches [][][]any
	block   chan struct{}
	fail    bool
}

func (f *fakeInserter) InsertBatch(_ context.Context, rows [][]any) error {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("db down")
	}
	f.batches = append(f.batches, rows)
	return nil
}

func (f *fakeInserter) count() (batches, rows int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.batches {
		rows += len(b)
	}
	return len(f.batches), rows
}

func ev(i int) audit.Event {
	return audit.Event{Time: time.Now(), Type: audit.TypeAuthzRefused, Outcome: audit.OutcomeRefused, Reason: audit.ReasonNoMatchingRule,
		LocalID: "spiffe://example.org/svc/a", CorrelationID: "c", Operation: "op", Attrs: map[string]string{"i": string(rune('0' + i%10))}}
}

func TestBatcherFlushesBySizeAndInterval(t *testing.T) {
	ins := &fakeInserter{}
	b := NewBatcher(ins, Options{BatchSize: 3, FlushInterval: 50 * time.Millisecond, QueueSize: 100})
	for i := 0; i < 7; i++ {
		b.Emit(context.Background(), ev(i))
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, rows := ins.count(); rows == 7 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	batches, rows := ins.count()
	if rows != 7 || batches < 3 {
		t.Fatalf("batches=%d rows=%d", batches, rows)
	}
	if b.Written() != 7 || b.Dropped() != 0 {
		t.Fatalf("written=%d dropped=%d", b.Written(), b.Dropped())
	}
	b.Close()
	b.Close()
	b.Emit(context.Background(), ev(0))
	if b.Dropped() != 1 {
		t.Fatal("emit after close must count as dropped")
	}
}

func TestBatcherNeverBlocksAndCountsDrops(t *testing.T) {
	ins := &fakeInserter{block: make(chan struct{})}
	b := NewBatcher(ins, Options{BatchSize: 1, QueueSize: 2, FlushInterval: time.Hour})
	start := time.Now()
	for i := 0; i < 50; i++ {
		b.Emit(context.Background(), ev(i))
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("Emit blocked")
	}
	if b.Dropped() == 0 {
		t.Fatal("expected drops with a blocked writer")
	}
	close(ins.block)
	b.Close()
}

func TestBatcherWriteErrorsAreReported(t *testing.T) {
	ins := &fakeInserter{fail: true}
	var reported int
	b := NewBatcher(ins, Options{BatchSize: 2, FlushInterval: time.Hour, OnError: func(err error, rows int) { reported += rows }})
	b.Emit(context.Background(), ev(1))
	b.Emit(context.Background(), ev(2))
	b.Close()
	if reported != 2 || b.Dropped() != 2 {
		t.Fatalf("reported=%d dropped=%d", reported, b.Dropped())
	}
}

func TestRowShape(t *testing.T) {
	r := Row(ev(1))
	if len(r) != len(Columns) || r[1] != "authz_refused" || r[5] != nil || r[13] == nil {
		t.Fatalf("row %v", r)
	}
	e := ev(1)
	e.Attrs = nil
	if Row(e)[13] == nil {
		t.Fatal("attrs must never be NULL")
	}
}
