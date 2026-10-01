# Feature Specification: Server-Side Pagination and Sorting for Every Data Table

**Feature Branch**: `032-server-side-tables`

**Created**: 2026-10-01

**Status**: Draft

**Input**: User description: "Next is a major UI issue: in all modules where the data table is used there is no pagination and sort. The pagination and sort should happen on the server side." Decisions (2026-10-01): numbered pages with a total count and a page-size picker; every data table in every module is in scope, including small configuration lists, for one consistent behaviour.

## Overview

The platform's browser front-ends (the gateway shell, the auth console and the
module remotes: asset, deployer, dns, hr, inventory, ipam, lcm, notification,
paperless, scheduler, signing, ticket, warden) show their records in one shared
data table — about 110 tables across 15 front-ends. Today that table:

- sorts only the rows the browser has already received, so "sort by name" on a
  list of 5,000 hosts orders the first page, not the list;
- pages with a "load more" button driven by an opaque cursor (where the list
  offers one at all), so there is no page number, no total and no way to jump
  to the end;
- in several modules receives the whole list in one response, which grows
  without bound.

This feature makes the **server** responsible for ordering and slicing every
list. Each table shows numbered pages, the total number of matching records,
a page-size choice and sortable column headers; every sort and page request is
answered by the owning module over the whole matching set. The behaviour is the
same in every module, and the current page, page size and sort survive a
reload and can be shared as a link.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse a long list page by page (Priority: P1)

An operator opens a large list (e.g. inventory hosts, IP addresses, documents,
tickets, audit entries). The table shows the first page, the range and total
("Showing 1–50 of 1,234"), numbered page controls and a page-size choice. The
operator moves to page 7, jumps to the last page, or switches to 100 rows per
page; each change shows exactly the requested slice of the whole list.

**Why this priority**: without paging, large lists are either truncated or slow;
this is the core of the complaint and delivers value on its own even before
sorting.

**Independent Test**: seed a list with 1,234 records; open the table; verify the
total, page count, first/last/next/previous navigation, page-size change and
that no record is missing or duplicated across pages.

**Acceptance Scenarios**:

1. **Given** 1,234 matching records and page size 50, **When** the table opens,
   **Then** it shows records 1–50, "Showing 1–50 of 1,234" and 25 pages.
2. **Given** page 1, **When** the operator selects the last page, **Then** records
   1,201–1,234 are shown and "next" is disabled.
3. **Given** page 7 at size 50, **When** the operator changes the size to 100,
   **Then** the table shows the page that contains the first record previously
   visible (record 301 → page 4 at size 100).
4. **Given** an empty list, **When** the table opens, **Then** it shows the empty
   state with no page controls and a total of 0.

---

### User Story 2 - Sort the whole list by a column (Priority: P1)

The operator clicks a sortable column header (e.g. "Created", "Name", "Size",
"Status"). The table reloads page 1 ordered by that column across **all**
matching records; clicking again reverses the direction. Columns that cannot be
sorted show no sort affordance.

**Why this priority**: sorting only the visible rows is actively misleading
(the "newest" item shown may not be the newest); correct whole-list sorting is
the second half of the complaint.

**Independent Test**: seed records whose newest item lives beyond the first
page under the default order; sort by "Created" descending; verify it is the
first row.

**Acceptance Scenarios**:

1. **Given** a list in its default order, **When** the operator sorts by
   "Created" descending, **Then** page 1 contains the most recently created
   records of the whole list.
2. **Given** a sorted list, **When** the operator clicks the same header again,
   **Then** the direction reverses and the table returns to page 1.
3. **Given** many records with the same value in the sort column, **When** the
   operator pages through, **Then** every record appears exactly once (stable
   order).
4. **Given** a column that is not sortable, **When** the operator views its
   header, **Then** it offers no sort control.

---

### User Story 3 - Filters and search work together with paging (Priority: P2)

The operator narrows a list with the existing filters or search box. Paging and
sorting then apply to the filtered set: the total reflects the matches, the
table returns to page 1 whenever the filter changes, and the chosen sort is
kept.

**Why this priority**: most tables already have filters; paging must not break
them, but they are an existing capability rather than new value.

**Independent Test**: on a table with a status filter, page to page 5, change
the filter, verify page 1 of the filtered set with the correct total and the
previous sort.

**Acceptance Scenarios**:

1. **Given** page 5 of an unfiltered list, **When** a filter is applied,
   **Then** page 1 of the filtered results is shown with the filtered total.
2. **Given** a filter that matches nothing, **When** it is applied, **Then** the
   empty state is shown with a total of 0.

---

### User Story 4 - Page, size and sort survive reloads and links (Priority: P2)

The operator sorts hosts by "Last seen", moves to page 3 at 100 rows and copies
the address to a colleague, or reloads the page. The same page, size and sort
come back. Two tables on the same screen keep independent state.

**Why this priority**: convenient and expected, but the feature is usable
without it.

**Independent Test**: set page/size/sort, reload, verify restored; open the
copied link in a new session with the same permissions, verify identical view.

**Acceptance Scenarios**:

1. **Given** a table on page 3, size 100, sorted by "Last seen" descending,
   **When** the page is reloaded, **Then** the same view is restored.
2. **Given** a screen with two tables, **When** one is paged, **Then** the other
   keeps its own page.
3. **Given** a link whose page number is now beyond the last page (records were
   deleted), **When** it is opened, **Then** the last existing page is shown.
4. **Given** a link with an unknown sort column or an out-of-range page size,
   **When** it is opened, **Then** the table falls back to its default sort and
   size without an error screen.

---

### User Story 5 - Same behaviour in every module (Priority: P3)

Wherever a data table appears — including short configuration lists such as
channels, templates, tags or mailboxes — it offers the same paging controls,
the same total, the same page sizes and the same sorting behaviour.

**Why this priority**: consistency is the user's explicit choice, but it is
delivered module by module once stories 1–4 exist in the shared table.

**Independent Test**: for each module, open every table and check the controls,
total and sorting against a common checklist.

**Acceptance Scenarios**:

1. **Given** any data table in any module, **When** it is opened, **Then** it
   shows the total, page controls (when more than one page), a page-size choice
   and sortable headers where defined.
2. **Given** a list with fewer records than one page, **When** it is opened,
   **Then** it shows the total and no multi-page navigation.

---

### Edge Cases

- A record is created or deleted while the operator is on page 3: the next page
  request reflects the current data; a record may shift between pages but the
  total is always current. Live-updating tables refresh the current page rather
  than appending rows.
- The requested page is beyond the last page (list shrank): the last existing
  page is returned/shown, not an empty error.
- Page size above the maximum or below 1, page below 1, non-numeric values: the
  server rejects them with a validation error; the browser never sends them
  (it clamps its own state).
- Sort by a column that is not allowed for that list: the server rejects the
  request with a validation error naming the parameter, never ignores it
  silently and never passes it into a query.
- Sorting by a column that is empty for some records: those records sort
  consistently (always last in ascending and descending order).
- Sorting text: case-insensitive, consistent across pages.
- Very large lists (hundreds of thousands of rows, e.g. audit, scan results,
  IP addresses): pages and counts still answer within the performance targets.
- Lists whose visibility depends on per-record permissions (paperless,
  warden, lcm, notification grants): the total counts only records the caller
  may see; hidden records never influence page boundaries in a way that reveals
  their existence.
- Existing API clients that still send the old cursor parameters (other
  modules, scripts, agents) keep working until they are migrated.
- The mobile "stacked cards" layout pages the same way as the desktop table.

## Requirements *(mandatory)*

### Functional Requirements

**Shared list contract**

- **FR-001**: Every list endpoint that backs a data table MUST accept a page
  number (1-based), a page size, a sort field and a sort direction (ascending or
  descending), and MUST return the requested page of records together with the
  total number of records matching the current filters, the page number and the
  page size actually applied.
- **FR-002**: Every list MUST declare a default sort (field and direction) used
  when no sort is requested, and a fixed set of sortable fields; any other sort
  field MUST be rejected with a validation error.
- **FR-003**: Ordering MUST be total and stable: records with equal values in
  the sort field MUST be ordered by a unique tie-breaker so that paging through
  a static list returns every record exactly once.
- **FR-004**: Page size MUST default to 25 and MUST be limited to a maximum of
  200; the offered choices are 10, 25, 50, 100 and 200.
- **FR-005**: A page number beyond the last page MUST return the last page (with
  its actual page number); a page number below 1, a size outside 1–200, a
  non-numeric value or an unknown direction MUST be rejected with a validation
  error.
- **FR-006**: Filters and search parameters MUST apply before counting and
  paging; the total MUST reflect only the filtered, permitted records.
- **FR-007**: Text sorting MUST be case-insensitive; empty values MUST sort last
  in both directions.
- **FR-008**: The shared contract (parameter names, response fields, error
  shape, bounds) MUST be identical across all modules and documented once, with
  each module's API description listing its sortable fields and default sort.

**Shared data table**

- **FR-009**: The shared data table MUST offer a server-driven mode in which the
  page, page size, sort field and direction are controlled state supplied by the
  screen, and user actions (page change, size change, sort change) are reported
  as events instead of being applied locally.
- **FR-010**: In server-driven mode the table MUST show the range and total
  ("Showing 51–100 of 1,234"), first/previous/next/last controls, numbered page
  links with elision for many pages, and a page-size choice; with a single page
  it shows the total but no multi-page navigation.
- **FR-011**: Only columns declared sortable for that list MUST show a sort
  control; the active column and direction MUST be visible and announced to
  assistive technology.
- **FR-012**: While a page is loading, the table MUST keep the previous rows
  visible with a loading indication and ignore responses that arrive for a
  superseded request.
- **FR-013**: The stacked-card (phone) layout MUST provide the same paging,
  total, size and sort controls.
- **FR-014**: A shared list-state helper MUST keep page, size and sort in the
  page address (per table, so several tables on one screen are independent),
  restore them on load, fall back to defaults for invalid values, and reset to
  page 1 when filters or sort change.
- **FR-015**: Client-side sorting of loaded rows and the "load more" cursor
  button MUST no longer be used by any module table once that module is
  migrated.

**Module rollout**

- **FR-016**: Every data table in every front-end listed in the Overview
  (including configuration lists) MUST use the server-driven mode and a list
  endpoint that implements the shared contract.
- **FR-017**: List endpoints that currently return everything in one response
  MUST be converted to the shared contract; endpoints that currently use cursor
  parameters MUST accept the shared parameters and MUST keep accepting the old
  cursor parameters for at least one release so other callers are not broken.
- **FR-018**: Service-to-service and agent callers of changed list endpoints
  MUST be identified and either migrated or confirmed to keep working.
- **FR-019**: Live updates (records created, changed or removed while a table is
  open) MUST refresh the current page and total instead of inserting rows
  client-side.
- **FR-020**: The rollout MUST deliver the shared contract, server helper and
  table mode first, then migrate modules independently; a module that is not
  yet migrated MUST keep working unchanged.

### Security Requirements *(mandatory — Constitution: Development Workflow)*

- **Trust boundaries crossed**: public HTTPS ingress through the gateway to each
  module's browser API (untrusted query parameters); unchanged
  service-to-service list calls over mTLS.
- **Data classification**: whatever each list already returns (internal
  operational data, personal data in HR/auth/ticket lists); this feature adds no
  new data, only ordering and counts.
- **Authentication/Authorization**: unchanged — each list keeps its gateway
  route permission, tenant scoping and per-record grants.
- **Threat scenarios**: injection through the sort parameter; denial of service
  through huge page sizes, deep pages or expensive sorts; information disclosure
  through totals or page boundaries that count records the caller may not see;
  enumeration of hidden columns by probing sort fields.
- **SR-001**: A sort field MUST be mapped to a query column only through the
  list's allow-list; request text MUST never be interpolated into a query.
- **SR-002**: Page size and page number MUST be bounded (FR-004/FR-005) and
  validated before any data access.
- **SR-003**: Totals MUST be computed with exactly the same tenant, filter and
  permission constraints as the page itself.
- **SR-004**: Sortable fields MUST be limited to fields the caller can already
  see in the list; sorting MUST NOT be offered on secret, sealed or write-only
  fields.
- **SR-005**: Validation errors MUST name the offending parameter without echoing
  arbitrary input or internal column names.
- **SR-006**: Every module MUST have negative tests for unknown sort fields,
  oversized and invalid page parameters, and cross-tenant/hidden-record counts.

### Key Entities

- **Page request**: page number, page size, sort field, sort direction, plus the
  list's existing filters.
- **Page result**: the records of the page, total matching records, page number
  and page size applied, and the sort applied.
- **List definition** (per endpoint): its sortable fields (each mapped to a
  stored attribute), its default sort, its unique tie-breaker and its filters.
- **Table state** (browser): page, size, sort field and direction, persisted in
  the page address under a per-table key.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of data tables in all 15 front-ends show a total, numbered
  paging and a page-size choice, and sort over the whole list (verified against
  a per-module checklist).
- **SC-002**: On a list of 100,000 records, opening any page, changing the page
  size or changing the sort shows the result in under 1 second for 95% of
  requests.
- **SC-003**: Paging through a static list of 1,234 records at any page size
  returns every record exactly once, for every sortable field in both
  directions.
- **SC-004**: No list response ever contains more than 200 records.
- **SC-005**: A reload or a shared link restores page, page size and sort in
  100% of tested tables.
- **SC-006**: Requests with an unknown sort field or out-of-range paging are
  rejected in 100% of endpoints (no silent fallback on the server).
- **SC-007**: No existing service-to-service or agent integration breaks during
  the rollout.

## Assumptions

- Offset paging with a total count is acceptable at current and expected data
  sizes; the largest lists (audit, IP addresses, scan results, inventory
  software) get supporting indexes for their sortable fields so deep pages and
  counts stay within SC-002.
- The existing filters and search boxes are kept as they are; this feature does
  not add new filter types.
- Each module chooses its sortable columns per list (at least the columns a user
  would reasonably sort by: names, dates, sizes, statuses); computed or joined
  columns are sortable only where the store can order by them efficiently.
- Totals are exact counts (no estimates).
- Live-updating tables re-request the current page on change events; a short
  debounce is acceptable.
- Tables that are not backed by a list endpoint (e.g. key/value detail tables,
  small fixed in-page arrays such as a form's own rows) are out of scope.
- The rollout is delivered as a kit/framework release followed by per-module
  releases; modules are migrated in any order.
- The gateway's existing request limits and timeouts are unchanged.
