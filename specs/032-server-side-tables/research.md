# Research: Server-Side Pagination and Sorting (032)

Findings from a read-only survey of the framework, the kit and all 15
front-ends on 2026-10-01, and the decisions they lead to.

## Current state

### Framework and kit

- The framework has **no** list/pagination helper: no `page`, `cursor`,
  `listquery` package, no query-parsing helper. Every module keeps its own error
  envelope in `internal/httpapi/errors.go` (`{"reason": "..."}`;
  `ErrValidation` → **422 `validation_failed`**, `ErrMalformed` → 400) and its own
  `queryInt`/`atoiDefault` helpers.
- Modules validate requests against their embedded OpenAPI document with
  kin-openapi `openapi3filter` (schema min/max/enum → 422). Unknown query
  parameters are not rejected. The gateway proxies the query string untouched.
- `UiDataTable` (`ui/kit/src/components/UiDataTable.vue`) sorts the rows it was
  given (local `sortKey`/`sortDir`), offers `hasMore` + `load-more`, a stacked
  phone layout with **no** sort UI, and a "show more" window above 200 rows.
  It emits `row-click`, `load-more`, `update:selected` only.
- `UiPagination` is a prev/next pager (`hasPrev`, `hasNext`); `UiAuditTable` is a
  cursor table (limit 50). No list-state composable exists; `vue-router` is
  already a kit peer dependency (used by `UiNavDrawer`).
- The kit is a **module-federation singleton provided by the shell**: a module
  remote gets the kit from the portal at runtime. A new kit component or prop is
  therefore usable by a module only once the portal ships that kit version.
- Release: tag `vX.Y.Z` = Go module version = `ui/kit/package.json` version;
  CI publishes `@go-tangra/ui`. Modules require the framework at v4.0.0–v4.2.4.

### Modules (≈110 tables)

| Module | Paging today | Sort today | Notable problems |
|---|---|---|---|
| asset | `limit`+`cursor` (id keyset); UI never pages | none (id DESC) | whole lists loaded |
| inventory | `limit`+`cursor`; agents paged in Go | none | UI sends `os`/`last_seen_from`, handler reads `os_name`, ignores last_seen; snapshots cursor/sort mismatch; `/changes` ignores `limit` |
| ipam | `limit`+`cursor`; UI never pages | none | `hostname`, `ip_version` filters ignored by handlers; live patch prepends rows |
| dns | **page/page_size/total** (zones; records sliced in Go from PowerDNS) | fixed | prior art |
| lcm | `limit`+`cursor` | fixed keysets | certificates "load more" returns page 1 again (cursor timestamp dropped); visibility filtered **in Go** in batches |
| notification | `limit`+`cursor` | fixed | channels/templates visibility in Go; `notification_log` hypertable |
| warden | `limit`+`cursor`; search uses offset cursor | name / relevance | root-folder visibility in Go |
| paperless | HTTP passes no limit → **silently capped at 100** | created_at DESC, no tie-breaker | none per-row (tenant-level check) |
| deployer | unbounded; jobs default 50 | created_at DESC | `job_type`/`target_id` filtered in Go **after** LIMIT; N+1 on targets |
| scheduler | **page/page_size/total** (OFFSET) | fixed | prior art |
| signing | **page/page_size/total** | fixed | backup walks pages in default order |
| ticket | **page/page_size/total** | fixed | gRPC `List` uses the same filter |
| hr | **page/page_size/total** | fixed | allowances sort by name needs a member join |
| auth console | unbounded; users **capped at 200**, no total; audit cursor on `ts` only (equal timestamps skipped) | none | |
| portal ops | audit cursor (limit 100, `{events,next_cursor}`); registrations/allowlist bare arrays | fixed | registrations come from the in-memory registry |

Service-to-service callers that use list functions (must keep working):
asset → inventory gRPC `ListHosts` (`limit`/`cursor_id`, id DESC keyset);
ipam/asset → inventory `ListHostReports` (separate keyset, untouched);
lcm `SVIDServer.Verify` → `Repo.ListCertificates`; paperless gRPC
`DocumentServer.List` (`CursorId`); deployer gRPC `ConfigurationServer.List`;
asset/ipam/inventory gRPC list RPCs (`limit`/`cursor_id`); ticket gRPC `List`
(page/page_size); module backups (cursor walks in asset/ipam/inventory, page walks
in signing); scheduler `TaskIDs`; HR `All:true` callers. **No HTTP list endpoint
is called by another repo** — HTTP callers are only the module's own UI and tests.

## Decisions

### D1 — One framework package for the list contract

- **Decision**: add `github.com/go-tangra/go-tangra/v4/listquery` (stdlib only):
  a per-endpoint `Spec` (sortable fields → constant SQL expressions, default
  sort, unique tie-breaker, default/max size), `Parse(url.Values, Spec)`
  returning a validated `Request` or a typed `*Error{Param}`, safe
  `OrderBy()` / `Limit()` / `Offset()` builders, `Clamp(total)` for
  beyond-last-page, a generic `Page[T]` response, and `SortSlice`/`Window` for
  in-memory lists (memstores, PowerDNS records, registry, inventory fleet).
- **Rationale**: 15 repos need identical parsing, bounds, error naming and SQL
  safety; a shared package makes the allow-list the only path from request text
  to SQL and gets 100% coverage + fuzzing once.
- **Alternatives**: per-module helpers (today's state — drift is the bug);
  OpenAPI-only validation (enforces enums but not the SQL mapping, and gRPC and
  memstores need the same logic).

### D2 — Validation errors keep the platform's 422 `validation_failed`

- **Decision**: invalid paging/sort parameters answer **422** with
  `{"reason":"validation_failed","detail":{"param":"sort"}}` (via each module's
  existing `WriteDetail`), and the OpenAPI declares `sort` as an enum and
  `page`/`page_size` with min/max so kin-openapi rejects most cases before the
  handler.
- **Rationale**: every module already maps validation to 422 and the UI's
  `ApiError` handling expects it; the spec's "validation error" is satisfied.
  The request said "400"; consistency with the existing envelope wins and is
  recorded here.
- **Alternatives**: 400 `malformed_body` (reserved for undecodable input).

### D3 — Count first, then fetch, in one transaction

- **Decision**: `SELECT count(*)` with the list's WHERE, `Clamp` the page, then
  `SELECT … ORDER BY <spec> LIMIT n OFFSET m`, both inside the tenant RLS
  transaction the repo already opens.
- **Rationale**: beyond-last-page must return the last page (FR-005) without a
  second round trip from the browser; one transaction keeps count and page
  consistent.
- **Alternatives**: `count(*) OVER()` (no rows → no total when page is beyond the
  end; extra re-query anyway); estimated counts (spec requires exact).

### D4 — Ordering expression

- **Decision**: `ORDER BY <expr> <dir> NULLS LAST, <tiebreak> <dir>` where text
  fields use `lower(col)` (case-insensitive, FR-007), and the tie-breaker is the
  table's unique id (uuid v7 → creation order). Direction words come from a
  closed enum, never from the request text.
- **Rationale**: total, stable order (FR-003); empty values last in both
  directions (FR-007).

### D5 — Visibility filtered in Go moves into SQL

- **Decision**: lists whose per-record visibility is filtered in Go today (lcm
  certificates/issuers/requests/jobs, notification channels/templates, warden
  root-folder secrets) compute the caller's readable-ID set first (the authz
  layer already has `ListAccessibleIDs`/`ReadableIDs`) and pass it to SQL as
  `id = ANY($n)` (or no constraint for tenant admins). Count and page use the
  same constraint.
- **Rationale**: exact totals and correct page boundaries without leaking hidden
  records (SR-003); warden search already works this way.
- **Alternatives**: post-filtering a page (short pages, wrong totals, leaks
  counts).

### D6 — Hypertable-backed logs keep exact counts within a time window

- **Decision**: audit and log tables (lcm/warden/auth/portal/ipam audit,
  notification log, scheduler executions) use the same contract with a
  **default time window** (last 7 days, user-adjustable through the existing
  from/to filters) and exact counts within the window; `(tenant_id, ts DESC)`
  indexes exist or are added. The default sort stays newest first.
- **Rationale**: exact `count(*)` over an unbounded hypertable is the one place
  SC-002 is at risk; a window bounds it while keeping the contract identical.
- **Alternatives**: keep cursors for audit (inconsistent UX, rejected by the
  "every table" decision); estimated totals (spec says exact).

### D7 — Backward compatibility

- **Decision**: HTTP list endpoints that accept `cursor`/`limit` keep accepting
  them for one release: a request with `cursor` or `limit` and no
  `page`/`page_size`/`sort` takes the legacy path and returns the legacy shape
  plus `total`. gRPC list RPCs and backup walks are unchanged (repo filters gain
  optional `Sort`/`Page` fields; `CursorID` stays). Bare-array responses (portal
  registrations/allowlist) become `Page` objects; their only clients are the
  portal's own UI and tests, which move in the same release.
- **Rationale**: no HTTP list is consumed cross-repo, but external scripts may
  exist; gRPC callers (asset→inventory, lcm Verify, paperless List) must not
  break (SC-007).

### D8 — Kit: server mode in `UiDataTable`, a numbered pager and a URL-state composable

- **Decision** (kit 4.3.0):
  - `UiDataTable` gains server mode when `total` is set: props `page`,
    `pageSize`, `pageSizes`, `sort` (`{key, dir}`), `total`; emits
    `update:page`, `update:pageSize`, `update:sort`; no client sort, no
    window/load-more in server mode; stacked layout gets a sort select.
  - New `UiPager` (range "Showing a–b of N", first/prev/numbered with
    elision/next/last, page-size select); `UiPagination` stays for old callers.
  - New `useListQuery(key, defaults)` composable: reactive `{page, pageSize,
    sort, order}` bound to the route query under a per-table prefix
    (`<key>.page`, …), validated against the table's allowed fields, reset to
    page 1 on filter/sort change, and a request token to drop superseded
    responses.
  - `UiAuditTable` switches to the page contract.
- **Rationale**: one place implements FR-009–FR-015 and the a11y rules.
- **Alternatives**: a separate `UiServerTable` (duplicates columns/slots/stack
  layout).

### D9 — Live updates refresh the current page

- **Decision**: tables that patch or prepend rows from SSE events (ipam
  addresses/scans, lcm certificates, asset) switch to a debounced (300 ms)
  reload of the current page; in-place patches of fields that cannot change
  the row's position (e.g. processing status, last-seen) may stay.
- **Rationale**: client-side inserts ignore sort and page boundaries (FR-019).

### D10 — Indexes

- **Decision**: each module adds `(tenant_id, <sort expr>, id)` indexes for its
  sortable fields that lack one, in a new migration, prioritising large tables:
  asset assets (name, asset_tag, created_at), lcm issued_certificates
  (created_at, not_after) and certificate_jobs (created_at), paperless documents
  (created_at, lower(name)), deployer jobs (created_at), ipam scan jobs
  (created_at), warden secrets (lower(name), created_at), ticket tickets
  (updated_at), signing templates/submissions (lower(name/title)), scheduler
  executions (status), auth users (lower(email), created_at), portal audit
  (ts) for tenant-less ops queries. Small config tables are not indexed beyond
  their existing unique keys.
- **Rationale**: SC-002 (100k rows, < 1 s p95).

### D11 — Rollout order

1. go-tangra **v4.3.0**: `listquery` + kit 4.3.0.
2. go-tangra-portal: kit 4.3.0 in the shell (prerequisite for every module UI),
   plus its own ops tables.
3. Modules in any order, each a minor release: framework bump to v4.3.0, list
   specs + repo/memstore changes + indexes + OpenAPI params + UI migration +
   live-reload changes + negative tests. Suggested waves: (a) prior-art modules
   (scheduler, signing, ticket, hr, dns) — mostly adding sort; (b) cursor
   modules without per-record authz (asset, inventory, ipam, deployer,
   paperless); (c) per-record authz modules (lcm, notification, warden); (d)
   auth console.
4. One release later: remove the legacy cursor path from HTTP lists.

### D12 — Dependencies

- **Decision**: no new dependencies. Go uses stdlib (`net/url`, `strconv`,
  `strings`, `sort`); the kit uses Vue + vue-router (existing peer).
- `govulncheck` must stay clean.

## Threat model (STRIDE, for the list contract)

| Threat | Vector | Mitigation |
|---|---|---|
| **Tampering / injection** | `sort=name;drop table` or `order=desc,(select…)` | Allow-list lookup returns a constant SQL expression; direction from enum; OpenAPI enum rejects first; fuzz test on `Parse` asserts output only ever contains Spec constants (SR-001). |
| **Denial of service** | `page_size=1000000`, `page=99999999`, sorting on an unindexed column of a large table | Bounds (1–200, page ≥ 1, int32), clamp beyond-last-page, only indexed fields sortable on large tables, D6 window for hypertables, existing gateway timeouts (SR-002). |
| **Information disclosure** | Totals or page boundaries counting records the caller cannot see; sorting by a hidden column to infer its values | Count uses the same tenant/permission constraint as the page (SR-003, D5); sortable fields ⊆ visible fields, never secret/sealed fields (SR-004); negative tests per module (SR-006). |
| **Spoofing / elevation** | — | Unchanged: gateway permission per route, tenant RLS, per-record grants. |
| **Repudiation** | — | List reads are not audited today; unchanged. |
| **Error leakage** | Echoing a bad sort value or internal column names | Errors carry only the parameter name (SR-005). |
