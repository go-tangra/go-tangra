package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-tangra/go-tangra/v4/audit"
	"github.com/go-tangra/go-tangra/v4/identity"
	"github.com/go-tangra/go-tangra/v4/observe"
	"github.com/go-tangra/go-tangra/v4/transport"
	"github.com/go-tangra/go-tangra/v4/transport/tlsconf"
)

// NewClient returns an HTTP client that presents the runtime's SVID and only
// completes connections to the peer whose SPIFFE ID is expected (TLS 1.3,
// chain and SAN verified by the framework, never by hostname). Refused
// handshakes are audited as authn_refused. Timeouts come from the runtime
// limits; the caller may shorten them per request with a context.
func NewClient(rt transport.Runtime, expected identity.SPIFFEID) (*http.Client, error) {
	if expected.IsZero() {
		return nil, errors.New("client: expected peer identity is required")
	}
	if !transport.LocalIdentityValid(rt) {
		return nil, transport.ErrLocalIdentityUnavailable
	}
	opts := transport.TLSOptions(rt)
	opts.OnRefusal = func(reason identity.Reason, claimed string) {
		_ = rt.Audit().Emit(context.Background(), audit.Event{
			Type: audit.TypeAuthnRefused, Outcome: audit.OutcomeRefused, Reason: audit.Reason(reason),
			LocalID: rt.LocalID(), ClaimedPeerID: claimed, Operation: "dial " + expected.String(),
			CorrelationID: observe.NewCorrelationID(),
		})
	}
	tlsCfg, err := tlsconf.ClientConfig(rt.Provider(), expected, opts)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	lim := rt.Limits()
	tr := &http.Transport{
		TLSClientConfig:        tlsCfg,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    lim.HandshakeTimeout,
		IdleConnTimeout:        lim.IdleTimeout,
		ResponseHeaderTimeout:  lim.RequestTimeout,
		MaxResponseHeaderBytes: int64(lim.MaxHeaderBytes),
		DialContext:            (&net.Dialer{Timeout: lim.HandshakeTimeout}).DialContext,
	}
	return &http.Client{Transport: tr, Timeout: 0, CheckRedirect: noRedirect}, nil
}

// noRedirect refuses to follow redirects: a module answer is relayed as-is and
// a redirect could otherwise steer the client to an unverified peer.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// RequestTimeout is the per-request deadline the client should be given when
// the caller has no tighter budget.
func RequestTimeout(rt transport.Runtime) time.Duration { return rt.Limits().RequestTimeout }
