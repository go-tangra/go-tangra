# Contract: Wire Protocol Between Freya Peers

## Transport

| Item | Value |
|------|-------|
| TLS version | 1.3 only (`MinVersion = MaxVersion = TLS 1.3`); anything lower is refused at handshake |
| Cipher suites | Go's TLS 1.3 set (AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305); not configurable |
| ALPN | `h2` (gRPC and HTTP/2). HTTP/1.1 is not offered |
| Client auth | `RequireAndVerifyClientCert` |
| 0-RTT | disabled |
| Handshake timeout | 10 s |
| Identity | X.509 leaf with exactly one URI SAN `spiffe://<trust-domain>/svc/<service-name>` |
| Callee verification | client compares leaf URI SAN to the expected `spiffe://<td>/svc/<callee>`; server hostname/IP is never used |

## Metadata / headers

| Key | Direction | Rules |
|-----|-----------|-------|
| `x-request-id` | both | Correlation ID; generated (UUIDv7) if absent; `[A-Za-z0-9._-]{1,128}` else replaced |
| `traceparent`, `tracestate` | both | W3C Trace Context (OpenTelemetry) |
| `x-freya-caller` | **never** trusted | If present it is ignored; identity comes only from the TLS peer certificate |

## Error mapping (what a peer sees)

| Condition | gRPC code | HTTP | Body / message | Audit `reason` |
|-----------|-----------|------|----------------|----------------|
| No client certificate | handshake failure (no code) | — | TLS alert `certificate_required` | `no_identity` |
| Chain not trusted / wrong trust domain | handshake failure | — | TLS alert `bad_certificate` | `untrusted` |
| Expired / not yet valid | handshake failure | — | TLS alert `certificate_expired` | `identity_expired` / `identity_not_yet_valid` |
| Revoked (denylist) | `UNAUTHENTICATED` | 401 | `{"reason":"identity_revoked"}` | `identity_revoked` |
| SAN vs claimed name mismatch (middleware re-check) | `UNAUTHENTICATED` | 401 | `{"reason":"name_mismatch"}` | `name_mismatch` |
| TLS < 1.3 offered | handshake failure | — | TLS alert `protocol_version` | `downgrade_refused` |
| Policy deny (explicit or no match) | `PERMISSION_DENIED` | 403 | `{"reason":"denied"}` | `explicit_deny` / `no_matching_rule` / `no_policy` |
| Limit exceeded | `RESOURCE_EXHAUSTED` | 413 / 431 / 429 | `{"reason":"limit_exceeded"}` | `limit_exceeded` |
| Timeout | `DEADLINE_EXCEEDED` | 504 | `{"reason":"timeout"}` | — |

Error bodies never include rule IDs, policy contents, trust-store details, internal
addresses, stack traces, or version strings (SR-007). Rule IDs appear only in the audit
stream.

## Health and metrics (admin listener only)

| Path | Semantics |
|------|-----------|
| `GET /healthz` | process alive |
| `GET /readyz` | 200 only when identity valid, bundle non-empty, policy loaded (or allow-all opted in) |
| `GET /metrics` | OpenMetrics; series listed in research.md §7 |
| `GET /debug/pprof/*` | only when `Admin.EnablePprof` |

The admin listener is plain HTTP on loopback by default; it is never the same listener as
the service port.
