# Data Model: Server-Side Pagination and Sorting (032)

No new persisted entities. The feature introduces request/response value types,
a per-list definition and browser table state, plus indexes in each module.

## Page request (`listquery.Request`)

| Field | Type | Rules |
|---|---|---|
| `Page` | int | ≥ 1; default 1; beyond the last page is clamped to the last page after counting |
| `PageSize` | int | 1–200 (Spec max may be lower, never higher); default 25 (Spec may override for a list, e.g. 50 for logs) |
| `Sort` | string | one of the Spec's field names; default the Spec's default |
| `Order` | enum | `asc` \| `desc`; default the Spec's default for that field |
| filters | per list | the list's existing filters, unchanged; applied before counting |

Validation failures produce `*listquery.Error{Param: "page"|"page_size"|"sort"|"order"}`.

## List definition (`listquery.Spec`, one per endpoint)

| Field | Meaning |
|---|---|
| `Fields` | map of public field name → `Field{Expr: constant SQL expression, Text: bool}`; `Text` fields order by `lower(expr)` |
| `Default` | default sort field + direction |
| `TieBreak` | constant unique column expression (normally `id`) appended to every ORDER BY |
| `DefaultSize`, `MaxSize` | page-size default and cap (MaxSize ≤ 200) |

Invariant: every `Expr` and `TieBreak` is a compile-time constant in module code;
`OrderBy()` output is built only from them and the direction enum.

## Page result (`listquery.Page[T]`)

| Field | JSON | Meaning |
|---|---|---|
| `Items` | `items` | the records of the page (≤ PageSize) |
| `Total` | `total` | records matching filters and permissions |
| `Page` | `page` | page actually returned (after clamping) |
| `PageSize` | `page_size` | size applied |
| `Sort`, `Order` | `sort`, `order` | sort applied |

Modules with extra response fields (e.g. scheduler executions `counts`) keep them
alongside.

## Table state (browser, `useListQuery`)

| Field | Persisted as | Rules |
|---|---|---|
| page | `?<key>.page=` | integer ≥ 1, else default |
| pageSize | `?<key>.size=` | one of 10/25/50/100/200, else default |
| sort | `?<key>.sort=` | one of the table's sortable keys, else default |
| order | `?<key>.order=` | asc/desc, else default |

`<key>` is a short per-table id unique on the screen (e.g. `hosts`, `snap`).
Transitions: filter change → page 1; sort change → page 1; size change → page
containing the first visible record; response for a superseded request →
ignored.

## Indexes (per module, new migration)

`CREATE INDEX IF NOT EXISTS … ON <table> (tenant_id, <sort expr>, id)` for each
sortable field of a large table lacking one (see research D10 and
`contracts/sortable-fields.md`). Hypertable logs: `(tenant_id, <ts> DESC)`.
