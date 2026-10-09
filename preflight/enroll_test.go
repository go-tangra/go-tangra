package preflight

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var clock = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

const tenant = "00000000-0000-0000-0000-000000000001"

// jwtWith builds an unsigned-looking compact JWT carrying claims; the
// signature segment is junk because the checks never verify it.
func jwtWith(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "EdDSA", "kid": "k1"}) + "." + enc(claims) + ".c2lnbmF0dXJl"
}

func claims(mut func(map[string]any)) map[string]any {
	c := map[string]any{
		"iss": "https://auth.example.org", "sub": tenant, "aud": []string{"lcm"},
		"iat": clock.Add(-5 * time.Minute).Unix(), "nbf": clock.Add(-5 * time.Minute).Unix(),
		"exp": clock.Add(25 * time.Minute).Unix(), "jti": "j1",
		"tid": tenant, "spiffe_paths": []string{"spiffe://infra.example.org/svc/sms-gw", "spiffe://infra.example.org/svc/other"},
	}
	if mut != nil {
		mut(c)
	}
	return c
}

func writeToken(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

var expect = EnrollExpect{TrustDomain: "infra.example.org", ServiceName: "sms-gw", TenantID: tenant, Now: func() time.Time { return clock }}

func TestDecodeEnrollToken(t *testing.T) {
	tok, err := DecodeEnrollToken(" " + jwtWith(t, claims(func(c map[string]any) { c["aud"] = "lcm" })) + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if tok.TenantID != tenant || len(tok.SpiffePaths) != 2 || len(tok.Audience) != 1 || tok.Audience[0] != "lcm" ||
		!tok.ExpiresAt.Equal(clock.Add(25*time.Minute)) || tok.Issuer != "https://auth.example.org" {
		t.Fatalf("decoded %+v", tok)
	}
	for _, bad := range []string{"", "abc", "a.b", "a.!!!.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("[1]")) + ".c",
		"a." + base64.RawURLEncoding.EncodeToString([]byte(`{"aud":7}`)) + ".c"} {
		if _, err := DecodeEnrollToken(bad); err == nil {
			t.Errorf("%q must not decode", bad)
		}
	}
}

func TestEnrollTokenChecks(t *testing.T) {
	type want struct{ format, expiry, tenant, identity Status }
	allPass := want{Pass, Pass, Pass, Pass}
	cases := []struct {
		name    string
		content string
		expect  EnrollExpect
		want    want
		detail  string // expected in some result's detail
		fix     string // expected in some result's fix
	}{
		{name: "valid", want: allPass, detail: "expires in 25 min"},
		{name: "expired", content: jwtWith(t, claims(func(c map[string]any) { c["exp"] = clock.Add(-47 * time.Minute).Unix() })),
			want: want{Pass, Fail, Pass, Pass}, detail: "expired 47 min ago", fix: "Gateway operations > Enrolment tokens"},
		{name: "expires soon", content: jwtWith(t, claims(func(c map[string]any) { c["exp"] = clock.Add(3 * time.Minute).Unix() })),
			want: want{Pass, Warn, Pass, Pass}, detail: "expires in 3 min"},
		{name: "not yet valid", content: jwtWith(t, claims(func(c map[string]any) { c["nbf"] = clock.Add(10 * time.Minute).Unix() })),
			want: want{Pass, Fail, Pass, Pass}, detail: "not valid for another 10 min", fix: "NTP"},
		{name: "small clock skew tolerated", content: jwtWith(t, claims(func(c map[string]any) { c["nbf"] = clock.Add(30 * time.Second).Unix() })),
			want: allPass},
		{name: "no expiry", content: jwtWith(t, claims(func(c map[string]any) { delete(c, "exp") })),
			want: want{Pass, Fail, Pass, Pass}, detail: "no expiry"},
		{name: "other tenant", content: jwtWith(t, claims(func(c map[string]any) { c["tid"] = "11111111-1111-1111-1111-111111111111" })),
			want: want{Pass, Pass, Fail, Pass}, detail: "token is for tenant 11111111", fix: "set the configured tenant to 11111111"},
		{name: "tenant not configured", expect: EnrollExpect{TrustDomain: "infra.example.org", ServiceName: "sms-gw", Now: expect.Now},
			want: want{Pass, Pass, Skip, Pass}},
		{name: "wrong trust domain", expect: EnrollExpect{TrustDomain: "example.org", ServiceName: "sms-gw", TenantID: tenant, Now: expect.Now},
			want: want{Pass, Pass, Pass, Fail}, detail: "does not authorise spiffe://example.org/svc/sms-gw",
			fix: "trust_domain example.org but the token names spiffe://infra.example.org/svc/sms-gw: set trust_domain: infra.example.org"},
		{name: "wrong service", expect: EnrollExpect{TrustDomain: "infra.example.org", ServiceName: "mail-gw", TenantID: tenant, Now: expect.Now},
			want: want{Pass, Pass, Pass, Fail}, fix: "service_name mail-gw but the token names spiffe://infra.example.org/svc/sms-gw"},
		{name: "unrelated identity", content: jwtWith(t, claims(func(c map[string]any) { c["spiffe_paths"] = []string{"not-a-spiffe-id"} })),
			want: want{Pass, Pass, Pass, Fail}, fix: "Enrolment tokens) for spiffe://infra.example.org/svc/sms-gw"},
		{name: "invalid configured identity", expect: EnrollExpect{TrustDomain: "Bad Domain", ServiceName: "sms-gw", Now: expect.Now},
			want: want{Pass, Pass, Skip, Skip}},
		{name: "access token", content: jwtWith(t, claims(func(c map[string]any) { c["aud"] = "portal" })),
			want: want{Fail, Pass, Pass, Pass}, detail: "not an enrolment token"},
		{name: "claims missing", content: jwtWith(t, claims(func(c map[string]any) { delete(c, "tid") })),
			want: want{Fail, Pass, Fail, Pass}, detail: "lacks the tenant"},
		{name: "garbage", content: "hello", want: want{Fail, Skip, Skip, Skip}, detail: "not a JWT"},
		{name: "empty file", content: "", want: want{Fail, Skip, Skip, Skip}, detail: "is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.content
			if content == "" && tc.name != "empty file" {
				content = jwtWith(t, claims(nil))
			}
			exp := tc.expect
			if exp.Now == nil {
				exp = expect
			}
			res := Run(context.Background(), EnrollTokenChecks("enrol token", writeToken(t, content), exp))
			got := want{res[0].Status, res[1].Status, res[2].Status, res[3].Status}
			if got != tc.want {
				t.Fatalf("statuses %v, want %v: %+v", got, tc.want, res)
			}
			if res[0].Name != "enrol token: format" || res[3].Name != "enrol token: identity" {
				t.Fatalf("names: %+v", res)
			}
			var details, fixes strings.Builder
			for _, r := range res {
				details.WriteString(r.Detail + "\n")
				fixes.WriteString(r.Fix + "\n")
				if len(content) > 8 && (strings.Contains(r.Detail, content) || strings.Contains(r.Fix, content)) {
					t.Fatal("the token must never be reported")
				}
			}
			if !strings.Contains(details.String(), tc.detail) {
				t.Errorf("details lack %q:\n%s", tc.detail, details.String())
			}
			if !strings.Contains(fixes.String(), tc.fix) {
				t.Errorf("fixes lack %q:\n%s", tc.fix, fixes.String())
			}
		})
	}
}

func TestEnrollTokenMissingFile(t *testing.T) {
	res := Run(context.Background(), EnrollTokenChecks("tok", filepath.Join(t.TempDir(), "absent"), expect))
	if res[0].Status != Fail || !strings.Contains(res[0].Detail, "does not exist") || res[1].Status != Skip {
		t.Fatalf("%+v", res)
	}
}

func TestEnrollTokenFormatSaysUnverified(t *testing.T) {
	res := Run(context.Background(), EnrollTokenChecks("tok", writeToken(t, jwtWith(t, claims(nil))), expect))
	if !strings.Contains(res[0].Detail, "signature NOT verified") {
		t.Fatalf("format detail must state the check is local: %q", res[0].Detail)
	}
}
