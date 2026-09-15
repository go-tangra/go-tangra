# Implementation Plan: Secure Service-to-Service Channel

**Branch**: `001-secure-service-channel` | **Date**: 2026-09-15 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-secure-service-channel/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Deliver the first slice of the Freya framework: a Go library on top of go-kratos v3 that
lets a service call another service by logical name over a mutually authenticated,
encrypted, integrity-protected channel with zero security code in the caller or callee.
Technical approach: wrap Kratos's gRPC/HTTP transports in Freya constructors that always
install a TLS 1.3-only `*tls.Config` fed by a pluggable, auto-rotating `identity.Provider`
(SPIFFE Workload API first-class); enforce peer verification and a deny-by-default,
hot-reloadable authorization policy in Kratos middleware that handlers cannot bypass;
emit a schema-stable audit stream and OpenTelemetry signals with enforced redaction.
No database is required by the core; TimescaleDB (audit sink) and Valkey (policy
distribution / revocation cache) are optional `contrib` modules behind interfaces.

## Technical Context

**Language/Version**: Go 1.25 (minimum required by Kratos v3; `go.mod` pins `go 1.25.0`,
toolchain pinned via `toolchain` directive)

**Primary Dependencies**:
- `github.com/go-kratos/kratos/v3` v3.0.0 — app lifecycle, gRPC/HTTP transports,
  middleware chain, registry/discovery interfaces, config loader
- `google.golang.org/grpc` (transitive via Kratos) — gRPC transport, `credentials.TLSInfo`
- `github.com/spiffe/go-spiffe/v2` — Workload API client, X509-SVID rotation, trust
  bundle, `tlsconfig` helpers (default identity provider)
- `go.opentelemetry.io/otel` (+ Kratos tracing/metrics middleware) — traces, metrics
- `log/slog` (stdlib) — structured logging and audit emission
- Optional contrib only: `github.com/jackc/pgx/v5` (TimescaleDB audit sink),
  `github.com/valkey-io/valkey-go` (policy notifications, revocation cache)
- Test only: stdlib `testing` + `go test -fuzz`; `github.com/testcontainers/testcontainers-go`
  behind `//go:build integration` for contrib modules

**Storage**: None required by the core feature. Optional: TimescaleDB (hypertable
`audit_events`) as a durable audit sink; Valkey as KV for policy version/pub-sub
change notifications and a short-TTL revocation cache. Both are opt-in `contrib` modules.

**Testing**: `go test ./...` (unit, table-driven), `go test -run Fuzz -fuzz` for policy
and identity parsers, contract tests against the public API in `tests/contract`,
integration tests with an in-process fake Workload API and, for contrib, testcontainers.
Gates: `go vet`, `staticcheck`, `gosec`, `govulncheck`, coverage ≥80% overall / 100% in
`identity/`, `authn/`, `authz/`, `transport/tlsconf`.

**Target Platform**: Linux servers/containers (Kubernetes primary; bare VM supported).
Linux/amd64 and linux/arm64 builds.

**Project Type**: Library (Go module) with an `examples/` directory and a small
`cmd/freya-devca` helper for the explicitly opted-in local development mode.

**Performance Goals**: Secure call overhead ≤15% p50 sequential and ≤25% per call under
8-way concurrency vs. plaintext gRPC with identical non-security middleware (SC-006;
measured 1.12× / 1.22×); one TLS handshake per peer connection, reused across calls;
policy decision ≤50 µs p99 in-process (LRU cache); identity renewal never blocks a call
(SC-004).

**Constraints**: TLS 1.3 minimum, no downgrade path (SR-004); fail closed on identity,
trust-bundle, or policy loss (SR-003); zero secret material in logs/traces/metrics/errors
(SR-002); default limits 1 MiB body, 8 KiB headers, 30 s request timeout, 60 s idle,
100 concurrent streams per connection (constitution); no custom cryptography.

**Scale/Scope**: Hundreds of services, thousands of instances per cluster; policy files
up to ~10k rules; identity lifetime default 1 h, renewal at half-life; Valkey/Timescale
contribs are optional and must not be on the core dependency graph.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Evaluate this feature against `.specify/memory/constitution.md` (v1.0.0). Mark each gate
PASS, FAIL (with Complexity Tracking entry), or N/A (with one-line reason).

- [x] **I. Secure by Default**: PASS. Freya never exposes Kratos's raw `NewServer`/
      `NewClient`; its constructors always inject a TLS 1.3 mTLS config. Insecure and
      self-signed modes exist only as `freya.WithInsecureLocalDev()` and log a startup
      warning. Kratos's `http.DefaultServeMux` fallback (GHSA-jj45-xvq5-rhh9) is
      overridden with 404/405 handlers in every Freya-built HTTP server.
- [x] **II. Zero Trust**: PASS. mTLS with SPIFFE IDs (or equivalent X.509 SAN URI) on
      every inbound/outbound channel; `authn` and `authz` middleware are prepended by the
      framework and the handler registration API offers no way to remove them; identities
      are 1 h by default and rotated by the provider.
- [x] **III. Boundary Validation**: PASS. Policy files and identity documents are parsed
      through schema-validated loaders with fuzz tests; default size/timeout/concurrency
      limits are set on every listener; peer identity is verified in the TLS layer and
      re-checked (name match) in middleware — no layer trusts the previous one.
- [x] **IV. Test-First (NON-NEGOTIABLE)**: PASS. tasks.md will list test tasks before
      implementation per story; the negative matrix (no/expired/revoked/untrusted/
      mismatched identity, downgrade attempt) and fuzz targets for policy and SVID
      parsing are planned; 100% coverage in `identity/`, `authn/`, `authz/`,
      `transport/tlsconf` is achievable because they are pure logic with fakes.
- [x] **V. Observability**: PASS. Audit events (schema in `contracts/audit-event.schema.json`)
      via a dedicated `slog` handler; redaction enforced by an `slog.Handler` wrapper that
      strips keys tagged secret and never logs `*tls.Config`/private keys; W3C
      `traceparent` + `x-request-id` propagated by middleware; health/metrics on a
      separate admin listener bound to localhost/pod-IP only.
- [x] **VI. Supply Chain**: PASS with one tracked risk. New direct deps: Kratos v3,
      go-spiffe v2, OpenTelemetry — each justified in research.md; contrib deps isolated
      in separate Go modules so the core graph stays small. `govulncheck` currently flags
      Kratos GHSA-jj45-xvq5-rhh9 (no upstream fix); mitigated in code, tracked in
      research.md, and the CI gate carries an explicit, expiring allow-list entry for that
      ID only. No custom crypto: all primitives are stdlib `crypto/tls`/`crypto/x509`.
- [x] **VII. Simplicity**: PASS. One Go module for the core, typed `freya.Config` validated
      at startup, no reflection-based wiring, no global mutable state (the only global —
      `http.DefaultServeMux` — is explicitly avoided). Middleware chain is fixed and short
      (recover → local-identity guard → correlation → tracing → instrumentation → authn →
      authz → handler); size/timeout/stream limits are applied at the transport layer.
- [x] **Threat Model**: PASS. STRIDE model for the channel, identity provider, and policy
      source is in research.md §8.

*Post-Phase 1 re-check (2026-09-15)*: all gates still PASS. Design introduced no new
dependencies beyond those listed; contrib modules remain separate `go.mod`s; the
public API in `contracts/go-api.md` has no constructor that yields a plaintext server.

## Project Structure

### Documentation (this feature)

```text
specs/001-secure-service-channel/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── go-api.md                  # Public Go API surface (stable contract)
│   ├── wire-protocol.md           # TLS/ALPN/metadata/error-code contract between peers
│   ├── policy.schema.json         # Authorization policy document schema
│   └── audit-event.schema.json    # Audit event schema
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
go.mod                         # module github.com/go-freya/freya (core; go 1.25)
freya.go                       # App builder: New(cfg, opts...) wrapping kratos.App
config/
├── config.go                  # Typed Config, Validate(), opt-out warnings
└── limits.go                  # Default resource limits
identity/
├── provider.go                # Provider, Identity, Bundle, RevocationChecker interfaces; Watch()
├── lifecycle.go               # State machine: renewal at RenewAt, backoff, expiry, bundle updates
├── spiffe/                    # Workload API provider (go-spiffe v2)
├── file/                      # PEM files + fsnotify-free polling reload (tests/CI)
└── localdev/                  # Self-issued CA; only via WithInsecureLocalDev()
transport/
├── runtime.go                 # Runtime contract, local-identity guard, handshake/limit audit helpers
├── tlsconf/                   # Builds *tls.Config: TLS1.3 only, custom verifier (+VerifyConnection), cert callbacks
├── grpc/                      # NewServer (audited creds, stats handler), Pool, SecurityChain
└── http/                      # NewServer wrapper: eager TLS listener, security filter, 404/405, ErrorEncoder
authn/
├── peer.go                    # PeerIdentity, FromContext()
└── middleware.go              # Name-match re-check, refusal → audit
authz/
├── policy.go                  # Policy model, Decision
├── loader.go                  # YAML/JSON loader + schema validation
├── matcher.go                 # Deterministic rule matcher (deny-by-default)
├── source.go                  # Source interface (file, kratos config, contrib valkey)
└── middleware.go
audit/
├── event.go                   # Event struct = contracts/audit-event.schema.json
├── emitter.go                 # Emitter, slog sink, bounded async fan-out
└── redact.go                  # slog.Handler wrapper enforcing SR-002
observe/
├── correlation.go             # x-request-id (UUIDv7) propagation
├── tracing.go                 # W3C traceparent propagation, server/client spans
├── metrics.go                 # OTel instruments, Instrument middleware (metrics + per-hop request log), OpenMetrics renderer
└── admin.go                   # Separate admin listener: /healthz /readyz /metrics (mTLS when non-loopback)
discovery/
└── static.go                  # Static name→endpoint map; kratos registry passthrough
contrib/
├── audit-timescale/           # separate go.mod; pgx hypertable sink
└── policy-valkey/             # separate go.mod; policy version pub/sub + revocation cache
cmd/freya-devca/               # Dev-only CA for localdev mode
examples/two-services/         # A calls B; used by quickstart.md
internal/testutil/             # Fake Workload API, test CA, packet-capture harness
tests/
├── contract/                  # Public API contract tests (contracts/go-api.md)
├── integration/               # Two-process A→B tests, negative matrix, rotation
└── fuzz/                      # Fuzz targets for policy loader and identity parsing
```

**Structure Decision**: Single Go module library (`github.com/go-freya/freya`) at the
repository root with domain packages (`identity`, `transport`, `authn`, `authz`, `audit`,
`observe`) — no `src/` wrapper, following Go conventions. Optional storage integrations
live under `contrib/` as independent modules so the core dependency graph never includes
`pgx` or `valkey-go` (Principle VI). Tests that need two OS processes or containers live
under `tests/` with build tags; unit tests sit beside the code.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations. (Tracked risk, not a violation: Kratos GHSA-jj45-xvq5-rhh9 is unpatched
upstream; mitigated by Freya's HTTP server constructor and allow-listed in `govulncheck`
with an expiry — see research.md §2.)
