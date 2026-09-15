package authz

import (
	"strings"
	"testing"
)

const goodYAML = `version: v1
rules:
  - id: a-to-b
    from: ["spiffe://example.org/svc/a"]
    to: ["b"]
    operations: ["/pkg.Svc/Method"]
    effect: allow
  - id: block-legacy
    from: ["spiffe://example.org/svc/legacy-*"]
    to: ["*"]
    effect: deny
`

func TestLoadYAMLAndJSON(t *testing.T) {
	p, err := Load(strings.NewReader(goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "v1" || len(p.Rules) != 2 || p.Rules[1].Operations[0] != "*" || p.LoadedAt.IsZero() {
		t.Fatalf("policy %+v", p)
	}
	js := `{"version":"v2","rules":[{"id":"x","from":["spiffe://example.org/svc/a"],"to":["b"],"effect":"allow"}]}`
	p, err = Load(strings.NewReader(js))
	if err != nil || p.Version != "v2" || p.Rules[0].Operations[0] != "*" {
		t.Fatalf("json: %+v %v", p, err)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"empty":          ``,
		"no version":     `rules: []`,
		"no rules key":   `version: v1`,
		"dup id":         "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n",
		"empty from":     "version: v1\nrules:\n  - {id: a, from: [], to: [b], effect: allow}\n",
		"empty to":       "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [], effect: allow}\n",
		"bad effect":     "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: maybe}\n",
		"bad from glob":  "version: v1\nrules:\n  - {id: a, from: [\"https://x/svc/a\"], to: [b], effect: allow}\n",
		"bad to glob":    "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [\"B\"], effect: allow}\n",
		"bad id":         "version: v1\nrules:\n  - {id: \"a b\", from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n",
		"unknown field":  "version: v1\nextra: 1\nrules: []\n",
		"unknown rule f": "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], effect: allow, when: never}\n",
		"empty op":       "version: v1\nrules:\n  - {id: a, from: [\"spiffe://x/svc/a\"], to: [b], operations: [\"\"], effect: allow}\n",
		"long version":   "version: " + strings.Repeat("v", 129) + "\nrules: []\n",
		"not a map":      `[1,2]`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(body)); err == nil {
				t.Fatalf("expected error for %q", body)
			}
		})
	}
	var sb strings.Builder
	sb.WriteString("version: v1\nrules:\n")
	for i := 0; i < 10001; i++ {
		sb.WriteString("  - {id: r" + strings.Repeat("x", 0) + itoa(i) + ", from: [\"spiffe://x/svc/a\"], to: [b], effect: allow}\n")
	}
	if _, err := Load(strings.NewReader(sb.String())); err == nil {
		t.Fatal(">10000 rules must be rejected")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
