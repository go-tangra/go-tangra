package edge

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-freya/freya/audit"
	"github.com/go-freya/freya/internal/testutil"
	"github.com/go-freya/freya/observe"
	"github.com/go-freya/freya/transport"
)

// certLoader serves the public certificate and reloads it from disk.
type certLoader struct {
	certFile, keyFile string
	cur               atomic.Pointer[tls.Certificate]
	hash              [32]byte
	mu                sync.Mutex
	stopCh            chan struct{}
	once              sync.Once
	stopOnce          sync.Once
	rt                transport.Runtime
}

func newCertLoader(cfg Config, rt transport.Runtime) (*certLoader, error) {
	l := &certLoader{certFile: cfg.CertFile, keyFile: cfg.KeyFile, stopCh: make(chan struct{}), rt: rt}
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		if strings.EqualFold(cfg.Env, "production") {
			return nil, errors.New("edge: a public certificate (cert_file/key_file) is required in production")
		}
		ca, err := testutil.NewCA(rt.TrustDomain())
		if err != nil {
			return nil, err
		}
		crt, err := ca.Issue("edge-dev", testutil.IssueOptions{NotAfter: time.Now().Add(24 * time.Hour)})
		if err != nil {
			return nil, err
		}
		l.cur.Store(&crt)
		rt.Logger().Warn("edge listener using a generated self-signed certificate; development only")
		_ = rt.Audit().Emit(context.Background(), audit.Event{
			Type: audit.TypeInsecureModeEnabled, Outcome: audit.OutcomeOK, Reason: audit.ReasonLocalDev,
			LocalID: rt.LocalID(), CorrelationID: observe.NewCorrelationID(), Attrs: map[string]string{"component": "edge_certificate"},
		})
		return l, nil
	}
	if err := l.reload(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *certLoader) reload() error {
	certPEM, err := os.ReadFile(l.certFile) // #nosec G304 -- operator-supplied path
	if err != nil {
		return fmt.Errorf("edge: cert: %w", err)
	}
	keyPEM, err := os.ReadFile(l.keyFile) // #nosec G304 -- operator-supplied path
	if err != nil {
		return fmt.Errorf("edge: key: %w", err)
	}
	sum := sha256.Sum256(append(append([]byte{}, certPEM...), keyPEM...))
	l.mu.Lock()
	defer l.mu.Unlock()
	if sum == l.hash && l.cur.Load() != nil {
		return nil
	}
	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("edge: certificate: %w", err)
	}
	l.cur.Store(&crt)
	l.hash = sum
	return nil
}

func (l *certLoader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	c := l.cur.Load()
	if c == nil {
		return nil, errors.New("edge: no certificate")
	}
	return c, nil
}

func (l *certLoader) start(interval time.Duration) {
	if l.certFile == "" {
		return
	}
	l.once.Do(func() {
		go func() {
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-l.stopCh:
					return
				case <-t.C:
					if err := l.reload(); err != nil {
						l.rt.Logger().Error("edge certificate reload failed; keeping the current certificate", "err", err.Error())
					}
				}
			}
		}()
	})
}

func (l *certLoader) stop() { l.stopOnce.Do(func() { close(l.stopCh) }) }
