# Changelog

## 0.2.0 — unreleased

### Added

- `services/dns`: PowerDNS management-plane module (spec `015-dns-service`,
  go-tangra-dns replica): tenant-owned zones on one shared PowerDNS
  Authoritative server with global name ownership and cross-tenant overlap
  refusal (owner never revealed), per-type record validation (miekg/dns),
  record sets with multi-value/disabled/comment editing, zone templates with
  `[ZONE]` placeholders, supermasters (create/delete platform-admin only), BIND
  export and NOTIFY, recursor forward-zone reconciliation, **IPAM sync**
  (verified `ipam.ip_address.*` events → A/AAAA + subnet-sized PTR zones), the
  lcm-only `dns.v1.Challenges` ACME DNS-01 surface plus the **Freya DNS**
  provider in lcm, platform-admin server configuration (typed model, rendered
  include files, restart-only Docker socket client limited to the two PowerDNS
  containers), a curated Prometheus dashboard (no PromQL from the browser), SSE
  live updates, tenant backup (zones re-linked, never re-created), `dns.v1.Zones`
  + `pkg/dnsclient` for other modules and a federated UI remote. Wired into
  `deploy/stack` (DB/role, Valkey user, allow-list `svc/dns=/api/dns;dns`,
  `pdns-auth` 4.9 + `pdns-recursor` 5.3 pinned by digest with internal-only APIs,
  DNS on loopback :5300/:5301, dev `file:` API keys from `dns-secrets-init`,
  optional `metrics` Prometheus profile).

- `services/ticket`: helpdesk module (spec `014-ticket-service`, go-tangra-ticket
  replica): tenant-scoped tickets with status/priority/assignee, conversation
  timeline (internal notes + emailed public replies with RFC 5322 threading
  headers and a reference token), an off-mesh **inbound mail edge** (`:9957`,
  relay token + mailbox → tenant routing, iris/KumoMTA-compatible headers) with an
  RFC 822/MIME parser, email threading back into tickets, loop-safe RFC 3834
  auto-acknowledgements, sandboxed CEL triage rules (cost limit + deadline),
  tags, mailboxes, history, statistics, SSE live updates, tenant backup,
  server-side HTML sanitisation (bluemonday) shown in a sandboxed `srcdoc`
  iframe, attachments in RustFS, `ticket.v1` gRPC + `pkg/ticketclient` for other
  modules, and a federated UI remote. Wired into `deploy/stack` (DB/role, Valkey
  user, allow-list `svc/ticket=/api/ticket;ticket`, Mailpit relay, dev relay token).
  Coverage 93 % (authz/sealed/secrets/thread and the rules compile path 100 %).

- `services/ipam` feature parity with go-tangra-ipam: IP/host group CRUD with
  member editing (new `PUT /host-groups/{id}/members/{mid}`), manual subnet
  create/edit/add-child, subnet split (`POST /subnets/{id}/split`, dry-run
  preview, skips taken blocks), location CRUD with typed locations and a rack
  elevation (device placement by unit/height, overlap + fit checks), device
  create/edit with rack fields. Subnet overlap checks are now hierarchy-aware
  (a child must sit inside its parent; ancestors/descendants are not overlaps).
- Right-hand drawers replace centred dialogs for create/edit forms and record
  views across all modules (subnets get a go-tangra-style detail drawer that
  carries scan/split/add-child/edit/delete); only yes/no confirmations stay
  dialogs. `UiRecordDrawer` gains `close-on-save`; drawers no longer dim the page.
- `@freya/ui/vite` `breakpointSpecificity()` build plugin, used by every remote:
  responsive utilities outrank plain ones regardless of which module stylesheet
  loads last (fixes layouts collapsing depending on remote load order).

- `ui/kit` (`@freya/ui`), every front-end (spec `013-flyonui-frontend-rework`):
  one shared component kit on FlyonUI 2 / Tailwind 4 with Zod 4 form
  validation replaces Vuetify in the gateway shell, the auth console and the
  eight module remotes. Kit: 60 components, `useZodForm` / `zodToFields` /
  `UiRecordDialog` form recipe, `createApi` transport, two themes, a catalogue
  with screenshot + axe baselines at 320/768/1280, coverage gate (forms/api
  100 %). Remotes carry only their own code (`import: false` singletons) —
  shell + asset bundles are 58 % of the Vuetify baseline. Static checks
  (`check-duplicates`, `check-no-legacy`, `check-bundle-size`) with self-tests;
  Vuetify, `vite-plugin-vuetify` and `@mdi/font` are gone from the lockfile.
- `services/notification`: notification & messaging module (spec `006-notification-service`):
  multi-channel channels (email over SMTP; sms/slack/sse declared) with
  encrypted settings, Go-template rendering, a notification log, Zanzibar-style
  access with `use`, internal messages with a scheduler and a per-user inbox, a
  gateway-relayed live SSE stream with a header bell, tenant backups, statistics
  and audit; `notification.v1` gRPC for services and a federated remote.
- `services/warden`: credential vault module (spec `005-warden-secrets`):
  folders, versioned secrets with material in HashiCorp Vault KV v2 and
  references in TimescaleDB, Zanzibar-style grants evaluated in SQL, Bitwarden
  import/export, tenant backups, statistics, health, password generator and
  external email shares; federated remote for the platform shell.
- `services/gateway`: manifest routes may set `client_address: true`; the
  dispatcher then forwards the client IP as `X-Gateway-Client-Addr` and every
  inbound `X-Gateway-*` header is dropped (`gatewayclient.Route.ClientAddress`).
- `services/auth`: member-level `GET /api/v1/users?q=` (public profile search)
  and `GET /api/v1/roles` (slug + display name) for subject pickers;
  `Authorization/RegisterPermissions` accepts `builtin_grants` so a module
  can grant its own permissions to built-in roles.
- `services/warden`: share links carry the token in the URL fragment
  (`/warden/share#<token>`) so it never reaches a request log; transfer and
  backup routes declare a 120 s gateway timeout.
- `services/gateway/shell`, `services/auth/console`: Materio design system
  (palette, Inter, detached top bar, module-grouped navigation menus, light and
  dark themes with a switch); the sign-in page follows it.
- `services/warden`: the Secrets and Folders views merge into one explorer
  (folder pane with new/rename/move/delete on the left, breadcrumb, subfolders
  and secrets on the right); `/warden/folders` redirects to `/warden`
  (manifest 1.1.0).
- `services/warden`: the audit trail and permission views show names: audit
  reads resolve secret, folder and share subjects (`subject_name`), and the UI
  resolves user ids and role slugs through the auth module (batch profile
  lookup, role list) for audit actors, grants and effective-access sources.

- `transport/edge`: browser-facing TLS 1.3 listener (server auth only) with
  security headers and CSP nonces, CSRF double-submit + Origin checks, per-IP
  and per-route token buckets, body limits, certificate hot reload and a
  development self-signed certificate outside production.
- `services/auth`: tenant authentication and authorization service (spec
  `002-tenant-auth-service`) with its console, `pkg/authclient` verifier
  library and `auth.v1` service API.
- `audit.ReasonCSRFRefused` in the closed audit vocabulary.
- `freya.App.AddServer`, `transport/edge.ClientIP`, `internal/testrt.NewTB`.
- `transport/http.NewClient(rt, expectedID)`: an `*http.Client` that presents the
  service SVID, pins the peer SPIFFE ID (TLS 1.3, chain + SAN verification, no
  hostname trust), never follows redirects, applies the runtime limits as
  timeouts and audits refused handshakes as `authn_refused`. Used by the
  application gateway (`services/gateway`, spec `003-application-gateway`) to
  forward HTTP traffic to modules.
- `transport/edge.Config.CSRFExempt`: a hook that lets a listener skip the
  double-submit check for state-changing requests that carry no cookie
  credential (bearer-token API clients through the gateway).
- `transport/edge.RateLimit` fields carry `yaml`/`json` tags (`per_second`,
  `burst`, `routes`).
- `services/gateway`: the application gateway (spec `003-application-gateway`):
  single public edge, module registration over mTLS with an allow-list and
  leases, per-route/method permission enforcement through the auth module,
  HTTP / gRPC / gRPC-web forwarding over the pinned channel, CASL abilities
  derived from API permissions, a Module Federation shell, operations API and
  UI, health circuit breaking, `pkg/gatewayclient` module SDK and a hello
  example module.
- `services/auth`: user groups and user profiles (spec
  `004-groups-user-profiles`): flat tenant groups carrying roles (OpenFGA
  `role.assignee: [user, group#member]`), effective roles in sessions and
  tokens, group administration API and console screens, profiles (first/last
  name, phone, avatar) with a content-validated avatar pipeline
  (`golang.org/x/image`), `auth.v1.Profiles/Lookup`, invitations into groups.
  Migrations `0005_groups_profiles`, `0006_session_reasons`.
- `services/gateway`: `/gateway/v1/me` carries `display_name` and
  `avatar_url`; the dispatcher honours `X-Freya-Identity-Refresh` from the auth
  module; the shell header shows the signed-in person.
- `services/auth`: gateway mode (`gateway.enabled`) serving the browser API
  and the federated console remote (`npm run build:remote`, `/ui/`) on the
  Freya HTTP server; `Sessions/Exchange` and `Sessions/MintToken` RPCs
  (audited as `token_exchanged`); console permissions registered and granted
  to builtin roles; `pkg/authmanifest`.


### Security

- `google.golang.org/grpc` upgraded to v1.83.2: `govulncheck` reported two
  advisories reachable from `transport/grpc` in v1.82.x.
- `identity` jitter falls back to the midpoint deterministically when the
  randomness source fails (now covered by a test).
- `audit` log redaction now also masks attributes whose key contains `phone`
  (PII introduced by the auth service's user profiles, spec
  `004-groups-user-profiles`).

### Fixed

- `services/ipam` UI: response-shape mismatches with the API — subnet/location
  trees (`tree`), check-IP (`matching_groups`), suggest (`suggestions`), search
  (`query`), dashboard stats, BMC power/sensors; subnet scan now follows the
  async job to completion; ping shows progress and its result in place; the
  device "Sync" button no longer wipes a device's package list.
- `@freya/ui`: `UiDropdownMenu` opens in the top layer (no longer clipped by
  scrolling tables); missing icons added to the safelist.
- `transport/edge`: `WithNonce` attaches a CSP nonce to a context. The gateway
  relays its edge nonce to modules in `X-CSP-Nonce` (client values dropped) and
  the auth console uses it in gateway mode: Vuetify's inline theme stylesheet
  was blocked by the gateway's CSP, leaving hover and focus states black.
- `services/auth` console and `services/gateway` shell: the Material Design
  Icons webfont (`@mdi/font`) is now bundled; Vuetify's default icon set
  needs it, so checkboxes, navigation and chip icons were invisible.
- `services/auth`: key rotation retires the active key before inserting its
  successor (the store allows one active key); `RevokeUser` with a kept
  session lists live sessions before marking them so the others' cached views
  are evicted; the session revocation vocabulary matches the database check.
- `services/auth`: audit rows are inserted with batched `INSERT`s — `COPY` is
  refused on row-level-security tables — and OpenFGA permission objects use
  `permission:<tenant>/<resource>~<action>` (object ids cannot contain `:`);
  permission registration is idempotent; the bootstrap command binds
  ephemeral ports so it runs beside a live service.

## 0.1.0 — unreleased

Initial implementation of the secure service-to-service channel
(spec `001-secure-service-channel`).

### Security-relevant defaults (all on unless explicitly overridden)

- TLS 1.3 only (`MinVersion = MaxVersion`), ALPN `h2`, session tickets disabled,
  client certificate required, `VerifyPeerCertificate` **and** `VerifyConnection`
  enforce SPIFFE identity, trust domain, validity (±5 min skew) and, for
  clients, the expected callee name.
- Identity lifetime 1 h, renewal at 50 %, ±10 % jitter; an expired local identity
  is never presented and every call is refused with `identity_expired` (503 /
  UNAVAILABLE) until renewed.
- Authorization is deny-by-default; explicit `deny` rules win; policy reloads
  without restart (file poll 2 s, Valkey pub/sub).
- Limits: request body 1 MiB, headers 8 KiB, request timeout 30 s, idle 60 s,
  handshake 10 s, 100 concurrent streams, connection age 30 min.
- gRPC reflection disabled; health service subject to policy like any RPC.
- HTTP: explicit 404/405 handlers (never `http.DefaultServeMux`), eager TLS
  handshakes with audited refusals, body limit filter, security chain applied to
  every route before routing.
- Admin listener (health, readiness, metrics, opt-in pprof) is plain HTTP on
  `127.0.0.1:9090` only; binding to any other address requires explicit opt-in
  and then serves **mTLS only** (plaintext listeners never leave loopback).
- Every resource-limit violation (message size, request body, request timeout,
  handshake timeout) is audited as `limit_exceeded`.
- Logs pass through a redacting handler: keys containing
  key/private/secret/token/password/authorization, byte slices, PEM text and
  TLS/key values are replaced by `[REDACTED]`.
- Error bodies contain only `{"reason": ...}`; 5xx collapse to `internal`.
- `WithInsecureLocalDev` and `WithAllowAllPolicy` are refused when
  `env: production`.

- Handshake refusal events carry the operator hint (e.g. clock-skew tolerance)
  in `attrs.detail`; the `reason` vocabulary stays closed.

- `transport/edge`: browser-facing listener — server-authenticated TLS 1.3 only
  (generated dev certificate refused in production), strict security headers with
  per-request CSP nonces, double-submit CSRF with Origin/Sec-Fetch-Site checks,
  per-IP/per-route rate limits (audited as `limit_exceeded`), body limits; new
  audit reason `csrf_refused`.

### Added
- `freya.App.AddServer` attaches extra Kratos transports (for example a
  `transport/edge` listener) to the application lifecycle.
- `transport/edge.ClientIP` exposes the proxy-aware client address the rate
  limiter attributed a request to.
- `internal/testrt.NewTB` builds the test runtime for any `testing.TB` (fuzz
  and benchmark harnesses).

- `freya.New/App` with `GRPC()`, `HTTP()`, `Client()`, `Identity()`, `Ready()`,
  `AdminURL()`, `IdentityState()`.
- Identity providers: `identity/spiffe` (Workload API), `identity/file`,
  `identity/localdev`; `identity.Lifecycle` state machine.
- `authn` middleware + `MemoryRevocationChecker`; `authz` policy engine with
  bounded decision cache and `authz/file` source.
- `audit` events (schema in `specs/.../contracts/audit-event.schema.json`),
  redacting handler, non-blocking emitter.
- `observe`: UUIDv7 correlation IDs, W3C trace propagation, OpenMetrics
  exposition without a Prometheus client dependency, per-hop request log.
- `contrib/audit-timescale` (batched hypertable sink) and
  `contrib/policy-valkey` (policy source + revocation denylist) as separate
  modules.
- Example services, `cmd/freya-devca`, CI gates (lint, gosec, govulncheck,
  coverage ≥ 80 % / 100 % for security packages, fuzz, reproducible build).
