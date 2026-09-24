package observe

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra/v4/config"
)

// Admin is the separate, non-public operations listener: health, readiness,
// metrics, and (opt-in) pprof. On loopback it is plain HTTP; on any other
// address it speaks TLS 1.3 mTLS only, because plaintext listeners are
// prohibited outside localhost (Constitution: Security Requirements).
type Admin struct {
	cfg   config.Admin
	lis   net.Listener
	srv   *http.Server
	log   *slog.Logger
	ready func() bool
	tls   bool
}

// NewAdmin binds the admin listener. metrics may be nil (404). tlsCfg is used
// only when the address is not loopback, where it is REQUIRED (the service's
// mTLS config from transport/tlsconf, so scrapers present a SPIFFE identity).
func NewAdmin(cfg config.Admin, ready func() bool, metrics http.Handler, log *slog.Logger, tlsCfg *tls.Config) (*Admin, error) {
	if err := validateAdminAddr(cfg); err != nil {
		return nil, err
	}
	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	secure := false
	if !isLoopbackAddr(lis.Addr()) {
		if tlsCfg == nil {
			_ = lis.Close()
			return nil, errors.New("observe: a non-loopback admin listener requires the mTLS configuration; plaintext is not allowed")
		}
		lis = tls.NewListener(lis, tlsCfg)
		secure = true
	}
	mux := http.NewServeMux() // never http.DefaultServeMux
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready != nil && ready() {
			_, _ = w.Write([]byte("ready\n"))
			return
		}
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	})
	if metrics != nil {
		mux.Handle("/metrics", metrics)
	}
	if cfg.EnablePprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		if log != nil {
			log.Warn("pprof enabled on the admin listener", "addr", lis.Addr().String())
		}
	}
	if secure && log != nil {
		log.Warn("admin listener bound to a non-loopback address; serving mTLS only", "addr", lis.Addr().String())
	}
	return &Admin{cfg: cfg, lis: lis, log: log, ready: ready, tls: secure, srv: &http.Server{
		Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10,
	}}, nil
}

func isLoopbackAddr(a net.Addr) bool {
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateAdminAddr(cfg config.Admin) error {
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return err
	}
	if cfg.AllowNonLoopback || host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("observe: admin listener is not loopback; set admin.allow_non_loopback to accept the risk")
	}
	return nil
}

// URL returns the base URL of the bound listener (https:// when mTLS is active).
func (a *Admin) URL() string {
	addr := a.lis.Addr().String()
	if strings.HasPrefix(addr, "[::]") || strings.HasPrefix(addr, "0.0.0.0") {
		_, port, _ := net.SplitHostPort(addr)
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	if a.tls {
		return "https://" + addr
	}
	return "http://" + addr
}

// Secure reports whether the listener requires mTLS.
func (a *Admin) Secure() bool { return a.tls }

// Serve blocks until Shutdown.
func (a *Admin) Serve() error {
	err := a.srv.Serve(a.lis)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown stops the listener.
func (a *Admin) Shutdown(ctx context.Context) error { return a.srv.Shutdown(ctx) }
