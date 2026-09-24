package http

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/go-tangra/go-tangra/v4/transport"
)

// tlsListener performs the TLS handshake eagerly and concurrently on accept, so
// that refused handshakes are audited with the remote address and a slow peer
// cannot stall the accept loop. Conns returned to the HTTP server are ready
// *tls.Conn values.
type tlsListener struct {
	net.Listener
	cfg     *tls.Config
	timeout time.Duration
	rt      transport.Runtime
	once    sync.Once
	conns   chan net.Conn
	done    chan struct{}
	err     error
	errMu   sync.Mutex
}

// NewTLSListener wraps inner with eager, concurrent TLS handshakes; refused
// handshakes are audited with the remote address (used by transport/edge too).
func NewTLSListener(inner net.Listener, cfg *tls.Config, timeout time.Duration, rt transport.Runtime) net.Listener {
	return newTLSListener(inner, cfg, timeout, rt)
}

func newTLSListener(inner net.Listener, cfg *tls.Config, timeout time.Duration, rt transport.Runtime) *tlsListener {
	return &tlsListener{Listener: inner, cfg: cfg, timeout: timeout, rt: rt, conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *tlsListener) run() {
	for {
		raw, err := l.Listener.Accept()
		if err != nil {
			l.errMu.Lock()
			l.err = err
			l.errMu.Unlock()
			close(l.done)
			return
		}
		go l.handshake(raw)
	}
}

func (l *tlsListener) handshake(raw net.Conn) {
	tc := tls.Server(raw, l.cfg)
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	err := tc.HandshakeContext(ctx)
	cancel()
	if err != nil {
		transport.AuditHandshakeRefusal(l.rt, err, raw.RemoteAddr().String())
		_ = tc.Close()
		return
	}
	select {
	case l.conns <- tc:
	case <-l.done:
		_ = tc.Close()
	}
}

// Accept returns the next successfully handshaken connection.
func (l *tlsListener) Accept() (net.Conn, error) {
	l.once.Do(func() { go l.run() })
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		l.errMu.Lock()
		defer l.errMu.Unlock()
		return nil, l.err
	}
}
