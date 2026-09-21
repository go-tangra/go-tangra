# Freya platform stack — quick setup

A one-command, fully containerized Freya platform, bootstrapped end to end by
**lcm** (the SPIFFE certificate authority). Every service runs in a container
and obtains its identity automatically; the only manual step is accepting the
first operator invite. Workstation credentials only — never use these outside a
laptop.

Services: `lcm` (CA), `auth` (identity/tokens), `gateway` (edge + module proxy),
`notification`, `warden` (secrets, backed by Vault), `deployer` (certificate
deployment to infrastructure targets — see `services/deployer/deploy/README.md`),
`paperless` (document management: S3 blobs, async text extraction, full-text
search, Zanzibar sharing — see `services/paperless/deploy/README.md`),
`inventory` (IT asset inventory: endpoint agents report hardware/software/network
snapshots to a central server with change tracking — see
`services/inventory/deploy/README.md`),
`ipam` (IP Address Management: subnets/IPs/devices/VLANs/locations/groups + active
network discovery scanning and out-of-band IPMI/KVM control — see
`services/ipam/deploy/README.md`),
`asset` (IT Asset Management: assets with an assign/unassign lifecycle, photos and
documents in RustFS, categories/locations/suppliers, consumables/licenses/insurance,
depreciation, lifecycle alerts and inventory-sync against `inventory` — see
`services/asset/deploy/README.md`).
Infra: TimescaleDB, Valkey,
OpenFGA, Mailpit, Vault, RustFS (object store), Tika + Gotenberg (extraction).

> Docker note: if your shell isn't in the active `docker` group, prefix commands
> with `sg docker -c '…'`.

## Bring it up (one command)

```sh
OPERATOR_EMAIL=you@example.org \
  docker compose -p freya-stack -f deploy/stack/compose.yaml up -d --build
# convenience wrapper (prints the operator accept link):
OPERATOR_EMAIL=you@example.org sg docker -c 'deploy/stack/up.sh'
```

This is idempotent (safe to re-run). It builds the images, then in order:

1. **infra** — TimescaleDB (+ `init-db.sql`), Valkey (ACL users), OpenFGA, Mailpit, Vault.
2. **`lcm-bootstrap`** — ensures the ONE DB-sealed **mesh root** and its default
   issuer, and mints the bootstrap SVIDs the control plane reads (`auth`).
3. **`gateway-bootstrap`** — seeds the gateway route allow-list (idempotent).
4. **`auth-bootstrap`** — seeds the platform tenant, roles, signing key and the
   first **operator invitation** (idempotent; reuses a pending invite, never
   double-sends).
5. **`vault-init`** — sets up warden's Vault KV mount, policy and AppRole.
6. **`*-token`** init jobs — mint each workload's single-use join token.
7. **services start**: `lcm` self-issues; `gateway`/`notification`/`warden`
   **enroll** over the network; `auth` reads its bootstrap SVID. All register
   with the gateway; the `renewer` keeps the file-based bootstrap certs fresh.

See **[ENROLLMENT.md](ENROLLMENT.md)** for how identities are issued and how to
enroll a new service.

## Sign in

The accept link is printed by `up.sh`, is in
`docker compose -p freya-stack logs auth-bootstrap`, and is emailed to Mailpit
(<http://localhost:8025>). Open it to set a password + TOTP for the operator,
then sign in at <https://localhost:8443> (accept the dev self-signed cert).

## Certificate lifetime / modes

- **Default:** SVIDs ~12 h, refreshed well before expiry, under a stable root.
- **Integrity mode** (short-lived SVIDs, proves non-disruptive rotation):
  ```sh
  CERT_TTL=5m RENEW_INTERVAL=210 \
    docker compose -p freya-stack -f deploy/stack/compose.yaml up -d --build
  bash deploy/stack/integrity-test.sh   # leaves rotate; root stays constant; leases hold
  ```

## Two kinds of certificate

lcm issues both:

1. **SVIDs** (`kind: svid`) — SPIFFE identities for mesh workloads
   (`spiffe://<trust-domain>/...`), used for service-to-service mTLS. This is
   what bootstrap and enrollment mint.
2. **Generic certificates** (`kind: generic`) — public/web certs for DNS names,
   obtained from an **ACME** issuer (Let's Encrypt-style, DNS-01).

Both appear in **Certificates** with a Kind chip; the "Request" dialog has an
**SVID (mesh)** tab and an **ACME / public** tab.

## ACME demo (Pebble)

The stack ships **Pebble**, a tiny test ACME CA, because no real CA is reachable
in the dev network. It runs with `PEBBLE_VA_ALWAYS_VALID=1`, so it skips the real
DNS-01 lookup and lcm's built-in **manual** DNS provider completes the order end
to end. `pebble-certs` mints Pebble a TLS cert whose SAN is `pebble`, and lcm
trusts it through `SSL_CERT_FILE` — TLS is verified, not disabled.

To issue a generic certificate from the UI:

1. **Issuers → New issuer**
   - Type **ACME**
   - Trust domain: `example.org`
   - ACME directory URL: `https://pebble:14000/dir`
   - DNS provider: **Manual / Out-of-band**
   - Save. (The ACME account key is generated server-side and sealed; you never
     handle key material.)
2. **Certificates → Request → ACME / public**
   - Pick the ACME issuer
   - Domains: e.g. `demo.example.com`
   - Request. A `kind: generic` certificate is issued by Pebble and listed.

Pebble's ACME directory is also exposed on the host at
<https://localhost:14000/dir> (self-signed; dev only).

## Reset / teardown

```sh
docker compose -p freya-stack -f deploy/stack/compose.yaml down -v   # wipes DB, CA, tokens, SVID state
```

## Troubleshooting

- **A module shows `registered:false` / `identity_not_allowed` / `prefix_not_granted`:**
  its SPIFFE id or route prefix isn't in the gateway allow-list. Re-apply:
  `bash deploy/stack/apply-allow.sh` (or `up -d --force-recreate gateway-bootstrap`).
  The allow-list is idempotent **per SPIFFE id** — to change an existing entry's
  prefixes you must update the `allow_list` row (see ENROLLMENT.md).
- **`lcm-bootstrap` (or another init) fails with `connection refused` to timescaledb on a *fresh* `up`:** a rare Postgres init-server race. The healthcheck is hardened (TCP probe) to prevent it; if you still hit it, just re-run `docker compose … up -d` — timescaledb is healthy by then and the idempotent init containers complete.
- **Disk fills up after many rebuilds** (`ENOSPC`): `docker builder prune -af`.
- **A restarted workload can't enroll:** its single-use token was already burned.
  With SVID persistence (a `*-state` volume) a restart reuses the stored SVID; a
  hard reset (`down -v`) clears state + mints a fresh token.
