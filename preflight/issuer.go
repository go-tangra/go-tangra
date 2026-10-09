package preflight

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

// JWKSPath is where the auth service publishes its signing keys, under the
// issuer's origin.
const JWKSPath = "/.well-known/jwks.json"

// maxJWKS bounds the key set read by IssuerJWKS.
const maxJWKS = 64 << 10

const issuerFix = "set the issuer to the portal's public origin, exactly as the core's auth service has it " +
	"(auth's 'issuer' setting, e.g. https://portal.example.com:8443): user tokens carry it and every other value refuses them"

// IssuerJWKS checks that issuer is the origin of the platform's auth service:
// it fetches issuer + JWKSPath and requires at least one Ed25519 signing key.
// A wrong host, port or scheme fails here instead of every console request
// later failing with "session ended". cfg is the TLS configuration to use
// (nil: system roots).
//
// It cannot prove the value is byte-for-byte what auth signs with (a
// different path on the same origin also serves the keys); IssuerOrigin
// complements it offline.
func IssuerJWKS(name, issuer string, cfg *tls.Config, timeout time.Duration) Check {
	return Check{Name: name, Network: true, Run: func(ctx context.Context) Result {
		u, err := url.Parse(issuer)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
			return Failf("issuer %q is not an https origin", issuer).WithFix("%s", issuerFix)
		}
		jwks := strings.TrimSuffix(u.String(), "/") + JWKSPath
		g, fail := httpsGet(ctx, jwks, cfg, timeout, maxJWKS)
		if fail != nil {
			// A wrong issuer is the likelier cause than the network.
			r := *fail
			if r.Fix == "" {
				return r.WithFix("%s", issuerFix)
			}
			return r.WithFix("%s; if the issuer is right: %s", issuerFix, r.Fix)
		}
		if g.status != 200 {
			return Failf("%s answered HTTP %d: no platform signing keys there, so %q is not the issuer", jwks, g.status, issuer).
				WithFix("%s", issuerFix)
		}
		n, ok := ed25519Keys(g.body)
		if !ok {
			return Failf("%s did not answer with a key set: %q is not the platform's issuer", jwks, issuer).
				WithFix("%s", issuerFix)
		}
		if n == 0 {
			return Failf("%s has no Ed25519 signing keys: %q is not the platform's issuer", jwks, issuer).
				WithFix("%s", issuerFix)
		}
		if g.insecure {
			return Warnf("%s publishes %d signing key(s), but its certificate was NOT verified (insecure)", jwks, n).
				WithFix("development only; production needs a verifiable certificate")
		}
		return Passf("%s publishes %d platform signing key(s)", jwks, n)
	}}
}

// ed25519Keys counts the EdDSA/Ed25519 signing keys of a JWKS document.
func ed25519Keys(body []byte) (int, bool) {
	if len(body) > maxJWKS {
		return 0, false
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
			Use string `json:"use"`
			X   string `json:"x"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil || set.Keys == nil {
		return 0, false
	}
	n := 0
	for _, k := range set.Keys {
		if k.Kty == "OKP" && k.Crv == "Ed25519" && k.X != "" && (k.Use == "" || k.Use == "sig") {
			n++
		}
	}
	return n, true
}

// IssuerOrigin warns, offline, when the issuer's origin differs from the
// origin of ref, another URL of the same portal (refName names it, e.g. the
// enrolment URL): in a standard install both are the portal's public origin.
func IssuerOrigin(name, issuer, refName, ref string) Check {
	return Check{Name: name, Run: func(context.Context) Result {
		iu, err := url.Parse(issuer)
		if err != nil || iu.Scheme != "https" || iu.Host == "" {
			return Failf("issuer %q is not an https origin", issuer).WithFix("%s", issuerFix)
		}
		ru, err := url.Parse(ref)
		if err != nil || ru.Host == "" {
			return Skipf("%s %q is not a URL to compare with", refName, ref)
		}
		if origin(iu) == origin(ru) {
			return Passf("issuer %s is the %s's origin", origin(iu), refName)
		}
		return Warnf("issuer origin %s differs from the %s's origin %s", origin(iu), refName, origin(ru)).
			WithFix("both are normally the portal's public origin; %s", issuerFix)
	}}
}

// origin is scheme://host:port with the default port made explicit.
func origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[strings.ToLower(u.Scheme)]
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Hostname()) + ":" + port
}
