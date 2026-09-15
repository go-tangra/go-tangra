package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-freya/freya/identity"
)

// Identity provider names settable through configuration. "localdev" is
// intentionally absent: it is reachable only via freya.WithInsecureLocalDev().
const (
	ProviderSPIFFE = "spiffe"
	ProviderFile   = "file"
	// ProviderProvided means freya.WithIdentityProvider/WithInsecureLocalDev supplied it.
	ProviderProvided = "provided"
)

// Authorization policy sources settable through configuration. "allow-all" is
// intentionally absent: it is reachable only via freya.WithAllowAllPolicy().
const (
	AuthzFile   = "file"
	AuthzValkey = "valkey"
	// AuthzProvided means freya.WithPolicySource/WithAllowAllPolicy supplied it.
	AuthzProvided = "provided"
)

var envRE = regexp.MustCompile(`^[A-Za-z0-9-]*$`)

// Config is the complete, typed Freya configuration. Validate MUST pass before
// an App is built; there is no partial or lazy configuration.
type Config struct {
	// ServiceName is the logical name; it must equal the identity's service name.
	ServiceName string `json:"service_name" yaml:"service_name"`
	// TrustDomain is the SPIFFE trust domain every peer must belong to.
	TrustDomain string `json:"trust_domain" yaml:"trust_domain"`
	// Env is a free-form environment label; "production" refuses insecure options.
	Env string `json:"env" yaml:"env"`

	Identity  Identity  `json:"identity" yaml:"identity"`
	Authz     Authz     `json:"authz" yaml:"authz"`
	Limits    Limits    `json:"limits" yaml:"limits"`
	Admin     Admin     `json:"admin" yaml:"admin"`
	Discovery Discovery `json:"discovery" yaml:"discovery"`
	Server    Server    `json:"server" yaml:"server"`
}

// Identity configures the identity provider.
type Identity struct {
	Provider       string        `json:"provider" yaml:"provider"`
	WorkloadSocket string        `json:"workload_socket" yaml:"workload_socket"`
	File           FileIdentity  `json:"file" yaml:"file"`
	RenewAt        float64       `json:"renew_at" yaml:"renew_at"`
	SkewTolerance  time.Duration `json:"skew_tolerance" yaml:"skew_tolerance"`
	MaxLifetime    time.Duration `json:"max_lifetime" yaml:"max_lifetime"`
	// StartupTimeout bounds the wait for the first identity from the provider.
	StartupTimeout time.Duration `json:"startup_timeout" yaml:"startup_timeout"`
}

// FileIdentity points at PEM files for the file provider.
type FileIdentity struct {
	Cert   string `json:"cert" yaml:"cert"`
	Key    string `json:"key" yaml:"key"`
	Bundle string `json:"bundle" yaml:"bundle"`
}

// Authz configures the policy source.
type Authz struct {
	Source        string `json:"source" yaml:"source"`
	Path          string `json:"path" yaml:"path"`
	ValkeyKey     string `json:"valkey_key" yaml:"valkey_key"`
	SampleAllowed bool   `json:"sample_allowed" yaml:"sample_allowed"`
}

// Limits are the resource-protection defaults from the constitution.
type Limits struct {
	MaxRequestBytes      int64         `json:"max_request_bytes" yaml:"max_request_bytes"`
	MaxHeaderBytes       int           `json:"max_header_bytes" yaml:"max_header_bytes"`
	RequestTimeout       time.Duration `json:"request_timeout" yaml:"request_timeout"`
	IdleTimeout          time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	HandshakeTimeout     time.Duration `json:"handshake_timeout" yaml:"handshake_timeout"`
	MaxConcurrentStreams uint32        `json:"max_concurrent_streams" yaml:"max_concurrent_streams"`
	// MaxConnectionAge forces long-lived connections to re-handshake (and so
	// re-verify identities) at least this often.
	MaxConnectionAge time.Duration `json:"max_connection_age" yaml:"max_connection_age"`
}

// Admin configures the separate, non-public operations listener.
type Admin struct {
	Addr             string `json:"addr" yaml:"addr"`
	AllowNonLoopback bool   `json:"allow_non_loopback" yaml:"allow_non_loopback"`
	EnablePprof      bool   `json:"enable_pprof" yaml:"enable_pprof"`
}

// Discovery maps logical service names to endpoints when no registry is used.
type Discovery struct {
	Static map[string][]string `json:"static" yaml:"static"`
}

// Server configures the public listeners. An empty address disables that transport.
type Server struct {
	GRPCAddr string `json:"grpc_addr" yaml:"grpc_addr"`
	HTTPAddr string `json:"http_addr" yaml:"http_addr"`
}

// Default returns the secure defaults. Callers set ServiceName, TrustDomain and
// provider-specific paths on top of it.
func Default() Config {
	return Config{
		Identity: Identity{
			Provider:       ProviderSPIFFE,
			WorkloadSocket: "unix:///run/spire/sockets/agent.sock",
			RenewAt:        0.5,
			SkewTolerance:  5 * time.Minute,
			MaxLifetime:    time.Hour,
			StartupTimeout: 30 * time.Second,
		},
		Authz: Authz{Source: AuthzFile},
		Limits: Limits{
			MaxRequestBytes:      1 << 20,
			MaxHeaderBytes:       8 << 10,
			RequestTimeout:       30 * time.Second,
			IdleTimeout:          60 * time.Second,
			HandshakeTimeout:     10 * time.Second,
			MaxConcurrentStreams: 100,
			MaxConnectionAge:     30 * time.Minute,
		},
		Admin:  Admin{Addr: "127.0.0.1:9090"},
		Server: Server{GRPCAddr: ":9443"},
	}
}

// Validate checks every field. It returns the first problem found; messages name
// the offending field in snake_case so they are greppable.
func (c Config) Validate() error {
	if !identity.ValidServiceName(c.ServiceName) {
		return errors.New("config: service_name must match ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$")
	}
	if !identity.ValidTrustDomain(c.TrustDomain) {
		return errors.New("config: trust_domain must be a lower-case DNS-like name")
	}
	if !envRE.MatchString(c.Env) {
		return errors.New("config: env may contain only letters, digits and '-'")
	}
	if err := c.Identity.validate(); err != nil {
		return err
	}
	if err := c.Authz.validate(); err != nil {
		return err
	}
	if err := c.Limits.validate(); err != nil {
		return err
	}
	return c.Admin.validate()
}

func (i Identity) validate() error {
	switch i.Provider {
	case ProviderSPIFFE:
		if i.WorkloadSocket == "" {
			return errors.New("config: identity.workload_socket is required for the spiffe provider")
		}
	case ProviderFile:
		if i.File.Cert == "" || i.File.Key == "" || i.File.Bundle == "" {
			return errors.New("config: identity.file.cert, identity.file.key and identity.file.bundle are required for the file provider")
		}
	case ProviderProvided:
	case "localdev":
		return errors.New("config: identity.provider=localdev cannot be set from configuration; use freya.WithInsecureLocalDev()")
	default:
		return fmt.Errorf("config: identity.provider %q is not one of spiffe, file", i.Provider)
	}
	if i.RenewAt < 0.3 || i.RenewAt > 0.8 {
		return errors.New("config: identity.renew_at must be between 0.3 and 0.8")
	}
	if i.SkewTolerance < 0 || i.SkewTolerance > 15*time.Minute {
		return errors.New("config: identity.skew_tolerance must be between 0 and 15m")
	}
	if i.MaxLifetime <= 0 || i.MaxLifetime > 24*time.Hour {
		return errors.New("config: identity.max_lifetime must be between 1s and 24h")
	}
	if i.StartupTimeout <= 0 {
		return errors.New("config: identity.startup_timeout must be positive")
	}
	return nil
}

func (a Authz) validate() error {
	switch a.Source {
	case AuthzFile:
		if a.Path == "" {
			return errors.New("config: authz.path is required for the file source")
		}
	case AuthzValkey:
		if a.ValkeyKey == "" {
			return errors.New("config: authz.valkey_key is required for the valkey source")
		}
	case AuthzProvided:
	case "allow-all":
		return errors.New("config: authz.source=allow-all cannot be set from configuration; use freya.WithAllowAllPolicy()")
	default:
		return fmt.Errorf("config: authz.source %q is not one of file, valkey", a.Source)
	}
	return nil
}

func (l Limits) validate() error {
	if l.MaxRequestBytes <= 0 || l.MaxHeaderBytes <= 0 || l.RequestTimeout <= 0 ||
		l.IdleTimeout <= 0 || l.HandshakeTimeout <= 0 || l.MaxConcurrentStreams == 0 || l.MaxConnectionAge <= 0 {
		return errors.New("config: limits must all be positive")
	}
	return nil
}

func (a Admin) validate() error {
	host, port, err := net.SplitHostPort(a.Addr)
	if err != nil {
		return fmt.Errorf("config: admin.addr must be host:port: %w", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("config: admin.addr port invalid: %w", err)
	}
	if !a.AllowNonLoopback && !isLoopback(host) {
		return errors.New("config: admin.addr is not loopback; set admin.allow_non_loopback=true to accept the risk")
	}
	return nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Warnings lists deviations from the secure defaults that an operator accepted.
// They are logged at startup (Constitution Principle I).
func (c Config) Warnings() []string {
	d := Default()
	var w []string
	if c.Limits.MaxRequestBytes > d.Limits.MaxRequestBytes {
		w = append(w, fmt.Sprintf("limits.max_request_bytes raised to %d (default %d)", c.Limits.MaxRequestBytes, d.Limits.MaxRequestBytes))
	}
	if c.Limits.MaxHeaderBytes > d.Limits.MaxHeaderBytes {
		w = append(w, fmt.Sprintf("limits.max_header_bytes raised to %d (default %d)", c.Limits.MaxHeaderBytes, d.Limits.MaxHeaderBytes))
	}
	if c.Limits.RequestTimeout > d.Limits.RequestTimeout {
		w = append(w, fmt.Sprintf("limits.request_timeout raised to %s (default %s)", c.Limits.RequestTimeout, d.Limits.RequestTimeout))
	}
	if c.Limits.IdleTimeout > d.Limits.IdleTimeout {
		w = append(w, fmt.Sprintf("limits.idle_timeout raised to %s (default %s)", c.Limits.IdleTimeout, d.Limits.IdleTimeout))
	}
	if c.Limits.MaxConcurrentStreams > d.Limits.MaxConcurrentStreams {
		w = append(w, fmt.Sprintf("limits.max_concurrent_streams raised to %d (default %d)", c.Limits.MaxConcurrentStreams, d.Limits.MaxConcurrentStreams))
	}
	if c.Admin.AllowNonLoopback {
		w = append(w, "admin listener allowed on a non-loopback address; health/metrics are reachable from the network")
	}
	if c.Admin.EnablePprof {
		w = append(w, "pprof enabled on the admin listener")
	}
	if c.Identity.SkewTolerance > d.Identity.SkewTolerance {
		w = append(w, fmt.Sprintf("identity.skew_tolerance raised to %s (default %s)", c.Identity.SkewTolerance, d.Identity.SkewTolerance))
	}
	return w
}

// IsProduction reports whether Env is "production" (case-insensitive).
func (c Config) IsProduction() bool { return strings.EqualFold(c.Env, "production") }

// LocalSPIFFEID returns the identity this service must present.
func (c Config) LocalSPIFFEID() string {
	return "spiffe://" + c.TrustDomain + "/svc/" + c.ServiceName
}
