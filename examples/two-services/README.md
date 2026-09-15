# Two services over a secure channel

```bash
make testca                                                     # dev CA + SVIDs in .dev/ca/
go run ./examples/two-services/inventory --config examples/two-services/inventory.yaml &
go run ./examples/two-services/orders    --config examples/two-services/orders.yaml
```

`orders` calls `inventory.v1.Inventory/Reserve` by logical name. Both sides
present SPIFFE identities from `.dev/ca/`, the channel is TLS 1.3 mTLS, and the
policy in `policy.yaml` is the only thing that permits the call. Try removing the
rule and calling again: `orders` gets `PERMISSION_DENIED`, `inventory` emits an
`authz_refused` audit event.
