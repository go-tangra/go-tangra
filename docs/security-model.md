# Security model

This document summarises what Freya guarantees for a service-to-service call,
how, and what it does not cover. Requirement IDs refer to
`specs/001-secure-service-channel/spec.md`; principles refer to the constitution.

## Trust model

- The network is hostile (zero trust). Nothing is inferred from IP addresses,
  segments, or a service mesh.
- The unit of trust is a **SPIFFE identity** (`spiffe://<td>/svc/<name>`) proven
  by an X.509 certificate chaining to the trust domain's bundle.
- Identities come from an external authority via a `identity.Provider`
  (Workload API by default). Freya never acts as a root CA outside the explicit
  `WithInsecureLocalDev` mode.

## Call path (server)

```
TLS 1.3 handshake  →  recover  →  local identity guard  →  correlation  →  tracing
  →  instrumentation  →  authn  →  authz  →  application middleware  →  handler
```

- **Handshake**: `transport/tlsconf` requires a client certificate, verifies the
  chain against the *current* bundle, requires exactly one URI SAN in the
  configured trust domain, applies ±skew to validity, and (client side) checks
  the SAN equals the expected callee. `VerifyConnection` repeats the checks for
  resumed sessions. Every refused handshake is audited with its reason and
  remote address (`authn_refused` / `channel_refused_downgrade`).
- **Local identity guard**: while this service has no valid identity of its own,
  every call is refused (503 `identity_expired`) regardless of connection state
  (FR-012, SR-003).
- **authn**: re-derives the peer from the TLS layer (defence in depth), checks
  revocation (a checker error = revoked), exposes `authn.PeerIdentity`
  read-only, annotates the span.
- **authz**: evaluates the policy for (peer, callee, operation); deny wins; no
  policy = deny; refusals carry rule id and policy version in the audit event.

## Wire contract

See `specs/001-secure-service-channel/contracts/wire-protocol.md` for headers,
error mapping and alerts. In short: `x-request-id` and `traceparent` propagate;
`x-freya-caller` is ignored; error bodies never carry internals.

## STRIDE summary

| Threat | Mitigation | Test |
|--------|------------|------|
| Spoofed callee | Client verifies callee SPIFFE ID, never the address | `TestRogueCallee` |
| Spoofed caller | Chain + SAN verification, 1 h lifetime, revocation hook | `TestNegativeMatrix` |
| Tampering | TLS 1.3 AEAD | `TestTamperProxy` |
| Repudiation | Audit stream with identities + correlation IDs | `TestNegativeMatrix` (one event per refusal) |
| Eavesdropping | TLS 1.3 | `TestNoPlaintextOnWire` |
| Secret leakage | Redacting handler, opaque errors, pprof off public port | `TestRedactionScan`, `TestHTTPServerNeverServesDefaultMux` |
| DoS | Default limits, handshake timeout, concurrent stream cap; every violation audited as `limit_exceeded` | `TestLimits` |
| Identity-provider outage | Fail closed, readiness 503, no plaintext fallback | `TestRotationProviderDown` |
| Lateral movement | Deny-by-default policy | `TestPolicy` |
| Downgrade | TLS 1.3 pinned | `TestNegativeMatrix/tls_1.2_client` |

## Out of scope (v1)

End-user identity propagation, asynchronous messaging, trust-domain federation,
TLS 1.2 compatibility mode.

## Known limitations

- SC-006 is measured against a plaintext server running the same non-security
  middleware: 1.12× sequential latency, 1.22× per call under concurrency
  (`make bench`). Against a bare plaintext gRPC server the sequential ratio is
  ≈ 2.1×, dominated by TLS record I/O in gRPC's transport, not framework logic.
- With `discovery.static` a callee that moves to a new address is not found
  until restart; use a registry (`WithDiscovery`) for dynamic topologies
  (`TestCalleeAddressChange`).
- X.509 validity has one-second granularity; identity lifetimes below a few
  seconds are not meaningful.
