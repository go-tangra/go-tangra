package fuzz

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-freya/freya/authz"
)

func FuzzPolicyLoad(f *testing.F) {
	f.Add("version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n")
	f.Add(`{"version":"v","rules":[]}`)
	f.Add("version: 1\nrules: [{id: a, from: [\"spiffe://x/svc/*\"], to: [\"*\"], operations: [\"GET /x/*\"], effect: deny}]")
	f.Add("")
	f.Fuzz(func(t *testing.T, in string) {
		p, err := authz.Load(strings.NewReader(in))
		if err != nil {
			return
		}
		b, err := json.Marshal(struct {
			Version string       `json:"version"`
			Rules   []authz.Rule `json:"rules"`
		}{p.Version, p.Rules})
		if err != nil {
			t.Fatalf("re-marshal: %v", err)
		}
		if _, err := authz.Load(bytes.NewReader(b)); err != nil {
			t.Fatalf("re-load of a valid policy failed: %v\n%s", err, b)
		}
	})
}
