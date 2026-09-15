# Changelog

## 0.1.0 — unreleased

Initial implementation of the secure service-to-service channel
(spec `001-secure-service-channel`).

### Security-relevant defaults (all on unless explicitly overridden)

- TLS 1.3 only (`MinVersion = MaxVersion`), ALPN `h2`, session tickets disabled,
  client certificate required, `VerifyPeerCertificate` **and** `VerifyConnection`
  enforce SPIFFE identity, trust domain, validity (±5 min skew) and, for
  clients, the expected callee name.
- Identity lifetime 1 h, renewal at 50 %, ±10 % jitter; an expired local identity
  is never presented and every call is refused with `identity_expired` (503 /
  UNAVAILABLE) until renewed.
- Authorization is deny-by-default; explicit `deny` rules win; policy reloads
  without restart (file poll 2 s, Valkey pub/sub).
- Limits: request body 1 MiB, headers 8 KiB, request timeout 30 s, idle 60 s,
  handshake 10 s, 100 concurrent streams, connection age 30 min.
- gRPC reflection disabled; health service subject to policy like any RPC.
- HTTP: explicit 404/405 handlers (never `http.DefaultServeMux`), eager TLS
  handshakes with audited refusals, body limit filter, security chain applied to
  every route before routing.
- Admin listener (health, readiness, metrics, opt-in pprof) is plain HTTP on
  `127.0.0.1:9090` only; binding to any other address requires explicit opt-in
  and then serves **mTLS only** (plaintext listeners never leave loopback).
- Every resource-limit violation (message size, request body, request timeout,
  handshake timeout) is audited as `limit_exceeded`.
- Logs pass through a redacting handler: keys containing
  key/private/secret/token/password/authorization, byte slices, PEM text and
  TLS/key values are replaced by `[REDACTED]`.
- Error bodies contain only `{"reason": ...}`; 5xx collapse to `internal`.
- `WithInsecureLocalDev` and `WithAllowAllPolicy` are refused when
  `env: production`.

- Handshake refusal events carry the operator hint (e.g. clock-skew tolerance)
  in `attrs.detail`; the `reason` vocabulary stays closed.

### Added

- `freya.New/App` with `GRPC()`, `HTTP()`, `Client()`, `Identity()`, `Ready()`,
  `AdminURL()`, `IdentityState()`.
- Identity providers: `identity/spiffe` (Workload API), `identity/file`,
  `identity/localdev`; `identity.Lifecycle` state machine.
- `authn` middleware + `MemoryRevocationChecker`; `authz` policy engine with
  bounded decision cache and `authz/file` source.
- `audit` events (schema in `specs/.../contracts/audit-event.schema.json`),
  redacting handler, non-blocking emitter.
- `observe`: UUIDv7 correlation IDs, W3C trace propagation, OpenMetrics
  exposition without a Prometheus client dependency, per-hop request log.
- `contrib/audit-timescale` (batched hypertable sink) and
  `contrib/policy-valkey` (policy source + revocation denylist) as separate
  modules.
- Example services, `cmd/freya-devca`, CI gates (lint, gosec, govulncheck,
  coverage ≥ 80 % / 100 % for security packages, fuzz, reproducible build).
