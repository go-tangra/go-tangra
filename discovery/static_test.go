package discovery

import (
	"context"
	"testing"
	"time"
)

func TestStaticResolves(t *testing.T) {
	s, err := NewStatic(map[string][]string{"inventory": {"10.0.0.5:9443", "inv.internal:9443"}})
	if err != nil {
		t.Fatal(err)
	}
	inst, err := s.GetService(context.Background(), "inventory")
	if err != nil || len(inst) != 2 {
		t.Fatalf("%v %v", inst, err)
	}
	if inst[0].Endpoints[0] != "grpcs://10.0.0.5:9443" || inst[0].Name != "inventory" || inst[0].ID == inst[1].ID {
		t.Fatalf("bad instance %+v", inst[0])
	}
	if _, err := s.GetService(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown service must error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	w, err := s.Watch(ctx, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	first, err := w.Next()
	if err != nil || len(first) != 2 {
		t.Fatalf("first Next: %v %v", first, err)
	}
	done := make(chan error, 1)
	go func() { _, err := w.Next(); done <- err }()
	select {
	case <-done:
		t.Fatal("second Next must block until stop")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("Next after cancel must error")
	}
	if err := w.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Watch(context.Background(), "unknown"); err == nil {
		t.Fatal("watch unknown must error")
	}
}

func TestStaticValidation(t *testing.T) {
	bad := []map[string][]string{
		{"Inventory": {"10.0.0.5:9443"}},
		{"inventory": {"10.0.0.5"}},
		{"inventory": {"grpcs://10.0.0.5:9443"}},
		{"inventory": {}},
		{"inventory": {""}},
	}
	for _, m := range bad {
		if _, err := NewStatic(m); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
	if s, err := NewStatic(nil); err != nil || s == nil {
		t.Fatal("empty map is allowed")
	}
}
