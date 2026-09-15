# contrib/audit-timescale

Durable audit sink for Freya on TimescaleDB. Optional; the in-process slog sink
is always active and this module never blocks the call path.

```go
sink, err := timescale.New(ctx, os.Getenv("AUDIT_DSN"), timescale.Options{})
app, err := freya.New(cfg, freya.WithAuditSink(sink))
```

- Apply `migrations/001_audit_events.sql` with a migration role (`timescale.Migrate`).
- Give the service role `INSERT` only on `audit_events` (append-only).
- Use `sslmode=verify-full` in the DSN.
- Retention 90 days, compression after 7 days (see the migration).
- `go test -tags integration ./...` runs against a real TimescaleDB via testcontainers.
