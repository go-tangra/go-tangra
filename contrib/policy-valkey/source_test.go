package valkey

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-freya/freya/identity"
)

type fakeKV struct {
	mu   sync.Mutex
	data map[string]string
	subs []func(string)
	fail error
}

func newFake() *fakeKV { return &fakeKV{data: map[string]string{}} }

func (f *fakeKV) Get(_ context.Context, k string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return "", false, f.fail
	}
	v, ok := f.data[k]
	return v, ok, nil
}

func (f *fakeKV) Exists(_ context.Context, k string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return false, f.fail
	}
	_, ok := f.data[k]
	return ok, nil
}

func (f *fakeKV) Subscribe(ctx context.Context, _ string, on func(string)) error {
	f.mu.Lock()
	f.subs = append(f.subs, on)
	f.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (f *fakeKV) Close() {}

func (f *fakeKV) set(k, v string) { f.mu.Lock(); f.data[k] = v; f.mu.Unlock() }
func (f *fakeKV) publish(msg string) {
	f.mu.Lock()
	subs := append([]func(string){}, f.subs...)
	f.mu.Unlock()
	for _, s := range subs {
		s(msg)
	}
}

const doc1 = "version: one\nrules:\n  - {id: a, from: [\"spiffe://example.org/svc/a\"], to: [b], effect: allow}\n"
const doc2 = "version: two\nrules:\n  - {id: a, from: [\"spiffe://example.org/svc/a\"], to: [b], effect: deny}\n"

func TestSourceLoadsAndReloadsOnPublish(t *testing.T) {
	kv := newFake()
	kv.set(docKey("example.org"), doc1)
	kv.set(versionKey("example.org"), "one")
	src, err := NewSource(context.Background(), kv, "example.org", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	pol, _ := src.Load(context.Background())
	if pol.Version != "one" || pol.Source != "valkey:freya:policy:example.org:doc" {
		t.Fatalf("%+v", pol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := src.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // let the subscription register
	kv.set(docKey("example.org"), doc2)
	kv.set(versionKey("example.org"), "two")
	kv.publish("two")
	select {
	case p := <-ch:
		if p.Version != "two" {
			t.Fatalf("got %+v", p)
		}
	case <-ctx.Done():
		t.Fatal("no reload")
	}
	// Same version again: no delivery.
	kv.publish("two")
	select {
	case p := <-ch:
		t.Fatalf("unexpected redelivery %+v", p)
	case <-time.After(100 * time.Millisecond):
	}
	// Invalid document keeps last-known-good and reports.
	kv.set(docKey("example.org"), "version: [broken")
	kv.set(versionKey("example.org"), "three")
	kv.publish("three")
	select {
	case err := <-src.Errors():
		if err == nil {
			t.Fatal("nil error")
		}
	case <-ctx.Done():
		t.Fatal("no error reported")
	}
	pol, _ = src.Load(context.Background())
	if pol.Version != "two" {
		t.Fatal("last-known-good lost")
	}
	// Version mismatch between keys is rejected.
	kv.set(docKey("example.org"), doc2)
	kv.set(versionKey("example.org"), "mismatch")
	kv.publish("mismatch")
	select {
	case err := <-src.Errors():
		if err == nil {
			t.Fatal("nil error")
		}
	case <-ctx.Done():
		t.Fatal("no mismatch error")
	}
	// Backend failure reports, keeps policy.
	kv.mu.Lock()
	kv.fail = errors.New("down")
	kv.mu.Unlock()
	kv.publish("x")
	select {
	case <-src.Errors():
	case <-ctx.Done():
		t.Fatal("no failure error")
	}
	if p, err := src.Load(context.Background()); err != nil || p.Version != "two" {
		t.Fatal("policy must survive backend outage")
	}
}

func TestSourceRejectsMissingOrInvalid(t *testing.T) {
	kv := newFake()
	if _, err := NewSource(context.Background(), kv, "example.org", 0); err == nil {
		t.Fatal("missing doc must fail")
	}
	kv.set(docKey("example.org"), "nope")
	if _, err := NewSource(context.Background(), kv, "example.org", 0); err == nil {
		t.Fatal("invalid doc must fail")
	}
	kv.mu.Lock()
	kv.fail = errors.New("down")
	kv.mu.Unlock()
	if _, err := NewSource(context.Background(), kv, "example.org", 0); err == nil {
		t.Fatal("backend error must fail")
	}
}

func TestRevocationChecker(t *testing.T) {
	kv := newFake()
	r := NewRevocationChecker(kv)
	id := identity.ForService("example.org", "orders")
	if ok, err := r.IsRevoked(context.Background(), id, "1"); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	kv.set(revokedKey(id.String(), "1"), "1")
	if ok, _ := r.IsRevoked(context.Background(), id, "1"); !ok {
		t.Fatal("serial revocation")
	}
	if ok, _ := r.IsRevoked(context.Background(), id, "2"); ok {
		t.Fatal("other serial must not be revoked")
	}
	kv.set(revokedKey(id.String(), ""), "1")
	if ok, _ := r.IsRevoked(context.Background(), id, "2"); !ok {
		t.Fatal("identity revocation")
	}
	kv.mu.Lock()
	kv.fail = errors.New("down")
	kv.mu.Unlock()
	if _, err := r.IsRevoked(context.Background(), id, "2"); err == nil {
		t.Fatal("backend failure must return an error (fail closed upstream)")
	}
}

func TestNewClientRequiresTLS(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("addresses required")
	}
	if _, err := NewClient(Config{Addresses: []string{"127.0.0.1:1"}}); err == nil {
		t.Fatal("TLS required by default")
	}
}
