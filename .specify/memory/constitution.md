<!--
Sync Impact Report
==================
Version change: (template, unversioned) → 1.0.0
Bump rationale: Initial ratification. All placeholder tokens replaced with concrete,
security-focused principles for the Freya microservice framework (MAJOR: first
binding governance).

Modified principles: N/A (initial fill)

Added sections:
- Core Principles (I–VII):
  I.   Secure by Default
  II.  Zero Trust Service Communication
  III. Boundary Validation & Defense in Depth
  IV.  Test-First with Security Verification (NON-NEGOTIABLE)
  V.   Observability & Auditability
  VI.  Supply Chain Integrity & Minimal Dependencies
  VII. Simplicity & Explicit Configuration
- Security Requirements (cryptography, secrets, transport, identity, data handling)
- Development Workflow & Quality Gates

Removed sections: none

Templates requiring updates:
- ✅ .specify/templates/plan-template.md (Constitution Check gates filled in)
- ✅ .specify/templates/tasks-template.md (tests made mandatory per Principle IV;
     security-review task added to Polish phase)
- ✅ .specify/templates/spec-template.md (Security Requirements subsection added)
- ✅ .claude/skills/speckit-tasks/SKILL.md ("tests optional" → mandatory per Principle IV)
- ✅ .claude/skills/speckit-*/SKILL.md (other skills reviewed; no agent-specific references to fix)

Follow-up TODOs: none. No runtime guidance docs (README.md, docs/) exist yet; when
created they MUST reference this constitution.
-->

# Freya Constitution

Freya is a Go framework for building microservices where security is the primary design
driver, not an add-on. Every decision in this project is evaluated first against the
question: "Does this make services built with Freya harder to compromise?"

## Core Principles

### I. Secure by Default

The framework MUST ship with the most secure configuration as the default. Insecure
options (plaintext transport, disabled authentication, permissive CORS, verbose error
bodies, debug endpoints) MUST require an explicit, named opt-out (e.g.
`WithInsecureTransport()`), and every opt-out MUST log a warning at startup. A service
built with zero configuration MUST refuse to start rather than start insecurely. Defaults
MUST be reviewed against the current OWASP ASVS and CWE Top 25 at every MINOR release.

Rationale: most breaches come from misconfiguration, not novel exploits. Making the safe
path the path of least resistance removes the largest class of real-world failures.

### II. Zero Trust Service Communication

No request is trusted because of its network origin. Every inbound and outbound call
between services MUST be mutually authenticated (mTLS with per-service identities, or an
equivalent cryptographically verified identity such as SPIFFE/SVID) and MUST be
authorized against an explicit policy before any handler code runs. Service identity,
not IP address or network segment, is the unit of trust. Authorization decisions MUST be
enforced in framework middleware so that a handler cannot be reached without passing
them; handlers MUST NOT be able to bypass or disable this middleware. Credentials
(tokens, certificates) MUST be short-lived and rotated automatically by the framework.

Rationale: microservice meshes are flat networks with many hops; a single compromised
pod must not grant lateral movement.

### III. Boundary Validation & Defense in Depth

All data crossing a trust boundary (HTTP/gRPC requests, message-queue payloads,
environment variables, config files, database rows from shared stores) MUST be validated
against an explicit schema before use, and validation MUST be declared in the contract
(OpenAPI/Protobuf) so it is enforced by the framework, not hand-written per handler.
Output encoding MUST be context-aware and applied by the framework. Every layer
(transport, middleware, handler, data access) MUST enforce its own controls and MUST NOT
assume an outer layer has already done so. Request size, rate, concurrency, and timeout
limits MUST be enforced by default at the framework edge.

Rationale: injection and deserialization flaws remain the most common exploitable bugs;
one layer of validation is one bug away from failure.

### IV. Test-First with Security Verification (NON-NEGOTIABLE)

TDD is mandatory: tests are written, reviewed, and confirmed failing before
implementation begins; the Red-Green-Refactor cycle is strictly enforced. In addition to
functional tests, every feature that touches authentication, authorization, input
parsing, cryptography, secrets, or transport MUST include negative security tests
(malformed input, missing/expired/forged credentials, privilege escalation attempts,
oversized payloads). Contract tests MUST exist for every public API surface. Fuzz tests
MUST exist for every parser and decoder. Line coverage MUST be ≥ 80% overall and 100%
for packages under `security/`, `auth/`, and `crypto/` (or their equivalents). No PR may
merge with a failing, skipped, or flaky test.

Rationale: security properties that are not tested regress silently; tests are the
executable specification of what "secure" means for this framework.

### V. Observability & Auditability

Every service MUST emit structured (JSON) logs, distributed traces, and metrics via the
framework with no per-service setup. Security-relevant events (authn success/failure,
authz denial, rate-limit trigger, validation rejection, config opt-outs, credential
rotation) MUST be logged to an append-only audit stream with a stable schema. Logs MUST
NOT contain secrets, credentials, tokens, or unmasked PII; the framework MUST provide
and enforce redaction. Every request MUST carry a correlation ID propagated across
service hops. Health, readiness, and metrics endpoints MUST be served on a separate,
non-public listener.

Rationale: detection and forensics are impossible without a trustworthy, complete,
and secret-free record of what happened.

### VI. Supply Chain Integrity & Minimal Dependencies

Every direct dependency MUST be justified in writing (purpose, alternatives rejected,
maintenance status) and the standard library MUST be preferred when it is adequate.
Dependencies MUST be pinned by checksum (`go.sum` committed, `GOFLAGS=-mod=readonly`),
scanned for known vulnerabilities (`govulncheck`) on every CI run, and reviewed on every
version bump. Releases MUST be reproducible, signed, and accompanied by an SBOM. Vendored
or forked code MUST carry its upstream commit hash and license. Cryptographic
primitives MUST come only from the Go standard library or `golang.org/x/crypto`;
custom or novel cryptography is prohibited.

Rationale: the framework is a dependency of every service built on it; its attack
surface is inherited by all of them.

### VII. Simplicity & Explicit Configuration

Start with the simplest design that satisfies the security requirements; YAGNI applies.
Configuration MUST be explicit, typed, and validated at startup — no reflection-based
magic, no implicit environment-variable discovery, no runtime config mutation. Every
public API MUST be documented with its security implications. Complexity that reduces
auditability (deep middleware chains, code generation that hides control flow, global
mutable state) MUST be justified in the plan's Complexity Tracking table or rejected.
Breaking changes to public APIs follow semantic versioning and MUST ship with a
migration guide.

Rationale: code that cannot be understood cannot be audited; simplicity is a security
control.

## Security Requirements

These constraints apply to the framework and to every service built with it:

- **Transport**: TLS 1.3 minimum; TLS 1.2 permitted only via explicit opt-out with an
  approved cipher list. HTTP/2 and gRPC MUST use ALPN. Plaintext listeners are
  prohibited outside `localhost` development mode.
- **Cryptography**: Only AEAD ciphers (AES-GCM, ChaCha20-Poly1305) for symmetric
  encryption; Ed25519 or ECDSA P-256 for signatures; Argon2id for password hashing;
  HKDF for key derivation. Key material MUST never be logged, serialized to config, or
  held in memory longer than needed. The framework MUST expose a single `crypto`
  package so services never call primitives directly.
- **Secrets**: Secrets MUST be loaded from a secrets provider interface (file mount,
  Vault, cloud KMS) at startup, never from CLI flags or hard-coded values. The
  framework MUST refuse to start if a secret is found in a config file or environment
  variable that is marked sensitive.
- **Identity & Authorization**: The framework MUST provide first-class support for
  JWT/OIDC (with mandatory `aud`, `iss`, `exp` validation and algorithm allow-lists),
  mTLS identity, and a pluggable policy engine. Deny-by-default is the only permitted
  policy default. Tokens MUST have a maximum lifetime of 1 hour unless justified.
- **Data Handling**: PII fields MUST be annotated in schemas; the framework MUST honor
  those annotations for redaction in logs and for encryption at rest hooks. Error
  responses to callers MUST NOT include stack traces, internal paths, or dependency
  version strings.
- **Resource Protection**: Default limits — request body 1 MiB, header 8 KiB, request
  timeout 30 s, idle timeout 60 s, max concurrent streams per connection 100 — apply
  unless explicitly overridden with a documented reason.
- **Compliance Baseline**: OWASP ASVS Level 2 is the minimum verification target for
  framework releases; Level 3 for `auth/` and `crypto/` packages.

## Development Workflow & Quality Gates

- **Specification**: Every feature spec MUST include a Security Requirements
  subsection identifying trust boundaries, data classification, and threat scenarios
  before planning begins.
- **Planning**: Every plan MUST pass the Constitution Check gate (all seven principles)
  before Phase 0 research and again after Phase 1 design. Violations MUST be recorded in
  the Complexity Tracking table with justification or the plan is rejected.
- **Threat Modeling**: Features touching authentication, authorization, transport,
  parsing, or secrets MUST include a lightweight STRIDE threat model in `research.md`.
- **Code Review**: Every PR requires at least one approving review. PRs touching
  `security/`, `auth/`, `crypto/`, or transport code require a second reviewer with
  security ownership. Reviewers MUST explicitly confirm constitution compliance in the
  PR description checklist.
- **CI Gates** (all blocking): `go build`, `go vet`, `staticcheck`, `gosec`,
  `govulncheck`, unit + contract + fuzz tests, coverage thresholds from Principle IV,
  license check, and reproducible-build verification. A red gate cannot be overridden by
  merge permissions.
- **Release**: Releases are tagged with semantic versions, signed, and published with an
  SBOM and a changelog that lists every security-relevant change and every default that
  changed. Security fixes ship as PATCH releases within 72 hours of confirmed
  vulnerability for supported versions.
- **Vulnerability Disclosure**: A `SECURITY.md` with a private reporting channel MUST
  exist at the repository root before the first public release.

## Governance

This constitution supersedes all other development practices, style guides, and
conventions in this repository. Where a template, script, or guidance document
conflicts with this constitution, the constitution wins and the conflicting artifact
MUST be corrected.

- **Amendments**: Any change to this document requires a pull request that (1) states
  the proposed change and its rationale, (2) includes the version bump per the policy
  below, (3) updates every dependent template listed in the Sync Impact Report, and (4)
  is approved by at least one maintainer with security ownership. Amendments that
  weaken a security control MUST additionally document the compensating control.
- **Versioning Policy**: MAJOR for removing or redefining a principle or weakening a
  security requirement; MINOR for adding a principle/section or materially expanding
  guidance; PATCH for clarifications and wording that do not change obligations.
- **Compliance Review**: Every plan's Constitution Check and every PR's review checklist
  serve as the compliance record. Maintainers MUST audit a sample of merged PRs against
  this constitution at each MINOR release and record findings in the release notes.
- **Runtime Guidance**: Agent- and tool-specific runtime guidance (e.g. `CLAUDE.md`,
  `AGENTS.md`, `docs/`) MUST reference this constitution and MUST NOT introduce rules
  that contradict it.

**Version**: 1.0.0 | **Ratified**: 2026-09-15 | **Last Amended**: 2026-09-15
