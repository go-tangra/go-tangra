# Performance spot check (T129, quickstart §5)

Run on 2026-10-01 against the released module code (ipam 98eaa43, inventory 70317a0,
notification dc86902, lcm 11f2a3d).

**Target**: SC-002, page, size or sort change under 1 s p95 on 100,000-row lists, with
`EXPLAIN` showing the list indexes in use.

## Setup

- A throwaway `timescale/timescaledb:latest-pg16` container (PG 16.15, shared_buffers 4 GB,
  work_mem 32 MB, the same settings as freya-stack-timescaledb). Each module had its own
  database, with every goose `Up` section applied in order and `<module>_app` created as
  NOBYPASSRLS.
- **Data**: tenant A had 100,000 rows. Tenants B and C had 50,000 rows each as other-tenant
  noise.
  - **ipam**: 5% of addresses are IPv6, 1,000 devices, ~33% of addresses linked to a device.
  - **notification**: tenant A had 100k rows inside the 7-day window and 100k older rows
    outside it, in 10 chunks.
  - **lcm**: 20 issuers per tenant, about 850 B of `cert_pem` per row.
- **Queries**: the exact SQL each repo builds: the same column lists, WHERE builders and
  `listquery.OrderBy` output (`<expr> <dir> NULLS LAST, <tie> <dir>`, `lower()` for text
  fields) with `LIMIT/OFFSET`. They were run as the app role inside a transaction after
  `set_config('app.tenant_id', A, true)`, so RLS is active.
- **Timings**: `EXPLAIN (ANALYZE, BUFFERS)`, 7 runs per query. ms = planning + execution
  time. The p50 and max come from runs 2–7, after a warm-up run. These are server-side
  times, and `EXPLAIN ANALYZE` instrumentation inflates them slightly.
- **Page sizes**: the default first page is 25 rows (the Spec default). Sort pages are page 1
  at 50 rows. The deep page is page 2000 at size 50 (OFFSET 99950).

## Summary

| Table | Query | Rows scanned | p50 ms | max ms | Index used for order? | vs SC-002 |
|---|---|---|---|---|---|---|
| ipam addresses | count | 100k | 44 | 70 | addresses_tenant (index only) | pass |
| ipam addresses | default p1 (address asc, inet) | 100k | **234** | 240 | no (top-N sort) | pass |
| ipam addresses | address asc / desc p1 | 100k | 144 / 144 | 220 / 166 | no | pass |
| ipam addresses | hostname, mac asc/desc p1 | 100k | 224–236 | 282 | no (`lower()` has no index) | pass |
| ipam addresses | status asc p1 | 25k | 49 | 51 | addresses_status + incremental sort | pass |
| ipam addresses | status desc, type, last_seen, created_at p1 | 100k | 179–191 | 224 | no | pass |
| ipam addresses | deep p2000 (address / created_at / hostname) | 100k | 589 / 529 / 652 | 628 / 644 / **724** | no | pass (slow) |
| ipam addresses | filtered status=active + hostname ILIKE: count / p1 | 25k | 34 / 47 | 38 / 49 | addresses_status | pass |
| ipam addresses | filtered subnet (10k): count / p1 | 10k | 7.5 / 28 | 8 / 29 | addresses_subnet | pass |
| inventory hosts | count | 100k | 45 | 50 | index only | pass |
| inventory hosts | **default p1 (hostname asc)** | 25 | **0.32** | 0.36 | hosts_hostname_lower | pass |
| inventory hosts | created_at asc, last_seen asc p1 | 50 | 0.4–0.5 | 0.5 | hosts_created_at / hosts_last_seen | pass |
| inventory hosts | hostname desc, os_name, manufacturer asc/desc p1 | 100k | 105–110 | 132 | no | pass |
| inventory hosts | status asc / desc p1 | 100k | 88 / 101 | 145 | asc: hosts_status + incr. sort; desc: no | pass |
| inventory hosts | created_at desc, last_seen desc p1 (DefaultDir desc) | 100k | 95 / 88 | 101 | **no** (NULLS LAST mismatch) | pass |
| inventory hosts | deep p2000 hostname asc / created_at desc / last_seen desc | 100k | 171 / 214 / 208 | 251 | asc: yes; desc: no | pass |
| inventory hosts | filtered hostname ILIKE + status: count / p1 | ~13k | 76 / 98 | 122 | hosts_status / hosts_hostname_lower | pass |
| inventory hosts | filtered tag env=prod: count / p1 | 10k | 69 / 1.1 | 87 | GIN unused for count; p1 hostname_lower | pass |
| notification log | count, 7-day window | 100k | 56 | 60 | chunk notification_log_tenant (index only) | pass |
| notification log | **default p1 (created_at desc)** | 100k | **195** | 197 | **no** (NULLS LAST mismatch) | pass |
| notification log | created_at asc p1 | 50 | 1.1 | 1.1 | yes (backward scan) | pass |
| notification log | status asc / desc, channel asc/desc p1 | 25k / 100k | 49 / 202–208 | 237 | asc status: yes; rest no | pass |
| notification log | deep p2000 default / status asc | 100k | 248 / 369 | 250 / 397 | no | pass |
| notification log | filtered status=failed + recipient: count / p1 | ~20k | 35 / 33 | 46 / 37 | notification_log_status | pass |
| notification log | non-admin (own sends, `sender_id`): count / p1 / p40 | 2k | 2.1 / 4.6 / 5.2 | 5.7 | notification_log_sender | pass |
| notification log | prepared statement, forced generic plan: count / p1 / non-admin p1 | 100k | 143 / 216 / 86 | 248 | tenant index only, `($n='' OR …)` not folded | pass |
| lcm certificates | count | 100k | 47 | 48 | certs_tenant_status_exp (index only) | pass |
| lcm certificates | **default p1 (created_at desc)** | 100k | **204** | 214 | **no**: certs_tenant_created unusable | pass |
| lcm certificates | created_at asc p1 | 100k | 199 | 209 | no (index is `created_at DESC, id` ASC) | pass |
| lcm certificates | not_after asc p1 | 50 | 0.29 | 0.35 | certs_tenant_not_after | pass |
| lcm certificates | not_after desc, not_before, kind, status desc p1 | 100k | 189–199 | 241 | no | pass |
| lcm certificates | status asc p1 | 20k | 66 | 68 | certs_tenant_status_exp + incr. sort | pass |
| lcm certificates | identity asc / desc p1 | 100k | 279 / 276 | 308 / 370 | no | pass |
| lcm certificates | **issuer asc / desc p1** | 100k | **399 / 483** | 434 / 505 | no (correlated subquery per row) | pass, >300 ms |
| lcm certificates | deep p2000 default / not_after asc | 100k | 701 / 733 | 750 / 773 | no; **108 MB external merge sort to disk** | pass (marginal) |
| lcm certificates | **deep p2000 issuer asc** | 100k | **1031** | **1091** | no; subquery ×100k + 109 MB disk sort | **FAIL** |
| lcm certificates | filtered status=expiring: count / p1 | 20k | 14 / 48 | 20 / 65 | certs_tenant_status_exp | pass |
| lcm certificates | filtered issuer (5k): count / p1 | 100k | 99 / 121 | 149 | no (`issuer_id::text =` defeats indexes) | pass |
| lcm certificates | non-admin, 1,000 readable ids (`c.id = ANY`): count / p1 / identity p1 / p20 | 1k | 4.0 / 10.8 / 8.4 / 13.9 | 23 | issued_certificates_pkey | pass |

**Verdict**:
- 88 of the 89 measured queries meet SC-002. The one failure is the lcm certificates list
  sorted by issuer on a deep page (p2000 at size 50): about 1.03–1.09 s.
- No default first page is above 300 ms p50. The worst are ipam (234 ms), lcm (204 ms) and
  notification (195 ms).
- However, `EXPLAIN` shows that the new or existing list indexes are **not** used for
  several default orders. That part of quickstart §5 fails for lcm certificates
  (default created_at desc), the notification log (default created_at desc) and ipam
  addresses (default inet address order). These pages are fast now only because 100k rows
  still fit a top-N sort.

## Problems found

1. **The NULLS LAST mismatch is systemic and the main finding.**
   - `listquery.OrderBy` always emits `NULLS LAST`. A plain or `DESC` btree index scanned in
     either direction yields `ASC NULLS LAST` or `DESC NULLS FIRST`.
   - So every **descending** sort, including all `DefaultDir: Desc` defaults, cannot use the
     index. The planner ignores NOT NULL constraints when matching pathkeys.
   - Affected:
     - lcm `certs_tenant_created (tenant_id, created_at DESC, id)`: unusable in both
       directions, because `DESC NULLS FIRST` does not match, and neither does `id ASC`.
     - notification `notification_log_tenant (tenant_id, created_at DESC)`: the default
       sort is never index-backed.
     - inventory `hosts_created_at` / `hosts_last_seen`: only the asc direction works.
     - ipam `scan_jobs_tenant_created (… created_at DESC, id DESC)`: not measured, but it
       has the same shape, so it will not be used either.
   - Verified: the existing notification index serves `ORDER BY created_at DESC, id DESC`
     (no NULLS clause) in **0.6 ms** instead of 195 ms. An index built as
     `(tenant_id, created_at DESC NULLS LAST, id DESC)` takes the lcm default page from
     204 ms to **0.16 ms** and the notification default page from 195 ms to **0.25 ms**.
2. **The ipam address (inet) sort cannot be indexed as written.**
   - `pg_input_is_valid` is STABLE, so the `CASE WHEN pg_input_is_valid(address,'inet')
     THEN address::inet END` expression cannot go in an index.
   - The default page is a full top-N sort of the tenant: 234 ms at 100k. It grows linearly.
3. **ipam deep pages evaluate the per-row subqueries for every skipped row.**
   - `addrCols` has two correlated subqueries: the switch name and the `addrLinksCol`
     json_agg. On the deep page they are projected below the OFFSET, so `SubPlan loops=100000`.
   - Result: 589–652 ms p50, up to 724 ms.
4. **lcm sorts wide rows.**
   - `cert_pem` is about 850 B per row and is carried through the sort. Deep pages
     (OFFSET 99950) do a **108 MB external merge sort on disk** per request at
     work_mem 32 MB, which takes about 700 ms.
   - Under concurrency this is the most fragile query measured. It is also how this run hit
     "No space left on device" (see Notes).
5. **The lcm issuer sort is a correlated subquery per row.**
   - `(SELECT i.name FROM issuers i …)` runs 100k times before the sort.
   - p1 takes 399–483 ms. The deep page takes 1.03 s, which **fails SC-002**.
6. **Text sorts on Text fields have no `lower()` index.** These sorts are 100–280 ms at 100k
   (acceptable, but O(n)):
   - ipam hostname and mac
   - inventory os_name and manufacturer
   - lcm identity

   The inventory 0008 migration comment says that "the other host sort fields are served by
   the existing (tenant_id, <column>) indexes". That is wrong for the `lower()` fields and
   for every desc direction.
7. **Minor issues:**
   - **lcm issuer filter**: it uses `c.issuer_id::text = $n` (`eqCond`), and the cast
     defeats any issuer_id index. There is none today: the 5k-row filter costs 99 ms for the
     count and 121 ms for p1.
   - **notification log under a generic plan**: the `($n = '' OR col = $n)` pattern means
     that when pgx's cached prepared statement switches to a generic plan, the status and
     sender indexes are lost. The non-admin page goes from 4.6 ms to 86 ms and the count
     from 56 ms to 143 ms. This is still within target.
   - **lcm RLS policy**: it compares `tenant_id::text = current_setting(...)`. Row estimates
     collapse to about 1% (998 instead of 100k), which steers lcm toward nested-loop plans.
   - **RLS cost in all modules**: the per-row RLS filter (`current_setting` per row) costs
     about 3× on full scans. In ipam the count takes 44 ms with RLS and 12 ms without.

## Proposed fixes (not implemented)

1. **listquery (go-tangra, preferred, one place)**:
   - Add `Field.NotNull bool`, or `Nullable` with NOT NULL as the default.
   - When the column cannot be null, `OrderBy` omits the `NULLS LAST` clause. A plain
     `(tenant_id, col, id)` btree then serves both directions through forward and backward
     scans.
   - Mark created_at, not_after, not_before, status, kind and channel_type as NotNull. Mark
     hostname and name as NotNull too where the column is `NOT NULL DEFAULT ''`.
   - This needs no UX change, because these columns hold no nulls.
   - Alternative per module: build the DESC-default indexes as `DESC NULLS LAST`.
2. **lcm migration** (with fix 1, or with explicit NULLS):
   - Replace `certs_tenant_created` with
     `CREATE INDEX … ON issued_certificates (tenant_id, created_at DESC NULLS LAST, id DESC)`,
     matching the default.
   - With fix 1, `(tenant_id, created_at, id)` covers both directions.
3. **Deferred join for page queries** (lcm `runPage`, ipam `pageRows`, inventory, notification):
   - Shape: `SELECT cols FROM t JOIN (SELECT id FROM t WHERE … ORDER BY … LIMIT n OFFSET m) p USING (id) ORDER BY …`.
   - Sorting and skipping then touch only narrow tuples, or an index-only scan, and the
     per-row subqueries run only for the page.
   - Measured on lcm:
     - deep p2000 default, with the fix-2 index: 701 ms → **61 ms**
     - deep not_after asc: 733 ms → **62 ms**
     - deep kind asc, no index at all: → **227 ms**, with no disk spill
   - This also fixes ipam problem 3.
4. **lcm issuer sort**:
   - Sort through `LEFT JOIN issuers i ON i.tenant_id = c.tenant_id AND i.id = c.issuer_id`
     in the page FROM, with Expr `i.name`, instead of the scalar subquery.
   - Combined with fix 3, the deep page drops from 1031 ms to **383 ms**.
   - The remaining cost is the nested loop chosen because of the lcm RLS misestimate (see
     problem 7). If 380 ms is still too much, denormalise `issuer_name` onto
     `issued_certificates` with a `(tenant_id, lower(issuer_name), id)` index.
   - Or drop `issuer` from the sortable fields for this large table (research D10 /
     SR-002: "only indexed fields sortable on large tables").
5. **ipam inet sort**:
   - Add `CREATE FUNCTION ipam_try_inet(text) RETURNS inet LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$ SELECT CASE WHEN pg_input_is_valid($1,'inet') THEN $1::inet END $$;`
     It is safe to declare IMMUTABLE because inet input is immutable.
   - Add `CREATE INDEX addresses_tenant_inet ON ipam_ip_addresses (tenant_id, ipam_try_inet(address), id);`
     and set `AddressList.address.Expr` to `ipam_try_inet(address)`. Do the same for subnets
     `cidr`.
   - Measured: default p1 goes from 234 ms to **0.3 ms**. The desc direction still sorts
     (247 ms) until fix 1 or a NULLS-FIRST pair exists, because invalid addresses map to
     NULL, so this column is genuinely nullable.
   - Optional: add `(tenant_id, lower(hostname), id)`. Measured: p1 goes from 225 ms to
     **0.2 ms**.
6. **Optional**:
   - Use `c.issuer_id = $n::uuid` in the lcm `eqCond` for uuid columns.
   - Use `(tenant_id, lower(os_name), id)` and `(tenant_id, lower(manufacturer), id)` for
     inventory if those sorts matter.
   - Correct the inventory 0008 migration comment.

## Trimmed EXPLAIN excerpts

lcm, default p1. The new `certs_tenant_created` index is not used:
```
Limit (actual time=…204 ms rows=25)
  ->  Sort  Sort Key: created_at DESC NULLS LAST, id DESC   Sort Method: top-N heapsort
        ->  Index Scan using certs_tenant_status_exp on issued_certificates c (rows=100000)
              Filter: (((tenant_id)::text = current_setting('app.tenant_id', true)) OR …)
```
lcm, deep p2000 issuer asc (FAIL):
```
Limit (actual time=1025.7..1026.2 rows=50)  Buffers: shared hit=257227, temp read=13600 written=13606
  ->  Sort  Sort Key: (lower((SubPlan 1))), c.id   Sort Method: external merge  Disk: 108800kB
        ->  Index Scan using certs_tenant_status_exp on issued_certificates c (rows=100000)
              SubPlan 1 -> Index Scan using issuers_pkey on issuers i (loops=100000)
Execution Time: 1050.175 ms
```
lcm with fix 2 and fix 3, deep p2000 default:
```
Nested Loop (actual time=60.5..60.7 rows=50)
  ->  Limit -> Index Only Scan using x_created on issued_certificates c_1 (rows=100000)
  ->  Index Scan using issued_certificates_pkey on issued_certificates c (loops=50)
Execution Time: 60.945 ms
```
notification, default p1. `notification_log_tenant` is not used for the order:
```
Limit -> Sort (Sort Key: created_at DESC NULLS LAST, id DESC; top-N heapsort)
      -> Result -> Custom Scan (ChunkAppend) on notification_log   (100k rows read)   ~195 ms
```
notification, the same index with `ORDER BY created_at DESC, id DESC` (fix 1):
```
Limit -> Incremental Sort (Presorted: created_at)
      -> ChunkAppend -> Index Scan using _hyper_1_2_chunk_notification_log_tenant (rows=26)
Execution Time: 0.642 ms
```
ipam, default p1 (inet):
```
Limit (rows=25) -> Result -> Sort (Sort Key: CASE WHEN pg_input_is_valid(address,'inet') THEN address::inet END, id; top-N)
  -> Index Scan using addresses_tenant (rows=100000, RLS filter per row)          ~230 ms
```
ipam, deep p2000. The subqueries are evaluated for every skipped row:
```
Limit (rows=50) -> Gather Merge (rows=100000) -> Sort (quicksort 15.7 MB) -> Parallel Index Scan using addresses_tenant
  SubPlan 1 (switch name) loops=100000;  SubPlan 2 (addrLinksCol json_agg) loops=100000      ~589 ms
```
ipam with fix 5 (`ipam_try_inet` expression index), default p1:
```
Limit -> Index Scan using x_addr_inet on ipam_ip_addresses (rows=25)   Execution Time: 0.315 ms
```
inventory, default p1. The index works as designed:
```
Limit -> Index Scan using hosts_hostname_lower on inventory_hosts (rows=25)   0.32 ms
```
inventory, created_at desc p1. This field defaults to desc, and the index is not used:
```
Limit -> Gather Merge -> Sort (created_at DESC NULLS LAST, id DESC) -> Parallel Index Scan using hosts_report_changed  ~95 ms
```

## Notes

- **Host disk**: the host root filesystem was at 100% when the run started (96 MB free). The
  lcm deep-page sort failed once with "No space left on device" until finished databases
  were dropped. The perf container and its volume were removed afterwards, and about 2 GB is
  free now. The same 108 MB-per-request temp spill would hit freya-stack's TimescaleDB,
  which uses the same work_mem.
- **Not measured**: network, JSON encoding and gateway time. Seeded data is synthetic, and
  value distributions affect status and filter selectivity.
