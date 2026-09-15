# Constitution Compliance Review: Secure Service-to-Service Channel

**Purpose**: Record the walk of all seven principles (constitution v1.0.0) against the implemented codebase (task T112)
**Created**: 2026-09-15
**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

## I. Secure by Default

- [x] CHK001 No exported constructor yields a plaintext server/client — `tests/contract/api_surface_test.go` (reflection walk)
- [x] CHK002 Insecure modes are named opt-ins (`WithInsecureLocalDev`, `WithAllowAllPolicy`), warn, audit, refused in production — `freya_test.go`
- [x] CHK003 Zero-config start refuses (identity + policy required) — `TestNewRefusesBrokenSetups`
- [x] CHK004 Kratos `DefaultServeMux` fallback overridden — `TestHTTPServerNeverServesDefaultMux`
- [x] CHK005 gRPC reflection disabled; admin listener is plain HTTP on loopback only and switches to mTLS when bound elsewhere (`TestAdminPprofAndNonLoopback`) — resolves analysis finding C1

## II. Zero Trust Service Communication

- [x] CHK006 mTLS with SPIFFE identities on every channel; callee verified by ID, never address — `TestRogueCallee`, `TestPoolReusesAndVerifiesName`
- [x] CHK007 authn/authz middleware fixed in `transport/grpc.SecurityChain`; HTTP filter applies before routing — no bypass path
- [x] CHK008 Short-lived identities rotated automatically — `TestRotation` (≥10 renewals, 0 failed calls)

## III. Boundary Validation & Defense in Depth

- [x] CHK009 Policy documents schema-validated with unknown fields rejected; fuzzed — `authz/loader_test.go`, `FuzzPolicyLoad`
- [x] CHK010 SPIFFE IDs and audit events parsed strictly; fuzzed — `FuzzParseSPIFFEID`, `FuzzAuditEventUnmarshal`
- [x] CHK011 Default limits on every listener, and every violation audited as `limit_exceeded` (message size, request body, request timeout, handshake timeout) — `TestLimits`, `TestOversizedRequestRefused` — resolves analysis finding G1
- [x] CHK012 authn re-derives the peer from TLS state rather than trusting the transport

## IV. Test-First with Security Verification

- [x] CHK013 Tests written before implementation in every phase (tasks.md ordering)
- [x] CHK014 Negative security matrix — `TestNegativeMatrix` (9 cases, one audit event each)
- [x] CHK015 Coverage gate: 88.1% total; 100% in identity/, authn/, authz/, transport/tlsconf/ — `make cover`
- [x] CHK016 No skipped or flaky tests in the suite (the rotation soak always runs; ~25 s)

## V. Observability & Auditability

- [x] CHK017 Audit stream with stable schema; every refusal and lifecycle event — `contracts/audit-event.schema.json`, `audit/event_test.go`
- [x] CHK018 Redaction enforced — `audit/redact_test.go`, `TestRedactionScan`, `make redaction-scan`
- [x] CHK019 Correlation ID across hops; trace context propagated — `TestMultiHopTrace`
- [x] CHK020 Health/readiness/metrics on a separate listener; never on the service port — `TestServerServesVerifiedPeerAndRejectsOversize`

## VI. Supply Chain Integrity & Minimal Dependencies

- [x] CHK021 Direct dependencies justified — `docs/dependencies.md`
- [x] CHK022 `go.sum` committed; `go mod verify` clean; `govulncheck` clean (allow-list only GO-2026-5471, expiring 2026-12-31)
- [x] CHK023 No custom cryptography; only `crypto/tls`, `crypto/x509` primitives
- [x] CHK024 Contrib integrations are separate modules (pgx, valkey-go not in the core graph)
- [x] CHK025 Reproducible build step in CI (`cmp` of two `-trimpath` builds)

## VII. Simplicity & Explicit Configuration

- [x] CHK026 Typed `config.Config`, validated at startup, warnings for overrides — `config/config_test.go`
- [x] CHK027 No reflection-based wiring, no global mutable state (only `klog.SetDefault` for Kratos's own logger in `Run`)
- [x] CHK028 Middleware chain fixed and short; documented in `docs/security-model.md`

## Findings

- SC-006 was re-based (analysis finding P1) to a baseline with identical non-security middleware: measured 1.12× sequential / 1.22× under 8-way concurrency against thresholds of 1.15× / 1.25× (`make bench`).
- `golangci-lint` is not installed in this environment; `staticcheck` and `gosec` ran clean directly. CI installs golangci-lint via `make tools`.
- Docker is unavailable here, so the contrib `integration`-tagged tests were compiled (`go vet -tags integration`) but not executed.
