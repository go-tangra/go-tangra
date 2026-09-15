# Configuration reference

Load with `config.Load(path)` (YAML, durations in Go syntax) or build a
`config.Config` in code starting from `config.Default()`. `Validate()` runs in
`freya.New`; the application refuses to start on any error. Overrides of secure
defaults are logged at startup (`config.Warnings`).

| Key | Default | Notes |
|-----|---------|-------|
| `service_name` | — | required; `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`; must equal the identity's service name |
| `trust_domain` | — | required; lower-case DNS-like |
| `env` | `""` | `production` refuses insecure options |
| `identity.provider` | `spiffe` | `spiffe` or `file` (`localdev` only via `freya.WithInsecureLocalDev()`) |
| `identity.workload_socket` | `unix:///run/spire/sockets/agent.sock` | spiffe provider |
| `identity.file.{cert,key,bundle}` | — | PEM paths for the file provider (polled every 2 s) |
| `identity.renew_at` | `0.5` | fraction of the validity window; 0.3–0.8 |
| `identity.skew_tolerance` | `5m` | ≤ 15 m |
| `identity.max_lifetime` | `1h` | ≤ 24 h; also the local-dev SVID lifetime |
| `identity.startup_timeout` | `30s` | wait for the first identity |
| `authz.source` | `file` | `file` or `valkey` (`allow-all` only via `freya.WithAllowAllPolicy()`) |
| `authz.path` | — | policy file (schema: `contracts/policy.schema.json`) |
| `authz.valkey_key` | — | used with `contrib/policy-valkey` |
| `authz.sample_allowed` | `false` | emit `authz_allowed` events |
| `limits.max_request_bytes` | `1048576` | gRPC and HTTP bodies |
| `limits.max_header_bytes` | `8192` | |
| `limits.request_timeout` | `30s` | per-call deadline; server options cannot exceed it |
| `limits.idle_timeout` | `60s` | |
| `limits.handshake_timeout` | `10s` | TLS handshake and HTTP header read |
| `limits.max_concurrent_streams` | `100` | per connection |
| `limits.max_connection_age` | `30m` | forces re-handshake (re-verification) |
| `admin.addr` | `127.0.0.1:9090` | plain HTTP on loopback only; non-loopback needs `admin.allow_non_loopback: true` **and then serves mTLS only** (scrapers present a SPIFFE identity from the same trust domain) |
| `admin.enable_pprof` | `false` | |
| `discovery.static` | — | `name: ["host:port", ...]`; or pass any Kratos `registry.Discovery` via `freya.WithDiscovery` |
| `server.grpc_addr` | `:9443` | |
| `server.http_addr` | `""` | empty disables the HTTP server |

## Programmatic options

| Option | Effect |
|--------|--------|
| `WithIdentityProvider(p)` | custom `identity.Provider` (must be from this module to expose credentials) |
| `WithPolicySource(s)` | custom `authz.Source` (e.g. contrib/policy-valkey) |
| `WithRevocationChecker(c)` | in-lifetime revocation; errors fail closed |
| `WithDiscovery(d)` | any Kratos registry |
| `WithAuditSink(s)` | additional asynchronous sink |
| `WithLogger(h)` | log handler (wrapped by the redacting handler) |
| `WithTracerProvider(tp)` | OpenTelemetry provider for spans |
| `WithInsecureLocalDev()` | self-issued CA; warns; refused in production |
| `WithAllowAllPolicy()` | permit every authenticated caller; warns; refused in production |
