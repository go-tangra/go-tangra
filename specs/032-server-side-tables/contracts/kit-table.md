# Contract: `@go-tangra/ui` 4.3.0 table additions

## `UiDataTable` — server mode

Server mode is on when the `total` prop is a number. Existing props, slots and
the client mode (no `total`) are unchanged.

| Prop | Type | Meaning |
|---|---|---|
| `total` | `number` | total matching records (turns server mode on) |
| `page` | `number` | current page (1-based) |
| `pageSize` | `number` | current page size |
| `pageSizes` | `number[]` | choices, default `[10, 25, 50, 100, 200]` |
| `sort` | `{ key: string; dir: 'asc' \| 'desc' } \| null` | active sort |

| Emit | Payload | When |
|---|---|---|
| `update:page` | `number` | page control used |
| `update:pageSize` | `number` | size changed |
| `update:sort` | `{ key, dir }` | sortable header (or stacked sort select) used; first click uses the column's `defaultDir` (default `asc`), next click reverses |

Column gains `defaultDir?: 'asc' | 'desc'`. In server mode: no client sorting,
no 200-row window, no `load-more`; `loading` keeps the previous rows with a busy
indicator (`aria-busy`); the header button exposes `aria-sort`; the stacked
(phone) layout shows a "Sort by" select and direction toggle for sortable
columns; `UiPager` renders below the table.

## `UiPager` (new)

Props: `page`, `pageSize`, `total`, `pageSizes?`, `label?`.
Emits: `update:page`, `update:pageSize`.
Renders "Showing a–b of N", first/previous, numbered pages with elision
(1 … 4 5 [6] 7 8 … 25), next/last, and a page-size select; with one page only
the range text and size select; with `total = 0` nothing.
Keyboard and screen-reader accessible (axe clean, both themes). Below `lg` only
the current page number is shown between the arrows; the button row wraps in
narrow containers.

## `useListQuery(key, options)` (new composable)

```ts
const lq = useListQuery('hosts', {
  sortable: ['hostname', 'last_seen', 'status'],
  defaultSort: { key: 'hostname', dir: 'asc' },
  defaultSize: 25,
})
lq.page; lq.pageSize; lq.sort          // refs bound to ?hosts.page / .size / .sort / .order
lq.query                               // { page, page_size, sort, order } for the API call
lq.resetPage()                         // call on filter change
lq.track(promise)                      // value of the latest request; superseded requests resolve with null
lq.clampTo(responsePage)               // adopt the server's clamped page
```

Invalid query values fall back to defaults silently (US4 scenario 4). Uses
`vue-router` (existing peer dependency); without a router it keeps state in
memory.

## `UiAuditTable`

New `paging="page"` prop switches it to the HTTP list contract (`page`,
`page_size`, newest first) and server mode; `paging="cursor"` (default) keeps
the old load-more behaviour for one release, so a module opts in when its audit
endpoint is migrated.

## Compatibility

`UiPagination` (prev/next) stays exported for one release. A module UI that uses
server mode requires a shell that provides kit ≥ 4.3.0 (module-federation
singleton), so the portal ships 4.3.0 before any module migrates.
