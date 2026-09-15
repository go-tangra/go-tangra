---

description: "Task list for Secure Service-to-Service Channel"
---

# Tasks: Secure Service-to-Service Channel

**Input**: Design documents from `/specs/001-secure-service-channel/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Tests are MANDATORY (Constitution Principle IV, NON-NEGOTIABLE). Every user story lists its test tasks before its implementation tasks; tests MUST be written and confirmed failing before implementation. Every story here touches auth, transport, parsing, or secrets, so each includes negative security tests, and every parser has a fuzz target.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. Module path is `github.com/go-freya/freya` (research.md §10). Kratos is `github.com/go-kratos/kratos/v3` v3.0.0.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Single Go module library at repository root (plan.md "Project Structure"): domain packages `config/`, `identity/`, `transport/`, `authn/`, `authz/`, `audit/`, `observe/`, `discovery/`; unit tests beside code (`*_test.go`); cross-process tests in `tests/{contract,integration,fuzz}`; optional storage integrations as separate modules under `contrib/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, toolchain, and CI gates required by the constitution

- [X] T001 Initialize Go module `github.com/go-freya/freya` with `go 1.25.0` and `toolchain` directive in go.mod; add `github.com/go-kratos/kratos/v3@v3.0.0`, `github.com/spiffe/go-spiffe/v2`, `go.opentelemetry.io/otel` and run `go mod tidy` so go.sum is committed
- [X] T002 Create package skeleton directories with `doc.go` files per plan.md: config/, identity/{spiffe,file,localdev}/, transport/{tlsconf,grpc,http}/, authn/, authz/, audit/, observe/, discovery/, internal/testutil/, tests/{contract,integration,fuzz}/, examples/two-services/, cmd/freya-devca/, contrib/
- [X] T003 [P] Add .golangci.yml enabling govet, staticcheck, gosec, errcheck, gocritic, revive with gosec severity ≥ medium failing the build in .golangci.yml
- [X] T004 [P] Create Makefile with targets `lint`, `vuln`, `test`, `cover`, `fuzz`, `testca`, `redaction-scan`, `bench` matching quickstart.md §1 in Makefile
- [X] T005 [P] Create govulncheck wrapper script that allow-lists exactly `GO-2026-5471` with mitigation note and hard expiry 2026-12-31 (fails after expiry) in scripts/vulncheck.sh
- [X] T006 [P] Create coverage gate script enforcing ≥80% overall and 100% for identity/, authn/, authz/, transport/tlsconf/ in scripts/coverage-gate.sh
- [X] T007 [P] Create CI workflow running build, vet, lint, vuln, test (unit+contract+fuzz smoke 10s), coverage gate, reproducible-build check (`go build -trimpath` twice, compare hashes) on linux/amd64 and linux/arm64 in .github/workflows/ci.yml
- [X] T008 [P] Add .gitignore (`.dev/`, coverage files, binaries) and .editorconfig at repository root
- [X] T009 [P] Write docs/dependencies.md justifying each direct dependency (purpose, alternatives rejected, maintenance status) per Constitution Principle VI, using research.md §1, §3, §6

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, test fixtures, config, identity primitives, audit/redaction, and the TLS builder that every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (write first, confirm failing)

- [X] T010 [P] Unit tests for `config.Config.Validate()`: required fields, RenewAt bounds 0.3–0.8, SkewTolerance ≤15m, Admin non-loopback requires AllowNonLoopback, limit overrides logged, `localdev`/`allow-all` unsettable via plain config in config/config_test.go
- [X] T011 [P] Unit tests for `identity.ParseSPIFFEID`: valid, wrong scheme, missing `/svc/`, name regex violations, trust-domain case, length limits in identity/spiffeid_test.go
- [X] T012 [P] Fuzz target `FuzzParseSPIFFEID` asserting no panic and round-trip `String()` stability in tests/fuzz/spiffeid_fuzz_test.go
- [X] T013 [P] Unit tests for `audit.NewRedactingHandler`: keys `key|private|secret|token|password|authorization` (case-insensitive) → `[REDACTED]`; values of type `*tls.Config`, `tls.Certificate`, `crypto.PrivateKey`, `[]byte` containing `-----BEGIN` → `[REDACTED]`; nested groups handled in audit/redact_test.go
- [X] T014 [P] Unit tests for `audit.Event` JSON marshalling against contracts/audit-event.schema.json (schema-validate every emitted example; closed reason vocabulary enforced) in audit/event_test.go
- [X] T015 [P] Fuzz target `FuzzAuditEventUnmarshal` in tests/fuzz/audit_fuzz_test.go
- [X] T016 [P] Unit tests for `identity/file` provider: loads PEM cert/key/bundle, rejects key/cert mismatch, rejects cert without URI SAN, rejects two SANs, detects file change on poll and emits Update, never exposes key via `Identity` interface in identity/file/provider_test.go
- [X] T017 [P] Unit tests for `transport/tlsconf`: server and client configs have `MinVersion==MaxVersion==TLS1.3`, `ClientAuth==RequireAnyClientCert` with Freya's verifier as `VerifyPeerCertificate` and `VerifyConnection`, `NextProtos==["h2"]`, `GetCertificate` returns the provider's current cert after rotation, `VerifyPeerCertificate` rejects untrusted/expired/not-yet-valid/wrong-domain/wrong-name/missing-SAN/two-SAN chains and accepts within ±skew in transport/tlsconf/tlsconf_test.go
- [X] T018 [P] Unit tests for `observe.CorrelationID`: generated UUIDv7 when absent, valid incoming value preserved, invalid chars replaced and logged, ≤128 length in observe/correlation_test.go

### Implementation for Foundational

- [X] T019 [P] Implement typed `config.Config` with nested Identity/Authz/Limits/Admin/Discovery structs, `Validate()`, and constitution default limits (1 MiB body, 8 KiB header, 30 s request, 60 s idle, 100 streams) in config/config.go and config/limits.go
- [X] T020 [P] Implement `identity.SPIFFEID`, `ParseSPIFFEID`, `Identity`, `Bundle`, `Provider`, `Update`, `RevocationChecker` interfaces exactly as contracts/go-api.md in identity/spiffeid.go and identity/provider.go
- [X] T021 [P] Implement `audit.Event` struct with JSON tags matching contracts/audit-event.schema.json, `Type`/`Outcome`/`Reason` enums, and `Validate()` in audit/event.go
- [X] T022 Implement `audit.NewRedactingHandler` (slog.Handler wrapper) with key deny-list and value-type detection in audit/redact.go
- [X] T023 Implement `audit.Sink` interface, non-blocking `slogSink`, and fan-out `Emitter` with bounded queue and `freya_audit_dropped_total` counter hook in audit/emitter.go
- [X] T024 Implement `internal/testutil` test CA: `NewCA(trustDomain)`, `IssueSVID(name, notBefore, notAfter)`, helpers for untrusted CA, wrong domain, two-SAN, no-SAN certs, plus `CaptureLogs()` helper in internal/testutil/ca.go and internal/testutil/logs.go
- [X] T025 Implement `identity/file` provider (PEM load, 2 s poll for change, atomic swap, `Watch` channel) in identity/file/provider.go
- [X] T026 Implement `transport/tlsconf` builder: `ServerConfig(provider, opts)` and `ClientConfig(provider, expectedPeer SPIFFEID)` with cert callbacks, bundle-aware `VerifyPeerCertificate`, skew tolerance, and package-private key access in transport/tlsconf/tlsconf.go
- [X] T027 Implement `observe.CorrelationID`, `HeaderCorrelationID`, and Kratos middleware that generates/validates/propagates `x-request-id` in observe/correlation.go
- [X] T028 Implement `cmd/freya-devca` (writes test CA + per-service SVIDs to `.dev/ca/`, used by `make testca`) reusing internal/testutil in cmd/freya-devca/main.go

**Checkpoint**: Foundation ready — `go test ./config/... ./identity/... ./audit/... ./transport/tlsconf/... ./observe/...` green with 100% coverage in identity/, audit/redact.go, transport/tlsconf/

---

## Phase 3: User Story 1 - Call Another Service Securely (Priority: P1) 🎯 MVP

**Goal**: Service A calls Service B by logical name over TLS 1.3 mTLS with zero security code in either service; both sides see the verified peer; traffic is unreadable and tamper-evident.

**Independent Test**: Run `examples/two-services` with the file provider and test CA; A→B round trip succeeds; `tshark` capture contains no plaintext; byte-flip proxy makes the call fail (quickstart.md §2–§3).

### Tests for User Story 1 (MANDATORY) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T029 [P] [US1] Contract test `TestNoPlaintextConstructor`: reflection walk over exported funcs of freya, transport/grpc, transport/http asserting none accepts/returns `*tls.Config`, `grpc.DialOption`, or `credentials.TransportCredentials` in tests/contract/api_surface_test.go
- [X] T030 [P] [US1] Contract test `TestHTTPServerNeverServesDefaultMux`: import `net/http/pprof`, build Freya HTTP server, `GET /debug/pprof/` → 404 and unknown method → 405 in tests/contract/http_defaultmux_test.go
- [X] T031 [P] [US1] Contract test `TestIdentityHasNoKeyAccessor`: reflection asserts `identity.Identity`, `authn.PeerIdentity`, `freya.App.Identity()` expose no `crypto.PrivateKey`/`tls.Certificate`/`[]byte` key material in tests/contract/no_key_exposure_test.go
- [X] T032 [P] [US1] Unit tests for `authn.FromContext`/`WithPeer` and the authn middleware placing verified `PeerIdentity` from `credentials.TLSInfo` into ctx in authn/peer_test.go
- [X] T033 [P] [US1] Unit tests for `transport/grpc.NewServer`: default limits applied (`MaxRecvMsgSize` 1 MiB, `MaxHeaderListSize` 8 KiB, `MaxConcurrentStreams` 100, keepalive 60 s, timeout 30 s), middleware order recover→correlation→authn→authz→limits→tracing, `ServerOption` cannot replace TLS or remove middleware in transport/grpc/server_test.go
- [X] T034 [P] [US1] Unit tests for `transport/http.NewServer`: same limits via `MaxHeaderBytes`/`MaxBytesReader`/`ReadHeaderTimeout`, NotFound/MethodNotAllowed overrides always set in transport/http/server_test.go
- [X] T035 [P] [US1] Unit tests for client pool: one `*grpc.ClientConn` per callee name, reused across calls, expected SPIFFE ID derived from callee name + trust domain, conn dropped on close in transport/grpc/client_test.go
- [X] T036 [P] [US1] Unit tests for `discovery/static`: name→endpoints from config, unknown name error, Kratos `registry.Discovery` passthrough in discovery/static_test.go
- [X] T037 [P] [US1] Unit tests for `freya.New`: refuses to start with missing ServiceName/TrustDomain, identity name ≠ ServiceName, provider unavailable; `Ready()` false until identity valid; `WithInsecureLocalDev` logs WARN and emits `insecure_mode_enabled` audit event; refused when `Env=="production"` in freya_test.go
- [X] T038 [US1] Integration test `TestHappyPath`: spawn inventory+orders example binaries with file provider; A→B call succeeds; both log verified peer IDs; `x-request-id` identical on both sides in tests/integration/happy_path_test.go
- [X] T039 [US1] Integration test `TestTamperProxy`: TCP proxy flipping one byte of TLS application data between A and B; call fails with TLS record error; no response body delivered; server emits no handler log in tests/integration/tamper_test.go
- [X] T040 [US1] Integration test `TestNoPlaintextOnWire`: capture loopback traffic during 20 calls via `gopacket`-free raw socket or `tshark` if present (skip with reason otherwise); assert zero occurrences of the request payload marker and of `inventory.v1` in tests/integration/capture_test.go

### Implementation for User Story 1

- [X] T041 [P] [US1] Implement `authn.PeerIdentity`, `FromContext`, `WithPeer` (test-only), and gRPC/HTTP middleware extracting `credentials.TLSInfo`/`r.TLS` verified chains into ctx in authn/peer.go and authn/middleware.go
- [X] T042 [P] [US1] Implement `discovery/static` and Kratos `registry.Discovery` adapter in discovery/static.go
- [X] T043 [US1] Implement `transport/grpc.NewServer` wrapping `kgrpc.NewServer` with `kgrpc.TLSConfig(tlsconf.ServerConfig(...))`, fixed middleware chain, default limits, `RegisterService`, and bounded `ServerOption`s in transport/grpc/server.go and transport/grpc/options.go
- [X] T044 [US1] Implement `transport/http.NewServer` wrapping `khttp.NewServer` with `khttp.TLSConfig`, `khttp.NotFoundHandler(http.NotFoundHandler())`, `khttp.MethodNotAllowedHandler(405)`, limits, and the same middleware chain in transport/http/server.go
- [X] T045 [US1] Implement pooled mTLS client: `kgrpc.DialInsecure` never used; `kgrpc.WithTLSConfig(tlsconf.ClientConfig(provider, expected))`, `WithDiscovery`, per-callee `sync.Map` pool, correlation-ID outgoing interceptor in transport/grpc/client.go
- [X] T046 [US1] Implement `freya.App`: `New` (validate config → start provider → build servers → wire audit/logging), `Run` with graceful stop, `GRPC()`, `HTTP()`, `Client()`, `Identity()`, `Ready()`, options `WithIdentityProvider/WithDiscovery/WithAuditSink/WithLogger/WithInsecureLocalDev` in freya.go and options.go
- [X] T047 [US1] Implement `identity/localdev` self-issued in-memory CA provider (constructed only by `WithInsecureLocalDev`, WARN at startup, refused in production) in identity/localdev/provider.go
- [X] T048 [US1] Create example services: proto `examples/two-services/api/inventory/v1/inventory.proto` (Reserve RPC), generated code, `inventory/main.go` server, `orders/main.go` client calling `Reserve` by name, YAML configs pointing at `.dev/ca/` in examples/two-services/
- [X] T049 [US1] Add `make testca` and example run instructions verified against quickstart.md §2 in Makefile and examples/two-services/README.md

**Checkpoint**: User Story 1 fully functional — quickstart §2–§3 and §8 pass; this is the MVP

---

## Phase 4: User Story 2 - Reject Unauthenticated or Impersonating Callers (Priority: P1)

**Goal**: Every peer lacking a valid, unexpired, trusted, name-matching identity is refused before any handler runs; refusal is mutual; each refusal produces exactly one audit event with a closed-vocabulary reason and no secret material.

**Independent Test**: `TestNegativeMatrix` (quickstart.md §4) — 9 refusal cases all refused with the expected reason and exactly one `authn_refused` event each; valid identity succeeds.

### Tests for User Story 2 (MANDATORY) ⚠️

- [X] T050 [P] [US2] Unit tests for authn name re-check middleware: SAN service name ≠ expected → `ErrNameMismatch` (UNAUTHENTICATED/401), `authn_refused reason=name_mismatch` event emitted once, error body contains only `{"reason":"name_mismatch"}` in authn/middleware_test.go
- [X] T051 [P] [US2] Unit tests for `identity.RevocationChecker` integration: revoked (id, serial) → `ErrRevoked`, checker error → fail closed (`ErrRevoked` with reason `provider_unavailable`), in-memory checker for tests in authn/revocation_test.go
- [X] T052 [P] [US2] Unit tests for error encoder: every authn/authz/limit error maps to the gRPC code / HTTP status / body in contracts/wire-protocol.md and never includes rule IDs, trust-store info, addresses, versions, or stack traces in transport/errors_test.go
- [X] T053 [P] [US2] Unit tests for `x-freya-caller` header: presence is ignored for identity and logged at DEBUG only in authn/middleware_test.go
- [X] T054 [US2] Integration test `TestNegativeMatrix` (table-driven): no cert, expired, not-yet-valid, revoked (denylist), untrusted CA, wrong trust domain, wrong service name, TLS 1.2-only client, missing SAN, two SANs — each refused with expected reason, handler never invoked, exactly one `authn_refused`/`channel_refused_downgrade` audit event validated against contracts/audit-event.schema.json in tests/integration/negative_matrix_test.go
- [X] T055 [US2] Integration test `TestRogueCallee`: server presenting a valid cert for `spiffe://…/svc/other` on the address discovered for `inventory`; orders client refuses with `name_mismatch` and emits audit event; no request bytes sent after handshake in tests/integration/rogue_callee_test.go
- [X] T056 [US2] Integration test `TestBundleRevocation`: remove issuer from trust bundle at runtime (file provider update); next handshake from previously valid peer refused with `untrusted`; `trust_bundle_updated` event emitted in tests/integration/bundle_revocation_test.go

### Implementation for User Story 2

- [X] T057 [P] [US2] Implement authn refusal path: name re-check against expected callee/caller, `RevocationChecker` hook (fail closed), audit emission with closed reasons, sentinel errors `ErrNoIdentity…ErrDowngrade` in authn/middleware.go and authn/errors.go
- [X] T058 [P] [US2] Implement in-memory `authn.MemoryRevocationChecker` (for tests and as reference) in authn/revocation.go
- [X] T059 [US2] Implement Freya error encoder for gRPC (status codes) and HTTP (JSON `{"reason":…}` bodies) per contracts/wire-protocol.md replacing Kratos default error encoder, stripping internals in transport/errors.go
- [X] T060 [US2] Emit `authn_refused` events from TLS handshake failures: wrap `net.Listener` / use `tls.Config.VerifyPeerCertificate` error classification to map alerts to reasons (`no_identity`, `untrusted`, `identity_expired`, `identity_not_yet_valid`, `downgrade_refused`) with `RemoteAddr` and no cert bodies in transport/tlsconf/handshake_audit.go
- [X] T061 [US2] Wire audit sink counters and ensure exactly-once emission per refusal (handshake-level vs middleware-level dedupe by connection ID) in transport/tlsconf/handshake_audit.go and authn/middleware.go

**Checkpoint**: User Stories 1 and 2 together satisfy SC-002; quickstart §4 passes

---

## Phase 5: User Story 3 - Control Which Services May Call Which (Priority: P2)

**Goal**: Deny-by-default authorization policy, declared outside code, per-service and per-operation, hot-reloadable without restart; authz refusals are distinct from authn refusals and audited with rule ID and policy version.

**Independent Test**: `TestPolicy` (quickstart.md §5) — A→B allowed, A→C and B→A denied, no policy → denied, file edit takes effect ≤30 s without restart.

### Tests for User Story 3 (MANDATORY) ⚠️

- [X] T062 [P] [US3] Unit tests for `authz.Load`: valid YAML and JSON accepted, schema violations rejected (duplicate rule id, empty from/to, bad glob pattern, >10 000 rules, unknown field), defaults `operations=["*"]` in authz/loader_test.go
- [X] T063 [P] [US3] Fuzz target `FuzzPolicyLoad` (no panic, valid output re-marshals and re-loads) in tests/fuzz/policy_fuzz_test.go
- [X] T064 [P] [US3] Unit tests for `Policy.Decide`: explicit allow, explicit deny wins over allow, no matching rule, no policy, glob semantics for SPIFFE ID/service/operation (`*`, `?`, prefix), gRPC `/pkg.Svc/Method` and HTTP `GET /v1/x/*` operations in authz/matcher_test.go
- [X] T065 [P] [US3] Property-style test: `Decide` is deterministic and order-independent for the same rule set (shuffle rules, same result) in authz/matcher_test.go
- [X] T066 [P] [US3] Unit tests for decision cache: hit/miss, bounded LRU 10 000, full invalidation on policy version change in authz/cache_test.go
- [X] T067 [P] [US3] Unit tests for `authz/file` source: loads at start, `Watch` emits on change via Kratos config watcher, invalid new file → keeps last-known-good, emits `policy_load_failed`, never yields nil policy in authz/file/source_test.go
- [X] T068 [P] [US3] Unit tests for authz middleware: runs after authn, uses `PeerIdentity` from ctx, DENY → `ErrDenied` (PERMISSION_DENIED/403) with body `{"reason":"denied"}`, `authz_refused` event carries `rule_id`+`policy_version`, `authz_allowed` sampled only when enabled in authz/middleware_test.go
- [X] T069 [P] [US3] Unit tests for `WithAllowAllPolicy`: WARN log + `insecure_mode_enabled reason=allow_all` event; refused in production; absent policy without the option → `no_policy` deny + startup warning in freya_test.go
- [X] T070 [US3] Integration test `TestPolicy` with services A, B, C: allow A→B only; assert A→B ok, A→C denied (`no_matching_rule`), B→A denied; rewrite policy.yaml to add A→C; assert success within 30 s without restart; assert `policy_loaded` event with new version in tests/integration/policy_test.go

### Implementation for User Story 3

- [X] T071 [P] [US3] Implement `authz.Policy`, `Rule`, `Effect`, `Decision`, `Reason` types and `Load` with embedded JSON-schema validation of contracts/policy.schema.json in authz/policy.go and authz/loader.go
- [X] T072 [P] [US3] Implement deterministic matcher with compiled globs and `Decide` algorithm from data-model.md in authz/matcher.go
- [X] T073 [US3] Implement bounded LRU decision cache keyed `(peerID, callee, operation)` invalidated on version change in authz/cache.go
- [X] T074 [US3] Implement `authz.Source` interface and `authz/file` source using Kratos `config` file source with `Watch`, last-known-good retention, and audit events in authz/source.go and authz/file/source.go
- [X] T075 [US3] Implement authz middleware (gRPC + HTTP), operation string derivation (`/pkg.Svc/Method`, `VERB /path`), `ErrDenied`, audit emission in authz/middleware.go
- [X] T076 [US3] Wire `WithPolicySource`, `WithAllowAllPolicy`, `no_policy` startup warning, and readiness dependency on policy loaded into freya.go and options.go
- [X] T077 [US3] Add `examples/two-services/policy.yaml` (allow orders→inventory Reserve) and point both example configs at it in examples/two-services/

**Checkpoint**: Story 3 independently testable; quickstart §5 passes

---

## Phase 6: User Story 4 - Rotate Identities Without Downtime (Priority: P2)

**Goal**: Identities are short-lived (1 h default) and renewed automatically at half-life via the SPIFFE Workload API provider; renewal never fails a call; loss of identity fails closed and flips readiness.

**Independent Test**: `TestRotation` (quickstart.md §6) — fake Workload API issuing 3-second SVIDs under 200 calls/s for ≥10 renewals with zero failed calls; `TestRotationProviderDown` → unready, refuses calls, no plaintext fallback.

### Tests for User Story 4 (MANDATORY) ⚠️

- [X] T078 [P] [US4] Unit tests for identity state machine: Pending→Valid→Renewing→Valid, Expired when no renewal, Revoked on bundle change, renewal at `RenewAt` fraction, jitter bounds, backoff on failure in identity/lifecycle_test.go
- [X] T079 [P] [US4] Unit tests for `identity/spiffe` provider against the fake Workload API: initial fetch, streamed update → new Identity+Bundle, socket unavailable → error (no cached fallback beyond current validity), context cancel closes stream in identity/spiffe/provider_test.go
- [X] T080 [P] [US4] Unit tests for tlsconf rotation: after provider Update, next `GetCertificate`/`GetClientCertificate` returns new serial while existing connections stay open in transport/tlsconf/rotation_test.go
- [X] T081 [P] [US4] Unit tests for readiness: `Ready()` false and `/readyz` 503 when identity expired/absent or bundle empty; `identity_expired` and `identity_renewal_failed` events emitted with closed reasons in freya_test.go and observe/admin_test.go
- [X] T082 [US4] Integration test `TestRotation`: fake Workload API with 3 s SVIDs, load generator 200 calls/s for ≥10 renewals; assert 0 failed calls, ≥10 `identity_renewed` events, serial changes each cycle on both services in tests/integration/rotation_test.go
- [X] T083 [US4] Integration test `TestRotationProviderDown`: stop fake Workload API; after expiry service returns `/readyz` 503, refuses new inbound (handshake) and outbound calls, emits `identity_expired`, and never opens a plaintext listener (port scan assertion) in tests/integration/provider_down_test.go

### Implementation for User Story 4

- [X] T084 [P] [US4] Implement fake SPIFFE Workload API server (unix socket, `FetchX509SVID` stream, configurable TTL, controllable stop/restart) in internal/testutil/workloadapi.go
- [X] T085 [P] [US4] Implement `identity/spiffe` provider using `workloadapi.NewX509Source` with update hooks mapping to `identity.Update` in identity/spiffe/provider.go
- [X] T086 [US4] Implement identity lifecycle manager (state machine, renewal scheduling for providers that need pull-renewal such as `file`, jittered backoff, audit events `identity_issued/renewed/renewal_failed/expired`, `trust_bundle_updated`) in identity/lifecycle.go
- [X] T087 [US4] Wire lifecycle into `freya.App`: readiness gate, refuse-to-serve on expiry (per-call local-identity guard on every listener, expired credential never presented, readiness 503), retry with backoff at startup when provider unavailable in freya.go
- [X] T088 [US4] Client pool: re-verify and rebuild connection when local identity rotates and the previous connection is older than the identity's `NotBefore` in transport/grpc/client.go

**Checkpoint**: Story 4 independently testable; quickstart §6 passes (SC-004)

---

## Phase 7: User Story 5 - Observe Secure Calls (Priority: P3)

**Goal**: Structured logs, OpenTelemetry traces and per-peer metrics with one correlation ID across hops; admin listener separate from the service port; zero secret material anywhere in output.

**Independent Test**: `TestMultiHopTrace` and `make redaction-scan` (quickstart.md §7) — A→B→C reconstructed from one correlation/trace ID; 0 secret matches across all captured output.

### Tests for User Story 5 (MANDATORY) ⚠️

- [X] T089 [P] [US5] Unit tests for metrics: `freya_calls_total{peer,outcome}`, `freya_authn_refusals_total{reason}`, `freya_authz_refusals_total{peer,operation}`, `freya_identity_renewals_total{outcome}`, `freya_call_duration_seconds` registered and incremented via in-memory OTel reader in observe/metrics_test.go
- [X] T090 [P] [US5] Unit tests for admin listener: default `127.0.0.1:9090`, `/healthz`, `/readyz`, `/metrics` served; `/debug/pprof/` 404 unless `EnablePprof`; non-loopback bind refused without `AllowNonLoopback` and logs WARN when allowed in observe/admin_test.go
- [X] T091 [P] [US5] Unit tests for tracing middleware: span per call with `peer.service` attribute set from verified identity, `traceparent` propagated to outbound calls, trace ID present in audit events in observe/tracing_test.go
- [X] T092 [P] [US5] Unit test asserting the service listener never serves `/metrics`, `/healthz`, `/readyz` (returns 404/UNIMPLEMENTED) in transport/http/server_test.go
- [X] T093 [US5] Integration test `TestMultiHopTrace`: three services A→B→C; assert 3 spans share one trace ID; querying captured logs by `x-request-id` returns 3 hop lines with caller/callee IDs, outcome, duration in tests/integration/multihop_test.go
- [X] T094 [US5] Redaction scan test: run full integration suite with log/trace/metrics/error-body capture, grep for `-----BEGIN`, test-key SHA-256 fingerprints, `Bearer `, and raw serials of private keys; assert 0 matches (SC-008) in tests/integration/redaction_scan_test.go

### Implementation for User Story 5

- [X] T095 [P] [US5] Implement OTel metrics (counters/histogram) with per-peer labels and wiring into authn/authz/transport in observe/metrics.go
- [X] T096 [P] [US5] Implement tracing middleware using Kratos `middleware/tracing` with `peer.service`/`peer.spiffe_id` attributes and trace-ID injection into audit events in observe/tracing.go
- [X] T097 [US5] Implement admin listener (plain HTTP, loopback default, health/ready/metrics, opt-in pprof) as a separate Kratos HTTP server with its own `NotFoundHandler` in observe/admin.go
- [X] T098 [US5] Add per-hop structured request log line (caller, callee, operation, outcome, duration, correlation, trace) through the redacting handler in observe/requestlog.go
- [X] T099 [US5] Add `make redaction-scan` target executing T094 with artifact capture directory in Makefile and scripts/redaction-scan.sh

**Checkpoint**: All five user stories independently testable; quickstart §2–§8 pass

---

## Phase 8: Optional Storage Integrations (contrib)

**Purpose**: User-directed persistence — TimescaleDB for the audit stream, Valkey for policy distribution and emergency revocation — as separate Go modules that never enter the core dependency graph (Principle VI). Depends on US2 (revocation hook), US3 (policy source), US5 (audit events).

### Tests for contrib (MANDATORY) ⚠️

- [X] T100 [P] Integration tests (`//go:build integration`, testcontainers TimescaleDB) for `contrib/audit-timescale`: migration creates hypertable, indexes, retention (90 d) and compression (7 d) policies; batched inserts; INSERT-only role; queue overflow increments `freya_audit_dropped_total` and never blocks `Emit` in contrib/audit-timescale/sink_test.go
- [X] T101 [P] Unit tests for Timescale sink batching (≤500 rows / 1 s flush, bounded queue, graceful drain on close) using a fake pgx interface in contrib/audit-timescale/batch_test.go
- [X] T102 [P] Integration tests (`//go:build integration`, testcontainers Valkey with TLS) for `contrib/policy-valkey`: loads doc+version, subscribes `freya:policy:changed`, reloads in <2 s, invalid doc keeps last-known-good, read-only ACL user, TLS client auth required in contrib/policy-valkey/source_test.go
- [X] T103 [P] Integration tests for Valkey `RevocationChecker`: `freya:revoked:<id>` with TTL → `IsRevoked` true; Valkey unreachable → fail closed (error) in contrib/policy-valkey/revocation_test.go

### Implementation for contrib

- [X] T104 [P] Create `contrib/audit-timescale` module (own go.mod, requires core + `pgx/v5`), migration SQL from data-model.md, `Sink` implementation with batching and drop counter in contrib/audit-timescale/{go.mod,sink.go,batch.go,migrations/001_audit_events.sql}
- [X] T105 [P] Create `contrib/policy-valkey` module (own go.mod, requires core + `valkey-go`), `authz.Source` implementation with pub/sub watch and `RevocationChecker` with TLS via `tlsconf` in contrib/policy-valkey/{go.mod,source.go,revocation.go}
- [X] T106 Add contrib modules to CI matrix under the `integration` tag with Docker service containers in .github/workflows/ci.yml and document configuration in contrib/README.md

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T107 [P] Benchmarks `BenchmarkCall/mtls` vs `BenchmarkCall/plaintext` (baseline under `//go:build bench`) with CI threshold p50 ≤1.10×, p95 ≤1.20× (SC-006) in tests/integration/bench_test.go and scripts/bench-gate.sh
- [X] T108 [P] Documentation: README.md (overview, constitution reference, quickstart link), docs/security-model.md (STRIDE table from research.md §8, wire-protocol summary), docs/configuration.md (every `config.Config` field with defaults) in README.md and docs/
- [X] T109 [P] Create SECURITY.md with private disclosure channel and 72 h patch SLA per constitution in SECURITY.md
- [X] T110 [P] Add CHANGELOG.md entry for 0.1.0 listing every security-relevant default in CHANGELOG.md
- [X] T111 [P] Negative security tests sweep: ensure every new boundary added in T041–T098 has a malformed-input test (oversized `x-request-id`, 2 MiB body → RESOURCE_EXHAUSTED/413, 9 KiB header → 431, 101st concurrent stream refused, slow-loris header timeout) in tests/integration/limits_test.go
- [X] T112 Security hardening and constitution compliance review: walk all seven principles against the codebase, record findings in specs/001-secure-service-channel/checklists/constitution-review.md
- [X] T113 Verify coverage thresholds (≥80% overall, 100% for identity/, authn/, authz/, transport/tlsconf/) via `make cover` and fix gaps
- [X] T114 Run `gosec`, `staticcheck`, `govulncheck` (allow-list only GO-2026-5471) and `go mod verify`; justify any dependency added since T009 in docs/dependencies.md
- [X] T115 Run quickstart.md §1–§10 end-to-end on a clean checkout and record results in specs/001-secure-service-channel/quickstart-results.md
- [X] T116 Code cleanup and refactoring pass (no behaviour change; tests stay green) across freya.go, transport/, authn/, authz/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories
- **User Story 1 (P1)**: Depends on Foundational — MVP
- **User Story 2 (P1)**: Depends on Foundational and on US1's transport/App (T043–T046); extends the same middleware
- **User Story 3 (P2)**: Depends on Foundational and US1 (App + middleware chain); independent of US2 and US4
- **User Story 4 (P2)**: Depends on Foundational and US1; independent of US2 and US3
- **User Story 5 (P3)**: Depends on Foundational and US1; its redaction scan (T094) is most valuable after US2–US4 exist
- **Contrib (Phase 8)**: Depends on US2 (revocation hook), US3 (policy source), US5 (audit events)
- **Polish (Phase N)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: Foundation only — delivers the secure call
- **US2 (P1)**: US1 (uses its servers/clients to exercise refusals)
- **US3 (P2)**: US1 — can proceed in parallel with US2 and US4
- **US4 (P2)**: US1 — can proceed in parallel with US2 and US3
- **US5 (P3)**: US1 — can proceed in parallel with US2–US4; final scan after all

### Within Each User Story

- Tests MUST be written and FAIL before implementation (Constitution Principle IV)
- Models/types before middleware
- Middleware before App wiring
- Unit before integration tests running green
- Story complete before moving to next priority

### Parallel Opportunities

- Setup: T003–T009 all parallel after T001–T002
- Foundational tests T010–T018 all parallel; implementations T019–T021 parallel, then T022–T028
- US1 tests T029–T037 parallel; T041–T042 parallel; T043–T047 sequential (shared freya.go/transport)
- US2 tests T050–T053 parallel; T057–T058 parallel
- US3 tests T062–T069 parallel; T071–T072 parallel
- US4 tests T078–T081 parallel; T084–T085 parallel
- US5 tests T089–T092 parallel; T095–T096 parallel
- After US1: US2, US3, US4, US5 can be worked by different developers concurrently
- Contrib T100–T105 all parallel (two independent modules)
- Polish T107–T111 parallel

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Contract test TestNoPlaintextConstructor in tests/contract/api_surface_test.go"
Task: "Contract test TestHTTPServerNeverServesDefaultMux in tests/contract/http_defaultmux_test.go"
Task: "Contract test TestIdentityHasNoKeyAccessor in tests/contract/no_key_exposure_test.go"
Task: "Unit tests for authn.FromContext in authn/peer_test.go"
Task: "Unit tests for transport/grpc.NewServer in transport/grpc/server_test.go"
Task: "Unit tests for transport/http.NewServer in transport/http/server_test.go"
Task: "Unit tests for client pool in transport/grpc/client_test.go"
Task: "Unit tests for discovery/static in discovery/static_test.go"
Task: "Unit tests for freya.New in freya_test.go"

# Then implement in parallel where files differ:
Task: "Implement authn.PeerIdentity in authn/peer.go and authn/middleware.go"
Task: "Implement discovery/static in discovery/static.go"
```

## Parallel Example: After US1 — stories in parallel

```bash
Developer A: Phase 4 (US2) — negative matrix, error encoder, handshake audit
Developer B: Phase 5 (US3) — policy loader/matcher/source/middleware
Developer C: Phase 6 (US4) — SPIFFE provider, lifecycle, fake Workload API
Developer D: Phase 7 (US5) — metrics, tracing, admin listener
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T009)
2. Complete Phase 2: Foundational (T010–T028) — CRITICAL, blocks everything
3. Complete Phase 3: User Story 1 (T029–T049)
4. **STOP and VALIDATE**: quickstart §2, §3, §8 pass; `make cover` green for foundational packages
5. Demo: two services, one secure mTLS call, packet capture shows only ciphertext

Note: US1 alone already refuses peers without a trusted certificate (TLS layer), so the MVP is secure; US2 adds the complete refusal matrix, audit of refusals, revocation, and the rogue-callee test.

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. Add US1 → test independently → demo (MVP)
3. Add US2 → negative matrix green → both P1 stories done (SC-002)
4. Add US3 (policy) and US4 (rotation) in parallel → demo hot reload and zero-downtime renewal
5. Add US5 → tracing, metrics, redaction scan (SC-007, SC-008)
6. Add contrib (Timescale audit sink, Valkey policy/revocation) if persistence is wanted
7. Polish → benchmarks (SC-006), docs, SECURITY.md, compliance review, 0.1.0

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. One developer completes US1 (others write US2–US5 tests against the contracts meanwhile)
3. Once US1 is stable: US2, US3, US4, US5 in parallel by story
4. Contrib modules by whoever finishes first
5. Polish together

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Never add an exported constructor that can produce a plaintext server or client (contracts/go-api.md design rule); the reflection contract test T029 will catch it
