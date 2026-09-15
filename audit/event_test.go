package audit

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func sample() Event {
	return Event{
		Time:           time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
		Type:           TypeAuthzRefused,
		Outcome:        OutcomeRefused,
		Reason:         ReasonNoMatchingRule,
		LocalID:        "spiffe://example.org/svc/inventory",
		ClaimedPeerID:  "spiffe://example.org/svc/billing",
		VerifiedPeerID: "spiffe://example.org/svc/billing",
		Operation:      "/inventory.v1.Inventory/Reserve",
		CorrelationID:  "0190f7c2-6a3e-7c1a-9b2e-2f6f9d1b4c55",
		PolicyVersion:  "2026-09-15.1",
		RemoteAddr:     "10.0.3.17:48122",
	}
}

func TestEventJSONFieldNamesMatchSchema(t *testing.T) {
	raw, err := os.ReadFile("../specs/001-secure-service-channel/contracts/audit-event.schema.json")
	if err != nil {
		t.Skip("schema not available:", err)
	}
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for k := range got {
		if _, ok := schema.Properties[k]; !ok {
			t.Errorf("emitted field %q not in schema", k)
		}
	}
	for _, r := range schema.Required {
		if _, ok := got[r]; !ok {
			t.Errorf("required field %q missing", r)
		}
	}
	if got["time"] != "2026-09-15T12:00:00.000000000Z" {
		t.Errorf("time format %v", got["time"])
	}
	// Every enum value we define must be in the schema enum.
	var props struct {
		Type   struct{ Enum []string }
		Reason struct{ Enum []string }
	}
	_ = json.Unmarshal(schema.Properties["type"], &props.Type)
	_ = json.Unmarshal(schema.Properties["reason"], &props.Reason)
	for _, ty := range AllTypes() {
		if !contains(props.Type.Enum, string(ty)) {
			t.Errorf("type %q missing from schema enum", ty)
		}
	}
	for _, r := range AllReasons() {
		if !contains(props.Reason.Enum, string(r)) {
			t.Errorf("reason %q missing from schema enum", r)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestEventValidate(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		mut  func(*Event)
	}{
		{"zero time", func(e *Event) { e.Time = time.Time{} }},
		{"unknown type", func(e *Event) { e.Type = "bogus" }},
		{"unknown outcome", func(e *Event) { e.Outcome = "meh" }},
		{"unknown reason", func(e *Event) { e.Reason = "peer said so" }},
		{"missing local id", func(e *Event) { e.LocalID = "" }},
		{"bad local id", func(e *Event) { e.LocalID = "https://x" }},
		{"missing correlation", func(e *Event) { e.CorrelationID = "" }},
		{"bad correlation", func(e *Event) { e.CorrelationID = "has space" }},
		{"long correlation", func(e *Event) { e.CorrelationID = strings.Repeat("a", 129) }},
		{"bad trace id", func(e *Event) { e.TraceID = "xyz" }},
		{"long operation", func(e *Event) { e.Operation = strings.Repeat("a", 257) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := sample()
			tc.mut(&e)
			if err := e.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestEventJSONRoundTrip(t *testing.T) {
	e := sample()
	e.Attrs = map[string]string{"k": "v"}
	b, _ := json.Marshal(e)
	var back Event
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Time.Equal(e.Time) || back.Type != e.Type || back.Attrs["k"] != "v" {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	if strings.Contains(string(b), `"rule_id"`) {
		t.Fatalf("empty optional fields must be omitted: %s", b)
	}
}
