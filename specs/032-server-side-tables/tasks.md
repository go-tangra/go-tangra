# Tasks: Server-Side Pagination and Sorting for Every Data Table

**Input**: Design documents from `specs/032-server-side-tables/` (plan.md, spec.md,
research.md, data-model.md, contracts/, quickstart.md)

**Tests**: MANDATORY (Constitution IV). Every group lists its tests first; they
must fail before the implementation tasks of that group. Parsing of untrusted
query input gets negative tests and a fuzz test.

**Organization**: Phase 2 delivers the shared contract (framework `listquery` +
kit 4.3.0 + release). User stories 1–4 are proven end to end on the portal's
gateway-operations tables (SQL-backed allow-list, in-memory registrations,
hypertable audit), which also ships the shell on kit 4.3.0 — the prerequisite
for every module UI. User story 5 rolls the contract out to every module in the
waves of plan.md.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files/repos, no dependency on an
  incomplete task)
- **[Story]**: US1–US5 from spec.md
- Paths are relative to `/home/jadmin/projects/go-tangra/`; the first path
  segment names the repo.

## Conventions used by every module task group (US5)

- **Spec file**: `<repo>/internal/store/lists.go` — one `listquery.Spec` package
  value per list, fields and defaults exactly as in
  `go-tangra/specs/032-server-side-tables/contracts/sortable-fields.md`.
- **Repo**: each list function gets a `listquery.Request` (filters keep their
  struct); it runs `SELECT count(*) … WHERE …`, `req = req.Clamp(total)`, then
  `SELECT … WHERE … ORDER BY req.OrderBy(spec) LIMIT $n OFFSET $m` in the
  existing tenant transaction, returning `(items, total, req)`. Existing
  cursor/limit parameters and gRPC callers keep their current code path.
- **Handler**: `req, err := listquery.Parse(r.URL.Query(), spec)`; a
  `*listquery.Error` → `WriteDetail(w, 422, "validation_failed",
  map[string]any{"param": e.Param})`; `listquery.Legacy(q)` → old path + `total`;
  response `listquery.NewPage(items, total, req)`.
- **OpenAPI**: add the shared `page`, `pageSize`, `order` parameters
  (contracts/http-list.md) and a per-operation `sort` enum; response schema gains
  `total`, `page`, `page_size`, `sort`, `order`.
- **UI**: store keeps `{items, total}`; the view uses
  `useListQuery('<key>', {sortable, defaultSort})`, passes `lq.query` plus
  filters to the store, calls `lq.resetPage()` on filter change, and renders
  `<UiDataTable :total :page :page-size :sort @update:page @update:page-size
  @update:sort>`; columns get `sortable` only for Spec fields.
- **Live**: SSE handlers that insert/prepend rows call a debounced (300 ms)
  reload of the current page instead.
- **Release**: `go.mod` requires `github.com/go-tangra/go-tangra/v4 v4.3.0`,
  `ui/package.json` `@go-tangra/ui ^4.3.0`; PR, CI green, merge, tag next minor,
  image built; prod deploy only when the user says so.

---

## Phase 1: Setup

- [x] T001 Confirm branch `032-server-side-tables` in go-tangra is rebased on `main` (v4.2.6) and create `listquery/doc.go` with the package comment in go-tangra/listquery/doc.go
- [ ] T002 [P] Add the 4.3.0 section skeleton ("Server-side pagination and sorting") to go-tangra/CHANGELOG.md
- [ ] T003 [P] Create module feature branches `032-server-side-tables` in each repo: go-tangra-portal-v4, go-tangra-{asset,deployer,dns,inventory,ipam,lcm,notification,paperless,scheduler,signing,ticket,warden}-v4, hr-service-v4, go-tangra-auth (record base commit of each in go-tangra/specs/032-server-side-tables/rollout.md)

---

## Phase 2: Foundational (blocking prerequisites)

**Purpose**: the shared list contract in Go and in the kit, released as go-tangra v4.3.0. No module work starts before T030.

### Tests for `listquery` (write first, must fail)

- [x] T004 [P] Table tests for `Parse` defaults, valid values, every invalid case (`page=0`, `page=-1`, `page=abc`, `page_size=0/201/abc`, size above a Spec `MaxSize`, unknown `sort`, `order=up`, mixed `cursor`+`page` → `Param:"cursor"`) and `Legacy` in go-tangra/listquery/parse_test.go
- [x] T005 [P] Tests for `OrderBy` (text field → `lower(expr)`, `NULLS LAST`, tie-breaker in the same direction, DefaultDir), `Limit`, `Offset`, `Clamp` (beyond last page, total 0, exact multiple) in go-tangra/listquery/sql_test.go
- [x] T006 [P] Tests for `Spec.Validate` (default not in Fields, empty TieBreak, MaxSize > 200, empty Expr) and `NewPage` (nil items → `[]`, JSON field names) in go-tangra/listquery/spec_test.go
- [x] T007 [P] Tests for `SortSlice`/`Window` (string/int64/float64/time/nil keys, nil last both directions, case-insensitive strings, stable tie-breaker, clamp, exactly-once over all pages) in go-tangra/listquery/slice_test.go
- [x] T008 [P] Fuzz test `FuzzParse` asserting `OrderBy` output contains only Spec expressions, `lower(`, `ASC`/`DESC`, `NULLS LAST`, commas and spaces, and that errors only ever name page/page_size/sort/order/cursor in go-tangra/listquery/fuzz_test.go

### Implementation for `listquery`

- [x] T009 Implement `Dir`, `Field`, `Spec`, `Validate`, constants per contracts/listquery-go.md in go-tangra/listquery/spec.go
- [x] T010 Implement `Request`, `Error`, `Parse`, `Legacy` in go-tangra/listquery/parse.go
- [x] T011 Implement `OrderBy`, `Limit`, `Offset`, `Clamp` in go-tangra/listquery/sql.go
- [x] T012 Implement `Page[T]`, `NewPage` in go-tangra/listquery/page.go
- [x] T013 Implement `SortSlice`, `Window` in go-tangra/listquery/slice.go
- [x] T014 Reach 100% statement coverage, run `go vet`, golangci-lint and `govulncheck` for go-tangra/listquery/

### Tests for kit 4.3.0 (write first, must fail)

- [ ] T015 [P] `UiPager` tests: range text, elision windows (1, 2, 7, 25, 1000 pages), first/prev/next/last disabled states, page-size select emits, single page shows range only, total 0 renders nothing, axe in both themes in go-tangra/ui/kit/tests/components/pager.spec.ts
- [ ] T016 [P] `UiDataTable` server-mode tests: no local sort, `update:sort` with column `defaultDir` then toggle, `aria-sort`, non-sortable columns inert, no window/load-more, rows kept while `loading` with `aria-busy`, pager wired, stacked layout sort select + direction toggle at phone width, client mode unchanged, in go-tangra/ui/kit/tests/components/data.spec.ts
- [ ] T017 [P] `useListQuery` tests: read/write `?<key>.page/.size/.sort/.order`, invalid values → defaults, two keys independent, `resetPage`, size change keeps first visible record's page, `track` drops superseded responses, `clampTo`, works without a router, in go-tangra/ui/kit/tests/composables.spec.ts
- [ ] T018 [P] `UiAuditTable` tests on the page contract (requests `page`/`page_size`, renders pager, filters reset page) in go-tangra/ui/kit/tests/components/audit.spec.ts

### Implementation for kit 4.3.0

- [ ] T019 Implement `UiPager` in go-tangra/ui/kit/src/components/UiPager.vue
- [ ] T020 Add server mode (props `total`, `page`, `pageSize`, `pageSizes`, `sort`; emits `update:page`, `update:pageSize`, `update:sort`; `Column.defaultDir`; stacked sort select) to go-tangra/ui/kit/src/components/UiDataTable.vue
- [ ] T021 Implement `useListQuery` in go-tangra/ui/kit/src/composables/useListQuery.ts
- [ ] T022 Switch `UiAuditTable` to the page contract and server mode in go-tangra/ui/kit/src/components/UiAuditTable.vue
- [ ] T023 Export `UiPager`, `useListQuery`, their types in go-tangra/ui/kit/src/index.ts and add any new icons to go-tangra/ui/kit/src/icons.ts (ICONS + ICON_CLASS_SAFELIST)
- [ ] T024 [P] Add server-mode table + pager demo with a fake 1,234-row source to go-tangra/ui/kit/catalogue/src/pages/Data.vue and refresh catalogue screenshots in go-tangra/ui/kit/catalogue/tests
- [ ] T025 Run kit `vitest`, lint, build and catalogue Playwright (phone-320, tablet-768) in go-tangra/ui/kit

### Release go-tangra v4.3.0

- [ ] T026 Bump go-tangra/ui/kit/package.json and go-tangra/package-lock.json (`ui/kit`) to 4.3.0 and complete go-tangra/CHANGELOG.md
- [ ] T027 Document the list contract (link to contracts/http-list.md, `listquery` usage, kit server mode) in go-tangra/README.md
- [ ] T028 Open PR from `032-server-side-tables` to `main` in go-tangra, CI green (rerun the known `contrib/audit-timescale` timing flake if it fails), merge
- [ ] T029 Tag `v4.3.0` in go-tangra, confirm the Go module tag and the `@go-tangra/ui@4.3.0` publish job succeeded
- [ ] T030 Record the release in go-tangra/specs/032-server-side-tables/rollout.md

**Checkpoint**: `listquery` and kit 4.3.0 published — portal and modules can start.

---

## Phase 3: User Story 1 — Browse a long list page by page (P1) 🎯 MVP

**Goal**: numbered pages with exact totals, page-size choice and last-page clamping, proven on the portal's gateway-operations tables, and the shell on kit 4.3.0.

**Independent Test**: seed 1,234 allow-list rows and audit events; open Gateway operations → Allow-list and Audit; verify total, page count, first/last/next/prev, size change and exactly-once paging (quickstart §3–4).

### Tests for User Story 1

- [ ] T031 [P] [US1] Store integration tests: allow-list and audit count + LIMIT/OFFSET, clamp beyond last page, exactly-once across pages at sizes 10/25/200, tenant-less ops scope unchanged in go-tangra-portal-v4/internal/store/lists_integration_test.go
- [ ] T032 [P] [US1] HTTP tests: `/gateway/v1/ops/allowlist`, `/ops/audit`, `/ops/registrations` return the Page shape; 422 for `page=0`, `page_size=201`; registrations windowed in memory in go-tangra-portal-v4/internal/httpapi/ops_list_test.go
- [ ] T033 [P] [US1] Shell unit tests: ops views render `UiPager`, request `page`/`page_size`, show "Showing a–b of N" in go-tangra-portal-v4/shell/tests/unit/ops.spec.ts

### Implementation for User Story 1

- [ ] T034 [US1] Require `github.com/go-tangra/go-tangra/v4 v4.3.0` in go-tangra-portal-v4/go.mod and `@go-tangra/ui ^4.3.0` in go-tangra-portal-v4/shell/package.json (install with `NODE_AUTH_TOKEN=$(gh auth token)`)
- [ ] T035 [US1] Add Specs `allowlistList`, `auditList`, `registrationList` in go-tangra-portal-v4/internal/store/lists.go
- [ ] T036 [US1] Implement counted, paged `ListAllowlist` and `ListAudit` (audit: tie-breaker on event id) in go-tangra-portal-v4/internal/store/repos.go and the memstore equivalents in go-tangra-portal-v4/internal/memstore/memstore.go
- [ ] T037 [US1] Page the in-memory registrations with `listquery.Window` and switch the three ops handlers to `Parse`/`NewPage` (audit `events` → `items`; legacy `cursor` path kept one release) in go-tangra-portal-v4/internal/httpapi/ops.go
- [ ] T038 [US1] Declare page/page_size/order params and Page response schemas for the ops operations in go-tangra-portal-v4/api/openapi/gateway.yaml
- [ ] T039 [US1] Migrate ops views to server mode with `UiPager` in go-tangra-portal-v4/shell/src/views/ops/Allowlist.vue, go-tangra-portal-v4/shell/src/views/ops/Registrations.vue, go-tangra-portal-v4/shell/src/views/ops/Audit.vue
- [ ] T040 [US1] Update ops e2e helpers for the new response shape in go-tangra-portal-v4/tests/e2e/helpers.ts

**Checkpoint**: paging works end to end on the portal.

---

## Phase 4: User Story 2 — Sort the whole list by a column (P1)

**Goal**: whole-list sorting from table headers with a stable order and server-side rejection of unknown fields.

**Independent Test**: put the newest audit event and the alphabetically last allow-list entry beyond page 1 of the default order; sort; verify they appear first; `?sort=bogus` → 422.

### Tests for User Story 2

- [ ] T041 [P] [US2] Store tests: every sortable field in both directions returns each record exactly once with equal-value runs (tie-breaker), nulls last both directions, case-insensitive text in go-tangra-portal-v4/internal/store/lists_integration_test.go
- [ ] T042 [P] [US2] Negative HTTP tests: unknown sort, `order=up`, `sort=spiffe_id;drop` → 422 `{param}` without echoing input; secret/internal columns not sortable in go-tangra-portal-v4/internal/httpapi/ops_list_test.go
- [ ] T043 [P] [US2] Shell tests: clicking a sortable header requests `sort`/`order`, second click reverses, non-sortable header inert in go-tangra-portal-v4/shell/tests/unit/ops.spec.ts

### Implementation for User Story 2

- [ ] T044 [US2] Add sort enums per ops operation in go-tangra-portal-v4/api/openapi/gateway.yaml
- [ ] T045 [US2] Mark Spec fields `sortable` (with `defaultDir`) on ops view columns and wire `update:sort` in go-tangra-portal-v4/shell/src/views/ops/Allowlist.vue, Registrations.vue, Audit.vue
- [ ] T046 [US2] Add `(ts DESC, id)` and `(spiffe_id)` supporting indexes if missing in go-tangra-portal-v4/internal/store/migrations/0004_list_indexes.sql

**Checkpoint**: sorting is whole-list and validated.

---

## Phase 5: User Story 3 — Filters and search work with paging (P2)

**Goal**: filters apply before counting, change resets to page 1, sort kept; hypertable lists use the default 7-day window (research D6).

**Independent Test**: on Audit, page to 5, change the module filter → page 1 of filtered set, correct total, same sort; no-match filter → empty state, total 0.

### Tests for User Story 3

- [ ] T047 [P] [US3] Store tests: audit module/event_type/from/to filters change the total; default window applies when from/to absent; explicit from/to override it in go-tangra-portal-v4/internal/store/lists_integration_test.go
- [ ] T048 [P] [US3] Shell tests: filter change resets `page` to 1 and keeps `sort`; empty result shows empty state in go-tangra-portal-v4/shell/tests/unit/ops.spec.ts

### Implementation for User Story 3

- [ ] T049 [US3] Apply the default 7-day window when from/to are absent and keep filters in count and page queries in go-tangra-portal-v4/internal/store/repos.go
- [ ] T050 [US3] Call `lq.resetPage()` on filter changes and show the active window in the filter bar in go-tangra-portal-v4/shell/src/views/ops/Audit.vue

**Checkpoint**: filters + paging + sort combine correctly.

---

## Phase 6: User Story 4 — Page, size and sort survive reloads and links (P2)

**Goal**: per-table URL state with safe fallbacks.

**Independent Test**: set page 3 / size 100 / sort last_seen desc, reload → restored; copied link in a new session → same view; link with page beyond end → last page; link with bogus sort → default.

### Tests for User Story 4

- [ ] T051 [P] [US4] Shell tests: URL round-trip per ops table, two tables independent, beyond-last-page link adopts server-clamped page, invalid sort/size in URL fall back silently in go-tangra-portal-v4/shell/tests/unit/ops.spec.ts

### Implementation for User Story 4

- [ ] T052 [US4] Bind ops views to `useListQuery('allow'|'reg'|'audit', …)` and `lq.clampTo(res.page)` in go-tangra-portal-v4/shell/src/views/ops/Allowlist.vue, Registrations.vue, Audit.vue
- [ ] T053 [US4] Portal release: PR, CI, merge, tag next minor (v4.5.0), image built, record in go-tangra/specs/032-server-side-tables/rollout.md; prod deploy (gateway container, `.env` backup first) when the user approves
- [ ] T054 [US4] Bump the portal pin (`GATEWAY_IMAGE`) in go-tangra-docker/.env.example via PR to `v4`

**Checkpoint**: US1–US4 verified on the portal; shell provides kit 4.3.0 — module UIs may migrate.

---

## Phase 7: User Story 5 — Same behaviour in every module (P3)

**Goal**: every data table in every module on the contract (contracts/sortable-fields.md), with the defects found in research fixed.

**Independent Test** (per module): quickstart §3–5 for each list of the module; the module's table checklist in contracts/sortable-fields.md fully ticked.

Each module group is independent once Phase 6 is done; groups marked [P] can run in parallel. Within a group: tests → go.mod → migration → specs → repo/memstore → handlers/OpenAPI → UI → live → release.

### Wave A — scheduler (already pages; add sort)

- [ ] T055 [P] [US5] Tests: tasks/executions every sort field × direction exactly-once, totals per tenant, executions `counts` unchanged, `TaskIDs` keeps name order in go-tangra-scheduler-v4/internal/repo/repodb/lists_integration_test.go
- [ ] T056 [P] [US5] Tests: 422 for bogus sort/order/page_size>200, Page shape incl. `counts` in go-tangra-scheduler-v4/internal/httpapi/lists_test.go; UI tests for header sort + URL state in go-tangra-scheduler-v4/ui/tests/unit/tasks.spec.ts
- [ ] T057 [US5] go.mod v4.3.0, ui kit ^4.3.0; Specs `taskList`, `executionList` in go-tangra-scheduler-v4/internal/store/lists.go; index `(tenant_id, status, created_at DESC)` in go-tangra-scheduler-v4/internal/store/migrations/0004_list_indexes.sql
- [ ] T058 [US5] Replace fixed ORDER BY with `req.OrderBy` in `ListTasks`/`ListExecutions` (go-tangra-scheduler-v4/internal/repo/repodb/db.go), memstore `SortSlice` (go-tangra-scheduler-v4/internal/memstore/memstore.go), handlers `Parse`/`NewPage` (go-tangra-scheduler-v4/internal/httpapi/tasks.go, executions.go), sort enums in go-tangra-scheduler-v4/api/openapi/scheduler.yaml
- [ ] T059 [US5] UI: `useListQuery` + server-mode tables replacing `UiPagination` in go-tangra-scheduler-v4/ui/src/views/tasks/index.vue, history.vue and stores go-tangra-scheduler-v4/ui/src/stores/tasks.ts, executions.ts; release scheduler minor

### Wave A — signing

- [ ] T060 [P] [US5] Tests: templates/submissions/certificates/inbox sort × direction exactly-once, inbox join stable, backup page walk pins `id` order in go-tangra-signing-v4/tests/integration/lists_test.go
- [ ] T061 [P] [US5] Tests: 422 negatives + Page shape in go-tangra-signing-v4/internal/httpapi/lists_test.go; UI tests (inbox gains pager) in go-tangra-signing-v4/ui/tests/unit/lists.spec.ts
- [ ] T062 [US5] go.mod/kit bump; Specs `templateList`, `submissionList`, `certificateList`, `inboxList` in go-tangra-signing-v4/internal/store/lists.go; indexes lower(name), lower(title) in go-tangra-signing-v4/internal/store/migrations/0005_list_indexes.sql
- [ ] T063 [US5] Repo ORDER BY via Specs in go-tangra-signing-v4/internal/repo/repodb/db.go (backup in go-tangra-signing-v4/internal/backup/backup.go passes an explicit id-ordered request), memstore, handlers (templates.go, submissions.go, admin.go), OpenAPI sort enums in go-tangra-signing-v4/api/openapi/signing.yaml
- [ ] T064 [US5] UI server mode + `useListQuery` in go-tangra-signing-v4/ui/src/views/{templates,submissions,admin,inbox}/index.vue and stores; dropdown loaders keep `page_size:100`; release signing minor

### Wave A — ticket

- [ ] T065 [P] [US5] Tests: tickets sort × direction exactly-once incl. priority and assignee, gRPC `List` default unchanged and optional sort honoured in go-tangra-ticket-v4/tests/integration/lists_test.go
- [ ] T066 [P] [US5] Tests: 422 negatives; mailboxes/rules/tags paged in go-tangra-ticket-v4/internal/httpapi/lists_test.go; UI tests in go-tangra-ticket-v4/ui/tests/unit/tickets.spec.ts
- [ ] T067 [US5] go.mod/kit bump; Specs in go-tangra-ticket-v4/internal/store/lists.go; index `(tenant_id, updated_at DESC)` in go-tangra-ticket-v4/internal/store/migrations/0004_list_indexes.sql; optional `sort`/`order` fields in go-tangra-ticket-v4/api/proto/ticket/v1/ticket.proto (regenerate)
- [ ] T068 [US5] Repo/memstore/handlers (tickets.go + mailboxes/rules/tags lists) and OpenAPI in go-tangra-ticket-v4/api/openapi/ticket.yaml; gRPC server passes sort in go-tangra-ticket-v4/internal/grpcapi/tickets.go
- [ ] T069 [US5] UI server mode in go-tangra-ticket-v4/ui/src/views/{tickets,mailboxes,rules,tags}/index.vue and stores; release ticket minor

### Wave A — hr

- [ ] T070 [P] [US5] Tests: requests/allowances sort × direction incl. `user` (member display-name join) exactly-once; `All:true` callers unaffected in hr-service-v4/tests/integration/lists_test.go
- [ ] T071 [P] [US5] Tests: 422 negatives, holidays/absence-types paged in hr-service-v4/internal/httpapi/lists_test.go; UI tests in hr-service-v4/ui/tests/unit/lists.spec.ts
- [ ] T072 [US5] go.mod/kit bump; Specs (user sort → `lower(m.display_name)` via LEFT JOIN hr_members) in hr-service-v4/internal/store/lists.go
- [ ] T073 [US5] Repo (hr-service-v4/internal/repo/repodb/db.go), memstore, handlers (hr-service-v4/internal/httpapi/routes.go), OpenAPI hr-service-v4/api/openapi/hr.yaml
- [ ] T074 [US5] UI server mode in hr-service-v4/ui/src/views/{requests/list,allowances/index,holidays/index,absence-types/index}.vue; release hr minor

### Wave A — dns

- [ ] T075 [P] [US5] Tests: zones sort × direction exactly-once; records (PowerDNS, in memory) `SortSlice`/`Window` with page_size max 200; templates/supermasters paged in go-tangra-dns-v4/tests/integration/lists_test.go and go-tangra-dns-v4/internal/records/records_test.go
- [ ] T076 [P] [US5] Tests: 422 negatives, gRPC zones List unchanged in go-tangra-dns-v4/internal/httpapi/lists_test.go; UI tests in go-tangra-dns-v4/ui/tests/unit/zones.spec.ts
- [ ] T077 [US5] go.mod/kit bump; Specs in go-tangra-dns-v4/internal/store/lists.go; repo/memstore (go-tangra-dns-v4/internal/repo/repodb/db.go), records sorting via `listquery` (go-tangra-dns-v4/internal/records/records.go), handlers zones.go/records.go, OpenAPI go-tangra-dns-v4/api/openapi/dns.yaml
- [ ] T078 [US5] UI server mode replacing `UiPagination` in go-tangra-dns-v4/ui/src/views/zones/index.vue, records.vue, templates/index.vue, supermasters/index.vue; release dns minor

### Wave B — asset

- [ ] T079 [P] [US5] Tests: assets, suppliers, consumables, licenses, insurance, covered assets, assignments — sort × direction exactly-once, filters before count, tenant isolation; gRPC list RPCs and backup cursor walks unchanged in go-tangra-asset-v4/internal/repo/repodb/lists_integration_test.go
- [ ] T080 [P] [US5] Tests: 422 negatives, legacy `cursor`/`limit` path returns old shape + total, mixed styles 422 in go-tangra-asset-v4/internal/httpapi/lists_test.go; UI tests in go-tangra-asset-v4/ui/tests/unit/lists.spec.ts
- [ ] T081 [US5] go.mod/kit bump; Specs in go-tangra-asset-v4/internal/store/lists.go; indexes on assets (lower(name), asset_tag, created_at) in go-tangra-asset-v4/internal/store/migrations/0006_list_indexes.sql
- [ ] T082 [US5] Repo paged/counted variants beside the cursor functions in go-tangra-asset-v4/internal/repo/repodb/db.go (shared `listOpts` helper extended); memstore `SortSlice`/`Window` in go-tangra-asset-v4/internal/memstore/memstore.go
- [ ] T083 [US5] Handlers `Parse`/`NewPage`/legacy in go-tangra-asset-v4/internal/httpapi/handlers.go; OpenAPI go-tangra-asset-v4/api/openapi/asset.yaml
- [ ] T084 [US5] UI server mode in go-tangra-asset-v4/ui/src/views/{assets,suppliers,consumables,licenses,insurance}/index.vue, assets/detail.vue (assignments) and stores; asset.* SSE → debounced reload in go-tangra-asset-v4/ui/src/stores/live.ts; release asset minor

### Wave B — inventory

- [ ] T085 [P] [US5] Tests: hosts, auto-enroll, snapshots (cursor/sort mismatch fixed), changes (size honoured) exactly-once; agents fleet `Window`; asset→`ListHosts` gRPC id-DESC keyset unchanged; `ListHostReports` untouched in go-tangra-inventory-v4/internal/repo/repodb/lists_integration_test.go and go-tangra-inventory-v4/internal/upgrades/fleet_test.go
- [ ] T086 [P] [US5] Tests: 422 negatives, legacy path, `os_name` and `last_seen_from` filters applied in go-tangra-inventory-v4/internal/httpapi/lists_test.go; UI tests (hosts filters send `os_name`) in go-tangra-inventory-v4/ui/tests/unit/hosts.spec.ts
- [ ] T087 [US5] go.mod/kit bump; Specs in go-tangra-inventory-v4/internal/store/lists.go; index `(tenant_id, created_at)` on hosts if missing in go-tangra-inventory-v4/internal/store/migrations/0008_list_indexes.sql
- [ ] T088 [US5] Repo (go-tangra-inventory-v4/internal/repo/repodb/db.go: ListHosts paged variant, ListSnapshotsForHost consistent order, ListChanges limit), fleet `Window` (go-tangra-inventory-v4/internal/upgrades/fleet.go), memstore
- [ ] T089 [US5] Handlers (go-tangra-inventory-v4/internal/httpapi/handlers.go, upgrades.go) incl. `last_seen_from`; OpenAPI go-tangra-inventory-v4/api/openapi/inventory.yaml
- [ ] T090 [US5] UI server mode in go-tangra-inventory-v4/ui/src/views/hosts/index.vue (filter param `os_name`), agents/index.vue, agents/AutoEnrollCard.vue, hosts/detail.vue (snapshots, changes) and stores; release inventory minor (signing job needs user approval)

### Wave B — ipam

- [ ] T091 [P] [US5] Tests: addresses (inet order), devices, subnets (inet order), vlans, scans, group members, device sub-lists exactly-once; gRPC lists + backup cursor walks unchanged in go-tangra-ipam-v4/internal/repo/repodb/lists_integration_test.go
- [ ] T092 [P] [US5] Tests: 422 negatives, legacy path, `hostname` and `ip_version` filters honoured in go-tangra-ipam-v4/internal/httpapi/lists_test.go; UI tests (live event → reload, no prepend) in go-tangra-ipam-v4/ui/tests/unit/addresses.spec.ts
- [ ] T093 [US5] go.mod/kit bump; Specs (address/cidr sort on inet columns) in go-tangra-ipam-v4/internal/store/lists.go; index `(tenant_id, created_at)` on scan jobs in go-tangra-ipam-v4/internal/store/migrations/0010_list_indexes.sql
- [ ] T094 [US5] Repo paged variants in go-tangra-ipam-v4/internal/repo/repodb/db.go (ListAddresses, ListDevices, ListSubnets, ListVlans, ListScanJobs, members, device sub-lists); memstore `paginate` → `Window` in go-tangra-ipam-v4/internal/memstore/memstore.go
- [ ] T095 [US5] Handlers in go-tangra-ipam-v4/internal/httpapi/handlers.go (read `hostname`, `ip_version`); OpenAPI go-tangra-ipam-v4/api/openapi/ipam.yaml
- [ ] T096 [US5] UI server mode in go-tangra-ipam-v4/ui/src/views/{addresses,devices,subnets,vlans,scans,groups}/index.vue, devices/detail.vue and stores; addresses/scans SSE → debounced reload in go-tangra-ipam-v4/ui/src/stores/live.ts; release ipam minor

### Wave B — deployer

- [ ] T097 [P] [US5] Tests: configurations, targets (no N+1), jobs with `job_type`/`target_id` filtered in SQL before LIMIT, children/history exactly-once; gRPC `ConfigurationServer.List` unchanged in go-tangra-deployer-v4/internal/repo/repodb/lists_integration_test.go
- [ ] T098 [P] [US5] Tests: 422 negatives in go-tangra-deployer-v4/internal/httpapi/lists_test.go; UI tests (job SSE patch keeps page) in go-tangra-deployer-v4/ui/tests/unit/jobs.spec.ts
- [ ] T099 [US5] go.mod/kit bump; Specs in go-tangra-deployer-v4/internal/store/lists.go; index `(tenant_id, created_at DESC)` on jobs in go-tangra-deployer-v4/internal/store/migrations/0004_list_indexes.sql
- [ ] T100 [US5] Repo: paged/counted lists, job_type/target_id into WHERE, batch-load target configuration ids in go-tangra-deployer-v4/internal/repo/repodb/db.go and go-tangra-deployer-v4/internal/targets/targets.go; memstore
- [ ] T101 [US5] Handlers go-tangra-deployer-v4/internal/httpapi/handlers.go; OpenAPI go-tangra-deployer-v4/api/openapi/deployer.yaml
- [ ] T102 [US5] UI server mode in go-tangra-deployer-v4/ui/src/views/{configurations,targets,jobs}/index.vue (+ job detail tables, dashboard recent jobs) and stores; release deployer minor

### Wave B — paperless

- [ ] T103 [P] [US5] Tests: documents sort × direction exactly-once with id tie-breaker, more than 100 documents all reachable, filters before count; gRPC `DocumentServer.List` cursor unchanged in go-tangra-paperless-v4/internal/repo/repodb/lists_integration_test.go
- [ ] T104 [P] [US5] Tests: 422 negatives in go-tangra-paperless-v4/internal/httpapi/lists_test.go; UI tests in go-tangra-paperless-v4/ui/tests/unit/views.spec.ts
- [ ] T105 [US5] go.mod/kit bump; Spec `documentList` in go-tangra-paperless-v4/internal/store/lists.go; indexes `(tenant_id, created_at DESC, id)`, `(tenant_id, lower(name), id)` in go-tangra-paperless-v4/internal/store/migrations/0005_list_indexes.sql
- [ ] T106 [US5] Repo paged/counted `ListDocuments` (replace `fmt.Sprintf` ORDER with `req.OrderBy`) in go-tangra-paperless-v4/internal/store/repos.go and go-tangra-paperless-v4/internal/repo/repodb; memstore; handler go-tangra-paperless-v4/internal/httpapi/handlers.go; OpenAPI go-tangra-paperless-v4/api/openapi/paperless.yaml
- [ ] T107 [US5] UI server mode in go-tangra-paperless-v4/ui/src/views/documents/index.vue and go-tangra-paperless-v4/ui/src/stores/documents.ts; release paperless minor

### Wave C — lcm (visibility into SQL)

- [ ] T108 [P] [US5] Tests: certificates, issuers, requests, jobs, secrets, webhooks, audit — exact totals and pages for a user with partial grants equal the admin view filtered by grants; hidden records never counted; `SVIDServer.Verify` unchanged; certificate pages no longer repeat page 1 in go-tangra-lcm-v4/tests/integration/lists_test.go
- [ ] T109 [P] [US5] Tests: 422 negatives, audit default window, legacy path in go-tangra-lcm-v4/internal/httpapi/lists_test.go; UI tests (SSE → reload) in go-tangra-lcm-v4/ui/tests/unit/certificates.spec.ts
- [ ] T110 [US5] go.mod/kit bump; Specs in go-tangra-lcm-v4/internal/store/lists.go; indexes on issued_certificates `(tenant_id, created_at DESC, id)`, `(tenant_id, not_after, id)` and certificate_jobs `(tenant_id, created_at DESC, id)` in go-tangra-lcm-v4/internal/store/migrations/0009_list_indexes.sql
- [ ] T111 [US5] Readable-ID set from `ListAccessibleIDs` (admins: unrestricted) passed as `id = ANY($n)` to count and page in go-tangra-lcm-v4/internal/issue/certificates.go, issue/issuers.go, enroll/requests.go, enroll/jobs.go and go-tangra-lcm-v4/internal/store/repos.go; memstore
- [ ] T112 [US5] Handlers (certificates.go, issuers.go, enroll.go, secrets.go, ops.go audit window) and OpenAPI go-tangra-lcm-v4/api/openapi/lcm.yaml
- [ ] T113 [US5] UI server mode in go-tangra-lcm-v4/ui/src/views/{certificates,issuers,requests,secrets,permissions,audit}/index.vue and stores; certificates SSE → debounced reload in go-tangra-lcm-v4/ui/src/stores/live.ts; release lcm minor

### Wave C — notification

- [ ] T114 [P] [US5] Tests: channels/templates totals with partial grants (readable IDs in SQL), messages (sender scope), log within default window, categories, audit exactly-once in go-tangra-notification-v4/tests/integration/lists_test.go
- [ ] T115 [P] [US5] Tests: 422 negatives in go-tangra-notification-v4/internal/httpapi/lists_test.go; UI tests in go-tangra-notification-v4/ui/tests/unit/lists.spec.ts
- [ ] T116 [US5] go.mod/kit bump; Specs in go-tangra-notification-v4/internal/store/lists.go; indexes only where missing in go-tangra-notification-v4/internal/store/migrations/0006_list_indexes.sql
- [ ] T117 [US5] Readable-ID set into SQL for channels/templates (go-tangra-notification-v4/internal/notify/channels.go, templates.go), paged/counted repo (go-tangra-notification-v4/internal/store/repos.go) incl. `LogPage` window; memstore; handlers (channels.go, templates.go, messages.go, notifications.go, ops.go) and OpenAPI go-tangra-notification-v4/api/openapi/notification.yaml
- [ ] T118 [US5] UI server mode in go-tangra-notification-v4/ui/src/views/{channels,templates,messages,log,categories,permissions}/index.vue and stores; release notification minor

### Wave C — warden

- [ ] T119 [P] [US5] Tests: folder secrets list with folder-inherited and direct grants (readable IDs in SQL) exact totals; search paged by relevance/name; shares; audit window in go-tangra-warden-v4/tests/integration/lists_test.go
- [ ] T120 [P] [US5] Tests: 422 negatives (sort on secret value fields impossible) in go-tangra-warden-v4/internal/httpapi/lists_test.go; UI tests (folders first, then paged secrets) in go-tangra-warden-v4/ui/tests/unit/secrets.spec.ts
- [ ] T121 [US5] go.mod/kit bump; Specs in go-tangra-warden-v4/internal/store/lists.go; indexes `(tenant_id, folder_id, lower(name), id) WHERE deleted_at IS NULL`, `(tenant_id, created_at)` in go-tangra-warden-v4/internal/store/migrations/0005_list_indexes.sql
- [ ] T122 [US5] Visibility into SQL for root folder listing and paged/counted `SecretsInFolder`/search in go-tangra-warden-v4/internal/secrets/secrets.go and go-tangra-warden-v4/internal/store/repos.go; memstore; handlers (secrets.go, share.go, ops.go) and OpenAPI go-tangra-warden-v4/api/openapi/warden.yaml
- [ ] T123 [US5] UI server mode in go-tangra-warden-v4/ui/src/views/secrets/index.vue, permissions/index.vue, components/SecretDetails.vue (shares) and stores; release warden minor

### Wave D — auth console

- [ ] T124 [P] [US5] Tests: users beyond 200 all reachable with total; audit with equal timestamps never skipped (tie-breaker id) and default window; groups, members, roles, clients, operator tenants, sessions, directories paged in go-tangra-auth/tests/integration/lists_test.go
- [ ] T125 [P] [US5] Tests: 422 negatives, legacy audit cursor path in go-tangra-auth/internal/httpapi/lists_test.go; console unit tests in go-tangra-auth/console/tests/unit/lists.spec.ts
- [ ] T126 [US5] Create branch from the auth v4 line, go.mod/kit bump; Specs in go-tangra-auth/internal/store/lists.go; index `(tenant_id, lower(email), id)` on users in go-tangra-auth/internal/store/migrations/0012_list_indexes.sql
- [ ] T127 [US5] Repo paged/counted lists (go-tangra-auth/internal/store/repos.go, groups.go; remove the 200 cap in go-tangra-auth/internal/user/admin.go), memstore, handlers (admin.go, groups.go, roles.go, operator.go, signin.go, directory.go), OpenAPI go-tangra-auth/api/openapi/console.yaml
- [ ] T128 [US5] Console server mode in go-tangra-auth/console/src/views/admin/{Users,Audit,Groups,Roles,Clients,Sessions,Directories}.vue and operator tenant view; release auth minor

**Checkpoint**: every table in contracts/sortable-fields.md migrated; SC-001 checklist complete.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T129 [P] Performance spot checks per quickstart §5 (100k rows: ipam addresses, inventory hosts, notification log, lcm certificates) with `EXPLAIN` evidence recorded in go-tangra/specs/032-server-side-tables/rollout.md
- [ ] T130 [P] Security review of every module's Specs (sortable ⊆ visible, no secret/sealed fields, constant expressions only) and SR-006 negative tests present; record results in go-tangra/specs/032-server-side-tables/rollout.md
- [ ] T131 [P] Bump all module image pins in go-tangra-docker/.env.example and document the list contract for operators in go-tangra-docker/PRODUCTION.md (via PR to `v4`)
- [ ] T132 Cleanup release (one release after the last module): remove legacy `cursor`/`limit` HTTP paths and `listquery.Legacy` usage from every module handler, remove `UiPagination` from go-tangra/ui/kit/src/index.ts, update CHANGELOGs
- [ ] T133 Run quickstart.md end to end on freya-stack and on prod (read-only checks) and update the project memory note for feature 032

---

## Dependencies & Execution Order

### Phase dependencies

- Phase 1 → Phase 2 (T004–T030) → Phase 3 (US1) → Phase 4 (US2) → Phase 5 (US3) → Phase 6 (US4, portal release T053) → Phase 7 (US5) → Phase 8.
- US2–US4 extend the same portal views as US1, so they run in order on the portal; their kit capabilities already exist after Phase 2.
- **Hard gate**: no module UI using server mode is deployed before the portal release T053 is live (kit is a shell-provided singleton). Module backends may be released earlier.

### User story dependencies

- US1: needs Phase 2.
- US2: needs US1 (same views/endpoints).
- US3: needs US2.
- US4: needs US3; completes the portal pilot.
- US5: needs US4 (T053). Module groups are mutually independent.

### Within each group

Tests first (must fail) → go.mod/kit bump → migration → Specs → repo/memstore → handlers/OpenAPI → UI → live → release.

### Parallel opportunities

- T004–T008 (listquery tests), T015–T018 (kit tests) in parallel; T024 alongside T019–T023.
- T031–T033, T041–T043 in parallel within their phases.
- In Phase 7 every module group can proceed in parallel; within a group the two test tasks are [P].
- Phase 8 T129–T131 in parallel.

## Parallel Example: User Story 5 (wave B)

```text
Agent 1: T079–T084 asset
Agent 2: T085–T090 inventory
Agent 3: T091–T096 ipam
Agent 4: T097–T102 deployer
Agent 5: T103–T107 paperless
```

## Implementation Strategy

### MVP (US1)

Phase 1 → Phase 2 (listquery + kit 4.3.0 released) → Phase 3 on the portal:
numbered paging with totals on the gateway-operations tables. Validate with
quickstart §3–4 before continuing.

### Incremental delivery

1. US2 (sort) and US3 (filters) on the portal, then US4 (URL state) and the
   portal release — the shell now provides kit 4.3.0.
2. Wave A modules (small changes, prior art), then wave B (biggest user-visible
   gains: asset, inventory, ipam, deployer, paperless), then wave C (authz into
   SQL), then wave D (auth console).
3. Each module is released and deployed independently with its own backups.
4. Cleanup release removes the legacy cursor paths.

### Parallel team strategy

After T053, one agent per module group; the framework and portal remain the
single owners of `listquery`, the kit and the shell.

## Notes

- Each task is one commit (or a small set) on the repo's
  `032-server-side-tables` branch; no Co-Authored-By trailers.
- Prod deploys happen only on the user's explicit instruction, with DB and
  `.env` backups first.
- Inventory releases need the user to approve the `release` environment signing
  job.
