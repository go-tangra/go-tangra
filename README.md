# Freya

Freya is a Go microservice framework, built on [go-kratos v3](https://github.com/go-kratos/kratos),
in which every channel between services is **mutually authenticated, encrypted,
authorized, and audited by construction**. A service developer supplies a
logical peer name and a payload; the framework supplies the security.

Governed by the [project constitution](.specify/memory/constitution.md)
(v1.0.0): secure by default, zero trust, defence in depth, test-first with
security verification, observability, supply-chain integrity, simplicity.

## What you get

| Concern | How Freya handles it |
|---------|----------------------|
| Identity | X.509 SVIDs with SPIFFE IDs (`spiffe://<trust-domain>/svc/<name>`) from the SPIFFE Workload API (default), PEM files, or an explicit local-dev CA |
| Transport | TLS 1.3 only, client certificates required, no downgrade path; peers are verified by identity, never by address |
| Authorization | Deny-by-default policy (YAML/JSON), per service and per operation, hot-reloaded from a file or Valkey |
| Rotation | Short-lived identities renewed automatically; existing calls are never interrupted; an expired identity fails closed |
| Audit | Schema-stable events for every refusal and identity/policy lifecycle change; secrets can never reach logs |
| Observability | Correlation IDs across hops, OpenTelemetry traces and metrics, per-hop request log, separate admin listener |
| Limits | 1 MiB bodies, 8 KiB headers, 30 s requests, 100 streams/connection, 10 s handshakes — on by default |

## Quickstart

```bash
make testca                                                     # dev CA + SVIDs in .dev/ca/
go run ./examples/two-services/inventory --config examples/two-services/inventory.yaml &
go run ./examples/two-services/orders    --config examples/two-services/orders.yaml
```

`orders` calls `inventory` over mTLS; `examples/two-services/policy.yaml` is the
only thing that permits it. The full validation guide is
[specs/001-secure-service-channel/quickstart.md](specs/001-secure-service-channel/quickstart.md).

## Using it

```go
cfg, err := config.Load("service.yaml")           // typed, validated at startup
app, err := freya.New(cfg)                         // refuses to start without identity + policy
inventoryv1.RegisterInventoryServer(app.GRPC(), svc)
go app.Run(ctx)

conn, err := app.Client(ctx, "inventory")          // pooled mTLS connection, callee verified by SPIFFE ID
peer, _ := authn.FromContext(ctx)                  // inside a handler: who is calling, verified
```

Options that weaken security (`WithInsecureLocalDev`, `WithAllowAllPolicy`) are
named, log a warning, emit an `insecure_mode_enabled` audit event, and are
refused when `env: production`.

## Repository layout

```
freya.go            App builder (config → identity → policy → servers)
config/             Typed configuration, YAML loader, secure defaults
identity/           SPIFFE IDs, Provider interface, lifecycle; providers: spiffe/, file/, localdev/
transport/          Runtime contract; tlsconf/ (TLS 1.3 mTLS builder), grpc/, http/
authn/              Peer verification middleware, revocation checker
authz/              Policy model, loader, matcher, cache, middleware; file/ source
audit/              Event schema, redacting log handler, emitter
observe/            Correlation IDs, tracing, metrics, request log, admin listener
discovery/          Static name → endpoint map (any Kratos registry works too)
contrib/            Optional modules: audit-timescale, policy-valkey
examples/           two-services demo
tests/              contract, integration, fuzz
```

## Development

```bash
make lint vuln test cover     # all CI gates
make fuzz                      # longer fuzzing of every parser
make redaction-scan            # SC-008: no secret material in any output
make bench                     # SC-006: mTLS overhead vs plaintext
```

See [docs/security-model.md](docs/security-model.md),
[docs/configuration.md](docs/configuration.md),
[docs/dependencies.md](docs/dependencies.md) and [SECURITY.md](SECURITY.md).
