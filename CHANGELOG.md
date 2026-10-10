# Changelog

## 4.7.0 — module catalogue: descriptor, release entry and action

### Added

- `catalogue` package: strict `tangra-module.yaml` (module descriptor) and
  `catalogue-entry.json` (release entry) types and validation — allow-list
  scope limited to `/api/<m>`, `/m/<m>`, `/<m>`; host inputs with anchored
  patterns; placeholders the gateway fills are reserved — deterministic
  `bundle.zip` packing, hostile-zip checks and X.Y.Z version ordering.
- `cmd/tangra-catalogue validate|build`: builds a release's catalogue assets.
- `.github/actions/catalogue-entry`: composite action for module releases —
  builds the entry and bundle, checks the image is pullable, attests both
  (GitHub artifact attestation) and attaches them and
  `catalogue.sigstore.json` to the release.
- `@go-tangra/ui` 4.7.0: version bump only (released in lockstep), no changes.

### Fixed

- `contrib/audit-timescale`: a timing race in `TestBatcherFlushesBySizeAndInterval`.

## 4.6.0 — preflight: the token issuer is checked

### Added

- `preflight.IssuerJWKS` (network check): fetches `<issuer>/.well-known/jwks.json`
  and requires the platform's Ed25519 signing keys there, so a wrong
  `gateway.issuer` fails preflight instead of every console request later
  answering "session ended". `preflight.IssuerOrigin` (offline) warns when the
  issuer's origin differs from another URL of the portal (the enrolment URL).
- `@go-tangra/ui` 4.6.0: version bump only (released in lockstep), no changes.

## 4.5.0 — preflight: every configuration and environment problem before a module starts

### Added

- `preflight` package: modules offer `<module> preflight -config <file> [-json]
  [-offline]`, which runs every check and reports them all at once (checklist
  with a fix hint per problem, or JSON); exit 0 when nothing failed, 1 when a
  check failed, 2 on usage errors. It changes nothing, never consumes the
  enrolment token and never prints secrets. Reusable checks: file readable,
  directory writable, TLS key pair, TCP dial, HTTPS/TLS probe (connection and
  TLS failures told apart), and the enrolment token decoded locally (expiry,
  tenant, whether it names the service's SPIFFE id with a trust-domain hint,
  audience). `-offline` skips the checks that contact other hosts (marked
  `Check.Network`), for CI and hosts prepared before their network is open.
- `config.Config.ValidateAll() []error` collects every validation error;
  `Validate()` returns the first, unchanged for service start.
- `@go-tangra/ui` 4.5.0: version bump only (released in lockstep), no changes.

## 4.4.0 — Session expiry hook for the UI kit

### Added

- `@go-tangra/ui/api`: `onUnauthenticated(fn)` (and the
  `UnauthenticatedListener` type). Every 401 a `createApi` client receives —
  calls and uploads — is reported to the listeners before the `ApiError` is
  thrown; a throwing listener never changes the caller's error. The module is a
  federation singleton, so the shell registers once and covers every module
  remote: it confirms the session and sends the person to sign-in.
- Go module: version bump only (released in lockstep), no changes.

## 4.3.1 — listquery: index-friendly ORDER BY for not-null fields

### Changed

- `listquery.Field.NotNull`: declares that the sort expression never yields
  NULL. `OrderBy` then omits `NULLS LAST` (`h.created_at DESC, h.id DESC`), so
  a plain btree index such as `(tenant_id, created_at, id)` serves both
  directions through forward and backward scans. With `NULLS LAST` on every
  field, no descending sort — including every "newest first" default — could
  use an index and Postgres sorted the whole tenant per page (notification
  default page 195 ms → 0.6 ms with its existing index). Fields without
  `NotNull` keep `NULLS LAST`; the tie-breaker never had a NULLS clause.
  `SortSlice` is unchanged: a NotNull field has no nil keys, so its order is
  the same as the SQL one. Modules opt in per field after checking the column
  is `NOT NULL`.
- `@go-tangra/ui` 4.3.1: version bump only (released in lockstep), no changes.

## 4.3.0 — Server-side pagination and sorting (feature 032)

### Added

- `listquery` package: the platform list contract. A per-list `Spec` maps
  public sort names to constant SQL expressions with a unique tie-breaker;
  `Parse` validates `page`, `page_size` (1–200, default 25), `sort` and
  `order` and returns an `*Error` naming only the parameter; `OrderBy` builds
  `ORDER BY` from the Spec's constants and a closed direction enum (`NULLS
  LAST`, case-insensitive text); `Clamp` turns a page past the end into the
  last page; `Page[T]` is the response shape; `SortSlice`/`Window` give
  in-memory lists the same semantics; `Legacy` detects old cursor/limit
  requests during the migration. 100% covered and fuzzed.
- `@go-tangra/ui`: `UiDataTable` server mode (`total`, `page`, `pageSize`,
  `pageSizes`, `sort` props; `update:sort`, `update:page`, `update:pageSize`
  events; no local sort or row window; rows kept while loading; stacked
  layout sort select), `Column.defaultDir`, new `UiPager` (range and total,
  numbered pages with elision, page-size choice) and `useListQuery` (per-table
  page/size/sort in the route query, safe fallbacks, superseded responses
  dropped). `UiAuditTable` can opt into the page contract with
  `paging="page"`.

### Fixed

- `UiCombobox` attribute order (lint).

## 4.2.6 — UI kit: dropdown stays open when its scrollbar is used

### Fixed

- `UiCombobox`: pressing the list itself (its scrollbar or padding) moved focus
  off the input, which closed the popup; the list now cancels `mousedown` so
  scrolling by dragging or clicking the scrollbar keeps it open.

## 4.2.5 — UI kit: dropdown lists and sidebar no longer scroll sideways

### Fixed

- `UiCombobox` popups (permission drawers, form selects): FlyonUI's `.menu`
  wraps its items, so a list taller than its max height laid the options out in
  extra columns and scrolled horizontally; the `flex-nowrap` utility lost to it.
  `theme.css` now pins `.menu[role="listbox"]` to one column with no horizontal
  overflow (`:not(._)` specificity, as for the soft inks). The list may grow
  past a narrow input to fit its labels (up to 24rem); longer labels wrap.
- `UiNavDrawer`: an expanded nav group was `w-full` on top of FlyonUI's nested
  menu indent and overhung the sidebar by 4 px, giving it a horizontal
  scrollbar.

## 4.2.4 — edge connect sources

### Added

- `edge.Config.ConnectSources`: extra https origins the served pages may
  connect to (`connect-src`), e.g. a local signing application such as
  B-Trust BISS on `https://localhost:53952`–`53955` (signing v4, feature
  027). Entries are validated like frame sources; empty keeps the CSP
  byte-for-byte unchanged. Only connections are allowed — no script, frame or
  image sources are added.

`@go-tangra/ui` 4.2.4 is a version-only release (unchanged kit).

## 4.2.3 — UI kit: scheduler icons, visible unchecked switches

### Fixed

- `@go-tangra/ui` icon safelist gains the scheduler's icons (`mdi-calendar-clock`,
  `mdi-chart-timeline-variant`, `mdi-play`, `mdi-stop`, `mdi-run`,
  `mdi-play-circle-outline`, `mdi-stop-circle-outline`, `mdi-puzzle-remove-outline`,
  `mdi-code-json`, `mdi-form-select`); the shell emits no CSS for unlisted names,
  so its menu entry and the bulk Start/Stop buttons rendered without icons.
- `theme.css`: an unchecked `UiSwitch` was near-invisible in the dark theme
  (FlyonUI's off track is neutral at 22 % with a base-100 knob and no border).
  Track, border and knob now derive from base-content in both themes.

The Go module is unchanged apart from the version.

## 4.2.2 — frame sources

### Added

- `edge.Config.FrameSources` (`edge.frame_sources`): extra https origins a page
  may embed in an iframe (`frame-src`), e.g. the gateway's KVM console origin
  on port 8444. Empty keeps the CSP byte-for-byte unchanged.

`@go-tangra/ui` 4.2.2 is a version-only release (unchanged kit).

## 4.2.2 — edge frame sources

### Added

- `edge.Config.FrameSources`: extra https origins the edge's pages may frame,
  emitted as `frame-src 'self' <origins>` in the Content-Security-Policy.
  `NewServer` refuses entries that are not exactly an https origin. Empty
  (the default) leaves the policy byte-for-byte unchanged; every other
  header, including `frame-ancestors 'none'` and `X-Frame-Options: DENY`, is
  unchanged. Used by the gateway's KVM console origin (portal feature 025).

## UI kit 4.2.1 (`@go-tangra/ui`)

### Fixed

- Dark theme: checked switches/checkboxes and colourless soft badges were
  near-invisible in module pages. Every module remote re-emits FlyonUI's base
  component rules after the shell's stylesheet, so they beat the shell's colour
  modifiers (`switch-primary`) and the kit's soft-ink contrast fix.
  - `theme.css`: the soft badge/alert ink rules carry one extra class of
    specificity.
  - `@go-tangra/ui/vite` (`breakpointSpecificity()`): rules naming a component
    colour modifier (`switch-primary`, `badge-error`, …) get one extra class.
    Use the plugin in the shell as well as in remotes.
  - Remotes must import the utilities into the shell's layer:
    `@import "tailwindcss/utilities.css" layer(utilities);` — unlayered remote
    rules otherwise beat every layered rule of the shell.

## UI kit 4.1.1 (`@go-tangra/ui`)

### Fixed

- Light theme on a dark OS: the `freya-dark` OS-preference rule now targets
  `:root:not([data-theme])`, so a chosen `data-theme` (the toggle) always wins
  over `prefers-color-scheme` (a light choice used to render dark).
- `UiCheckbox` / `UiSwitch`: the control and its text sit on one centred row
  (FlyonUI 2 has no row layout for `label.label`; the text dropped below the
  box). Consumers can drop local workarounds such as auth's `kit-fixes.css`.

### Added

- `UiDataTable` `rowSelectable?: (row) => boolean`: with `selectable`, rejected
  rows get a disabled checkbox and "select all" toggles only selectable rows
  (locked rows keep their state). Optional; existing tables are unchanged.

### Changed

- User-visible product name is Tangra (catalogue title, README, console
  warning prefix `[go-tangra/ui]`); theme ids `freya-light` / `freya-dark` and
  the `freya.theme` storage key are unchanged.

## 4.1.0 — verified first enrollment

### Added

- `config.EnrollTLS` (`enroll.insecure`, `enroll.ca_file`,
  `enroll.server_spiffe_id`) with `Validate(trustDomain, production)`,
  `ServerID` and `Warnings`: one rule for every service's first-enrollment TLS.
- `tlsconf.EnrollClientConfig` / `tlsconf.LoadEnrollClientConfig`: verify the
  enroll server against the mesh trust bundle and an expected SPIFFE ID (no
  host name check), for enrolling directly at lcm's keyless listener; public
  mode (system roots + host name) otherwise. See `docs/configuration.md`.

### Changed

- Production refuses `enroll.insecure` for every service that adopts
  `config.EnrollTLS`, the gateway included: it now enrolls at lcm with
  `enroll.ca_file` instead.
- `tlsconf` shares one SPIFFE verifier between the mesh and the enroll client
  (no behaviour change for mesh connections).

## 4.0.0 — go-tangra v4: platform extracted from go-freya

The platform moves out of the go-freya monorepo into `go-tangra/go-tangra`, with
its history. Services move to their own `go-tangra-<name>` repositories.

### Changed

- Go module path is `github.com/go-tangra/go-tangra/v4`; contrib modules are
  `github.com/go-tangra/go-tangra/contrib/<name>/v4` and build standalone
  (`replace` of the root module with `../..`, ignored by consumers).
- The UI kit is `@go-tangra/ui` 4.0.0 (formerly `@freya/ui`), published to
  GitHub Packages on `v*` tags. The root npm workspace holds only `ui/kit`.
- `deploy/stack` runs every service from `ghcr.io/go-tangra/<repo>:${TANGRA_VERSION:-4.0.0}`
  instead of building from the monorepo; policy files come from the images,
  the PowerDNS configs and the test OpenLDAP image moved into `deploy/stack`.
  `compose.override.yaml` builds a service from a local checkout.
- CI tests the root and contrib modules with `GOWORK=off`, builds, lints and
  tests the kit, and publishes it on tags. No platform container image.
- `make testca` uses the framework's own `cmd/freya-devca`.
- Vulnerability reports go through GitHub private vulnerability reporting; 4.x
  is the supported line.

### Removed

- `services/*` and their specs, the monorepo `go.work`, and the migration
  tooling (it stays in go-freya).

## 0.2.0 — unreleased

### Added

- `services/auth`: LDAP directory import (spec `016-auth-ldap-import`).
  Tenant administrators with the new `directory:manage` permission manage
  directory connections (Active Directory / OpenLDAP / other attribute
  presets, `ldaps` or `starttls`, TLS 1.3 minimum with a per-connection
  `allow_tls12` opt-in limited to ECDHE+AEAD suites, optional pinned CA,
  bind password sealed with the KEK under per-connection associated data,
  write-only). They can test a connection step by step, search with a
  compiled and canonicalised RFC 4515 filter AND-combined with the base
  filter, narrow the search to a base DN below the connection base, and
  import selected people by unique id. Import re-fetches each person from the
  directory and never sends e-mail. Imported people become `imported` users
  (no password, indistinguishable from unknown accounts at sign-in, recovery
  and accept; no lockout oracle). Administrators activate them in bulk (≤ 100)
  with ordinary invitations carrying the chosen roles and groups, or remove
  them while they are still imported. Outbound dials go through a dial-time
  target policy: an always-denied set covering loopback, link-local and
  metadata, unspecified and multicast addresses, plus operator `deny_cidrs`,
  `allow_cidrs` overrides and allowed ports. Tests and searches report coarse
  outcomes only and are rate limited per tenant. Limits cover search
  size/time, filter size/depth, 500 ids per import, 8 MiB per LDAP message
  and 10 connections per tenant. New `directory` config section, migration
  `0008_ldap_import.sql`, console "Directories" page, import page and
  activation drawer. Dev stack gets an optional `ldap` compose profile.
  Everything is audited, with ids and counts only and no directory PII.

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

- `services/auth` (behaviour change): plain invitations now run the same
  role-grant escalation check as role assignment. A non-owner can no longer
  invite someone with `owner`/`admin`, or with a role or group carrying a
  permission they do not hold. Such requests get `403 self_escalation`.
  Deactivate, reactivate, role assignment and group membership refuse
  `imported` users with `409 invalid_state` (or skip them), so an imported
  account cannot become active without an invitation.
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
