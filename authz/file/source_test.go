package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const p1 = "version: one\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n"
const p2 = "version: two\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: deny}\n"

func TestFileSourceLoadsAndWatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(p1), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := New(path, WithPollInterval(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pol, err := src.Load(ctx)
	if err != nil || pol.Version != "one" || pol.Source != "file:"+path {
		t.Fatalf("%+v %v", pol, err)
	}
	ch, err := src.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := os.WriteFile(path, []byte(p2), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-ch:
		if p == nil || p.Version != "two" {
			t.Fatalf("got %+v", p)
		}
	case <-ctx.Done():
		t.Fatal("no reload")
	}
	// Invalid rewrite keeps last-known-good and reports the failure.
	errs := src.Errors()
	if err := os.WriteFile(path, []byte("version: [broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-errs:
		if e == nil {
			t.Fatal("nil error")
		}
	case <-ctx.Done():
		t.Fatal("no error reported")
	}
	select {
	case p := <-ch:
		t.Fatalf("invalid file must not yield a policy: %+v", p)
	case <-time.After(100 * time.Millisecond):
	}
	pol, err = src.Load(ctx)
	if err != nil || pol.Version != "two" {
		t.Fatalf("last-known-good lost: %+v %v", pol, err)
	}
}

func TestFileSourceRejects(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing file must fail")
	}
	path := filepath.Join(t.TempDir(), "bad.yaml")
	_ = os.WriteFile(path, []byte("nope"), 0o600)
	if _, err := New(path); err == nil {
		t.Fatal("invalid file must fail")
	}
}
