# 032 Security review (T130)

Date: 2026-10-01. Read-only review of server-side paged/sorted lists across all
15 modules (default branches, post-release). Reviewer: ecc:security-reviewer.

**Result: CRITICAL 0 · HIGH 0 · MEDIUM 2 · LOW 7 · INFO 4.**

Framework (`listquery`) is sound: `OrderBy` emits only Spec constants + Dir enum;
`Error` carries only the parameter name; `Parse` rejects unknown sort, invalid
order, page_size > 200 and mixed cursor/page styles; `Clamp` runs after count so
OFFSET is never attacker-sized. Every module builds ORDER BY from constant Spec
expressions and binds all request values. No sealed/secret/credential/value
column is sortable or filterable. Visibility-bearing lists (warden, lcm,
notification, hr, signing, auth) apply visibility in the shared WHERE of count
and page and fail closed on empty sets.

## Per-module checks

1 sortable ⊆ visible, non-secret · 2 constant ORDER BY, bound values · 3 count/page
share WHERE incl. tenant + visibility, fail-closed · 4 errors name param, never
value · 5 page_size ≤ 200, clamp after count · 6 legacy cursor can't bypass
visibility/probe · 7 SR-006 negative tests · 8 DoS bounded.
P = pass, P* = pass with note, F = finding.

| Module | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|---|---|---|---|---|---|---|---|---|
| portal | P | P | P | P | P | P* F-10 | P | F F-2 |
| scheduler | P | P | P* F-6 | P | P | n/a | P | P |
| signing | P | P | P* F-5 | P | P | n/a | P* no mixed-style test | P |
| ticket | P | P | P* F-6 | P | P | n/a | P | P |
| hr | P | P | P* F-5 | P | P | n/a | P* no mixed-style test | P |
| dns | P | P | P | P | P | n/a | P | P |
| asset | P | P | P | P | P | F F-1 | P | F F-1, F-4 |
| inventory | P | P | P | P | P | F F-1 | P | F F-1, F-4 |
| ipam | P | P | P | P | P | F F-1 | P* no echo test | F F-1, F-4 |
| deployer | P | P | P | P | P | n/a | P | P |
| paperless | P | P | P* F-9 | P | P | n/a | P | P |
| lcm | P* F-3 | P | P | P | P | P | P | F F-2 |
| notification | P | P | P | P | P | P | P | F F-2 |
| warden | P | P | P | P | P | P* F-8 | P | F F-2 |
| auth | P | P | P | P | P | P | P | F F-2 |

## Findings

### MEDIUM

- **F-1 Legacy cursor/limit lists unbounded in asset, inventory, ipam.** A request
  with only `cursor`/`limit` takes the legacy path, which passes `limit` unclamped;
  repos treat `limit <= 0` or huge as "all rows". Sites: inventory
  httpapi/handlers.go:40,146, upgrades.go:264, repodb/db.go:289,570; ipam
  httpapi/handlers.go:54,223,400,594,689,785,940,1087 and repodb `if f.Limit > 0`;
  asset httpapi/handlers.go:40,170, repodb/db.go:217,318,532. (lcm, notification,
  warden clamp at service level; inventory changes uses `legacyLimit`.)
  Fix: clamp legacy limit to 1..`listquery.MaxPageSize` (default 50) like
  `legacyLimit`; drop legacy paths in the cleanup release.
- **F-2 Explicit wide `from` defeats the 7-day audit/log window.** A caller with
  audit permission can send `from=1970-01-01` and force exact `count(*)` + OFFSET
  over the whole hypertable on every page. Sites: warden audit/query.go:105-114;
  lcm httpapi/ops.go:46-67; auth audit/query.go:116-123,149-152; notification
  store/lists.go:84-92 (also log); portal audit/query.go:30-37.
  Fix: cap the span (e.g. ≤ 90 days → 422 `{param: from}`) or cap the count
  (count over `LIMIT 10001`, report capped).

### LOW

- **F-3** lcm certificate sort `issuer` orders by issuer name (store/lists.go:24)
  for callers who may not read the issuer → name-ordering oracle. Fix: sort by
  issuer_id or require issuer read.
- **F-4** LIKE wildcards not escaped / no length cap: asset repodb/lists.go:62-73;
  ipam repodb/lists.go:62,85,125 + prefix/MAC LIKE; inventory repodb/db.go:249;
  warden store/pages.go:80, repos.go:240 (q capped by SearchMax). Fix: reuse the
  escape helpers from auth/notification/signing; cap q ≈ 200 chars.
- **F-5** "empty means unrestricted" sentinels: signing submissions.go:572-575
  (`CreatedBy`), hr repo.go:34,45 and requests/queries.go:57. Not exploitable
  (middleware rejects empty UserID). Fix: Forbidden on empty UserID or explicit
  `Unrestricted` flag.
- **F-6** Tenant isolation only via RLS in ticket `ListTickets` and scheduler
  `taskWhere`/`ListExecutions`. Fix: explicit `tenant_id = $n` or a test proving
  the RLS GUC is set.
- **F-7** Sort LEFT JOINs without tenant predicate: asset categories/locations,
  notification channels (repos.go:132), ipam host members → devices. Fix: add
  `AND x.tenant_id = y.tenant_id`.
- **F-8** warden legacy search offset cursor is O(offset) (secrets.go ~574). Fix:
  goes away with legacy removal.
- **F-9** paperless list uses a type-level read check (documents.go:225); per-
  document grants not considered. Unchanged from before 032 — confirm intended.

### INFO

- **F-10** portal legacy audit `MaxLimit = 500`; paperless gRPC List caps at 500.
- **F-11** dns zones.go:96 (and similar helpers in inventory/ticket/signing) return
  without writing on a non-`*listquery.Error` — unreachable today.
- **F-12** inventory `listConnected` silently falls back to id sort for
  hostname/state.
- **F-13** asset document search accepts `order=asc` but forces desc.

## Negative tests found (grep-based)

portal ops_list_test.go:110-116 (`sort=ts;drop table x`); paperless
lists_test.go:141-142 (`sort=content_text`); signing lists_test.go:16-24
(`sort=pdf_key`); deployer lists_test.go:135-136,180; hr lists_test.go:21,42 and
integration lists_test.go:93. Visibility-total tests: warden
`TestSecretListsVisibilityAndTotals`, lcm `TestListsTotalsMatchPerRecordChecks`,
notification `TestListChannelsTemplatesVisibility`, hr `TestCRUDAndVisibility`.

## Follow-ups

1. F-1 clamp legacy limit (asset, inventory, ipam) — next patch release.
2. F-2 cap audit/log span or count.
3. F-3, F-4, F-7, F-5, F-6 hardening in ordinary patch releases.
4. Cleanup release (T132) removes legacy paths (clears F-1, F-8, F-10).
