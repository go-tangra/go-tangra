# Quickstart validation results

**Date**: 2026-09-15 · **Go**: go1.26.8 (toolchain) · **Host**: linux/amd64, 4 CPUs · **Docker**: unavailable

| § | Scenario | Result | Evidence |
|---|----------|--------|----------|
| 1 | Static gates | ✅ | `go vet`, `staticcheck` clean, `gosec` clean, `scripts/vulncheck.sh`: no reachable vulnerabilities, `go mod verify` OK, coverage 88.1 % / 100 % security packages |
| 2 | Two services, one secure call | ✅ | `TestHappyPath` runs the real example binaries; `orders` logs `call ok`, `inventory` logs the same `x-request-id` with `peer=spiffe://example.org/svc/orders` |
| 3 | Confidentiality & integrity | ✅ | `TestNoPlaintextOnWire` (recording proxy: no payload, method or header text on the wire), `TestTamperProxy` (byte flip → call fails, handler never runs) |
| 4 | Negative identity matrix | ✅ | `TestNegativeMatrix`: no cert, expired, not-yet-valid, untrusted CA, wrong trust domain, missing SAN, two SANs, TLS 1.2, revoked — each refused with the expected reason and exactly one audit event; `TestRogueCallee`, `TestBundleRevocation` |
| 5 | Policy & hot reload | ✅ | `TestPolicy`: A→B allowed, A→C and B→C denied, no rules → denied; file change applied in < 3 s (limit 30 s); `TestNoPolicyDeniesAndWarns` |
| 6 | Rotation without downtime | ✅ | `TestRotation`: 3 s SVIDs from the fake Workload API, 4 workers ≈ 200 calls/s, ≥ 10 renewals per side, **0 failed calls**; `TestRotationProviderDown`: readiness 503, calls refused, no plaintext, recovery after the agent returns |
| 7 | Observability & redaction | ✅ | `TestMultiHopTrace`: one trace ID across A→B→C, two hop lines by correlation ID, per-peer metrics on the admin listener; `TestRedactionScan`: 0 secret markers in logs, error bodies, metrics |
| 8 | Secure by default / pprof | ✅ | admin listener off-loopback is mTLS-only (`TestAdminPprofAndNonLoopback`); limit violations audited (`TestLimits`) |
| 8a | (was) | ✅ | `tests/contract`: no plaintext constructor, `/debug/pprof/` → 404 with `net/http/pprof` imported, options refused in production, no key accessors |
| 9 | Performance (SC-006, re-based) | ✅ | `make bench` vs a plaintext Kratos server with identical non-security middleware: sequential 191 → 215 µs/op (**1.12×**, max 1.15), 8-way throughput 50.6 → 61.8 µs/op (**1.22×**, max 1.25). Against a *bare* plaintext gRPC server (no middleware) the sequential ratio is ≈ 2.1×, dominated by TLS record syscalls in gRPC's transport |
| 10 | Contrib checks | ⚠️ | Unit tests pass (`contrib/*`); `-tags integration` tests compile but need Docker (testcontainers) to run |

Full suite: `go test -race ./...` — all packages `ok` (integration ≈ 37 s).
