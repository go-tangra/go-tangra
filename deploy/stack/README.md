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
`services/asset/deploy/README.md`),
`ticket` (helpdesk: tickets, conversations with emailed replies via Mailpit, CEL
triage rules, tags, mailboxes, and an off-mesh inbound mail edge published on
`https://localhost:9957/inbound/mail` — see `services/ticket/deploy/README.md`),
`dns` (PowerDNS management plane: zones/records/templates/supermasters on the
shared `pdns-auth` server, forwarding through `pdns-recursor`, IPAM sync, the
Freya DNS ACME provider for lcm, server configuration with container restarts
via the Docker socket, dashboard — see `services/dns/deploy/README.md`).
Infra: TimescaleDB, Valkey,
OpenFGA, Mailpit, Vault, RustFS (object store), Tika + Gotenberg (extraction),
PowerDNS Authoritative 4.9 + Recursor 5.3 (optional Prometheus: `--profile metrics`;
optional test OpenLDAP for the auth directory import: `--profile ldap`).

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

### The browser certificate stays the same

`edge-cert-init` generates the gateway's browser certificate (`CN=localhost`,
SANs `localhost` + `127.0.0.1`, ~2 years) **once** into the `edge-cert` volume;
restarts and image rebuilds reuse it, so a browser exception you accepted keeps
working. It changes only after `down -v` (or deleting the `edge-cert` volume).

To stop seeing the warning at all, trust it once on the host:

```sh
docker compose -p freya-stack cp edge-cert-init:/edge/tls.crt ./freya-dev-edge.crt
# Linux (Chrome/Chromium use the NSS store):
certutil -d sql:$HOME/.pki/nssdb -A -t "P,," -n "Freya dev stack" -i ./freya-dev-edge.crt
# macOS: open the file in Keychain Access and set it to "Always Trust".
```

Only trust it on your own development machine; the private key lives in the
Docker volume.

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

## Web UI

Every module's UI is a federated remote on the shared kit `@freya/ui`
(`ui/kit`, FlyonUI + Zod — see `docs/frontend.md`). Images build the kit and the
module UI inside the Dockerfile's workspace stage, so a UI change needs an image
rebuild: `docker compose -p freya-stack -f deploy/stack/compose.yaml build <service>`
then `up -d <service>`. The shell lists a module in its navigation once the
module registers (`registered:true` in its health output); a remote built against
another kit major shows an error card with a retry in its own area only.

Browser flows (`services/<module>/ui/tests/e2e/*-flow.spec.ts`, `a11y.spec.ts`)
run against this stack with `E2E_OPERATOR_EMAIL` / `E2E_OPERATOR_PASSWORD`
(`PW_CHANNEL=chrome` to use the system Chrome).

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

## Ticket inbound mail (dev)

`ticket` publishes its inbound mail edge on the host (`:9957`, TLS with the same
dev edge certificate as the gateway). `ticket-secrets-init` generates the relay
token once into the `ticket-secrets` volume; replies and acknowledgements go to
Mailpit. After creating a mailbox (Tickets → Mailboxes, e.g. `support@example.org`):

```sh
TOKEN=$(docker compose -p freya-stack exec -T ticket cat /secrets/relay.token)
curl -sk https://localhost:9957/inbound/mail \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: message/rfc822" \
  -H "X-Iris-Recipient: support@example.org" \
  --data-binary @services/ticket/testdata/mail/plain.eml
# -> 202 {"outcome":"created","ticket_id":"…"}
```


## DNS (dev)

`dns-secrets-init` generates the PowerDNS and recursor API keys once into the
`dns-secrets` volume and writes the API include snippets for `pdns-auth` /
`pdns-recursor`; their HTTP APIs stay on the internal network. DNS is published
on **loopback only, on alternative ports** (host :53 is systemd-resolved,
:5353 is mDNS):

```sh
dig @127.0.0.1 -p 5300 example.test SOA        # authoritative (pdns-auth)
dig @127.0.0.1 -p 5301 www.example.test A      # resolver (pdns-recursor)
```

`dns` mounts `/var/run/docker.sock` **read-write** (`group_add: ${DOCKER_GID}`,
exported by `up.sh`) so a platform admin's Configuration save can restart
`freya-pdns-auth` / `freya-pdns-recursor` — and nothing else. The socket is
root-equivalent on the host; see the risk note in
`services/dns/deploy/README.md`. Prometheus for the DNS dashboard:
`docker compose -p freya-stack --profile metrics up -d prometheus` (without it
the dashboard shows "metrics unavailable").

## LDAP directory import (dev, optional profile `ldap`)

The `ldap` profile adds a seeded test OpenLDAP (`openldap`, built from
`services/auth/tests/integration/testdata/openldap/` — the same image the auth
integration tests use) for trying the auth console's Directories → import flow:

```sh
docker compose -p freya-stack -f deploy/stack/compose.yaml --profile ldap up -d --build openldap
```

`ldap-certs` mints a stack test CA + server certificate (SANs `openldap`,
`localhost`, `127.0.0.1`) once into the `ldap-tls` volume and writes the CA to
**`deploy/stack/ldap/ca.pem`** (git-ignored; the CA key is discarded, a new CA
only after `down -v`). Connection settings for the console:

| Field | Value |
|---|---|
| URL | `ldaps://openldap:636` (or `ldap://openldap:389` with StartTLS) |
| CA PEM | contents of `deploy/stack/ldap/ca.pem` |
| Bind DN / password | `cn=reader,dc=example,dc=test` / `reader-password` |
| Base DN | `ou=Engineering,dc=example,dc=test` |

`openldap` publishes no host ports and shares the isolated `ldap` network
(`172.31.250.0/29`, fixed address `172.31.250.2`) with `auth` only.
`configs/auth.yaml` keeps `directory.allow_plaintext: false`, refuses the Docker
bridge ranges (`deny_cidrs: 172.16.0.0/12`) and re-allows exactly
`172.31.250.2/32` — auth logs the matching `allow_cidrs` warning at startup.
Without the profile the `ldap` network still exists but is empty. The console
e2e `services/auth/console/tests/e2e/directory.spec.ts` uses these defaults
(Mailpit's UI is not published on the host here — point `E2E_MAILPIT_URL` at a
reachable Mailpit).
