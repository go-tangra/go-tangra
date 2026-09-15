package audit

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type memSink struct {
	mu   sync.Mutex
	evs  []Event
	slow chan struct{}
}

func (m *memSink) Emit(_ context.Context, e Event) {
	if m.slow != nil {
		<-m.slow
	}
	m.mu.Lock()
	m.evs = append(m.evs, e)
	m.mu.Unlock()
}
func (m *memSink) n() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.evs) }

func TestEmitterSyncAndValidation(t *testing.T) {
	var buf bytes.Buffer
	prim := NewSlogSink(slog.New(NewRedactingHandler(slog.NewJSONHandler(&buf, nil))))
	extra := &memSink{}
	em := NewEmitter(prim, 0, extra)
	ev := sample()
	ev.Time = time.Time{}
	ev.Attrs = map[string]string{"api_key": "SECRET", "note": "fine"}
	if err := em.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if extra.n() != 1 || extra.evs[0].Time.IsZero() {
		t.Fatalf("extra sink: %+v", extra.evs)
	}
	out := buf.String()
	if !strings.Contains(out, `"msg":"audit"`) || !strings.Contains(out, "no_matching_rule") || !strings.Contains(out, `"attr_note":"fine"`) {
		t.Fatalf("slog sink output: %s", out)
	}
	if strings.Contains(out, "SECRET") {
		t.Fatalf("attrs must pass through redaction: %s", out)
	}
	bad := sample()
	bad.Reason = "peer text"
	if err := em.Emit(context.Background(), bad); err == nil || em.Invalid() != 1 {
		t.Fatal("invalid event must be rejected and counted")
	}
}

func TestEmitterAsyncDropsWhenFull(t *testing.T) {
	slow := &memSink{slow: make(chan struct{})}
	em := NewEmitter(nil, 2, slow)
	for i := 0; i < 10; i++ {
		if err := em.Emit(context.Background(), sample()); err != nil {
			t.Fatal(err)
		}
	}
	if em.Dropped() == 0 {
		t.Fatal("expected drops with a full queue")
	}
	close(slow.slow)
	em.Close()
	em.Close() // idempotent
	if got := slow.n() + int(em.Dropped()); got != 10 {
		t.Fatalf("delivered+dropped = %d, want 10", got)
	}
	if err := em.Emit(context.Background(), sample()); err != nil {
		t.Fatal(err)
	}
	em2 := NewEmitter(nil, 0)
	em2.AddSink(slow)
	_ = em2.Emit(context.Background(), sample())
	em2.Close()
}
