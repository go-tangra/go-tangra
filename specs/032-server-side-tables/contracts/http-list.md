# Contract: HTTP list endpoints

Applies to every browser-API list endpoint that backs a data table, in every
module. Prefix and filters stay module-specific.

## Request

`GET /api/<module>/v1/<collection>?page=&page_size=&sort=&order=&<filters>`

| Param | Schema (OpenAPI) | Default |
|---|---|---|
| `page` | integer, minimum 1, maximum 2147483647 | 1 |
| `page_size` | integer, minimum 1, maximum = list max (≤ 200) | list default (25) |
| `sort` | string, **enum** = the list's sortable fields | list default |
| `order` | string, enum `asc`, `desc` | the default direction of the chosen field |
| filters | unchanged | — |

Shared OpenAPI components (copied into each module's document):

```yaml
components:
  parameters:
    page:     { name: page, in: query, schema: { type: integer, minimum: 1, maximum: 2147483647 } }
    pageSize: { name: page_size, in: query, schema: { type: integer, minimum: 1, maximum: 200 } }
    order:    { name: order, in: query, schema: { type: string, enum: [asc, desc] } }
  # per operation: { name: sort, in: query, schema: { type: string, enum: [name, created_at, ...] } }
```

## Response — 200

```json
{
  "items": [ { "...": "..." } ],
  "total": 1234,
  "page": 3,
  "page_size": 50,
  "sort": "created_at",
  "order": "desc"
}
```

- `items.length ≤ page_size`; `total` counts records matching filters **and**
  the caller's permissions.
- `page` is the page actually returned: a request beyond the last page returns
  the last page (or page 1 with no items when `total = 0`).
- Extra fields a list already returns (e.g. `counts`) are kept.

## Errors — 422

```json
{ "reason": "validation_failed", "detail": { "param": "sort" } }
```

For: unknown `sort`, unknown `order`, `page < 1`, `page_size` outside 1–max,
non-numeric values. The detail never echoes the submitted value or internal
column names. Most cases are rejected by the OpenAPI validator before the
handler; `listquery.Parse` enforces the same rules for defence in depth and for
gRPC/internal callers.

## Legacy (one release)

Endpoints that accepted `cursor`/`limit` keep doing so: when `cursor` or `limit`
is present and none of `page`, `page_size`, `sort`, `order` is, the legacy path
runs and returns its previous shape (`items`, `next_cursor`) plus `total`.
Sending both styles is a 422 (`param: "cursor"`).

## Ordering guarantees

`ORDER BY <field expr> <order> NULLS LAST, <tie-breaker> <order>`; text fields
compare case-insensitively. Paging a static list returns every record once.
