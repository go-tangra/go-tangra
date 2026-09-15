# Research: Secure Service-to-Service Channel

**Feature**: 001-secure-service-channel | **Date**: 2026-09-15

All `NEEDS CLARIFICATION` items from the Technical Context are resolved below. Each entry
records the decision, the rationale, and the alternatives considered.

## 1. Language and framework base

**Decision**: Go 1.25 with go-kratos **v3.0.0** (`github.com/go-kratos/kratos/v3`).

**Rationale**: User directive ("Use Go… go-kratos as a base"). Verified on 2026-09-15:
v3.0.0 was released 2026-06-26 and its `go.mod` requires `go 1.25.0`; v2.9.2
(2025-12-05) is the last v2. v3 is the maintained line, so starting a new framework on
v2 would incur a migration immediately. Kratos provides exactly the primitives Freya
needs and nothing Freya must fight: `transport/grpc` and `transport/http` servers both
accept `TLSConfig(*tls.Config)`; the gRPC client accepts `WithTLSConfig` and otherwise
falls back to **insecure** credentials (which is why Freya must own the constructors);
middleware is a plain `func(Handler) Handler` chain; `registry.Discovery` abstracts
service discovery; `config` supports file sources with watch (hot reload).

**Alternatives considered**:
- Kratos v2.9.2 — rejected: superseded, same open advisory, forces a migration later.
- go-micro / go-zero — rejected: user asked for Kratos; go-zero's code generation hides
  control flow (Principle VII).
- Plain `net/http` + `google.golang.org/grpc` — viable and smaller, but the user chose
  Kratos and its lifecycle/registry/config abstractions save real work.

## 2. Known Kratos security issue and mitigation

**Finding**: GHSA-jj45-xvq5-rhh9 / CVE-2026-6993 / GO-2026-5471 ("Confused Deputy",
medium). `transport/http.NewServer` sets `http.DefaultServeMux` as the `NotFoundHandler`
and `MethodNotAllowedHandler`; anything registered on the global mux (notably
`net/http/pprof` from any imported package) is served on the public listener. Verified
still present in `v3.0.0` (`transport/http/server.go` lines 186–187). Fix PR #3814 is
open and unmerged; `first_patched_version` is null.

**Decision**: Freya's `transport/http.NewServer` always passes
`khttp.NotFoundHandler(http.NotFoundHandler())` and
`khttp.MethodNotAllowedHandler(<405 handler>)`; a contract test asserts that
`/debug/pprof/` on a Freya HTTP server returns 404 even when `net/http/pprof` is
imported. Freya's `observe/admin` serves pprof only on the admin listener, only when
`Config.Admin.EnablePprof` is true. CI: `govulncheck` runs with an allow-list containing
exactly `GO-2026-5471`, annotated with the mitigation and an expiry date (2026-12-31)
after which the build fails unless renewed — forcing a re-review when upstream ships a
fix.

**Alternatives considered**: forking Kratos (rejected: maintenance burden, Principle VI);
waiting for upstream (rejected: blocks the feature; the mitigation is two options).

## 3. Service identity and credential lifecycle

**Decision**: X.509 certificates with a SPIFFE ID URI SAN
(`spiffe://<trust-domain>/svc/<service-name>`) as the canonical service identity.
`identity.Provider` is a small interface (see contracts/go-api.md) implemented by:
1. `identity/spiffe` — SPIFFE Workload API via `github.com/spiffe/go-spiffe/v2`
   (`workloadapi.X509Source`); rotation and trust-bundle updates are pushed by the
   agent (SPIRE, Istio CA, cert-manager CSI, etc.). **Default in production.**
2. `identity/file` — PEM cert/key/bundle on disk, polled for change (no fsnotify dep);
   for CI and platforms that mount rotated certs as files.
3. `identity/localdev` — self-issued in-memory CA; constructed **only** by
   `freya.WithInsecureLocalDev()`, logs `WARN insecure local-dev identity in use`.

**Rationale**: SPIFFE is the industry standard for workload identity, is platform-neutral
(matches the spec's "pluggable identity provider" assumption), and the Workload API
delivers exactly stories 2 and 4 (trusted authority, short-lived, automatic rotation,
revocation via short TTL + bundle updates). go-spiffe uses only stdlib
`crypto/x509`/`crypto/tls` — no custom crypto (Principle VI). Kratos has no identity
concept of its own, so nothing conflicts.

**Alternatives considered**:
- JWT-SVID / bearer tokens for service identity — rejected for the channel itself:
  tokens authenticate requests, not the transport, and cannot prevent a rogue endpoint
  (mutual verification, story 2 scenario 6). JWT-SVID may return later for end-user
  identity propagation (out of scope).
- HashiCorp Vault PKI as the first provider — deferred: a fine second provider, but it
  needs a client library and secret-zero handling; the Workload API model is simpler.
- Kratos `middleware/auth/jwt` — not applicable (request-level, not transport-level).

**Revocation**: SR-005 is met primarily by short lifetime (1 h default, renew at 50%).
Explicit revocation is trust-bundle removal (provider-driven) plus an optional
`identity.RevocationChecker` interface; `contrib/policy-valkey` can serve a short-TTL
denylist of SPIFFE IDs for emergency revocation inside the lifetime window.

## 4. TLS configuration and downgrade prevention

**Decision**: `transport/tlsconf` builds one `*tls.Config` for servers and clients:
`MinVersion = MaxVersion = tls.VersionTLS13`, `ClientAuth: tls.RequireAnyClientCert`
with Freya's own verifier installed as **both** `VerifyPeerCertificate` and
`VerifyConnection` (resumed sessions are checked identically; session tickets are
disabled anyway), `GetCertificate`/`GetClientCertificate` bound to the provider (so
rotation needs no listener restart; an expired local credential is never presented).
The verifier (a) validates the chain against the *current* trust bundle, (b) requires
exactly one URI SAN in the configured trust domain, (c) applies the clock-skew tolerance
to the leaf's validity, (d) for clients, checks the SAN equals the expected callee's
SPIFFE ID, and (e) returns a closed-vocabulary `identity.VerifyError` (reason + hint)
so refusals are auditable precisely. `NextProtos = ["h2"]` for gRPC/HTTP2.

*Why not stdlib `RequireAndVerifyClientCert`*: its `ClientCAs` pool is fixed at config
creation (bundle rotation would need a rebuild), it cannot apply skew tolerance, and
its errors are free-text, so refusals could not be classified into the audit vocabulary. TLS 1.3 cipher suites are fixed by Go and not configurable, so
no weak-suite path exists; TLS 1.2 is **not** offered at all in v1 (the constitution's
opt-out is deliberately not implemented in this feature — adding it later is a MINOR
change with its own review).

**Rationale**: Go's `crypto/tls` implements TLS 1.3 with 0-RTT disabled by default (no
replay surface), and `MinVersion` makes downgrade below 1.3 impossible regardless of
what a peer offers (SR-004). Certificate callbacks are the idiomatic way to rotate
without reconnecting.

**Alternatives considered**: `go-spiffe`'s `tlsconfig.MTLSServerConfig` directly —
used internally where it fits, but wrapped so the file and localdev providers share the
same verification path and so Freya can enforce `MinVersion` and the name-match check
uniformly.

## 5. Authorization policy

**Decision**: A small declarative policy document (YAML or JSON, schema in
`contracts/policy.schema.json`): a list of rules `{ from: [spiffe-id|glob], to:
[service-name|glob], operations: [method-glob], effect: allow|deny }`, evaluated
deny-by-default with explicit denies winning. Loaded by `authz.Source` implementations:
`file` (Kratos config watcher for hot reload), and `contrib/policy-valkey` (policy body
+ version key in Valkey with pub/sub `freya:policy:changed` for sub-second propagation).
Decisions are cached per (peer, service, operation) with invalidation on policy version
change.

**Rationale**: Per-service and per-operation granularity satisfies story 3; the matcher
is a few hundred lines of pure Go, easy to fuzz and to cover 100%. Hot reload without
restart (FR-009, SC-005 ≤30 s) comes from the file watcher (Kratos config) or Valkey
pub/sub. The user's KV directive (Valkey) maps naturally onto policy distribution and
the revocation denylist; it stays optional.

**Alternatives considered**:
- OPA/Rego embedded — rejected for v1: ~40 transitive deps, and Rego evaluation hides
  control flow from reviewers (Principles VI, VII). An `authz.Policy` interface leaves
  room for a `contrib/policy-opa` later.
- Cedar (cedar-go) — promising but young; same interface-based deferral.
- Encoding policy in certificates (SPIFFE ID only) — insufficient for per-operation rules
  and requires re-issuing identities to change policy.

## 6. Storage: TimescaleDB and Valkey

**Decision**: The core feature needs **no database**. Per the user directive, when
persistence is wanted:
- **TimescaleDB** (`contrib/audit-timescale`, `pgx/v5`): durable audit sink. One
  hypertable `audit_events(ts timestamptz, event_type text, outcome text, caller_id
  text, callee_id text, operation text, reason text, correlation_id text, attrs jsonb)`
  partitioned by `ts`, with a retention policy (default 90 days) and compression after 7
  days. Writes are batched and asynchronous; the in-process `slog` sink remains the
  primary, always-on audit channel so audit never blocks or drops the call path.
- **Valkey** (`contrib/policy-valkey`, `valkey-go`): policy document + version, pub/sub
  change notifications, and the emergency revocation denylist (keys with TTL ≤ identity
  lifetime). Connections to Valkey are themselves TLS with client auth, configured via
  the same `tlsconf` package.

**Rationale**: Audit data is append-only time-series — TimescaleDB's hypertables,
retention and compression fit exactly. Valkey is a good fit for small, hot, versioned
documents and fan-out notifications. Keeping both in separate Go modules honours
Principle VI: services that don't need them never compile them in.

**Alternatives considered**: Kafka/NATS for audit (heavier, deferred); etcd for policy
(Kratos has a registry contrib but Valkey was requested and is lighter operationally).

## 7. Discovery, observability, limits

- **Discovery**: v1 ships `discovery/static` (name → list of endpoints from config) and
  accepts any Kratos `registry.Discovery` (etcd/consul/k8s contribs) via option. The
  client resolves a logical name to endpoints but verifies the **SPIFFE ID**, never the
  address (spec edge case "peer address changes"). Kratos `selector` provides
  client-side balancing.
- **Correlation & tracing**: W3C `traceparent` via OpenTelemetry (Kratos
  `middleware/tracing`) plus an `x-request-id` metadata key generated at the origin if
  absent and propagated by Kratos `middleware/metadata`. Both appear in every audit event.
- **Metrics**: OTel counters `freya_calls_total{peer,outcome}`,
  `freya_authn_refusals_total{reason}`, `freya_authz_refusals_total{peer,operation}`,
  `freya_identity_renewals_total{outcome}`, histogram `freya_call_duration_seconds`;
  served on the admin listener only.
- **Admin listener**: plain-HTTP on `127.0.0.1:9090` by default (health, readiness,
  metrics; pprof opt-in). Binding to a non-loopback address requires
  `Admin.AllowNonLoopback = true`, logs a warning, and switches the listener to mTLS
  with the service's own `tlsconf` configuration — plaintext never leaves loopback
  (constitution Transport rule).
- **Limit violations**: every refused oversized message/body, request timeout and
  handshake timeout emits one `limit_exceeded` audit event (gRPC via a
  `grpc.StatsHandler`, which sees rejections that happen before interceptors run).
- **Limits** (defaults from constitution): gRPC `MaxRecvMsgSize` 1 MiB,
  `MaxHeaderListSize` 8 KiB, `MaxConcurrentStreams` 100, keepalive/idle 60 s, per-call
  timeout 30 s via Kratos `Timeout` option; HTTP mirrors with `MaxHeaderBytes`,
  `http.MaxBytesReader`, `ReadHeaderTimeout`.
- **Redaction**: `audit.RedactingHandler` wraps the `slog.Handler`; attributes whose key
  matches a deny-list (`key`, `private`, `secret`, `token`, `password`, `authorization`)
  or whose value type is `*tls.Config`, `tls.Certificate`, `crypto.PrivateKey` are
  replaced by `"[REDACTED]"`. A test scans all output of the integration suite for PEM
  headers and known test-key fingerprints (SC-008).
- **Clock skew**: verification tolerates ±5 min (configurable, max 15 min) and refusal
  reason `identity_not_yet_valid`/`identity_expired` includes the skew hint.

## 8. Threat model (STRIDE)

| Threat | Asset / boundary | Mitigation | Verified by |
|--------|------------------|------------|-------------|
| **S**poofing a callee (rogue endpoint) | service ↔ service | Client verifies callee SPIFFE ID in `VerifyPeerCertificate`; discovery address never trusted | Integration test: rogue server with valid-but-wrong-name cert is refused |
| **S**poofing a caller (stolen/forged identity) | service ↔ service | mTLS chain validation against trust bundle; 1 h lifetime; optional denylist | Negative matrix: untrusted CA, expired, name mismatch |
| **T**ampering in transit | network | TLS 1.3 AEAD; no plaintext path | Integration: byte-flip proxy → call fails |
| **R**epudiation of a call/refusal | audit stream | Every refusal/lifecycle event audited with identities + correlation ID; Timescale sink append-only | SC-009 test: one event per refusal |
| **I**nformation disclosure (eavesdropping) | network | TLS 1.3 | Packet capture test finds no plaintext |
| **I**nformation disclosure (secrets in logs/errors) | logs, traces, error bodies | Redacting handler; error encoder strips internals; pprof off public listener | SC-008 scan; contract test for 404 on `/debug/pprof/` |
| **D**enial of service (oversized/slow/handshake flood) | listener | Default limits; handshake timeout 10 s; `MaxConcurrentStreams` | Load test with limits exceeded → disconnect + audit |
| **D**oS via identity/policy source outage | service ↔ IdP/policy | Fail closed; readiness false; retry with backoff; last-known-good policy retained until expiry of its TTL | Integration: kill fake Workload API → service unready, no plaintext fallback |
| **E**levation of privilege (lateral movement) | service ↔ service | Deny-by-default policy, per-operation rules, explicit deny wins | Story 3 tests |
| Downgrade attack | TLS negotiation | `MinVersion` 1.3 only; no 1.2 offered | Test: client offering ≤1.2 is refused |

## 9. Testing strategy notes

- Two-process integration tests spawn `examples/two-services` binaries with the
  `identity/file` provider and a test CA from `internal/testutil`; a small in-process
  fake Workload API (unix socket) exercises `identity/spiffe` rotation with 3-second
  SVIDs (X.509 validity is second-granular; ≥10 renewals in ~20 s) (SC-004).
- Negative matrix is a table-driven test over: no cert, expired, not-yet-valid, revoked
  (removed from bundle), untrusted CA, wrong trust domain, wrong service name, TLS 1.2
  client, missing SAN, two SANs.
- Fuzz targets: `authz.Load`, `identity.ParseSPIFFEID`, `audit.Event.UnmarshalJSON`.
- Benchmarks: `BenchmarkCall` (sequential) and `BenchmarkThroughput` (8-way) compare the
  Freya channel with a plaintext Kratos server running the identical non-security
  middleware; `scripts/bench-gate.sh` enforces 1.15× / 1.25× (SC-006).
- Contrib modules test against real TimescaleDB and Valkey via testcontainers under
  `//go:build integration`.

## 10. Module path

**Decision**: `github.com/go-freya/freya` (matches the repository name `go-freya`).
Changing it before the first tagged release is a one-line `go.mod` edit; it is recorded
here so downstream artifacts are consistent.
