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

## Enrollment TLS (`config.EnrollTLS`)

A service that obtains its SVID by network enrollment (the lcm SDK's
`lcmidentity.NewNet`) embeds `config.EnrollTLS` inline in its `enroll` /
`mesh_enroll` block. It governs only the **first** enrollment, a
server-auth-only HTTPS call authenticated by the join token; renewals run over
mTLS and are always SPIFFE-verified. Build the client configuration with
`tlsconf.LoadEnrollClientConfig(e, trustDomain)` and call
`e.Validate(trustDomain, cfg.IsProduction())` from the service's `Validate`.

| Key | Default | Notes |
|-----|---------|-------|
| `enroll.ca_file` | `""` | PEM mesh trust bundle. Set: the server must chain to these roots and present `server_spiffe_id` (one SPIFFE URI SAN; no host name check; TLS 1.3). Unset: system roots + host name of the enroll URL (TLS 1.2+) |
| `enroll.server_spiffe_id` | `spiffe://<trust_domain>/svc/lcm` | expected server identity; requires `ca_file`; must be in `trust_domain` |
| `enroll.insecure` | `false` | no server verification; excludes `ca_file`; warns; **refused in production** |

Which mode:

- enrolling **through the gateway edge** (`https://<public host>:8443/api/lcm/v1/enroll`):
  leave `ca_file` empty; the edge presents a publicly verifiable certificate;
- enrolling **directly at lcm's keyless listener** (`https://lcm:9947`, the
  gateway, which cannot enroll through itself): `ca_file` = the mesh root
  bundle (for example `/certs/ca.pem` written by `lcmsvc bootstrap`). lcm
  presents its own SVID, which has no DNS name and no public root, so the
  public mode can never verify it.

## Edge frame sources (`edge.Config.FrameSources`)

The edge's Content-Security-Policy frames nothing but its own origin
(`default-src 'self'`). `FrameSources` lists extra origins the served pages
may embed in an iframe, emitted as `frame-src 'self' <origins>`:

```go
edge.Config{ /* ... */ FrameSources: []string{"https://portal.example.com:8444"} }
```

- each entry must be exactly an https origin (`https://host[:port]`, no path,
  query, fragment or user info, no quotes, `;`, `,` or whitespace);
  `NewServer` refuses anything else;
- empty (the default) keeps the policy unchanged;
- nothing else changes: the edge's own pages still refuse to be framed
  (`frame-ancestors 'none'`, `X-Frame-Options: DENY`).

A framed origin runs its own scripts in the page; list only origins you
operate and keep them out of `AllowedOrigins`, so they never pass the CSRF
origin check. The gateway uses this for its KVM console listener
(portal feature 025, `edge.frame_sources` and `console.public_origin`).

## Edge connect sources (`edge.Config.ConnectSources`)

Scripts of the served pages may connect only to the edge's own origin
(`connect-src 'self'`). `ConnectSources` lists extra origins they may reach
with fetch, XHR or WebSocket, emitted as `connect-src 'self' <origins>`:

```go
edge.Config{ /* ... */ ConnectSources: []string{"https://localhost:53952"} }
```

- entries are validated exactly like frame sources (`https://host[:port]`);
- empty (the default) keeps the policy byte-for-byte unchanged;
- only connections are allowed: no script, frame or image source is added.

The portal uses this for the local B-Trust BISS application of qualified
signatures (signing feature 027, `edge.connect_sources`). The CSP is global,
so the allowance applies to every page of the edge; list only the local
application's origins and keep them out of `AllowedOrigins`.

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
