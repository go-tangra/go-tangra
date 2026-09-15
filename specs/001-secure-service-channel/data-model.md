# Data Model: Secure Service-to-Service Channel

**Feature**: 001-secure-service-channel | **Date**: 2026-09-15

Entities are in-memory Go types unless noted. Persistence exists only in the optional
`contrib` modules (§ Storage). Field names are the Go field names; wire/JSON names are in
`contracts/`.

## ServiceIdentity

A cryptographically verifiable claim that a running instance is a named service.

| Field | Type | Rules |
|-------|------|-------|
| `ID` | `SPIFFEID` (`spiffe://<trust-domain>/svc/<name>`) | Exactly one URI SAN; trust domain must equal the configured domain; `<name>` matches `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$` |
| `ServiceName` | `string` | Derived from `ID` path; the value handlers see |
| `TrustDomain` | `string` | Derived from `ID` host |
| `NotBefore`, `NotAfter` | `time.Time` | `NotAfter - NotBefore` ≤ 24 h; default 1 h; verified with ±5 min skew tolerance |
| `SerialNumber` | `string` | Used as `identity_serial` in audit events |
| `Chain` | `[]*x509.Certificate` | Leaf first; never logged |
| `privateKey` | `crypto.Signer` | Unexported; never serialised, never logged (redaction test) |

**State**: `Pending` → `Valid` → (`Renewing` → `Valid`) → `Expired`; `Revoked` from any
state when the ID appears in a `RevocationChecker` or its issuer leaves the bundle.
Renewal starts at 50% of lifetime (configurable 30–80%). A service whose identity is
`Expired` or `Revoked` with no replacement transitions the app to `Unready` and
refuses new channels (SR-003).

## TrustBundle

The set of authorities whose identities a service accepts.

| Field | Type | Rules |
|-------|------|-------|
| `TrustDomain` | `string` | One bundle per trust domain; v1 supports one configured domain (federation deferred) |
| `Roots` | `[]*x509.Certificate` | ≥1 required; empty bundle → provider error → fail closed |
| `Version` | `uint64` | Monotonic; incremented on every update; recorded in `trust_bundle_updated` audit events |
| `UpdatedAt` | `time.Time` | |

Updates are atomic swaps (`atomic.Pointer`); in-flight handshakes use the bundle they
started with.

## SecureChannel

A live, mutually verified, encrypted session between two instances. Conceptual entity:
it is realised by the TLS connection state and the pooled `*grpc.ClientConn`; no
`SecureChannel` type is exported.

| Field | Type | Rules |
|-------|------|-------|
| `Local` | `ServiceIdentity` | |
| `Peer` | `PeerIdentity` | Verified; exposed read-only via `authn.FromContext` |
| `Protocol` | `string` | Always `TLS1.3`; `h2` ALPN |
| `EstablishedAt` | `time.Time` | |
| `Transport` | `grpc` \| `http` | |

Channels are pooled per callee logical name (client side) and reused across calls;
the pool drops a connection when the local identity rotates *and* the peer requires
re-verification (gRPC keepalive handles the rest).

## PeerIdentity

Read-only view of the verified remote party, placed in the request context.

| Field | Type | Rules |
|-------|------|-------|
| `ID` | `SPIFFEID` | Verified against bundle at TLS layer, name-matched again in `authn` middleware |
| `ServiceName` | `string` | |
| `Serial` | `string` | |
| `VerifiedAt` | `time.Time` | |

## AuthorizationPolicy

Declarative rules deciding which caller may reach which callee operation. Schema:
`contracts/policy.schema.json`.

| Field | Type | Rules |
|-------|------|-------|
| `Version` | `string` | Free-form (e.g. git SHA, Valkey version); appears in audit events |
| `Rules` | `[]Rule` | 0..10 000; empty = deny everything |
| `Source` | `string` | `file:<path>` or `valkey:<key>`; informational |
| `LoadedAt` | `time.Time` | |

**Rule**

| Field | Type | Rules |
|-------|------|-------|
| `ID` | `string` | Unique within the policy; required; used in audit `rule_id` |
| `From` | `[]string` | SPIFFE IDs or globs (`spiffe://td/svc/order-*`); ≥1 |
| `To` | `[]string` | Callee service names or globs; ≥1 |
| `Operations` | `[]string` | Method globs (`/pkg.Service/Method`, `GET /v1/orders/*`); default `["*"]` |
| `Effect` | `allow` \| `deny` | |

**Decision algorithm** (deterministic, documented for reviewers): collect all rules whose
`From`, `To`, `Operations` all match; if any has `effect: deny` → **DENY** (reason
`explicit_deny`, rule id); else if any `allow` → **ALLOW**; else **DENY** (reason
`no_matching_rule`). Absence of a policy is **DENY** (reason `no_policy`) and logs a
startup warning. `WithAllowAllPolicy()` is a named development option that logs a
warning (Principle I).

**Decision** (in-memory, cached)

| Field | Type |
|-------|------|
| `Allowed` | `bool` |
| `Reason` | enum: `explicit_allow`, `explicit_deny`, `no_matching_rule`, `no_policy` |
| `RuleID` | `string` (may be empty) |
| `PolicyVersion` | `string` |

Cache key `(peerID, calleeName, operation)`; entire cache invalidated on policy version
change; bounded LRU (default 10 000 entries).

## AuditEvent

Immutable record of a security-relevant occurrence. Wire schema:
`contracts/audit-event.schema.json`.

| Field | Type | Rules |
|-------|------|-------|
| `Time` | `time.Time` (RFC 3339 nano, UTC) | |
| `Type` | enum | `authn_refused`, `authz_refused`, `authz_allowed` (sampled, off by default), `identity_issued`, `identity_renewed`, `identity_renewal_failed`, `identity_expired`, `trust_bundle_updated`, `policy_loaded`, `policy_load_failed`, `insecure_mode_enabled`, `limit_exceeded`, `channel_refused_downgrade` |
| `Outcome` | `refused` \| `ok` \| `failed` | |
| `Reason` | `string` (closed vocabulary per type, see contracts) | Never contains peer-supplied free text |
| `LocalID` | `string` | SPIFFE ID of the emitting service |
| `ClaimedPeerID` | `string` | What the peer presented (may be empty/unverified) |
| `VerifiedPeerID` | `string` | Empty unless verification succeeded |
| `Operation` | `string` | |
| `CorrelationID` | `string` | `x-request-id`; always present after middleware |
| `TraceID` | `string` | W3C trace id if tracing enabled |
| `RuleID`, `PolicyVersion` | `string` | authz events only |
| `RemoteAddr` | `string` | IP:port (informational; never used for trust) |
| `Attrs` | `map[string]string` | Extra structured fields; passes through the redactor |

Invariant (SR-002): no field may carry key material, tokens, or certificate bodies; the
redacting handler enforces it and a test asserts it on the whole suite's output.

## CorrelationID

Opaque, ≤128 chars, `[A-Za-z0-9._-]`. Generated (UUIDv7) at the first Freya hop if
absent; propagated as gRPC metadata / HTTP header `x-request-id`. Peer-supplied values
that fail the character rule are replaced (not echoed) and the event is logged.

## Config (startup-validated)

| Field | Type | Default | Rules |
|-------|------|---------|-------|
| `ServiceName` | string | — | required; must equal the identity's `ServiceName` at startup or the app refuses to start |
| `TrustDomain` | string | — | required |
| `Identity.Provider` | `spiffe` \| `file` \| `localdev` | `spiffe` | `localdev` only settable via `WithInsecureLocalDev()` |
| `Identity.WorkloadSocket` | string | `unix:///run/spire/sockets/agent.sock` | spiffe only |
| `Identity.RenewAt` | fraction | 0.5 | 0.3–0.8 |
| `Identity.SkewTolerance` | duration | 5m | ≤15m |
| `Authz.Source` | `file` \| `valkey` \| `allow-all` | `file` | `allow-all` only via `WithAllowAllPolicy()` |
| `Authz.Path` / `Authz.ValkeyKey` | string | — | required for the chosen source |
| `Limits.*` | see plan | constitution defaults | overrides logged at startup |
| `Admin.Addr` | string | `127.0.0.1:9090` | non-loopback requires `AllowNonLoopback` |
| `Admin.EnablePprof` | bool | false | |
| `Discovery.Static` | map[name][]endpoint | — | or a Kratos `registry.Discovery` via option |

## Storage (optional contrib modules)

### TimescaleDB — `audit_events` hypertable

```sql
CREATE TABLE audit_events (
  ts               TIMESTAMPTZ NOT NULL,
  event_type       TEXT        NOT NULL,
  outcome          TEXT        NOT NULL,
  reason           TEXT        NOT NULL,
  local_id         TEXT        NOT NULL,
  claimed_peer_id  TEXT,
  verified_peer_id TEXT,
  operation        TEXT,
  correlation_id   TEXT        NOT NULL,
  trace_id         TEXT,
  rule_id          TEXT,
  policy_version   TEXT,
  remote_addr      TEXT,
  attrs            JSONB       NOT NULL DEFAULT '{}'
);
SELECT create_hypertable('audit_events', 'ts', chunk_time_interval => INTERVAL '1 day');
CREATE INDEX ON audit_events (correlation_id, ts DESC);
CREATE INDEX ON audit_events (verified_peer_id, ts DESC);
SELECT add_retention_policy('audit_events', INTERVAL '90 days');
ALTER TABLE audit_events SET (timescaledb.compress, timescaledb.compress_segmentby = 'event_type');
SELECT add_compression_policy('audit_events', INTERVAL '7 days');
```

Append-only: the sink role has `INSERT` only. Writes are batched (≤500 rows / 1 s) from a
bounded queue; overflow increments `freya_audit_dropped_total` and is itself logged to
the primary `slog` sink — the call path never blocks on the database.

### Valkey — policy and revocation keys

| Key | Type | Contents / TTL |
|-----|------|----------------|
| `freya:policy:<trust-domain>:doc` | string | policy document (JSON) |
| `freya:policy:<trust-domain>:version` | string | opaque version; compared on notification |
| `freya:policy:changed` | pub/sub channel | message = new version |
| `freya:revoked:<spiffe-id>` | string | `"1"`, TTL = remaining identity lifetime (≤ configured max) |

Valkey connections use TLS 1.3 with client certificates from the same `tlsconf`
package; the Valkey ACL user for services has read + subscribe only.
