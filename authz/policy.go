package authz

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// Rule is one policy rule (contracts/policy.schema.json).
type Rule struct {
	ID         string   `json:"id" yaml:"id"`
	From       []string `json:"from" yaml:"from"`
	To         []string `json:"to" yaml:"to"`
	Operations []string `json:"operations,omitempty" yaml:"operations,omitempty"`
	Effect     Effect   `json:"effect" yaml:"effect"`
}

// Effect is allow or deny.
type Effect string

// Effects.
const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// Policy is a loaded, validated, immutable authorization document.
type Policy struct {
	Version  string
	Rules    []Rule
	Source   string
	LoadedAt time.Time
	compiled []compiledRule
}

type compiledRule struct {
	id            string
	from, to, ops []string
	effect        Effect
}

const (
	maxRules      = 10000
	maxVersionLen = 128
	maxListLen    = 256
)

var (
	ruleIDRE      = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	spiffeGlobRE  = regexp.MustCompile(`^spiffe://[a-z0-9.-]{1,255}/svc/[a-z0-9*?-]{1,63}$`)
	serviceGlobRE = regexp.MustCompile(`^[a-z0-9*?-]{1,63}$`)
)

// NewPolicy validates rules and builds the matcher. Rules are evaluated in a
// canonical order (sorted by ID) so decisions never depend on document order.
func NewPolicy(version string, rules []Rule) (*Policy, error) {
	if version == "" || len(version) > maxVersionLen {
		return nil, errors.New("authz: version is required and at most 128 characters")
	}
	if len(rules) > maxRules {
		return nil, fmt.Errorf("authz: %d rules exceed the maximum of %d", len(rules), maxRules)
	}
	seen := map[string]struct{}{}
	p := &Policy{Version: version, Rules: make([]Rule, len(rules)), LoadedAt: time.Now()}
	copy(p.Rules, rules)
	sort.SliceStable(p.Rules, func(i, j int) bool { return p.Rules[i].ID < p.Rules[j].ID })
	for i := range p.Rules {
		r := &p.Rules[i]
		if !ruleIDRE.MatchString(r.ID) {
			return nil, fmt.Errorf("authz: rule %d: id must match ^[A-Za-z0-9._-]{1,64}$", i)
		}
		if _, dup := seen[r.ID]; dup {
			return nil, fmt.Errorf("authz: duplicate rule id %q", r.ID)
		}
		seen[r.ID] = struct{}{}
		if len(r.From) == 0 || len(r.From) > maxListLen {
			return nil, fmt.Errorf("authz: rule %q: from must have 1..256 entries", r.ID)
		}
		for _, f := range r.From {
			if !spiffeGlobRE.MatchString(f) {
				return nil, fmt.Errorf("authz: rule %q: from %q is not a SPIFFE ID glob", r.ID, f)
			}
		}
		if len(r.To) == 0 || len(r.To) > maxListLen {
			return nil, fmt.Errorf("authz: rule %q: to must have 1..256 entries", r.ID)
		}
		for _, to := range r.To {
			if !serviceGlobRE.MatchString(to) {
				return nil, fmt.Errorf("authz: rule %q: to %q is not a service name glob", r.ID, to)
			}
		}
		if len(r.Operations) == 0 {
			r.Operations = []string{"*"}
		}
		if len(r.Operations) > maxListLen {
			return nil, fmt.Errorf("authz: rule %q: operations must have at most 256 entries", r.ID)
		}
		for _, op := range r.Operations {
			if op == "" || len(op) > 256 {
				return nil, fmt.Errorf("authz: rule %q: operations entries must be 1..256 characters", r.ID)
			}
		}
		if r.Effect != Allow && r.Effect != Deny {
			return nil, fmt.Errorf("authz: rule %q: effect must be allow or deny", r.ID)
		}
		p.compiled = append(p.compiled, compiledRule{id: r.ID, from: r.From, to: r.To, ops: r.Operations, effect: r.Effect})
	}
	return p, nil
}

// Authorize implements Authorizer: collect all matching rules; any deny wins;
// otherwise any allow; otherwise deny (no_matching_rule). A nil policy denies
// with no_policy.
func (p *Policy) Authorize(_ context.Context, peer identity.SPIFFEID, callee, operation string) Decision {
	if p == nil {
		return Decision{Allowed: false, Reason: ReasonNoPolicy}
	}
	peerS := peer.String()
	allowID := ""
	for i := range p.compiled {
		r := &p.compiled[i]
		if !anyMatch(r.from, peerS) || !anyMatch(r.to, callee) || !anyMatch(r.ops, operation) {
			continue
		}
		if r.effect == Deny {
			return Decision{Allowed: false, Reason: ReasonExplicitDeny, RuleID: r.id, PolicyVersion: p.Version}
		}
		if allowID == "" {
			allowID = r.id
		}
	}
	if allowID != "" {
		return Decision{Allowed: true, Reason: ReasonExplicitAllow, RuleID: allowID, PolicyVersion: p.Version}
	}
	return Decision{Allowed: false, Reason: ReasonNoMatchingRule, PolicyVersion: p.Version}
}

func anyMatch(patterns []string, s string) bool {
	for _, pat := range patterns {
		if globMatch(pat, s) {
			return true
		}
	}
	return false
}

// globMatch supports '*' (any run, including '/') and '?' (one byte). It is
// iterative and linear in practice; there is no backtracking explosion because
// '*' matches greedily with a single retry point.
func globMatch(pattern, s string) bool {
	px, sx := 0, 0
	starPx, starSx := -1, -1
	for sx < len(s) {
		switch {
		case px < len(pattern) && (pattern[px] == '?' || pattern[px] == s[sx]):
			px++
			sx++
		case px < len(pattern) && pattern[px] == '*':
			starPx, starSx = px, sx
			px++
		case starPx >= 0:
			px = starPx + 1
			starSx++
			sx = starSx
		default:
			return false
		}
	}
	for px < len(pattern) && pattern[px] == '*' {
		px++
	}
	return px == len(pattern)
}
