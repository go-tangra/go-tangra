package fuzz

import (
	"encoding/json"
	"testing"

	"github.com/go-freya/freya/audit"
)

func FuzzAuditEventUnmarshal(f *testing.F) {
	f.Add([]byte(`{"time":"2026-09-15T12:00:00Z","type":"authn_refused","outcome":"refused","reason":"no_identity","local_id":"spiffe://example.org/svc/a","correlation_id":"abc"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"attrs":{"a":1}}`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, in []byte) {
		var e audit.Event
		if err := json.Unmarshal(in, &e); err != nil {
			return
		}
		if err := e.Validate(); err != nil {
			return
		}
		out, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("valid event failed to marshal: %v", err)
		}
		var again audit.Event
		if err := json.Unmarshal(out, &again); err != nil {
			t.Fatalf("re-unmarshal failed: %v", err)
		}
		if err := again.Validate(); err != nil {
			t.Fatalf("re-validated event invalid: %v", err)
		}
	})
}
