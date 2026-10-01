# Implementation Plan: Server-Side Pagination and Sorting for Every Data Table

**Branch**: `032-server-side-tables` | **Date**: 2026-10-01 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/032-server-side-tables/spec.md`

## Summary

Every data table on the platform (~110 tables, 15 front-ends) moves to
server-side paging and sorting with numbered pages and an exact total. The
framework gains one stdlib-only package, `listquery`, that turns
`page`/`page_size`/`sort`/`order` into a validated request whose ORDER BY is
built only from a per-list allow-list of constant SQL expressions, plus a
generic `Page[T]` response and in-memory helpers. The kit (4.3.0) gains a
server mode for `UiDataTable`, a numbered `UiPager` and a `useListQuery`
composable that keeps page/size/sort in the URL per table. Each module then
declares a `Spec` per list, counts and pages in SQL inside its tenant
transaction (moving Go-side visibility filtering into SQL where needed), adds
indexes for sortable columns, declares the parameters in its OpenAPI, migrates
its UI tables and live-update handling, and keeps legacy cursor parameters for
one release. Defects found during research (lcm cursor bug, paperless 100-row
cap, deployer post-limit filters, inventory/ipam ignored filters, auth 200-user
cap and audit skips) are fixed in the same module releases.

## Technical Context

**Language/Version**: Go 1.26 (framework module `github.com/go-tangra/go-tangra/v4`); TypeScript 5 / Vue 3.5 (kit and module UIs)

**Primary Dependencies**: stdlib (`net/url`, `strconv`, `strings`, `sort`) for `listquery`; existing pgx/raw SQL repos, kin-openapi validation, Vue + Pinia + vue-router (kit peer) — no new dependencies

**Storage**: PostgreSQL/TimescaleDB per module (tenant RLS); new btree indexes per module; PowerDNS API (dns records) and in-memory registry (portal) for computed lists

**Testing**: `go test` (unit, memstore, fuzz), testcontainers integration suites per module, Vitest + jsdom + axe for kit/UIs, Playwright catalogue

**Target Platform**: Linux containers (freya-stack / prod docker compose); evergreen browsers, 320 px phones to desktop

**Project Type**: framework library + shared UI kit + 14 module web services with micro-frontend remotes + shell

**Performance Goals**: page/sort/size change < 1 s p95 on 100,000-row lists (SC-002); no response > 200 records

**Constraints**: kit is a module-federation singleton (portal must ship kit 4.3.0 first); gRPC list RPCs and backup walks unchanged; exact totals; tenant/permission constraints identical for count and page

**Scale/Scope**: ~110 tables; ≈45 list endpoints; 15 repos; largest tables: ipam addresses, inventory hosts/snapshots, notification log, audit hypertables

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Secure by Default**: defaults are the bounded, safe values (size 25,
      max 200, default sort); no insecure option is introduced.
- [x] **II. Zero Trust**: no new endpoints; existing routes keep gateway
      permission, tenant RLS and per-record grants; parameters only narrow.
- [x] **III. Boundary Validation**: `page`/`page_size`/`sort`/`order` are
      schema-validated in each module's OpenAPI (enum/min/max) **and** by
      `listquery.Parse` in the handler; size bound enforced; sort mapped through
      an allow-list only (SR-001/002).
- [x] **IV. Test-First**: tests precede implementation in every task group;
      `listquery` at 100% coverage with a fuzz test; per-module negative tests
      for unknown sort, bad paging, cross-tenant/hidden-record totals (SR-006);
      module coverage ≥ 80% maintained.
- [x] **V. Observability**: no new security events (list reads are not audited
      today); validation errors carry only the parameter name; correlation IDs
      unchanged.
- [x] **VI. Supply Chain**: no new dependencies (research D12); govulncheck
      clean per release.
- [x] **VII. Simplicity**: one shared package and one table mode instead of
      per-module variants; typed `Spec`, no reflection or global mutable state
      (Specs are immutable package values validated in tests).
- [x] **Threat Model**: parsing of untrusted query input → STRIDE in
      research.md.

Post-design re-check: PASS — the design adds no trust boundary, keeps
authorization unchanged, and confines request-to-SQL mapping to constant
allow-lists.

## Project Structure

### Documentation (this feature)

```text
specs/032-server-side-tables/
├── plan.md              # this file
├── research.md          # survey, decisions D1–D12, STRIDE
├── data-model.md        # request/response/spec/table-state types, indexes
├── quickstart.md        # validation guide
├── contracts/
│   ├── http-list.md         # HTTP list contract (params, response, 422, legacy)
│   ├── listquery-go.md      # Go package API
│   ├── kit-table.md         # UiDataTable server mode, UiPager, useListQuery
│   └── sortable-fields.md   # per-module lists, sortable fields, defaults, fixes
└── tasks.md             # /speckit-tasks output
```

### Source Code

```text
go-tangra/                                  # framework + kit (v4.3.0)
├── listquery/                              # NEW: spec.go, parse.go, sql.go, page.go, slice.go (+ _test, fuzz)
├── ui/kit/src/components/UiDataTable.vue   # server mode
├── ui/kit/src/components/UiPager.vue       # NEW
├── ui/kit/src/components/UiAuditTable.vue  # page contract
├── ui/kit/src/composables/useListQuery.ts  # NEW
├── ui/kit/src/index.ts                     # exports
├── ui/kit/tests/components/data.spec.ts    # server mode + pager tests
├── ui/kit/tests/composables.spec.ts        # useListQuery tests
├── ui/kit/catalogue/src/pages/Data.vue     # demo
└── CHANGELOG.md

go-tangra-portal-v4/                        # shell kit 4.3.0 + ops tables
├── shell/package.json                      # @go-tangra/ui ^4.3.0
├── internal/httpapi/ops.go, internal/store/repos.go, internal/memstore
├── api/openapi/*.yaml
└── shell/src/views/ops/{Audit,Registrations,Allowlist}.vue

<each module repo>/                         # asset, deployer, dns, inventory, ipam, lcm,
                                            # notification, paperless, scheduler, signing,
                                            # ticket, warden (-v4), hr-service-v4, go-tangra-auth
├── go.mod                                  # framework v4.3.0
├── internal/<domain>/ or internal/repo/    # listquery.Spec per list
├── internal/repo/repodb/*.go               # count + ORDER BY/LIMIT/OFFSET (visibility in SQL)
├── internal/memstore/*.go                  # listquery.SortSlice/Window
├── internal/httpapi/*.go                   # Parse → 422 detail; Page response; legacy path
├── internal/store/migrations/00NN_list_indexes.sql
├── api/openapi/<module>.yaml               # page/page_size/sort/order params
├── ui/src/stores/*.ts                      # page state, query, total
├── ui/src/views/**                         # UiDataTable server mode + useListQuery
├── ui/src/stores/live.ts                   # debounced reload instead of client inserts
└── tests/integration/*                     # exactly-once paging, totals, negative tests
```

**Structure Decision**: the shared contract lives in the framework repo
(`listquery`) and the kit; each module owns its list Specs, SQL and indexes.
Delivery is one framework/kit release, then the portal shell, then independent
module releases (research D11).

## Rollout

1. **go-tangra v4.3.0** — `listquery` + kit 4.3.0 (server mode, `UiPager`,
   `useListQuery`, `UiAuditTable`), catalogue demo, CHANGELOG.
2. **portal** — shell on kit 4.3.0 (must precede module UI deploys), ops tables
   on the contract.
3. **Modules** (each a minor release + prod deploy):
   - wave A (prior art, add sort): scheduler, signing, ticket, hr, dns;
   - wave B (cursor → page, no per-record authz): asset, inventory, ipam,
     deployer, paperless;
   - wave C (per-record authz into SQL): lcm, notification, warden;
   - wave D: auth console.
4. **Cleanup release** — drop legacy cursor/limit on HTTP lists and the
   `UiPagination` export.

## Complexity Tracking

| Item | Why needed | Simpler alternative rejected because |
|---|---|---|
| Legacy cursor path kept for one release | SC-007, unknown external scripts | hard switch risks silent breakage; gRPC keeps cursors anyway |
| D6 default time window on audit/log hypertables | exact counts on unbounded hypertables would break SC-002 | estimated counts violate "exact totals"; keeping cursors violates "every table" |
| Visibility-ID set pushed into SQL (lcm, notification, warden) | exact totals and correct page boundaries | post-filtering pages gives short pages, wrong totals and count leaks |
