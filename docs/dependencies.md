# Dependency Justification (Constitution Principle VI)

Every direct dependency of the core module is listed with its purpose, the alternatives
rejected, and its maintenance status. Contrib modules carry their own `go.mod` so their
dependencies never enter the core graph.

| Dependency | Purpose | Alternatives rejected | Maintenance |
|------------|---------|-----------------------|-------------|
| `github.com/go-kratos/kratos/v3` v3.0.0 | App lifecycle, gRPC/HTTP transports, middleware chain, registry and config abstractions | plain `net/http`+`grpc` (user chose Kratos), go-micro, go-zero (codegen hides control flow) | Active; v3.0.0 2026-06-26. Open advisory GO-2026-5471 mitigated in `transport/http` (see `scripts/vulncheck.sh`) |
| `google.golang.org/grpc` (via Kratos) | gRPC transport; `credentials.TLSInfo` for peer chains | none viable | Active (Google) |
| `github.com/spiffe/go-spiffe/v2` v2.8.1 | SPIFFE Workload API client (`workloadapi.Client` + watcher; `X509Source` avoided because of an unsynchronised bundle read), `spiffeid`, proto types for the test fake | Vault PKI client (deferred), hand-rolled Workload API client (re-implementing a standard) | Active (CNCF SPIFFE project). Uses only stdlib crypto |
| `go.opentelemetry.io/otel`, `otel/trace`, `otel/metric`, `otel/sdk`, `otel/sdk/metric` v1.46.0 | Traces (W3C propagation) and metrics; the SDK's `ManualReader` feeds a dependency-free OpenMetrics renderer | Prometheus client (extra dependency for exposition only), OpenCensus (deprecated) | Active (CNCF) |
| `google.golang.org/grpc` v1.82.1 (direct) | gRPC transport, `credentials.TLSInfo`, keepalive and size options | none viable | Active; upgraded from v1.81.0 for GO-2026-6061 |
| `google.golang.org/protobuf` v1.36.x | Generated example messages | none | Active (Google) |
| `gopkg.in/yaml.v3` (via Kratos) | Policy/config YAML decoding | `sigs.k8s.io/yaml` (heavier) | Maintenance mode but stable; already transitive via Kratos |
| stdlib `log/slog`, `crypto/tls`, `crypto/x509` | Structured logging, all cryptography | third-party loggers, custom crypto (prohibited) | Go project |

Go toolchain: `go 1.25.0` minimum, `toolchain go1.26.8` pinned (stdlib advisories
GO-2026-5972/6088/6089/6090/6091 are fixed in 1.26.6+).

## Test-only

| Dependency | Purpose |
|------------|---------|
| stdlib `testing` + `go test -fuzz` | Unit, table-driven, and fuzz tests; no assertion library needed |
| `github.com/testcontainers/testcontainers-go` (contrib only, `integration` tag) | Real TimescaleDB and Valkey in CI |

## Contrib-only (separate modules)

| Dependency | Module | Purpose |
|------------|--------|---------|
| `github.com/jackc/pgx/v5` | `contrib/audit-timescale` | TimescaleDB audit sink |
| `github.com/valkey-io/valkey-go` | `contrib/policy-valkey` | Policy distribution, pub/sub, revocation denylist |

## Adding a dependency

1. Add a row here with purpose, alternatives, and maintenance status.
2. Run `go mod tidy && go mod verify && make vuln`.
3. Reviewer with security ownership approves the PR.
