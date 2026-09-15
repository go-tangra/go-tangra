# Feature Specification: Secure Service-to-Service Channel

**Feature Branch**: `001-secure-service-channel`

**Created**: 2026-09-15

**Status**: Draft

**Input**: User description: "I am building a microservice framework. The microservices should be able to comunicate each other over a secure channel."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Call Another Service Securely (Priority: P1)

A service developer using the framework wants their service (the caller) to send a request
to another service (the callee) and receive a response, with the framework guaranteeing
that the conversation is private (nobody on the network can read it), tamper-proof
(nobody can alter it in transit), and that both sides know exactly which service they are
talking to. The developer supplies only the callee's logical name and the request payload;
the framework handles every security concern without further code.

**Why this priority**: This is the core value of the feature. Without a working secure
call there is no framework. Every other story builds on it.

**Independent Test**: Start two services built with the framework on the same network.
Service A calls Service B by name and gets a correct response. A network observer
positioned between them sees only unreadable bytes and cannot identify the payload. The
test delivers a demonstrably private, authenticated round trip.

**Acceptance Scenarios**:

1. **Given** two services each provisioned with a valid identity, **When** Service A calls
   Service B by logical name, **Then** Service B receives the request, the response arrives
   at Service A, and both sides can read the verified identity of the other.
2. **Given** an active call between A and B, **When** a passive observer captures all
   traffic between them, **Then** the observer cannot recover any part of the request or
   response payload or headers.
3. **Given** an active call between A and B, **When** an on-path attacker modifies any
   byte in transit, **Then** the receiving side rejects the message and the call fails
   with a clear error rather than delivering corrupted data.
4. **Given** a developer with no security configuration in their service code, **When**
   they make a call using the framework's default client, **Then** the call is secured
   with no additional code (secure is the only default).

---

### User Story 2 - Reject Unauthenticated or Impersonating Callers (Priority: P1)

A platform operator needs assurance that a service will only talk to peers that can prove
who they are. Any party that cannot present a valid, unexpired identity — or presents an
identity that was not issued by the organisation's trusted authority — is refused before
any business logic runs, and the refusal is recorded.

**Why this priority**: Confidentiality alone is insufficient; an encrypted channel to an
impostor is still a breach. Mutual identity verification is what makes the channel
trustworthy, and it must ship together with Story 1 for the feature to be secure.

**Independent Test**: Attempt connections to a framework-built service from (a) a client
with no identity, (b) a client with an expired identity, (c) a client with an identity
issued by an untrusted authority, and (d) a client with a valid identity. Only (d)
succeeds; (a)–(c) are refused and each refusal appears in the audit record.

**Acceptance Scenarios**:

1. **Given** a running service, **When** a caller connects without presenting any
   identity, **Then** the connection is refused before any request is processed.
2. **Given** a running service, **When** a caller presents an identity that has expired
   or been revoked, **Then** the connection is refused.
3. **Given** a running service, **When** a caller presents an identity issued by an
   authority the service does not trust, **Then** the connection is refused.
4. **Given** a running service, **When** a caller presents a certificate whose SPIFFE
   URI is not of the form `spiffe://<trust-domain>/svc/<name>` (or carries no or several
   URIs), **Then** the connection is refused as untrusted; and **Given** a caller dialing
   service X, **When** the endpoint presents a valid identity for a different service,
   **Then** the caller refuses with `name_mismatch` (verification is by identity, not
   address).
5. **Given** any refusal above, **When** an operator reviews the audit stream, **Then**
   they can see the time, the claimed identity (if any), the reason for refusal, and the
   correlation ID — with no secret material included.
6. **Given** a caller connecting to a callee, **When** the callee cannot prove its own
   identity to the caller, **Then** the caller refuses to send the request (verification
   is mutual).

---

### User Story 3 - Control Which Services May Call Which (Priority: P2)

A platform operator wants to declare, outside of application code, which services are
allowed to call which other services (and optionally which operations). A verified
identity is necessary but not sufficient: a service that is authenticated but not
permitted to reach a given callee is refused, and the decision is recorded.

**Why this priority**: Limits the blast radius of a compromised service. Delivers real
value once Stories 1–2 exist but the framework is still usable (deny-by-default with an
explicit allow-all policy for development) without it.

**Independent Test**: Deploy three services A, B, C with a policy that allows A→B only.
A→B succeeds; A→C and B→A are refused with an authorization error and an audit record.
Changing the policy to also allow A→C takes effect without redeploying A or C.

**Acceptance Scenarios**:

1. **Given** a policy allowing A→B, **When** A calls B, **Then** the call succeeds.
2. **Given** a policy allowing A→B only, **When** A calls C, **Then** C refuses with an
   authorization error distinct from an authentication error.
3. **Given** no policy is configured, **When** any service calls another, **Then** the
   call is refused (deny by default) and startup logs explain that no policy is present.
4. **Given** a policy update, **When** it is applied, **Then** subsequent calls reflect
   the new rules without restarting the affected services.

---

### User Story 4 - Rotate Identities Without Downtime (Priority: P2)

A platform operator needs service identities to be short-lived and renewed automatically,
so that a leaked credential has a bounded useful lifetime. Renewal must be invisible to
service developers and must not interrupt in-flight or new calls.

**Why this priority**: Short-lived credentials are the primary mitigation against
credential theft, but the framework is usable (with long-lived identities) before this
exists.

**Independent Test**: Run two services with identities that expire every few minutes
under a continuous stream of calls. Over several expiry cycles, observe zero failed calls
caused by renewal and confirm each service is presenting a fresh identity after each
cycle.

**Acceptance Scenarios**:

1. **Given** a service whose identity is approaching expiry, **When** the renewal window
   is reached, **Then** the framework obtains a new identity before the old one expires.
2. **Given** a renewal in progress, **When** calls are made concurrently, **Then** no call
   fails due to the renewal.
3. **Given** renewal cannot be completed (identity provider unavailable), **When** the
   current identity expires, **Then** the service stops accepting and making calls,
   reports unhealthy, and logs the cause — rather than continuing with an expired or
   absent identity.
4. **Given** any renewal event, **When** an operator reviews the audit stream, **Then**
   the renewal (success or failure) is recorded.

---

### User Story 5 - Observe Secure Calls (Priority: P3)

A service developer or operator investigating an incident wants to trace a request across
several services, see the verified caller and callee identities at each hop, and see
timing and outcome — all without any secret material ever appearing in logs or traces.

**Why this priority**: Essential for operations and forensics but does not change whether
the channel is secure.

**Independent Test**: Make a call that traverses A→B→C. Using only the emitted logs and
traces, reconstruct the full path with one correlation ID, the identity at each hop, and
the outcome, and confirm by inspection that no credential, key, or token appears anywhere.

**Acceptance Scenarios**:

1. **Given** a multi-hop call, **When** logs are queried by correlation ID, **Then** every
   hop is returned with caller identity, callee identity, outcome, and duration.
2. **Given** any log, trace, or metric emitted by the framework, **When** searched for
   credential material, **Then** none is found.
3. **Given** a running service, **When** an operator queries metrics, **Then** counts of
   successful calls, authentication refusals, and authorization refusals are available
   per peer service.

---

### Edge Cases

- **Clock skew**: A caller and callee whose clocks differ by more than the tolerated
  skew treat a valid identity as expired or not-yet-valid; the framework applies a bounded
  tolerance (default 5 min) and the refusal's audit event carries the skew hint in
  `attrs.detail` (the closed `reason` vocabulary stays `identity_expired` /
  `identity_not_yet_valid`).
- **Identity provider unavailable at startup**: The service MUST NOT start serving; it
  reports why and retries with backoff.
- **Mixed versions**: A newer caller and older callee (or vice versa) with different
  supported protocol versions negotiate the strongest common version or refuse; they never
  silently downgrade below the minimum allowed.
- **Peer address changes**: A callee restarting on a new network address is still reachable
  by logical name and still verified by identity, not by address.
- **Oversized or slow requests**: Default size and timeout limits apply to secure calls
  exactly as to any other; a peer that exceeds them is disconnected and the event recorded.
- **Half-open connections**: A caller whose peer disappears mid-call receives a clear
  timeout/disconnect error within the configured deadline, not an indefinite hang.
- **Local development**: A developer running on a single machine can opt into an
  explicitly named insecure/self-issued mode; doing so prints a startup warning and cannot
  be the default.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The framework MUST allow a service to call another service by logical name,
  with the framework resolving the peer and establishing the secure channel without
  security-specific code in the caller.
- **FR-002**: Every service-to-service channel MUST be encrypted such that a passive
  observer cannot recover payloads, headers, or metadata beyond what is inherent to the
  network (addresses, sizes, timing).
- **FR-003**: Every channel MUST be integrity-protected such that any modification in
  transit is detected and the affected message rejected.
- **FR-004**: Both parties to a channel MUST prove their identity to the other before any
  request is delivered (mutual verification).
- **FR-005**: The framework MUST refuse connections from peers that present no identity,
  an expired identity, a revoked identity, an identity from an untrusted authority, or an
  identity that does not match the service name the peer claims.
- **FR-006**: The framework MUST expose the verified identity of the peer to the receiving
  service's code in a read-only form.
- **FR-007**: The framework MUST evaluate an authorization policy (which caller identities
  may reach which callee services/operations) on every inbound call, before handler code
  runs, and MUST deny by default when no policy grants access.
- **FR-008**: The framework MUST distinguish authentication failures from authorization
  failures in errors returned to callers and in audit records, without revealing internal
  detail that would help an attacker.
- **FR-009**: Policy MUST be definable outside application code and MUST be reloadable at
  runtime without restarting services.
- **FR-010**: Service identities MUST be time-limited and the framework MUST renew them
  automatically before expiry, without interrupting in-flight or new calls.
- **FR-011**: The framework MUST obtain identities from a pluggable identity provider
  interface so that different organisations can integrate their existing trust authority.
- **FR-012**: If a valid identity cannot be obtained or renewed, the service MUST refuse
  to make or accept calls and MUST report itself unhealthy.
- **FR-013**: The framework MUST record every authentication refusal, authorization
  refusal, identity issuance/renewal, and insecure-mode opt-in to the audit stream with a
  correlation ID and with no secret material.
- **FR-014**: The framework MUST propagate a correlation ID across every hop of a
  multi-service call.
- **FR-015**: The framework MUST emit per-peer metrics for successful calls,
  authentication refusals, and authorization refusals.
- **FR-016**: Secure communication MUST be the default; any insecure or self-issued mode
  MUST require an explicit, named opt-in and MUST emit a startup warning.
- **FR-017**: The framework MUST apply default request size, timeout, and concurrency
  limits to secure channels and MUST record limit violations.
- **FR-018**: Peers with incompatible protocol versions MUST negotiate the strongest
  mutually supported version at or above the framework's minimum, or refuse to connect;
  silent downgrade below the minimum MUST be impossible.

### Security Requirements *(mandatory — Constitution: Development Workflow)*

- **Trust boundaries crossed**: service-to-service network (assumed hostile even inside
  the cluster/VPC); service ↔ identity provider; service ↔ policy source.
- **Data classification**: credentials and private key material (secret, never logged);
  service identities (internal); request/response payloads (as classified by the calling
  service — may include PII; the channel must protect them regardless).
- **Authentication/Authorization**: cryptographically verified per-service identity issued
  by an organisation-trusted authority, verified mutually on every channel; deny-by-default
  policy engine evaluated per call in framework middleware.
- **Threat scenarios**: eavesdropping on the service network; on-path tampering or replay;
  impersonation of a callee (rogue endpoint) or caller (stolen/forged identity); lateral
  movement from a compromised service to peers it should not reach; protocol downgrade;
  credential leakage through logs or error messages; denial of service via oversized or
  slow requests; identity-provider outage forcing services into an insecure fallback.
- **SR-001**: System MUST reject any request lacking a valid, unexpired, trusted,
  name-matching service identity before any handler code executes.
- **SR-002**: System MUST never write private keys, credentials, tokens, or session
  secrets to logs, traces, metrics, error responses, or configuration files.
- **SR-003**: System MUST fail closed: loss of identity, policy source, or trust anchors
  results in refusal to communicate, never in an unauthenticated or unencrypted fallback.
- **SR-004**: System MUST make protocol or cipher downgrade below the configured minimum
  impossible, regardless of what a peer offers.
- **SR-005**: System MUST bound the useful lifetime of any single stolen identity to the
  configured identity validity period (default: at most 1 hour).
- **SR-006**: System MUST produce audit records, with a stable documented schema, for
  every event enumerated in FR-013 and for every limit violation (FR-017).
- **SR-007**: Error responses to peers MUST NOT reveal trust-store contents, policy
  details, internal addresses, or software versions.

### Key Entities

- **Service Identity**: A cryptographically verifiable statement that a running instance
  is a specific named service, issued by a trusted authority, with a validity window.
  Attributes: service name, issuing authority, not-before, not-after, revocation status.
- **Trust Anchor Set**: The set of authorities a service accepts identities from.
  Attributes: authority identifiers, validity, source.
- **Secure Channel**: An established, mutually verified, encrypted, integrity-protected
  session between two service instances. Attributes: caller identity, callee identity,
  negotiated protocol version, establishment time.
- **Authorization Policy**: A set of rules stating which caller identities may reach which
  callee services and operations. Attributes: rule list, version, source, load time.
- **Audit Event**: An immutable record of a security-relevant occurrence. Attributes:
  timestamp, event type (authn refusal, authz refusal, identity issued/renewed/failed,
  insecure opt-in, limit violation), claimed/verified identities, reason, correlation ID.
- **Correlation ID**: An opaque identifier attached to a request at its origin and carried
  through every hop.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A secure call between two framework-built services requires only the peer
  name and the payload: the caller and callee contain zero security-specific statements
  (verified by inspection of `examples/two-services`).
- **SC-002**: 100% of connection attempts lacking a valid, trusted, name-matching identity
  are refused before any handler runs, across the full negative test matrix (no identity,
  expired, revoked, untrusted authority, name mismatch, downgrade attempt).
- **SC-003**: A network observer with full packet capture between two services cannot
  recover any payload or header content; independent review confirms no plaintext
  application data leaves either process.
- **SC-004**: Under continuous load spanning at least 10 identity renewal cycles, zero
  calls fail because of renewal.
- **SC-005**: An authorization policy change takes effect on all affected services within
  30 seconds without a restart.
- **SC-006**: Compared with a plaintext gRPC call that runs the same non-security
  middleware (recover, correlation, tracing, instrumentation) on the same hardware, secure
  calls add no more than 15% latency at the median for sequential calls and no more than
  25% per-call cost under 8-way concurrency over one pooled connection; channel
  establishment is paid once per peer, not per call. (Measured 2026-09-15: 1.12× and
  1.22×; see quickstart-results.md §9.)
- **SC-007**: For any multi-hop request, an operator can reconstruct the complete path,
  identities, and outcomes from emitted logs/traces using only the correlation ID, in
  under 5 minutes.
- **SC-008**: Automated scanning of all logs, traces, metrics, and error responses
  produced by the full test suite finds zero occurrences of credential or key material.
- **SC-009**: Every security-relevant refusal or identity lifecycle event produces exactly
  one audit record conforming to the documented schema (100% coverage in tests).

## Assumptions

- **Communication style**: v1 covers synchronous request/response calls between services.
  Asynchronous messaging (queues, pub/sub) over a secure channel is out of scope for this
  feature and will be specified separately.
- **Identity issuance is external**: The framework consumes identities from an
  organisation-operated trust authority through a pluggable provider interface; it does
  not itself act as the root authority. A self-issued mode exists only for explicitly
  opted-in local development.
- **Service discovery**: Resolving a logical service name to a network location is a
  framework concern but its mechanism (static config, DNS, registry) is a planning
  decision; this feature only requires that identity — never address — is what is
  verified.
- **Policy source**: Authorization policy is provided as declarative configuration
  (file or configuration service). The policy language and its expressiveness (per-service
  vs per-operation granularity) is a planning decision; per-service granularity is the
  minimum.
- **End-user identity**: Propagating an end user's identity (as opposed to the calling
  service's identity) across hops is out of scope here and will be a separate feature.
- **Environment**: Services run on a network assumed to be hostile (zero trust); no
  reliance on network segmentation, firewalls, or a service mesh sidecar is assumed.
- **Address changes**: A callee that restarts on a new address stays reachable by name
  only when a registry (`freya.WithDiscovery`) is used; `discovery.static` describes a
  fixed topology and is re-read only at startup.
- **Protocol versions**: v1 offers TLS 1.3 only (`MinVersion = MaxVersion`); FR-018's
  "negotiate the strongest mutually supported version" therefore reduces to "refuse
  anything else". The constitution's TLS 1.2 opt-out is deferred to a later MINOR.
- **Certificate granularity**: X.509 validity is expressed in whole seconds, so identity
  lifetimes below a few seconds are not meaningful (tests use ≥ 3 s).
- **Identity validity**: Default identity lifetime is 1 hour with renewal beginning at
  half-life, per the constitution's token lifetime requirement.
- **Constitution alignment**: This feature is governed by Principles I (Secure by
  Default), II (Zero Trust), IV (Test-First), and V (Observability) of the project
  constitution v1.0.0; planning MUST include a STRIDE threat model.
