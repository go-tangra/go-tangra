package config

import (
	"strings"
	"testing"
	"time"
)

func valid() Config {
	c := Default()
	c.ServiceName = "orders"
	c.TrustDomain = "example.org"
	c.Identity.Provider = ProviderFile
	c.Identity.File = FileIdentity{Cert: "c.pem", Key: "k.pem", Bundle: "b.pem"}
	c.Authz.Source = AuthzFile
	c.Authz.Path = "policy.yaml"
	return c
}

func TestDefaultsMatchConstitution(t *testing.T) {
	c := Default()
	if c.Limits.MaxRequestBytes != 1<<20 || c.Limits.MaxHeaderBytes != 8<<10 ||
		c.Limits.RequestTimeout != 30*time.Second || c.Limits.IdleTimeout != 60*time.Second ||
		c.Limits.MaxConcurrentStreams != 100 || c.Limits.HandshakeTimeout != 10*time.Second {
		t.Fatalf("unexpected default limits: %+v", c.Limits)
	}
	if c.Identity.RenewAt != 0.5 || c.Identity.SkewTolerance != 5*time.Minute || c.Identity.MaxLifetime != time.Hour {
		t.Fatalf("unexpected identity defaults: %+v", c.Identity)
	}
	if c.Admin.Addr != "127.0.0.1:9090" || c.Admin.EnablePprof || c.Admin.AllowNonLoopback {
		t.Fatalf("unexpected admin defaults: %+v", c.Admin)
	}
	if c.Identity.Provider != ProviderSPIFFE || c.Identity.WorkloadSocket != "unix:///run/spire/sockets/agent.sock" {
		t.Fatalf("unexpected provider defaults: %+v", c.Identity)
	}
}

func TestValidateOK(t *testing.T) {
	c := valid()
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"missing service name", func(c *Config) { c.ServiceName = "" }, "service_name"},
		{"bad service name", func(c *Config) { c.ServiceName = "Orders_1" }, "service_name"},
		{"missing trust domain", func(c *Config) { c.TrustDomain = "" }, "trust_domain"},
		{"bad trust domain", func(c *Config) { c.TrustDomain = "Example.ORG/x" }, "trust_domain"},
		{"renew too low", func(c *Config) { c.Identity.RenewAt = 0.2 }, "renew_at"},
		{"renew too high", func(c *Config) { c.Identity.RenewAt = 0.9 }, "renew_at"},
		{"skew too large", func(c *Config) { c.Identity.SkewTolerance = 16 * time.Minute }, "skew_tolerance"},
		{"skew negative", func(c *Config) { c.Identity.SkewTolerance = -time.Second }, "skew_tolerance"},
		{"lifetime too long", func(c *Config) { c.Identity.MaxLifetime = 25 * time.Hour }, "max_lifetime"},
		{"localdev via config", func(c *Config) { c.Identity.Provider = "localdev" }, "WithInsecureLocalDev"},
		{"unknown provider", func(c *Config) { c.Identity.Provider = "vault" }, "identity.provider"},
		{"file provider missing paths", func(c *Config) { c.Identity.File.Key = "" }, "identity.file"},
		{"spiffe missing socket", func(c *Config) { c.Identity.Provider = ProviderSPIFFE; c.Identity.WorkloadSocket = "" }, "workload_socket"},
		{"allow-all via config", func(c *Config) { c.Authz.Source = "allow-all" }, "WithAllowAllPolicy"},
		{"unknown authz source", func(c *Config) { c.Authz.Source = "opa" }, "authz.source"},
		{"authz file missing path", func(c *Config) { c.Authz.Path = "" }, "authz.path"},
		{"authz valkey missing key", func(c *Config) { c.Authz.Source = AuthzValkey; c.Authz.ValkeyKey = "" }, "authz.valkey_key"},
		{"admin non-loopback without opt-in", func(c *Config) { c.Admin.Addr = "0.0.0.0:9090" }, "allow_non_loopback"},
		{"admin bad addr", func(c *Config) { c.Admin.Addr = "nonsense" }, "admin.addr"},
		{"zero request bytes", func(c *Config) { c.Limits.MaxRequestBytes = 0 }, "limits"},
		{"zero timeout", func(c *Config) { c.Limits.RequestTimeout = 0 }, "limits"},
		{"bad env", func(c *Config) { c.Env = "staging!" }, "env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestWarnings(t *testing.T) {
	c := valid()
	c.Limits.MaxRequestBytes = 4 << 20
	c.Limits.RequestTimeout = time.Minute
	c.Admin.Addr = "0.0.0.0:9090"
	c.Admin.AllowNonLoopback = true
	c.Admin.EnablePprof = true
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	w := strings.Join(c.Warnings(), "\n")
	for _, want := range []string{"max_request_bytes", "request_timeout", "non-loopback", "pprof"} {
		if !strings.Contains(w, want) {
			t.Errorf("warnings %q missing %q", w, want)
		}
	}
}

func TestIsProduction(t *testing.T) {
	c := valid()
	if c.IsProduction() {
		t.Fatal("empty env must not be production")
	}
	c.Env = "production"
	if !c.IsProduction() {
		t.Fatal("expected production")
	}
	c.Env = "PRODUCTION"
	if !c.IsProduction() {
		t.Fatal("env comparison must be case-insensitive")
	}
}

func TestSPIFFEIDForService(t *testing.T) {
	c := valid()
	if got := c.LocalSPIFFEID(); got != "spiffe://example.org/svc/orders" {
		t.Fatalf("got %q", got)
	}
}
