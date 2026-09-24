# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report privately through GitHub's private vulnerability reporting on
[go-tangra/go-tangra](https://github.com/go-tangra/go-tangra/security/advisories/new)
(for a service, on that service's `go-tangra-<name>` repository). Include a
description, affected versions, and a proof of concept if you have one. You
will receive an acknowledgement within 2 business days.

## Response targets

Per the project constitution (Development Workflow & Quality Gates):

- Confirmed vulnerabilities in supported versions are fixed in a **PATCH release
  within 72 hours** of confirmation.
- The changelog entry names the vulnerability, the affected versions, and every
  default that changed.

## Supported versions

| Version | Supported |
|---------|-----------|
| 4.x     | yes       |
| < 4.0   | no        |

## Known, mitigated upstream issues

- **GO-2026-5471 / GHSA-jj45-xvq5-rhh9** (go-kratos `transport/http` falls back
  to `http.DefaultServeMux`): mitigated in `transport/http.NewServer`, which
  always installs explicit 404/405 handlers; covered by the contract test
  `TestHTTPServerNeverServesDefaultMux`. The allow-list entry in
  `scripts/vulncheck.sh` expires on 2026-12-31 and must be re-reviewed then.
- **go-spiffe `X509Source` data race** (`GetX509BundleForTrustDomain` reads
  without the mutex): Freya does not use `X509Source`; `identity/spiffe` watches
  the Workload API through `workloadapi.Client` with its own locking.
