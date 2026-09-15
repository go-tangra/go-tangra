# Quickstart: Secure Service-to-Service Channel

Validation guide proving the feature end-to-end. Implementation details live in
`tasks.md`; API shapes in [contracts/go-api.md](contracts/go-api.md); wire behaviour in
[contracts/wire-protocol.md](contracts/wire-protocol.md).

## Prerequisites

- Go 1.25+ (`go version`)
- `make`, `openssl` (for inspecting certs), `tcpdump` or `tshark` (for SC-003)
- Optional (contrib checks only): Docker for testcontainers (TimescaleDB, Valkey)
- No SPIRE needed: the quickstart uses the `identity/file` provider with a test CA and
  the in-process fake Workload API for rotation.

## 1. Build and static gates

```bash
make lint        # go vet, staticcheck, gosec
make vuln        # govulncheck; only GO-2026-5471 is allow-listed (expiry 2026-12-31)
make test        # unit + contract + fuzz smoke (go test ./... -run . -fuzztime=10s)
make cover       # fails if <80% overall or <100% in identity/ authn/ authz/ transport/tlsconf
```

Expected: all green; the coverage report lists the four security packages at 100%.

## 2. Two services, one secure call (User Story 1)

```bash
make testca                          # writes ./.dev/ca/{ca.pem,orders.{pem,key},inventory.{pem,key}}
go run ./examples/two-services/inventory --config examples/two-services/inventory.yaml &
go run ./examples/two-services/orders    --config examples/two-services/orders.yaml
```

Expected output from `orders`:

```
INFO identity ready id=spiffe://example.org/svc/orders not_after=...
INFO policy loaded version=quickstart rules=1
INFO call ok peer=spiffe://example.org/svc/inventory op=/inventory.v1.Inventory/Reserve x-request-id=...
```

and `inventory` logs an `authz_allowed`-free (sampling off) but a request line whose
`peer=spiffe://example.org/svc/orders` matches `authn.FromContext`.

Check the handshake is TLS 1.3 / mTLS:

```bash
openssl s_client -connect 127.0.0.1:9443 -cert .dev/ca/orders.pem -key .dev/ca/orders.key \
  -CAfile .dev/ca/ca.pem -alpn h2 </dev/null 2>/dev/null | grep -E "Protocol|Verify return"
# Protocol  : TLSv1.3
# Verify return code: 0 (ok)
```

## 3. Confidentiality and integrity (SC-003, FR-003)

```bash
sudo tshark -i lo -f "tcp port 9443" -Y "tls.app_data" -T fields -e tls.app_data | head
```

Expected: only opaque hex; `grep -c "inventory.v1"` over the capture returns 0.

```bash
go test ./tests/integration -run TestTamperProxy -v
```

Expected: the byte-flipping proxy causes the call to fail with a TLS `bad_record_mac`
and no response body is delivered.

## 4. Negative identity matrix (User Story 2, SC-002)

```bash
go test ./tests/integration -run TestNegativeMatrix -v
```

Expected table (one line per case, all `refused`):

| case | expected reason |
|------|-----------------|
| no client cert | `no_identity` |
| expired | `identity_expired` |
| not yet valid | `identity_not_yet_valid` |
| untrusted CA | `untrusted` |
| wrong trust domain | `untrusted` |
| non-`/svc/` SPIFFE path, missing SAN, two SANs | `untrusted` |
| revoked via denylist | `identity_revoked` (after the handshake, by the authn middleware) |
| TLS 1.2 client | `downgrade_refused` (event type `channel_refused_downgrade`) |
| rogue callee (valid cert, wrong name) | client refuses: `name_mismatch` (`TestRogueCallee`) |

Each case must produce exactly one `authn_refused` audit event (SC-009) — the test
asserts this from the captured `slog` output.

Manual spot-check (no cert):

```bash
openssl s_client -connect 127.0.0.1:9443 -CAfile .dev/ca/ca.pem -alpn h2 </dev/null 2>&1 | grep alert
# ... alert certificate required
```

## 5. Authorization policy and hot reload (User Story 3, SC-005)

```bash
go test ./tests/integration -run TestPolicy -v
```

Expected: `A→B allow`, `A→C denied (no_matching_rule)`, `B→A denied`, `no policy →
denied (no_policy)`; after the test rewrites `policy.yaml` to allow A→C, the next call
succeeds within 30 s (test asserts ≤ 30 s, typically < 2 s) without restarting any
process.

Manual: edit `examples/two-services/policy.yaml`, remove the `orders-to-inventory`
rule, save; the next `orders` call logs `PERMISSION_DENIED` and `inventory` emits
`authz_refused reason=no_matching_rule`.

## 6. Identity rotation without downtime (User Story 4, SC-004)

```bash
go test ./tests/integration -run TestRotation -v -timeout 15m
```

Uses the fake Workload API issuing 3-second SVIDs (X.509 validity is second-granular)
while a load generator runs at 200 calls/s. Expected: ≥10 renewals observed (`identity_renewed` events), **0** failed
calls, and each service presents a new serial after every renewal. The follow-on
`TestRotationProviderDown` stops the fake API: after the current SVID expires the
service reports `/readyz` 503, refuses new calls, and emits `identity_expired` — never a
plaintext fallback.

## 7. Observability and redaction (User Story 5, SC-007, SC-008)

```bash
go test ./tests/integration -run TestMultiHopTrace -v
```

Expected: an A→B→C call yields three spans sharing one trace ID, and querying the
captured logs by `x-request-id` returns three lines with caller/callee identities and
durations.

```bash
make redaction-scan   # runs the whole integration suite, then greps all captured
                      # logs, traces, metrics and error bodies for PEM headers and
                      # the test keys' fingerprints
```

Expected: `0 matches`.

Metrics: `curl -s 127.0.0.1:9090/metrics | grep freya_` shows `freya_calls_total`,
`freya_authn_refusals_total`, `freya_authz_refusals_total`,
`freya_identity_renewals_total`. `curl -s 127.0.0.1:9443/metrics` (the service port)
must fail the handshake — metrics are never on the public listener.

## 8. Secure-by-default and pprof mitigation (Principle I, GHSA-jj45-xvq5-rhh9)

```bash
go test ./tests/contract -v
```

Expected: `TestNoPlaintextConstructor`, `TestHTTPServerNeverServesDefaultMux`
(`/debug/pprof/` → 404 with `net/http/pprof` imported), `TestOptionsRefusedInProduction`
all pass. Running an example with `FREYA_ENV=production` and
`WithInsecureLocalDev()` must exit non-zero with a clear error.

## 9. Performance (SC-006)

```bash
make bench     # go test -tags bench -bench 'Benchmark(Call|Throughput)' + scripts/bench-gate.sh
```

Expected: `BenchmarkCall/mtls` ≤ 1.15 × `BenchmarkCall/plaintext` (sequential) and
`BenchmarkThroughput/mtls` ≤ 1.25 × its plaintext counterpart (8-way). The baseline is a
plaintext Kratos server running the identical non-security middleware; it exists only
under `//go:build bench` and is not part of the library.

## 10. Optional contrib checks

```bash
go test -tags integration ./contrib/audit-timescale/...   # inserts events into a hypertable, verifies retention/compression policies exist
go test -tags integration ./contrib/policy-valkey/...     # publishes a new policy version; services reload in < 2 s; denylist revokes an ID
```

## Done when

All sections 1–9 pass on a clean checkout; section 10 passes when Docker is available.
