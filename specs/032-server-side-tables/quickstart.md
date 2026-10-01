# Quickstart: validating server-side pagination and sorting (032)

## Prerequisites

- go-tangra at the 032 branch (framework `listquery` + kit 4.3.0) built locally;
  the portal shell built against kit 4.3.0.
- For a module scenario: that module's branch, its integration test harness
  (`tests/integration`, testcontainers; run docker via `sg docker -c`), and the
  local freya-stack for UI checks.

## 1. Framework package

```sh
cd go-tangra
go test -cover ./listquery/...          # expect 100.0% coverage
go test -fuzz=FuzzParse -fuzztime=60s ./listquery/
```

Expected: defaults applied; `page=0`, `page_size=201`, `sort=bogus`,
`order=up`, `sort=name;drop` → `*Error` naming the parameter; `OrderBy` output
only ever contains Spec constants; `Clamp` beyond-last-page; `Window`/`SortSlice`
stable for equal keys.

## 2. Kit

```sh
cd go-tangra/ui/kit && npx vitest run && npm run lint
```

Expected: `UiDataTable` server mode emits `update:sort/page/pageSize`, never
sorts locally, keeps rows while loading, stacked layout has a sort select;
`UiPager` range text, elision, first/last, size select, axe clean in both
themes; `useListQuery` round-trips `?hosts.page=3&hosts.size=100&hosts.sort=last_seen&hosts.order=desc`,
falls back on invalid values, resets page on filter change, drops stale
responses.

## 3. Module list contract (per module)

Seed 1,234 rows (integration test fixture), then:

```sh
curl -sk -H "$AUTH" "$BASE/api/<module>/v1/<list>?page=1&page_size=50" | jq '{total,page,page_size,sort,order,n:(.items|length)}'
# → total 1234, page 1, page_size 50, n 50, default sort
curl ... "?page=999&page_size=50"            # → page 25, n 34
curl ... "?sort=<field>&order=desc"          # → items ordered over the whole set
curl ... "?sort=bogus"                       # → 422 {"reason":"validation_failed","detail":{"param":"sort"}}
curl ... "?page_size=500"                    # → 422 param page_size
curl ... "?cursor=<id>&limit=50"             # legacy path still answers (one release)
```

Integration tests assert: every record exactly once across all pages for each
sortable field and direction (SC-003); totals exclude other tenants and records
without a grant (SR-003); gRPC list RPCs unchanged.

## 4. UI (per module, freya-stack)

Open each table from the module's checklist (contracts/sortable-fields.md):
total shown, numbered pages, size picker, sortable headers sort the whole list,
filters reset to page 1, reload and copied link restore the view, phone width
(320 px) shows the same controls. Live-updating tables refresh the current page.

## 5. Performance spot check

Seed 100,000 rows in the largest table of the module (e.g. ipam addresses,
inventory hosts, notification log within the default window); time page 1,
a deep page and each sort: p95 < 1 s (SC-002). `EXPLAIN` shows the new
indexes in use.
