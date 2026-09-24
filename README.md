# go-tangra v4 platform

`github.com/go-tangra/go-tangra/v4` is the platform of go-tangra: the Go
microservice framework every go-tangra service is built on, its optional contrib
modules, the shared web UI kit `@go-tangra/ui`, and the containerized dev stack
that runs the whole platform. The services themselves live in their own
`go-tangra-<name>` repositories and consume this module and the kit by version.

The framework (Go package `freya`) is built on
[go-kratos v3](https://github.com/go-kratos/kratos): every channel between
services is **mutually authenticated, encrypted, authorized, and audited by
construction**. A service developer supplies a logical peer name and a payload;
the framework supplies the security.

Governed by the [project constitution](.specify/memory/constitution.md)
(v1.0.0): secure by default, zero trust, defence in depth, test-first with
security verification, observability, supply-chain integrity, simplicity.

## What you get

| Concern | How the framework handles it |
|---------|------------------------------|
| Identity | X.509 SVIDs with SPIFFE IDs (`spiffe://<trust-domain>/svc/<name>`) from the SPIFFE Workload API (default), PEM files, or an explicit local-dev CA |
| Transport | TLS 1.3 only, client certificates required, no downgrade path; peers are verified by identity, never by address |
| Authorization | Deny-by-default policy (YAML/JSON), per service and per operation, hot-reloaded from a file or Valkey |
| Rotation | Short-lived identities renewed automatically; existing calls are never interrupted; an expired identity fails closed |
| Audit | Schema-stable events for every refusal and identity/policy lifecycle change; secrets can never reach logs |
| Observability | Correlation IDs across hops, OpenTelemetry traces and metrics, per-hop request log, separate admin listener |
| Limits | 1 MiB bodies, 8 KiB headers, 30 s requests, 100 streams/connection, 10 s handshakes — on by default |

## Repositories

| Repository | Contents | Image |
|---|---|---|
| [go-tangra/go-tangra](https://github.com/go-tangra/go-tangra) (this repo) | framework module, `contrib/*`, `@go-tangra/ui` kit, `deploy/stack` | — |
| [go-tangra-auth](https://github.com/go-tangra/go-tangra-auth) | tenants, identity, tokens, OpenFGA decisions, operator console; `sdk/` | `ghcr.io/go-tangra/go-tangra-auth` |
| [go-tangra-portal](https://github.com/go-tangra/go-tangra-portal) | application gateway: edge, module registration, shell host; `sdk/` | `ghcr.io/go-tangra/go-tangra-portal` |
| [go-tangra-lcm](https://github.com/go-tangra/go-tangra-lcm) | certificate lifecycle: mesh CA, SVID enrollment, ACME; `sdk/` | `ghcr.io/go-tangra/go-tangra-lcm` |
| [go-tangra-warden](https://github.com/go-tangra/go-tangra-warden) | secrets vault (HashiCorp Vault backed); `sdk/` | `ghcr.io/go-tangra/go-tangra-warden` |
| [go-tangra-notification](https://github.com/go-tangra/go-tangra-notification) | notifications and internal messages | `ghcr.io/go-tangra/go-tangra-notification` |
| [go-tangra-inventory](https://github.com/go-tangra/go-tangra-inventory) | hardware/software inventory with endpoint agents; `sdk/` | `ghcr.io/go-tangra/go-tangra-inventory` |
| [go-tangra-paperless](https://github.com/go-tangra/go-tangra-paperless) | document management | `ghcr.io/go-tangra/go-tangra-paperless` |
| [go-tangra-deployer](https://github.com/go-tangra/go-tangra-deployer) | certificate deployment to infrastructure targets | `ghcr.io/go-tangra/go-tangra-deployer` |
| [go-tangra-ipam](https://github.com/go-tangra/go-tangra-ipam) | IP address management and active network operations; `sdk/` | `ghcr.io/go-tangra/go-tangra-ipam` |
| [go-tangra-asset](https://github.com/go-tangra/go-tangra-asset) | IT asset management | `ghcr.io/go-tangra/go-tangra-asset` |
| [go-tangra-ticket](https://github.com/go-tangra/go-tangra-ticket) | email helpdesk | `ghcr.io/go-tangra/go-tangra-ticket` |
| [go-tangra-dns](https://github.com/go-tangra/go-tangra-dns) | PowerDNS management plane | `ghcr.io/go-tangra/go-tangra-dns` |

Services talk to each other only through the framework channel and the
published `sdk/v4` modules (`github.com/go-tangra/go-tangra-<name>/sdk/v4`).

## Versioning

Everything in go-tangra v4 follows semantic versioning on the `v4` major line:

- Go module `github.com/go-tangra/go-tangra/v4`, tags `vX.Y.Z` on this repo.
- Contrib modules `github.com/go-tangra/go-tangra/contrib/<name>/v4` (tags
  `contrib/<name>/vX.Y.Z`). In this repo they `replace` the root module with
  `../..`; consumers ignore that and resolve the published tag.
- `@go-tangra/ui` has the same version as the platform tag (`v4.0.0` publishes
  `@go-tangra/ui@4.0.0`); CI publishes it to GitHub Packages on every `v*` tag.
- Services are versioned independently (`go-tangra-<name>` tags `vX.Y.Z`, images
  `ghcr.io/go-tangra/<repo>:X.Y.Z`); service sdks are tagged `sdk/vX.Y.Z`.

## Consuming the platform from a service

Go:

```bash
GOWORK=off go get github.com/go-tangra/go-tangra/v4@v4.0.0
# optional contrib modules
GOWORK=off go get github.com/go-tangra/go-tangra/contrib/policy-valkey/v4@v4.0.0
```

Front-end (`@go-tangra/ui` is published to GitHub Packages, not npmjs.org):

```bash
printf '@go-tangra:registry=https://npm.pkg.github.com\n' > .npmrc   # commit this; no token in it
npm install --no-audit --no-fund "--//npm.pkg.github.com/:_authToken=$(gh auth token)" @go-tangra/ui@^4.0.0
```

GitHub Packages needs a token even for public packages: locally a token with
`read:packages` (e.g. `gh auth token`), in CI the workflow's `GITHUB_TOKEN`, which
works once the `@go-tangra/ui` package grants the service repository read access
("Manage Actions access" in the package settings). Docker builds receive it as a
BuildKit secret (`--secret id=npm_token,env=NODE_AUTH_TOKEN`) so it never lands in
an image layer.

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
transport/          Runtime contract; tlsconf/ (TLS 1.3 mTLS builder), grpc/, http/, edge/
authn/              Peer verification middleware, revocation checker
authz/              Policy model, loader, matcher, cache, middleware; file/ source
audit/              Event schema, redacting log handler, emitter
observe/            Correlation IDs, tracing, metrics, request log, admin listener
discovery/          Static name → endpoint map (any Kratos registry works too)
freyatest/          Public test helpers for services (test CA, SVIDs, fixtures)
cmd/freya-devca     Throw-away dev CA + SVIDs for the examples (`make testca`)
contrib/            Optional modules: audit-timescale, policy-valkey (own go.mod each)
examples/           two-services demo
tests/              contract, integration, fuzz
ui/kit              @go-tangra/ui: components, Zod forms, API client, theme, catalogue
ui/scripts          front-end static checks and their self-tests
deploy/stack        containerized dev stack running every service from ghcr.io images
docs/               security model, configuration, dependencies, front-end architecture
specs/              design history of the platform features (001, 013)
```

## Development

```bash
make lint vuln test cover     # Go gates (root module)
make fuzz                      # longer fuzzing of every parser
make redaction-scan            # SC-008: no secret material in any output
make bench                     # SC-006: mTLS overhead vs plaintext
(cd contrib/policy-valkey && GOWORK=off go test ./...)   # each contrib module on its own
npm ci && npm run build && npm run lint && npm test        # @go-tangra/ui kit
```

CI (`.github/workflows/ci.yaml`) runs `go vet` + `go test -race` for the root and
every contrib module with `GOWORK=off`, and build + lint + vitest for the kit.
Pushing a `v*` tag additionally publishes `@go-tangra/ui` (the tag must equal
`ui/kit/package.json`'s version). No container image is built for the platform.

## Running the whole platform

`deploy/stack` brings up every service from its published image
(`ghcr.io/go-tangra/<repo>:${TANGRA_VERSION:-4.0.0}`) plus the infrastructure,
bootstrapped by lcm as the CA:

```bash
OPERATOR_EMAIL=you@example.org deploy/stack/up.sh
```

See [deploy/stack/README.md](deploy/stack/README.md), including how to build one
service from a local checkout with `compose.override.yaml`.

## Documentation

- [docs/security-model.md](docs/security-model.md)
- [docs/configuration.md](docs/configuration.md)
- [docs/dependencies.md](docs/dependencies.md)
- [docs/frontend.md](docs/frontend.md)
- [SECURITY.md](SECURITY.md), [CHANGELOG.md](CHANGELOG.md)
