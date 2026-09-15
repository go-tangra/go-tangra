package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// CA is an in-memory test certificate authority for one trust domain.
type CA struct {
	TrustDomain string
	Cert        *x509.Certificate
	key         *ecdsa.PrivateKey
	serial      atomic.Int64
}

// NewCA creates a CA whose issued SVIDs live in trustDomain.
func NewCA(trustDomain string) (*CA, error) {
	return NewCAWithValidity(trustDomain, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
}

// NewCAWithValidity creates a CA with an explicit validity window (negative tests).
func NewCAWithValidity(trustDomain string, notBefore, notAfter time.Time) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "freya-test-ca " + trustDomain},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	ca := &CA{TrustDomain: trustDomain, Cert: cert, key: key}
	ca.serial.Store(1)
	return ca, nil
}

// MustCA is NewCA that panics.
func MustCA(trustDomain string) *CA {
	ca, err := NewCA(trustDomain)
	if err != nil {
		panic(err)
	}
	return ca
}

// IssueOptions tune an issued leaf for negative tests.
type IssueOptions struct {
	NotBefore, NotAfter time.Time
	// URIs overrides the SAN list entirely (nil = single canonical SPIFFE ID).
	URIs []*url.URL
	// NoSAN issues a leaf with no URI SAN at all.
	NoSAN bool
	// TrustDomain overrides the trust domain in the SPIFFE ID (wrong-domain tests).
	TrustDomain string
}

// Issue creates a leaf for service name with an SPIFFE ID SAN, signed by the CA.
func (c *CA) Issue(name string, opts IssueOptions) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	td := c.TrustDomain
	if opts.TrustDomain != "" {
		td = opts.TrustDomain
	}
	nb, na := opts.NotBefore, opts.NotAfter
	if nb.IsZero() {
		nb = time.Now().Add(-time.Minute)
	}
	if na.IsZero() {
		na = time.Now().Add(time.Hour)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(c.serial.Add(1)),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    nb,
		NotAfter:     na,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	switch {
	case opts.NoSAN:
	case opts.URIs != nil:
		tmpl.URIs = opts.URIs
	default:
		tmpl.URIs = []*url.URL{{Scheme: "spiffe", Host: td, Path: "/svc/" + name}}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.Cert, &key.PublicKey, c.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, nil
}

// MustIssue is Issue that panics.
func (c *CA) MustIssue(name string, opts IssueOptions) tls.Certificate {
	crt, err := c.Issue(name, opts)
	if err != nil {
		panic(err)
	}
	return crt
}

// Pool returns a cert pool containing only this CA.
func (c *CA) Pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.Cert)
	return p
}

// BundlePEM returns the CA certificate as PEM.
func (c *CA) BundlePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Cert.Raw})
}

// CertPEM and KeyPEM encode a leaf for file-based providers.
func CertPEM(crt tls.Certificate) []byte {
	var out []byte
	for _, der := range crt.Certificate {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	return out
}

// KeyPEM encodes the private key as PKCS#8 PEM.
func KeyPEM(crt tls.Certificate) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(crt.PrivateKey)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// WriteSVID writes <dir>/<name>.pem, <dir>/<name>.key and <dir>/ca.pem and returns the three paths.
func (c *CA) WriteSVID(dir, name string, crt tls.Certificate) (certPath, keyPath, bundlePath string, err error) {
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", "", "", err
	}
	certPath = filepath.Join(dir, name+".pem")
	keyPath = filepath.Join(dir, name+".key")
	bundlePath = filepath.Join(dir, "ca.pem")
	if err = os.WriteFile(certPath, CertPEM(crt), 0o600); err != nil {
		return "", "", "", err
	}
	if err = os.WriteFile(keyPath, KeyPEM(crt), 0o600); err != nil {
		return "", "", "", err
	}
	if err = os.WriteFile(bundlePath, c.BundlePEM(), 0o600); err != nil {
		return "", "", "", err
	}
	return certPath, keyPath, bundlePath, nil
}

// KeyFingerprint returns a hex-ish marker of the leaf's private key that must never appear in logs.
func KeyFingerprint(crt tls.Certificate) string {
	k, ok := crt.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%x", k.D.Bytes())
}
