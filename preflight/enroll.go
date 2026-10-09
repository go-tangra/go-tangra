package preflight

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// MintHint tells an operator where a fresh enrolment token comes from.
const MintHint = "mint a new one in the portal (Gateway operations > Enrolment tokens)"

// enrollAudience is the audience auth gives every lcm enrolment token.
const enrollAudience = "lcm"

// EnrollToken is the content of an lcm enrolment (join) token decoded
// LOCALLY: its signature is NOT verified (lcm does that at enrolment), so it
// is only a sanity check of what the operator was given.
type EnrollToken struct {
	Issuer      string
	Audience    []string
	TenantID    string
	SpiffePaths []string
	IssuedAt    time.Time
	NotBefore   time.Time
	ExpiresAt   time.Time
}

// DecodeEnrollToken decodes the claims of a compact JWT without verifying it.
// Errors never contain the token.
func DecodeEnrollToken(raw string) (EnrollToken, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 3 {
		return EnrollToken{}, errors.New("not a JWT (expected three dot-separated parts)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return EnrollToken{}, errors.New("JWT payload is not base64url")
	}
	var c struct {
		Iss         string          `json:"iss"`
		Aud         json.RawMessage `json:"aud"`
		Tid         string          `json:"tid"`
		SpiffePaths []string        `json:"spiffe_paths"`
		Iat         *float64        `json:"iat"`
		Nbf         *float64        `json:"nbf"`
		Exp         *float64        `json:"exp"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return EnrollToken{}, errors.New("JWT payload is not a JSON claims object")
	}
	t := EnrollToken{Issuer: c.Iss, TenantID: c.Tid, SpiffePaths: c.SpiffePaths,
		IssuedAt: unix(c.Iat), NotBefore: unix(c.Nbf), ExpiresAt: unix(c.Exp)}
	if len(c.Aud) > 0 {
		var one string
		if json.Unmarshal(c.Aud, &one) == nil {
			t.Audience = []string{one}
		} else if json.Unmarshal(c.Aud, &t.Audience) != nil {
			return EnrollToken{}, errors.New("JWT aud claim is neither a string nor a list")
		}
	}
	return t, nil
}

func unix(v *float64) time.Time {
	if v == nil {
		return time.Time{}
	}
	sec := int64(*v)
	return time.Unix(sec, int64((*v-float64(sec))*1e9)).UTC()
}

// EnrollExpect is what the module's configuration expects of its token.
type EnrollExpect struct {
	TrustDomain string
	ServiceName string
	// TenantID is the tenant the module enrols into; empty skips that check.
	TenantID string
	// Now overrides the clock (tests); nil means time.Now.
	Now func() time.Time
}

// EnrollTokenChecks reads the token in tokenFile once and checks, without
// verifying its signature: its format and audience, its validity window, its
// tenant against want.TenantID and that it authorises
// spiffe://<trust_domain>/svc/<service_name>. Names are prefix + ": format",
// ": expiry", ": tenant" and ": identity". The token is never reported.
func EnrollTokenChecks(prefix, tokenFile string, want EnrollExpect) []Check {
	now := want.Now
	if now == nil {
		now = time.Now
	}
	var (
		once   sync.Once
		tok    EnrollToken
		bad    *Result
		loaded = func() (EnrollToken, *Result) {
			once.Do(func() {
				if r := fileReadable(tokenFile); r.Status != Pass {
					bad = &r
					return
				}
				raw, err := os.ReadFile(tokenFile) // #nosec G304 -- operator-supplied token path
				if err != nil {
					r := Failf("%s is not readable", tokenFile)
					bad = &r
					return
				}
				if tok, err = DecodeEnrollToken(string(raw)); err != nil {
					r := Failf("%s: %s", tokenFile, err).WithFix("the file must hold exactly the token text; " + MintHint)
					bad = &r
				}
			})
			return tok, bad
		}
	)
	skipped := Skipf("token unusable (see %s: format)", prefix)
	return []Check{
		{Name: prefix + ": format", Run: func(context.Context) Result {
			t, bad := loaded()
			if bad != nil {
				return *bad
			}
			if !slices.Contains(t.Audience, enrollAudience) {
				return Failf("audience %v is not %q: this is not an enrolment token", t.Audience, enrollAudience).
					WithFix("use an enrolment token, not an access token: " + MintHint)
			}
			if t.TenantID == "" || len(t.SpiffePaths) == 0 {
				return Failf("token lacks the tenant (tid) or spiffe_paths claim").WithFix(MintHint)
			}
			return Passf("decoded locally, signature NOT verified (lcm verifies it at enrolment); issuer %s", orNone(t.Issuer))
		}},
		{Name: prefix + ": expiry", Run: func(context.Context) Result {
			t, bad := loaded()
			if bad != nil {
				return skipped
			}
			n := now()
			switch {
			case t.ExpiresAt.IsZero():
				return Failf("token has no expiry (exp)").WithFix(MintHint)
			case !n.Before(t.ExpiresAt):
				return Failf("token expired %s ago (%s)", human(n.Sub(t.ExpiresAt)), stamp(t.ExpiresAt)).
					WithFix("%s; tokens live at most 30 min and are single-use, so mint it right before the first start", MintHint)
			case !t.NotBefore.IsZero() && n.Add(time.Minute).Before(t.NotBefore):
				return Failf("token not valid for another %s (from %s)", human(t.NotBefore.Sub(n)), stamp(t.NotBefore)).
					WithFix("this host's clock is behind the core's: check NTP")
			case t.ExpiresAt.Sub(n) < 5*time.Minute:
				return Warnf("token expires in %s (%s)", human(t.ExpiresAt.Sub(n)), stamp(t.ExpiresAt)).
					WithFix("start the module before then, or mint a fresh one")
			}
			return Passf("token valid, expires in %s (%s)", human(t.ExpiresAt.Sub(n)), stamp(t.ExpiresAt))
		}},
		{Name: prefix + ": tenant", Run: func(context.Context) Result {
			t, bad := loaded()
			switch {
			case bad != nil:
				return skipped
			case want.TenantID == "":
				return Skipf("no tenant configured")
			case strings.EqualFold(t.TenantID, want.TenantID):
				return Passf("token is for tenant %s, as configured", t.TenantID)
			}
			return Failf("token is for tenant %s but the configured tenant is %s", t.TenantID, want.TenantID).
				WithFix("set the configured tenant to %s, or mint a token for tenant %s", t.TenantID, want.TenantID)
		}},
		{Name: prefix + ": identity", Run: func(context.Context) Result {
			t, bad := loaded()
			if bad != nil {
				return skipped
			}
			id, err := identity.NewSPIFFEID(want.TrustDomain, want.ServiceName)
			if err != nil {
				return Skipf("trust_domain or service_name is invalid (see the config checks)")
			}
			if slices.Contains(t.SpiffePaths, id.String()) {
				return Passf("token authorises %s", id)
			}
			return Failf("token does not authorise %s (it names %s)", id, strings.Join(t.SpiffePaths, ", ")).
				WithFix("%s", identityFix(id, t.SpiffePaths))
		}},
	}
}

// identityFix explains the most likely mismatch between the configured
// identity and the ones the token names.
func identityFix(want identity.SPIFFEID, paths []string) string {
	for _, p := range paths {
		got, err := identity.ParseSPIFFEID(p)
		if err != nil {
			continue
		}
		if got.ServiceName() == want.ServiceName() && got.TrustDomain() != want.TrustDomain() {
			return fmt.Sprintf("trust_domain %s but the token names %s: set trust_domain: %s (the core's trust domain) if the token is right",
				want.TrustDomain(), got, got.TrustDomain())
		}
	}
	for _, p := range paths {
		got, err := identity.ParseSPIFFEID(p)
		if err == nil && got.TrustDomain() == want.TrustDomain() {
			return fmt.Sprintf("service_name %s but the token names %s: fix service_name, or %s for %s", want.ServiceName(), got, MintHint, want)
		}
	}
	return fmt.Sprintf("%s for %s", MintHint, want)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
