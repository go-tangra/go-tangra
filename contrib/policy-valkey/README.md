# contrib/policy-valkey

Policy distribution and emergency revocation for Freya on Valkey. Optional.

```go
kv, err := valkey.NewClient(valkey.Config{Addresses: []string{"valkey:6379"}, TLS: tlsCfg, Username: "freya", Password: pw})
src, err := valkey.NewSource(ctx, kv, "example.org", 30*time.Second)
app, err := freya.New(cfg, freya.WithPolicySource(src), freya.WithRevocationChecker(valkey.NewRevocationChecker(kv)))
```

Keys (per trust domain `td`):

| Key | Contents |
|-----|----------|
| `freya:policy:<td>:doc` | policy document (YAML/JSON) |
| `freya:policy:<td>:version` | opaque version; must equal the document's `version` |
| `freya:policy:changed` (channel) | publish the new version to trigger reloads (< 2 s) |
| `freya:revoked:<spiffe-id>` / `freya:revoked:<spiffe-id>#<serial>` | denylist entries with TTL ≤ identity lifetime |

TLS is required (`AllowPlaintext` exists for local development only). Give the
service ACL user read + subscribe only. `go test -tags integration ./...` runs
against a real Valkey via testcontainers.
