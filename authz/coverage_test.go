package authz

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/authn"
	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/identity"
)

type badReader struct{}

func (badReader) Read([]byte) (int, error) { return 0, errors.New("disk error") }

func TestCoverageGaps(t *testing.T) {
	c := NewCached(nil, 1)
	if c.Version() != "" || c.Policy() != nil {
		t.Fatal("nil policy accessors")
	}
	p := mustLoad(t, "version: v\nrules: []\n")
	c.Swap(p)
	if c.Policy() != p || c.Version() != "v" {
		t.Fatal("swap accessors")
	}
	if _, err := Load(badReader{}); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("read error: %v", err)
	}
	ops := make([]string, 257)
	for i := range ops {
		ops[i] = "op"
	}
	if _, err := NewPolicy("v", []Rule{{ID: "a", From: []string{"spiffe://x/svc/a"}, To: []string{"b"}, Operations: ops, Effect: Allow}}); err == nil {
		t.Fatal("257 operations must be rejected")
	}
	if !globMatch("ab**", "ab") || globMatch("ab*c", "ab") {
		t.Fatal("trailing star handling")
	}
	peer := authn.PeerIdentity{ID: identity.ForService("example.org", "a"), ServiceName: "a"}
	Config{}.emit(context.Background(), "authz_refused", "refused", Decision{}, peer, "op")
	saved := fileSourceFactory
	defer func() { fileSourceFactory = saved }()
	fileSourceFactory = nil
	if _, err := NewSourceFromConfig(config.Authz{Source: config.AuthzFile, Path: "x"}); err == nil {
		t.Fatal("unlinked file source must error")
	}
	if _, err := NewSourceFromConfig(config.Authz{Source: config.AuthzValkey}); err == nil {
		t.Fatal("valkey must point to contrib")
	}
	if _, err := NewSourceFromConfig(config.Authz{Source: "opa"}); err == nil {
		t.Fatal("unknown source must error")
	}
	RegisterFileSource(func(string) (Source, error) { return nil, errors.New("stub") })
	if _, err := NewSourceFromConfig(config.Authz{Source: config.AuthzFile, Path: "x"}); err == nil || err.Error() != "stub" {
		t.Fatalf("factory not used: %v", err)
	}
}

func TestDenyAll(t *testing.T) {
	d := DenyAll{}.Authorize(context.Background(), identity.ForService("example.org", "a"), "b", "op")
	if d.Allowed || d.Reason != ReasonNoPolicy {
		t.Fatalf("%+v", d)
	}
}
