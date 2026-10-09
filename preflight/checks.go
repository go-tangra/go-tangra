package preflight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// DialTimeout is the default bound of a reachability probe.
const DialTimeout = 3 * time.Second

// FileReadable checks that path names a non-empty regular file this process
// can read. It reads at most one byte and never reports the content.
func FileReadable(name, path string) Check {
	return Check{Name: name, Run: func(context.Context) Result { return fileReadable(path) }}
}

func fileReadable(path string) Result {
	if path == "" {
		return Failf("not configured")
	}
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Failf("%s does not exist", path).
			WithFix("create it or correct the path; in a container the path is inside the container, so check the volume mount")
	case err != nil:
		return Failf("%s: %s", path, errText(err)).WithFix("check the permissions of the parent directories")
	case fi.IsDir():
		return Failf("%s is a directory, not a file", path)
	case !fi.Mode().IsRegular():
		return Failf("%s is not a regular file", path)
	}
	f, err := os.Open(path) // #nosec G304 -- operator-supplied path, only probed
	if err != nil {
		return Failf("%s is not readable by uid %d", path, os.Getuid()).
			WithFix("make it readable by the service user (e.g. mode 0640 and a group the service runs with)")
	}
	defer func() { _ = f.Close() }()
	if n, err := f.Read(make([]byte, 1)); n == 0 {
		if err != nil && !errors.Is(err, io.EOF) {
			return Failf("%s cannot be read: %s", path, errText(err))
		}
		return Failf("%s is empty", path)
	}
	return Passf("%s readable (%d bytes)", path, fi.Size())
}

// DirWritable checks that dir exists and this process can create files in it
// (it creates and removes one probe file). For state that must survive a
// restart, such as the persisted SVID.
func DirWritable(name, dir string) Check {
	return Check{Name: name, Run: func(context.Context) Result {
		if dir == "" {
			return Failf("not configured")
		}
		fi, err := os.Stat(dir)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return Failf("directory %s does not exist", dir).
				WithFix("create it (in a container: mount a persistent volume there)")
		case err != nil:
			return Failf("%s: %s", dir, errText(err))
		case !fi.IsDir():
			return Failf("%s is not a directory", dir)
		}
		f, err := os.CreateTemp(dir, ".preflight-*")
		if err != nil {
			return Failf("directory %s is not writable by uid %d", dir, os.Getuid()).
				WithFix("make it writable by the service user (in a container: a writable volume, not a read-only mount)")
		}
		_ = f.Close()
		_ = os.Remove(f.Name())
		return Passf("directory %s writable", dir)
	}}
}

// KeyPair checks that certFile and keyFile form a usable TLS key pair and
// that the certificate is currently valid; it warns within 14 days of expiry.
// A nil now means time.Now.
func KeyPair(name, certFile, keyFile string, now func() time.Time) Check {
	if now == nil {
		now = time.Now
	}
	return Check{Name: name, Run: func(context.Context) Result {
		for _, p := range []string{certFile, keyFile} {
			if r := fileReadable(p); r.Status != Pass {
				return r
			}
		}
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return Failf("%s / %s: %s", certFile, keyFile, errText(err)).
				WithFix("the files must be PEM, and the key must belong to the certificate")
		}
		leaf := pair.Leaf
		if leaf == nil {
			if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
				return Failf("%s: %s", certFile, errText(err))
			}
		}
		t := now()
		switch {
		case t.After(leaf.NotAfter):
			return Failf("certificate %s expired %s ago (%s)", subject(leaf), human(t.Sub(leaf.NotAfter)), stamp(leaf.NotAfter)).
				WithFix("install a renewed certificate")
		case t.Before(leaf.NotBefore):
			return Failf("certificate %s is not valid before %s", subject(leaf), stamp(leaf.NotBefore)).
				WithFix("check this host's clock (NTP)")
		case leaf.NotAfter.Sub(t) < 14*24*time.Hour:
			return Warnf("certificate %s expires in %s (%s)", subject(leaf), human(leaf.NotAfter.Sub(t)), stamp(leaf.NotAfter)).
				WithFix("renew it soon")
		}
		return Passf("certificate %s valid until %s", subject(leaf), stamp(leaf.NotAfter))
	}}
}

// TCPDial checks that addr (host:port) accepts a TCP connection within
// timeout (DialTimeout when zero). It tells name resolution, refusal and
// silence apart.
func TCPDial(name, addr string, timeout time.Duration) Check {
	if timeout <= 0 {
		timeout = DialTimeout
	}
	return Check{Name: name, Network: true, Run: func(ctx context.Context) Result {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return Failf("%q is not host:port", addr)
		}
		start := time.Now()
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return DialFailure(addr, err, timeout)
		}
		_ = conn.Close()
		return Passf("%s reachable (%s)", addr, time.Since(start).Round(time.Millisecond))
	}}
}

// DialFailure classifies a failed connection to addr (name resolution,
// refusal, timeout, routing) into a FAIL result with a fix hint. Modules use
// it for their own clients (e.g. a database driver's dial error).
func DialFailure(addr string, err error, timeout time.Duration) Result {
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return Failf("%s: cannot resolve host %s", addr, dnsErr.Name).
			WithFix("check the host name, this host's DNS, or add it to /etc/hosts (extra_hosts in compose)")
	case errors.Is(err, syscall.ECONNREFUSED):
		return Failf("%s: connection refused", addr).
			WithFix("nothing listens there: check the port, and that the core publishes it to this host")
	case isTimeout(err):
		return Failf("%s: no answer within %s", addr, timeout).
			WithFix("a firewall drops the traffic or the address is not routed from this host")
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return Failf("%s: host or network unreachable", addr).
			WithFix("check routing between this host and the core")
	}
	return Failf("%s: %s", addr, errText(err))
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

// connError and tlsError mark the phase of an HTTPS probe that failed.
type connError struct{ err error }

func (e connError) Error() string { return e.err.Error() }
func (e connError) Unwrap() error { return e.err }

type tlsError struct{ err error }

func (e tlsError) Error() string { return e.err.Error() }
func (e tlsError) Unwrap() error { return e.err }

// HTTPSProbe sends one GET (no body) to rawURL using cfg, the TLS
// configuration the module itself would use (nil: system roots and the URL's
// host name), and reports connection failures distinctly from TLS failures.
// Any HTTP response passes: the probe checks reachability and trust, not the
// endpoint's semantics. A cfg that verifies nothing yields a warning.
func HTTPSProbe(name, rawURL string, cfg *tls.Config, timeout time.Duration) Check {
	if timeout <= 0 {
		timeout = DialTimeout
	}
	return Check{Name: name, Network: true, Run: func(ctx context.Context) Result {
		u, err := url.Parse(rawURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return Failf("%q is not an https URL", rawURL)
		}
		addr := u.Host
		if u.Port() == "" {
			addr = net.JoinHostPort(u.Hostname(), "443")
		}
		var tc *tls.Config
		if cfg != nil {
			tc = cfg.Clone()
		} else {
			tc = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		if tc.ServerName == "" {
			tc.ServerName = u.Hostname()
		}
		tc.NextProtos = []string{"http/1.1"}
		var (
			peer    []*x509.Certificate
			version uint16
		)
		tr := &http.Transport{
			DialTLSContext: func(ctx context.Context, network, a string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				raw, err := d.DialContext(ctx, network, a)
				if err != nil {
					return nil, connError{err}
				}
				conn := tls.Client(raw, tc)
				hctx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				if err := conn.HandshakeContext(hctx); err != nil {
					_ = raw.Close()
					return nil, tlsError{err}
				}
				cs := conn.ConnectionState()
				peer, version = cs.PeerCertificates, cs.Version
				return conn, nil
			},
		}
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr, Timeout: 2 * timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			return Failf("%s: %s", rawURL, errText(err))
		}
		resp, err := client.Do(req)
		if err != nil {
			var te tlsError
			if errors.As(err, &te) {
				return TLSFailure(addr, te.err)
			}
			return DialFailure(addr, err, timeout)
		}
		_ = resp.Body.Close()
		if tc.InsecureSkipVerify && tc.VerifyConnection == nil && tc.VerifyPeerCertificate == nil {
			return Warnf("%s answered HTTP %d, but its certificate was NOT verified (insecure)", addr, resp.StatusCode).
				WithFix("development only; production needs a verifiable certificate")
		}
		return Passf("%s answered HTTP %d over %s, certificate %s verified", addr, resp.StatusCode, tls.VersionName(version), peerName(peer))
	}}
}

// TLSFailure classifies a failed TLS handshake with addr (unknown
// authority, host name, validity, not TLS, alert, timeout) into a FAIL
// result with a fix hint.
func TLSFailure(addr string, err error) Result {
	var (
		unknown  x509.UnknownAuthorityError
		hostname x509.HostnameError
		invalid  x509.CertificateInvalidError
		record   tls.RecordHeaderError
		alert    tls.AlertError
	)
	switch {
	case errors.As(err, &unknown):
		issuer := "unknown issuer"
		if unknown.Cert != nil {
			issuer = "issuer " + unknown.Cert.Issuer.String()
		}
		return Failf("%s: TLS: certificate signed by an authority this host does not trust (%s)", addr, issuer).
			WithFix("trust the server's CA on this host (system trust store or the module's CA setting), or point the URL at an endpoint with a publicly trusted certificate")
	case errors.As(err, &hostname):
		return Failf("%s: TLS: certificate is not valid for %s (it names %s)", addr, hostname.Host, certNames(hostname.Certificate)).
			WithFix("use a host name in the URL that the certificate covers")
	case errors.As(err, &invalid):
		if invalid.Reason == x509.Expired {
			return Failf("%s: TLS: certificate expired or not yet valid", addr).
				WithFix("renew the server certificate, or check this host's clock (NTP)")
		}
		return Failf("%s: TLS: invalid certificate: %s", addr, errText(invalid))
	case errors.As(err, &record):
		return Failf("%s: TLS: the server did not answer with TLS (plain HTTP on this port?)", addr).
			WithFix("use the TLS port of the endpoint")
	case errors.As(err, &alert):
		return Failf("%s: TLS: the server refused the handshake (%s)", addr, errText(alert)).
			WithFix("the server may require a different TLS version or a client certificate")
	case isTimeout(err):
		return Failf("%s: TLS: handshake timed out", addr).
			WithFix("the port accepts TCP but does not complete TLS: check it is the right port")
	}
	return Failf("%s: TLS: %s", addr, errText(err))
}

func peerName(certs []*x509.Certificate) string {
	if len(certs) == 0 {
		return "(none)"
	}
	return subject(certs[0])
}

// subject names a certificate by its first URI SAN, DNS SAN or common name.
func subject(c *x509.Certificate) string {
	switch {
	case len(c.URIs) > 0:
		return c.URIs[0].String()
	case len(c.DNSNames) > 0:
		return c.DNSNames[0]
	case c.Subject.CommonName != "":
		return c.Subject.CommonName
	}
	return "(no name)"
}

func certNames(c *x509.Certificate) string {
	if c == nil {
		return "nothing"
	}
	var names []string
	names = append(names, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		names = append(names, ip.String())
	}
	for _, u := range c.URIs {
		names = append(names, u.String())
	}
	if len(names) == 0 {
		return "no host names"
	}
	return strings.Join(names, ", ")
}

// errText is err's message on one line.
func errText(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

// human renders a duration for a person: seconds, minutes or hours.
func human(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 72*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") }
