# Contract: per-module lists, sortable fields and defaults

Every list below adopts `contracts/http-list.md`. "Default" is the default sort;
the tie-breaker is the table's `id` unless noted. **Idx** = new index needed
(research D10). **Fix** = defect found during research, fixed in the same
module release. Final field lists are confirmed per module during its tasks;
additions must stay within visible, non-secret columns (SR-004).

## asset
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/assets` | asset_tag, name, status, category, location, assigned_to, purchase_date, warranty_end, created_at | asset_tag asc | Idx name, asset_tag, created_at |
| `/suppliers`, `/consumables`, `/licenses`, `/insurance-policies` | name, created_at (+ quantity / seats / expires_at / end_date where present) | name asc | |
| `/insurance-policies/{id}/assets` | asset_tag, name | asset_tag asc | |
| `/assets/{id}/assignments` | assigned_at, returned_at | assigned_at desc | |
| `/documents/search` | rank (fixed), paged only | rank desc | |

## inventory
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/hosts` | hostname, os_name, manufacturer, status, last_seen, created_at | hostname asc | Fix: UI `os`→`os_name`; honour `last_seen_from` |
| `/agents` (fleet, built in Go) | hostname, version, state, last_seen | hostname asc | in-memory `Window` |
| `/agents/auto-enroll` | name, created_at | name asc | |
| `/hosts/{id}/snapshots` | collected_at | collected_at desc | Fix: cursor/sort mismatch |
| `/hosts/{id}/changes` | detected_at, kind | detected_at desc | Fix: honour size |
| audit | ts | ts desc | D6 window |

## ipam
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/ip-addresses` | address (inet order), hostname, mac, status, address_type, last_seen, created_at | address asc | Fix: honour `hostname`; live patch → reload |
| `/devices` | name, device_type, status, manufacturer, location, created_at | name asc | |
| `/subnets` | cidr (inet order), name, vlan, location, status, utilization (if stored) | cidr asc | Fix: honour `ip_version` |
| `/vlans` | vlan_id, name, domain, status | vlan_id asc | |
| `/ip-scans` | created_at, status, subnet | created_at desc | Idx created_at; live patch → reload |
| `/ip-groups/{id}/members`, `/host-groups/{id}/members` | sequence, name | sequence asc | |
| device sub-lists (interfaces, packages, addresses, guests) | name (+ version for packages) | name asc | |
| audit | ts | ts desc | D6 window |

## dns
| Endpoint | Sortable | Default | Notes |
|---|---|---|---|
| `/zones` | name, kind, serial, updated_at | name asc | add sort to existing page/total |
| `/zones/{id}/records` (PowerDNS, in Go) | name, type, ttl | name asc (reversed-label order kept as `name`) | in-memory `SortSlice` + `Window`, max 500 → 200 |
| `/templates`, `/supermasters` | name / ip, nameserver | name / ip asc | |

## lcm (visibility → SQL, D5)
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/certificates` | common_name/spiffe_id, issuer, status, not_before, not_after, created_at | created_at desc | Idx created_at, not_after; Fix: broken cursor |
| `/issuers` | name, kind, created_at | name asc | |
| `/requests` | created_at, status, subject | created_at desc | |
| `/jobs` | created_at, status, run_after | created_at desc | Idx created_at |
| `/secrets`, `/webhooks` | name, created_at | name asc | |
| `/audit` | ts | ts desc | D6 window |

## notification (visibility → SQL for channels/templates)
| Endpoint | Sortable | Default | Notes |
|---|---|---|---|
| `/channels` | name, type, created_at | name asc | |
| `/templates` | name, channel, updated_at | name asc | |
| `/messages` | created_at, subject, status | created_at desc | |
| `/notifications` (log, hypertable) | created_at, status, channel | created_at desc | D6 window |
| `/categories` | name, sort_order | sort_order asc | |
| `/audit` | ts | ts desc | D6 window |

## warden (visibility → SQL for root folder)
| Endpoint | Sortable | Default | Idx / Notes |
|---|---|---|---|
| `/secrets` (folder view) | name, type, updated_at, created_at | name asc | Idx lower(name), created_at; folders listed first, then secrets |
| `/secrets/search` | relevance (default), name, updated_at | relevance desc | offset cursor → page |
| `/secrets/{id}/shares` | created_at, expires_at | created_at desc | |
| `/audit` | ts | ts desc | D6 window |

## paperless
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/documents` | name, file_size, mime_type, status, processing_status, created_at | created_at desc | Idx created_at, lower(name); Fix: 100-row cap, add id tie-breaker |

## deployer
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/configurations` | name, provider_type, status, created_at | name asc | |
| `/targets` | name, created_at | name asc | Fix: N+1 configuration ids |
| `/jobs` (+ dashboard) | created_at, status, job_type, completed_at | created_at desc | Idx created_at; Fix: job_type/target filters into SQL |
| job children / history | created_at | created_at desc | |

## scheduler
| Endpoint | Sortable | Default | Idx |
|---|---|---|---|
| `/tasks` | name, type, state, next_run_at, updated_at | name asc | (`TaskIDs` keeps name order) |
| `/executions` | created_at, status, duration, trigger | created_at desc | Idx status; D6 window not needed (indexed, retention 90 days) |

## signing
| Endpoint | Sortable | Default | Idx / Notes |
|---|---|---|---|
| `/templates` | name, status, updated_at | updated_at desc | Idx lower(name); backup pins its own order |
| `/submissions` | title, status, created_at, completed_at | created_at desc | Idx lower(title) |
| `/certificates` (admin) | subject, kind, status, not_after, created_at | created_at desc | |
| `/inbox` | created_at, title, status | created_at desc | gains pager |

## ticket
| Endpoint | Sortable | Default | Idx / Notes |
|---|---|---|---|
| `/tickets` | number, subject, status, priority, assignee, created_at, updated_at | created_at desc | Idx updated_at; gRPC `List` gains optional sort |
| `/mailboxes`, `/rules`, `/tags` | name (rules: sort_order) | name / sort_order asc | |

## hr
| Endpoint | Sortable | Default | Notes |
|---|---|---|---|
| `/requests` | start_date, end_date, status, days, created_at, user (name via member join) | start_date desc | `All:true` callers unchanged |
| `/allowances` | year, user (name via member join), type, total, remaining | year desc | |
| `/holidays`, `/absence-types` | date / name, sort_order | date asc / sort_order asc | |

## auth console
| Endpoint | Sortable | Default | Idx / Fix |
|---|---|---|---|
| `/api/v1/admin/users` | email, display_name, status, last_login_at, created_at | email asc | Idx lower(email); Fix: 200-row cap, add total |
| `/api/v1/admin/audit` | ts | ts desc | Fix: equal-timestamp skips (tie-breaker id); D6 window |
| `/admin/groups`, `/admin/groups/{id}/members`, roles, clients, operator tenants, sessions, directories | name / added_at / created_at as applicable | name asc (members: added_at desc) | |

## portal (gateway operations)
| Endpoint | Sortable | Default | Notes |
|---|---|---|---|
| `/gateway/v1/ops/audit` | ts, module, event_type | ts desc | D6 window; response `events` → `items` |
| `/gateway/v1/ops/registrations` (in-memory registry) | module, registered_at, expires_at | module asc | bare array → Page, `Window` |
| `/gateway/v1/ops/allowlist` | spiffe_id, created_at, revoked_at | spiffe_id asc | bare array → Page |
