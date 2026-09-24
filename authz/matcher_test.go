package authz

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/identity"
)

func mustLoad(t *testing.T, body string) *Policy {
	t.Helper()
	p, err := Load(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func id(name string) identity.SPIFFEID { return identity.ForService("example.org", name) }

func TestDecideMatrix(t *testing.T) {
	p := mustLoad(t, `version: m1
rules:
  - {id: orders-inv, from: ["spiffe://example.org/svc/orders"], to: ["inventory"], operations: ["/inventory.v1.Inventory/Reserve"], effect: allow}
  - {id: any-billing, from: ["spiffe://example.org/svc/*"], to: ["billing"], effect: allow}
  - {id: http-reads, from: ["spiffe://example.org/svc/web-?"], to: ["catalog"], operations: ["GET /v1/items/*"], effect: allow}
  - {id: block-legacy, from: ["spiffe://example.org/svc/legacy-*"], to: ["*"], effect: deny}
  - {id: legacy-billing, from: ["spiffe://example.org/svc/legacy-1"], to: ["billing"], effect: allow}
`)
	cases := []struct {
		peer, callee, op string
		allowed          bool
		reason           Reason
		rule             string
	}{
		{"orders", "inventory", "/inventory.v1.Inventory/Reserve", true, ReasonExplicitAllow, "orders-inv"},
		{"orders", "inventory", "/inventory.v1.Inventory/Release", false, ReasonNoMatchingRule, ""},
		{"orders", "catalog", "GET /v1/items/1", false, ReasonNoMatchingRule, ""},
		{"web-a", "catalog", "GET /v1/items/1", true, ReasonExplicitAllow, "http-reads"},
		{"web-ab", "catalog", "GET /v1/items/1", false, ReasonNoMatchingRule, ""},
		{"web-a", "catalog", "POST /v1/items/1", false, ReasonNoMatchingRule, ""},
		{"anyone", "billing", "/x.Y/Z", true, ReasonExplicitAllow, "any-billing"},
		{"legacy-1", "billing", "/x.Y/Z", false, ReasonExplicitDeny, "block-legacy"},
		{"legacy-9", "inventory", "/x.Y/Z", false, ReasonExplicitDeny, "block-legacy"},
		{"billing", "inventory", "/inventory.v1.Inventory/Reserve", false, ReasonNoMatchingRule, ""},
	}
	for _, tc := range cases {
		d := p.Authorize(context.Background(), id(tc.peer), tc.callee, tc.op)
		if d.Allowed != tc.allowed || d.Reason != tc.reason || d.RuleID != tc.rule || d.PolicyVersion != "m1" {
			t.Errorf("%s→%s %s: got %+v", tc.peer, tc.callee, tc.op, d)
		}
	}
	var nilPolicy *Policy
	if d := nilPolicy.Authorize(context.Background(), id("a"), "b", "c"); d.Allowed || d.Reason != ReasonNoPolicy {
		t.Fatalf("nil policy: %+v", d)
	}
	empty := mustLoad(t, "version: e\nrules: []\n")
	if d := empty.Authorize(context.Background(), id("a"), "b", "c"); d.Allowed || d.Reason != ReasonNoMatchingRule {
		t.Fatalf("empty policy: %+v", d)
	}
}

func TestDecideDeterministicUnderShuffle(t *testing.T) {
	base := mustLoad(t, `version: s
rules:
  - {id: a1, from: ["spiffe://example.org/svc/a"], to: ["b"], effect: allow}
  - {id: a2, from: ["spiffe://example.org/svc/*"], to: ["b"], effect: allow}
  - {id: d1, from: ["spiffe://example.org/svc/a"], to: ["*"], operations: ["/x.Y/Z"], effect: deny}
`)
	want := map[string]Decision{}
	probe := [][3]string{{"a", "b", "/x.Y/Z"}, {"a", "b", "/x.Y/W"}, {"c", "b", "/x.Y/Z"}, {"a", "c", "/x.Y/Z"}}
	for _, pr := range probe {
		want[strings.Join(pr[:], "|")] = base.Authorize(context.Background(), id(pr[0]), pr[1], pr[2])
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		rules := append([]Rule(nil), base.Rules...)
		r.Shuffle(len(rules), func(i, j int) { rules[i], rules[j] = rules[j], rules[i] })
		p, err := NewPolicy("s", rules)
		if err != nil {
			t.Fatal(err)
		}
		for _, pr := range probe {
			got := p.Authorize(context.Background(), id(pr[0]), pr[1], pr[2])
			if got != want[strings.Join(pr[:], "|")] {
				t.Fatalf("order-dependent decision for %v: %+v vs %+v", pr, got, want[strings.Join(pr[:], "|")])
			}
		}
	}
}

func TestGlobSemantics(t *testing.T) {
	cases := []struct {
		pattern, in string
		match       bool
	}{
		{"*", "anything", true}, {"a*", "abc", true}, {"a*", "b", false}, {"a?c", "abc", true}, {"a?c", "abbc", false},
		{"/x.Y/*", "/x.Y/Z", true}, {"/x.Y/*", "/x.Z/Z", false}, {"GET /v1/*", "GET /v1/a/b", true}, {"GET /v1/*", "POST /v1/a", false},
		{"exact", "exact", true}, {"exact", "exactly", false}, {"", "", true}, {"", "x", false},
	}
	for _, tc := range cases {
		if got := globMatch(tc.pattern, tc.in); got != tc.match {
			t.Errorf("globMatch(%q,%q)=%v", tc.pattern, tc.in, got)
		}
	}
}
