# Freya contrib modules

Optional integrations, each an independent Go module so that its dependencies
never enter the core `github.com/go-freya/freya` graph.

| Module | Purpose | Extra dependency |
|--------|---------|------------------|
| `audit-timescale` | Durable audit sink (TimescaleDB hypertable) | `github.com/jackc/pgx/v5` |
| `policy-valkey` | Policy distribution + revocation denylist (Valkey) | `github.com/valkey-io/valkey-go` |

Both ship unit tests (run always) and `//go:build integration` tests that use
testcontainers; CI runs the latter with Docker available.
