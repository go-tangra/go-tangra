package tlsconf

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/internal/cred"
)

// Options tune verification. TrustDomain is required.
type Options struct {
	TrustDomain string
	// SkewTolerance widens validity checks by this much on both ends (default 5m, max 15m).
	SkewTolerance time.Duration
	// Now overrides the clock (tests).
	Now func() time.Time
	// OnRefusal, if set, is called with the classified reason whenever this
	// config's verifier refuses a peer. Used to audit handshake failures.
	OnRefusal func(reason identity.Reason, claimed string)
}

// Peer is the verified remote identity extracted from a leaf certificate.
type Peer struct {
	ID          identity.SPIFFEID
	ServiceName string
	Serial      string
	NotAfter    time.Time
}

const (
	defaultSkew = 5 * time.Minute
	maxSkew     = 15 * time.Minute
)

func (o *Options) normalise(p identity.Provider) (cred.Source, error) {
	if p == nil {
		return nil, errors.New("tlsconf: identity provider is required")
	}
	src, ok := p.(cred.Source)
	if !ok {
		return nil, errors.New("tlsconf: identity provider does not expose credentials to the transport")
	}
	if !identity.ValidTrustDomain(o.TrustDomain) {
		return nil, errors.New("tlsconf: trust domain is required")
	}
	if o.SkewTolerance == 0 {
		o.SkewTolerance = defaultSkew
	}
	if o.SkewTolerance < 0 || o.SkewTolerance > maxSkew {
		return nil, fmt.Errorf("tlsconf: skew tolerance must be within 0..%s", maxSkew)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return src, nil
}

func base() *tls.Config {
	return &tls.Config{
		MinVersion:             tls.VersionTLS13,
		MaxVersion:             tls.VersionTLS13,
		NextProtos:             []string{"h2"},
		SessionTicketsDisabled: true,
		ClientSessionCache:     nil,
	}
}

// ServerConfig builds the listener configuration: TLS 1.3 only, client
// certificate required, peer verified against the provider's current bundle,
// server certificate fetched from the provider on every handshake (rotation
// without restart).
func ServerConfig(p identity.Provider, o Options) (*tls.Config, error) {
	src, err := o.normalise(p)
	if err != nil {
		return nil, err
	}
	cfg := base()
	cfg.ClientAuth = tls.RequireAnyClientCert
	cfg.VerifyPeerCertificate = verifier(p, o, nil)
	// Resumed sessions skip VerifyPeerCertificate; VerifyConnection re-runs the
	// same checks on every connection (session tickets are disabled anyway).
	cfg.VerifyConnection = connectionVerifier(p, o, nil)
	cfg.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return credential(src)
	}
	return cfg, nil
}

// ClientConfig builds the dial configuration for calling expected. Hostname
// verification is replaced by SPIFFE ID verification: the callee must present a
// chain to a trusted root whose single URI SAN equals expected.
func ClientConfig(p identity.Provider, expected identity.SPIFFEID, o Options) (*tls.Config, error) {
	src, err := o.normalise(p)
	if err != nil {
		return nil, err
	}
	if expected.IsZero() {
		return nil, errors.New("tlsconf: expected peer identity is required")
	}
	if expected.TrustDomain() != o.TrustDomain {
		return nil, fmt.Errorf("tlsconf: expected peer %s is outside trust domain %s", expected, o.TrustDomain)
	}
	cfg := base()
	// Hostname verification is meaningless for SPIFFE identities; the chain and
	// SAN are verified in VerifyPeerCertificate, which is always set.
	cfg.InsecureSkipVerify = true // #nosec G402 -- replaced by SPIFFE verification below
	cfg.VerifyPeerCertificate = verifier(p, o, &expected)
	cfg.VerifyConnection = connectionVerifier(p, o, &expected)
	cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return credential(src)
	}
	return cfg, nil
}

func credential(src cred.Source) (*tls.Certificate, error) {
	c, err := src.Credential()
	if err != nil {
		return nil, &identity.VerifyError{Reason: identity.ReasonProviderUnavailable}
	}
	if c == nil || len(c.Certificate) == 0 || c.PrivateKey == nil {
		return nil, &identity.VerifyError{Reason: identity.ReasonProviderUnavailable, Detail: "no credential"}
	}
	leaf := c.Leaf
	if leaf == nil {
		if leaf, err = x509.ParseCertificate(c.Certificate[0]); err != nil {
			return nil, &identity.VerifyError{Reason: identity.ReasonProviderUnavailable, Detail: "malformed credential"}
		}
	}
	// Fail closed: an expired local identity is never presented (SR-003).
	now := time.Now()
	if now.After(leaf.NotAfter) {
		return nil, &identity.VerifyError{Reason: identity.ReasonExpired, Detail: "local identity expired"}
	}
	if now.Before(leaf.NotBefore.Add(-maxSkew)) {
		return nil, &identity.VerifyError{Reason: identity.ReasonNotYetValid, Detail: "local identity not yet valid"}
	}
	return c, nil
}

// connectionVerifier applies verify to the connection state so that resumed
// sessions (which bypass VerifyPeerCertificate) are checked identically.
func connectionVerifier(p identity.Provider, o Options, expected *identity.SPIFFEID) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		raw := make([][]byte, 0, len(cs.PeerCertificates))
		for _, c := range cs.PeerCertificates {
			raw = append(raw, c.Raw)
		}
		return verify(p, o, expected, raw)
	}
}

// verifier returns a tls.Config.VerifyPeerCertificate that performs the full
// chain, validity (with skew), SAN and optional name-match verification.
func verifier(p identity.Provider, o Options, expected *identity.SPIFFEID) func([][]byte, [][]*x509.Certificate) error {
	return func(raw [][]byte, _ [][]*x509.Certificate) error {
		err := verify(p, o, expected, raw)
		if err != nil && o.OnRefusal != nil {
			claimed := ""
			var ve *identity.VerifyError
			if errors.As(err, &ve) {
				claimed = ve.Claimed
			}
			o.OnRefusal(Classify(err), claimed)
		}
		return err
	}
}

func verify(p identity.Provider, o Options, expected *identity.SPIFFEID, raw [][]byte) error {
	if len(raw) == 0 {
		return &identity.VerifyError{Reason: identity.ReasonNoIdentity}
	}
	certs := make([]*x509.Certificate, 0, len(raw))
	for _, der := range raw {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "malformed certificate"}
		}
		certs = append(certs, c)
	}
	leaf := certs[0]
	if len(leaf.URIs) != 1 {
		return &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "leaf must carry exactly one URI SAN"}
	}
	id, err := identity.ParseSPIFFEID(leaf.URIs[0].String())
	if err != nil {
		return &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "SAN is not a SPIFFE ID"}
	}
	claimed := id.String()
	if id.TrustDomain() != o.TrustDomain {
		return &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "foreign trust domain", Claimed: claimed}
	}
	_, bundle, err := p.Current(context.Background())
	if err != nil {
		return &identity.VerifyError{Reason: identity.ReasonProviderUnavailable, Claimed: claimed}
	}
	roots := bundle.Roots()
	if len(roots) == 0 {
		return &identity.VerifyError{Reason: identity.ReasonBundleEmpty, Claimed: claimed}
	}
	now := o.Now()
	at := now
	switch {
	case now.Before(leaf.NotBefore):
		if leaf.NotBefore.Sub(now) > o.SkewTolerance {
			return &identity.VerifyError{Reason: identity.ReasonNotYetValid, Detail: "beyond clock skew tolerance " + o.SkewTolerance.String(), Claimed: claimed}
		}
		at = leaf.NotBefore
	case now.After(leaf.NotAfter):
		if now.Sub(leaf.NotAfter) > o.SkewTolerance {
			return &identity.VerifyError{Reason: identity.ReasonExpired, Detail: "beyond clock skew tolerance " + o.SkewTolerance.String(), Claimed: claimed}
		}
		at = leaf.NotAfter
	}
	pool := x509.NewCertPool()
	for _, r := range roots {
		pool.AddCert(r)
	}
	inter := x509.NewCertPool()
	for _, c := range certs[1:] {
		inter.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots: pool, Intermediates: inter, CurrentTime: at,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		var inv x509.CertificateInvalidError
		if errors.As(err, &inv) && inv.Reason == x509.Expired {
			return &identity.VerifyError{Reason: identity.ReasonExpired, Detail: "issuer validity", Claimed: claimed}
		}
		return &identity.VerifyError{Reason: identity.ReasonUntrusted, Claimed: claimed}
	}
	if expected != nil && !id.Equal(*expected) {
		return &identity.VerifyError{Reason: identity.ReasonNameMismatch, Claimed: claimed}
	}
	return nil
}

// PeerFromChain extracts the verified peer from a leaf that already passed
// VerifyPeerCertificate.
func PeerFromChain(leaf *x509.Certificate) (Peer, error) {
	if leaf == nil || len(leaf.URIs) != 1 {
		return Peer{}, &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "no verified leaf"}
	}
	id, err := identity.ParseSPIFFEID(leaf.URIs[0].String())
	if err != nil {
		return Peer{}, &identity.VerifyError{Reason: identity.ReasonUntrusted, Detail: "SAN is not a SPIFFE ID"}
	}
	return Peer{ID: id, ServiceName: id.ServiceName(), Serial: leaf.SerialNumber.String(), NotAfter: leaf.NotAfter}, nil
}

// Classify maps any handshake or verification error to a closed-vocabulary reason.
func Classify(err error) identity.Reason {
	if err == nil {
		return ""
	}
	var ve *identity.VerifyError
	if errors.As(err, &ve) {
		return ve.Reason
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "didn't provide a certificate"), strings.Contains(s, "certificate required"):
		return identity.ReasonNoIdentity
	case strings.Contains(s, "unsupported versions"), strings.Contains(s, "protocol version"):
		return identity.ReasonDowngrade
	case strings.Contains(s, "expired"), strings.Contains(s, "not yet valid"):
		return identity.ReasonExpired
	default:
		return identity.ReasonUntrusted
	}
}
