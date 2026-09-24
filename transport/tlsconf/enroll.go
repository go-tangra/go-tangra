package tlsconf

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-tangra/go-tangra/v4/config"
	"github.com/go-tangra/go-tangra/v4/identity"
)

// EnrollOptions select the server verification of a workload's first
// enrollment (a server-auth-only HTTPS call; the workload has no SVID yet).
// See config.EnrollTLS for the three modes.
type EnrollOptions struct {
	// CAPEM is the mesh trust bundle (PEM roots). When set, the server must
	// chain to these roots and present ServerID; host names are not checked.
	CAPEM []byte
	// ServerID is the SPIFFE ID the server must present (required with CAPEM).
	ServerID identity.SPIFFEID
	// Insecure skips server verification (development only).
	Insecure bool
	// Now overrides the clock (tests).
	Now func() time.Time
}

// EnrollClientConfig builds the client TLS configuration for the first
// enrollment:
//
//   - CAPEM set: TLS 1.3, chain to CAPEM roots, exactly one SPIFFE URI SAN
//     equal to ServerID, server-auth key usage, validity within the default
//     skew tolerance; checked on every connection (VerifyConnection);
//   - Insecure: no server verification (TLS 1.2+);
//   - neither: standard verification against the system roots and the host
//     name of the URL (TLS 1.2+).
func EnrollClientConfig(o EnrollOptions) (*tls.Config, error) {
	switch {
	case o.Insecure && (len(o.CAPEM) > 0 || !o.ServerID.IsZero()):
		return nil, errors.New("tlsconf: enroll: insecure excludes a CA bundle and a server identity")
	case o.Insecure:
		return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, nil // #nosec G402 -- explicit development opt-out, refused in production by config.EnrollTLS
	case len(o.CAPEM) == 0 && !o.ServerID.IsZero():
		return nil, errors.New("tlsconf: enroll: a server identity requires the mesh CA bundle")
	case len(o.CAPEM) == 0:
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil
	case o.ServerID.IsZero():
		return nil, errors.New("tlsconf: enroll: the expected server identity is required with a CA bundle")
	}
	roots, err := parseRoots(o.CAPEM)
	if err != nil {
		return nil, err
	}
	vo := Options{TrustDomain: o.ServerID.TrustDomain(), SkewTolerance: defaultSkew, Now: o.Now}
	if vo.Now == nil {
		vo.Now = time.Now
	}
	expected := o.ServerID
	rootsFor := func(string) ([]*x509.Certificate, error) { return roots, nil }
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// The SVID carries no DNS name: host name verification is replaced by
		// the chain + SPIFFE ID check below, which runs on every connection.
		InsecureSkipVerify: true, // #nosec G402 -- replaced by VerifyConnection (mesh roots + SPIFFE ID)
		VerifyConnection: func(cs tls.ConnectionState) error {
			raw := make([][]byte, 0, len(cs.PeerCertificates))
			for _, c := range cs.PeerCertificates {
				raw = append(raw, c.Raw)
			}
			return verifyWith(rootsFor, vo, &expected, x509.ExtKeyUsageServerAuth, raw)
		},
	}, nil
}

// LoadEnrollClientConfig validates e (outside production; production refusal
// is the service's Validate), reads e.CAFile when set and returns
// EnrollClientConfig for it.
func LoadEnrollClientConfig(e config.EnrollTLS, trustDomain string) (*tls.Config, error) {
	if err := e.Validate(trustDomain, false); err != nil {
		return nil, err
	}
	o := EnrollOptions{Insecure: e.Insecure}
	if e.CAFile != "" {
		var err error
		if o.CAPEM, err = os.ReadFile(e.CAFile); err != nil {
			return nil, fmt.Errorf("tlsconf: enroll.ca_file: %w", err)
		}
		// Validated above; a zero ID would still fail closed in EnrollClientConfig.
		o.ServerID, _ = e.ServerID(trustDomain)
	}
	return EnrollClientConfig(o)
}

func parseRoots(b []byte) ([]*x509.Certificate, error) {
	var roots []*x509.Certificate
	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)
		if blk == nil {
			break
		}
		if blk.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(blk.Bytes)
		if err != nil {
			return nil, fmt.Errorf("tlsconf: enroll CA bundle: %w", err)
		}
		roots = append(roots, c)
	}
	if len(roots) == 0 {
		return nil, errors.New("tlsconf: enroll CA bundle holds no certificates")
	}
	return roots, nil
}
