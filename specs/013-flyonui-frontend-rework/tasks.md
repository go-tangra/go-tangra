# Tasks: Platform Frontend Rework (FlyonUI + Zod)

**Feature**: 013-flyonui-frontend-rework | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Organized by phase; user-story phases are independently shippable increments. Tests are
MANDATORY and precede implementation (Constitution IV). `[P]` = parallelizable (distinct
files, no dependency on an incomplete task). Kit package: `ui/kit` (`@freya/ui`).
Front-ends: `services/gateway/shell`, `services/auth/console`, `services/<module>/ui`.
Survey of today's duplicates that the kit absorbs: `StatsCard.vue` (8 modules),
`AuditTable.vue` + `PermissionDrawer.vue` (warden, notification, lcm), `SecretDrawer.vue`
(warden, lcm), `KeyValueTable.vue` (inventory, ipam, lcm), `*Drawer.vue` detail/edit
side-sheets (warden, notification, lcm, deployer, paperless), `RecordDialog.vue` /
`TreeNodes.vue` / `EntityDocuments.vue` (asset), `api/client.ts` (all ten).

## Phase 1: Setup (workspace + tooling)

- [X] T001 Create the root npm workspace `package.json` (workspaces: `ui/kit`, `services/gateway/shell`, `services/auth/console`, `services/*/ui`; scripts `lint`, `test`, `build`, `check`) and `.npmrc` (`ignore-scripts=true`, `save-exact=true`); run `npm install` and commit the single root `package-lock.json` (remove per-app lock files).
- [X] T002 [P] Scaffold `ui/kit/` (`package.json` name `@freya/ui` with subpath exports `.`, `./forms`, `./api`, `./theme.css`; `vite.config.ts` library build ESM + `vite-plugin-dts`; `tsconfig.json` strict; `eslint.config.js` mirroring the modules plus `vue/no-restricted-static-attribute`/`no-restricted-syntax` rules forbidding `style`/`:style` and `:rules`; `vitest` + `@vue/test-utils` + `vitest-axe` config).
- [X] T003 [P] Add `ui/kit/src/theme.css` (`@import "tailwindcss"; @plugin "flyonui"; @import "flyonui/variants.css"; @plugin "@iconify/tailwind4"`; daisyUI themes `freya-light` (default) and `freya-dark` with brand tokens; base layer) and `ui/kit/src/icons.ts` (`mdi-*` → `icon-[mdi--*]` class map covering every icon name in `services/*/pkg/*manifest/manifest.go` Nav entries).
- [X] T004 [P] Write `ui/MIGRATION.md` (table of the 10 front-ends, status `legacy`, baseline gzipped JS+CSS sizes of shell + asset measured from current `dist/`) and the static checks `ui/scripts/check-duplicates.mjs`, `ui/scripts/check-no-legacy.mjs`, `ui/scripts/check-bundle-size.mjs` per contracts/ui-kit-api.md, plus `ui/scripts/README.md`.
- [X] T005 [P] Add `ui/kit/catalogue/` (Vite + Vue app: one page per kit component rendering all variants, a theme toggle, `npm run -w ui/kit catalogue`) and Playwright config `ui/kit/catalogue/playwright.config.ts` for screenshot regression at 320/768/1280 px.
- [X] T006 Update CI `.github/workflows/ci.yml`: new `ui-kit` job (install workspace, `npm run -w ui/kit lint test build`, `node ui/scripts/check-duplicates.mjs`, `npm audit --audit-level=high --workspaces`); make every front-end job build the kit first (`npm run -w ui/kit build`).

## Phase 2: Foundational (kit core — blocks every story)

### Tests (write first, must fail)
- [X] T007 [P] `ui/kit/tests/forms/useZodForm.spec.ts` — blur/submit validation, first-invalid focus, no `onSubmit` while invalid, payload equals `z.output`, server `validation_failed` detail → field errors, `conflict` → banner, `reset`.
- [X] T008 [P] `ui/kit/tests/forms/schemas.spec.ts` + `ui/kit/tests/forms/schemas.fuzz.spec.ts` — shared primitives (`email`, `uuid`, `cidr`, `isoDate`, `money`, `nonEmpty`, `tagMap`, `slug`) accept/reject tables and property-based fuzz (random strings never throw, outputs are normalised).
- [X] T009 [P] `ui/kit/tests/forms/messages.spec.ts` — every API reason and every Zod issue code maps to text; unknown reason → generic fallback and `console.warn` without echoing detail (SR-006).
- [X] T010 [P] `ui/kit/tests/api/client.spec.ts` — `createApi({base})`, CSRF header on non-GET, `ApiError{status,reason}`, 204 handling, `upload()` multipart, `fileUrl()`; request-id passthrough.
- [X] T011 [P] `ui/kit/tests/components/primitives.spec.ts` — `UiPage/UiToolbar/UiGrid/UiCard/UiSection/UiAlert/UiBadge/UiStatusChip/UiEmptyState/UiErrorState/UiSkeleton/UiAvatar/UiCopyButton/UiIcon`: render, class passthrough, **no `style` attribute in output**, axe clean in both themes.
- [X] T012 [P] `ui/kit/tests/components/overlays.spec.ts` — `UiDialog` (focus trap, Esc, restore focus to trigger, full-screen below `md`), `UiConfirm`/`useConfirm`, `UiDrawer` (side sheet), `UiDropdownMenu` (arrow keys, outside click), `UiTooltip`, `UiToast`/`useToast` (timeout, reduced-motion), axe clean.
- [X] T013 [P] `ui/kit/tests/components/fields.spec.ts` — `UiField/UiInput/UiTextarea/UiSelect/UiCombobox/UiCheckbox/UiSwitch/UiDateInput/UiNumberInput/UiFilePicker/UiTagEditor` label/hint/error wiring (`aria-describedby`, `aria-invalid`), `UiSecretField` has `autocomplete="off"`, is never written to `localStorage`/drafts and masks by default (SR-004 negative test).
- [X] T014 [P] `ui/kit/tests/components/data.spec.ts` — `UiDataTable` (sort, stacked below `md`, contained horizontal scroll above, cursor `loadMore`, empty/loading, row actions, virtualised > 200 rows), `UiKeyValueTable`, `UiStatTile/UiStatGrid`, `UiPagination`, `UiTabs`, `UiAccordion`, `UiTree` (arrow keys, expand/collapse, selection), `UiLiveIndicator`.
- [X] T015 [P] `ui/kit/tests/components/composite.spec.ts` — `UiRecordDialog` (schema-driven fields, `zodToFields`, submit → `saved`, server refusal inline), `UiRecordDrawer`, `UiDocumentList` (list/upload/download/delete with a fake api), `UiAuditTable` (cursor paging, filters), `UiPermissionDrawer` (grant/revoke rows), `UiAppShell/UiNavDrawer/UiAppBar/UiUserMenu` (drawer overlay below `lg`, rail above), `UiRemoteBoundary` (error state + retry, version-mismatch message).
- [X] T016 [P] `ui/kit/tests/composables.spec.ts` — `useBreakpoint`, `useTheme` (sets `data-theme`, persists preference), `useFocusTrap`.

### Implementation
- [X] T017 [P] `ui/kit/src/forms/useZodForm.ts`, `ui/kit/src/forms/messages.ts`, `ui/kit/src/forms/schemas/*.ts`, `ui/kit/src/forms/zodToFields.ts`, `ui/kit/src/forms/index.ts` (zod 4).
- [X] T018 [P] `ui/kit/src/api/client.ts` + `ui/kit/src/api/index.ts` (port of the module `api/client.ts` with `createApi`, `upload`, `fileUrl`, `describe`, `ApiError`).
- [X] T019 [P] Primitives `ui/kit/src/components/{UiPage,UiToolbar,UiGrid,UiCard,UiSection,UiAlert,UiBadge,UiStatusChip,UiEmptyState,UiErrorState,UiSkeleton,UiAvatar,UiCopyButton,UiIcon}.vue`.
- [X] T020 [P] Overlays `ui/kit/src/components/{UiDialog,UiConfirm,UiDrawer,UiDropdownMenu,UiTooltip,UiToast}.vue` + `ui/kit/src/composables/{useFocusTrap,useToast,useConfirm}.ts` (Vue-owned state; FlyonUI classes only; reduced-motion).
- [X] T021 [P] Fields `ui/kit/src/components/{UiForm,UiField,UiInput,UiTextarea,UiSelect,UiCombobox,UiCheckbox,UiSwitch,UiDateInput,UiNumberInput,UiFilePicker,UiSecretField,UiTagEditor}.vue` bound to `useZodForm` field state.
- [X] T022 [P] Data `ui/kit/src/components/{UiDataTable,UiKeyValueTable,UiStatTile,UiStatGrid,UiPagination,UiTabs,UiAccordion,UiTree,UiLiveIndicator}.vue` + `ui/kit/src/composables/useBreakpoint.ts`.
- [X] T023 Composite `ui/kit/src/components/{UiRecordDialog,UiRecordDrawer,UiDocumentList,UiAuditTable,UiPermissionDrawer,UiAppShell,UiNavDrawer,UiAppBar,UiUserMenu,UiRemoteBoundary}.vue` + `ui/kit/src/composables/useTheme.ts` + `ui/kit/src/index.ts` exports.
- [X] T024 Catalogue pages in `ui/kit/catalogue/src/pages/*.vue` for every component (both themes, synthetic data only — SR-005) and the screenshot baseline; `npm run -w ui/kit build` produces `dist/` with types; `check-duplicates` passes.

## Phase 3: User Story 1 — Shared kit + re-platformed shell (Priority: P1) 🎯 MVP

**Goal**: the shell (host) runs on the kit; legacy remotes still load. **Independent test**: quickstart §2.

### Tests
- [X] T025 [P] [US1] `services/gateway/shell/tests/unit/layout.spec.ts` — `Default.vue`/`Bare.vue` on `UiAppShell`: drawer collapses below `lg`, nav entries from `/gateway/v1/me/modules` render with `UiIcon` (mdi names), user menu, theme toggle sets `data-theme`.
- [X] T026 [P] [US1] `services/gateway/shell/tests/unit/remote-boundary.spec.ts` — a remote whose shared `@freya/ui` version mismatches (`strictVersion`) renders `UiRemoteBoundary` error state with retry; a throwing remote is isolated (FR-011).
- [X] T027 [P] [US1] `services/gateway/shell/tests/e2e/shell-responsive.spec.ts` — Playwright at 320×640 / 768×1024 / 1280×800: no horizontal page scroll, drawer/menu reachable, CSP violation listener asserts zero `securitypolicyviolation` events (skips without `E2E_OPERATOR_PASSWORD`).

### Implementation
- [X] T028 [US1] `services/gateway/shell/package.json` + `vite.config.ts` + `module-federation.config.ts`: add `@freya/ui` (workspace), `zod`, `tailwindcss`, `@tailwindcss/vite`, `flyonui`, `@iconify/tailwind4`, `@iconify-json/mdi`; shared list per contracts/federation-changes.md (`zod`, `@freya/ui` strict; keep `vuetify` for now); `src/main.css` importing `@freya/ui/theme.css` with `@source` for the kit dist.
- [X] T029 [US1] Rebuild `services/gateway/shell/src/layouts/{Default,Bare}.vue`, `src/App.vue`, `src/components/RemoteBoundary.vue` (thin wrapper over `UiRemoteBoundary`), `src/views/{Home,Forbidden,NotFound,Outage,ModuleError}.vue` and `src/views/ops/*` on kit components; theme via `useTheme` replacing `src/theme/materio.{css,ts}` (keep the Vuetify instance only for legacy remotes).
- [X] T030 [US1] Mount the single `UiToast` host and confirm host in `App.vue`; expose `api` built with `createApi` from `@freya/ui/api` in `src/federation/runtime.ts` (the `ctx.api` remotes receive).
- [X] T031 [US1] Mark `gateway/shell` `migrated (host, transitional vuetify)` in `ui/MIGRATION.md`; `npm run lint && npm run build` green; update `specs/003-application-gateway/contracts/federation.md` with the new shared list (reference contracts/federation-changes.md).

## Phase 4: User Story 2 — Schema-driven form validation (Priority: P1)

**Goal**: forms across modules validate through Zod schemas with the shared vocabulary. **Independent test**: quickstart §3.

### Tests
- [X] T032 [P] [US2] `services/asset/ui/tests/unit/schemas.spec.ts` — asset/category/supplier/location/consumable/license/insurance schemas: required, ranges, `valid_to ≥ valid_from`, tag map limit, output types equal the OpenAPI payloads (`api/openapi/asset.yaml` field list).
- [X] T033 [P] [US2] `services/ipam/ui/tests/unit/schemas.spec.ts` — subnet CIDR/VLAN/address schemas incl. `cidr` negative cases (`999.1.1.1/8`, `10.0.0.0/33`) and fuzz.
- [X] T034 [P] [US2] `services/auth/console/tests/unit/schemas.spec.ts` — sign-in, MFA code (6 digits or `XXXXX-XXXXX` recovery), invitation accept (password policy), invite (email + roles) schemas with negative cases.
- [X] T035 [P] [US2] `services/deployer/ui/tests/unit/schemas.spec.ts` — target/configuration schemas incl. secret refs never echoed in error text.

### Implementation
- [X] T036 [P] [US2] `services/asset/ui/src/schemas/{asset,category,supplier,location,consumable,license,insurance,policyAsset,backup}.ts`; delete hand-written `*Input` interfaces in `src/api/types.ts` in favour of `z.output` types.
- [X] T037 [P] [US2] `services/ipam/ui/src/schemas/{subnet,address,device,vlan,location,group,scan}.ts`.
- [X] T038 [P] [US2] `services/auth/console/src/schemas/{signIn,mfa,acceptInvitation,recovery,password,invite,role,group,policy,client,tenant}.ts`.
- [X] T039 [P] [US2] `services/deployer/ui/src/schemas/{target,configuration,job}.ts`, `services/lcm/ui/src/schemas/{issuer,certificate,request,secret}.ts`, `services/notification/ui/src/schemas/{channel,template,message,category}.ts`, `services/warden/ui/src/schemas/{secret,folder,share,generator,import}.ts`, `services/paperless/ui/src/schemas/{document,category,search,upload}.ts`, `services/inventory/ui/src/schemas/{enrollToken,tags}.ts`.
- [X] T040 [US2] Extend `ui/scripts/check-no-legacy.mjs` to flag `:rules=`, `v-form` and inline validators outside `schemas/`; wire `node ui/scripts/check-no-legacy.mjs` into every front-end `npm run lint`.

## Phase 5: User Story 3 — Module remotes migrated one by one (Priority: P2)

**Goal**: each remote on the kit, like-for-like. **Independent test**: quickstart §4 per module. Order: asset (reference), inventory, ipam, paperless, deployer, lcm, notification, warden.

### Tests (per module, write first)
- [X] T041 [P] [US3] `services/asset/ui/tests/unit/views.spec.ts` — list/detail/tree/documents/dashboard render on kit components; manage controls hidden without abilities (`useAbility` stub); stream event patches the list.
- [X] T042 [P] [US3] `services/asset/ui/tests/e2e/asset-flow.spec.ts` — quickstart flow (create → assign → unassign → history, photo + document, sync preview, dashboard) at the three viewports; CSP-violation listener; skips without operator creds.
- [X] T043 [P] [US3] `services/inventory/ui/tests/e2e/inventory-flow.spec.ts` + `tests/unit/views.spec.ts` (hosts list/detail with `UiKeyValueTable`, agents enrol dialog shows the one-time token once via `UiSecretField`/`UiCopyButton`).
- [X] T044 [P] [US3] `services/ipam/ui/tests/e2e/ipam-flow.spec.ts` + `tests/unit/views.spec.ts` (subnet tree, address table, scans, power/KVM controls gated by platform-admin ability).
- [X] T045 [P] [US3] `services/paperless/ui/tests/e2e/paperless-flow.spec.ts` + `tests/unit/views.spec.ts` (upload dialog, category tree, search, document drawer).
- [X] T046 [P] [US3] `services/deployer/ui/tests/e2e/deployer-flow.spec.ts` + `tests/unit/views.spec.ts` (targets/configurations/jobs drawers, secret refs masked).
- [X] T047 [P] [US3] `services/lcm/ui/tests/e2e/lcm-flow.spec.ts` + `tests/unit/views.spec.ts` (issuers, certificates, requests, secrets drawer, `./header` cert badge).
- [X] T048 [P] [US3] `services/notification/ui/tests/e2e/notification-flow.spec.ts` + `tests/unit/views.spec.ts` (inbox, channels, templates, `./header` bell, backup dialog).
- [X] T049 [P] [US3] `services/warden/ui/tests/e2e/warden-flow.spec.ts` + `tests/unit/views.spec.ts` (folder tree, secret drawer reveal/copy never persists, share dialog, Bitwarden import parser negative cases, generator).

### Implementation — asset (reference)
- [X] T050 [US3] `services/asset/ui`: package/vite/federation config on the kit (T028 pattern), `src/main.css`, `src/api/client.ts` → `createApi({ base: '/api/asset/v1' })` re-export, delete `src/components/{RecordDialog,TreeNodes,EntityDocuments,StatsCard}.vue`.
- [X] T051 [US3] Rebuild `services/asset/ui/src/views/{assets/index,assets/detail,categories/index,suppliers/index,locations/index,consumables/index,licenses/index,insurance/index,inventory-sync/index,dashboard/index}.vue` on `UiDataTable/UiRecordDialog/UiTree/UiDocumentList/UiStatGrid/UiTabs` with the T036 schemas; keep routes/nav; mark `asset` `migrated`.

### Implementation — remaining remotes (each: config + css + api re-export, delete local duplicates, rebuild views, mark migrated)
- [X] T052 [P] [US3] `services/inventory/ui` — views `hosts/{index,detail}`, `agents/index`, `dashboard/index`; delete `components/{KeyValueTable,StatsCard,EnrollTokenDialog}.vue` (enrol dialog → `UiRecordDialog` + `UiSecretField`).
- [X] T053 [P] [US3] `services/ipam/ui` — views `subnets, addresses, devices, vlans, locations, groups, scans, dashboard`; delete `components/StatsCard.vue`; subnet tree on `UiTree`.
- [X] T054 [P] [US3] `services/paperless/ui` — views `documents, categories, search, dashboard`; delete `components/{CategoryNode,DocumentDrawer,StatsCard,UploadDialog}.vue` (→ `UiTree`, `UiRecordDrawer`, `UiStatGrid`, `UiFilePicker` dialog).
- [X] T055 [P] [US3] `services/deployer/ui` — views `targets, configurations, jobs, dashboard`; delete `components/{ConfigurationDrawer,JobDrawer,TargetDrawer,StatsCard}.vue` (→ `UiRecordDrawer`).
- [X] T056 [P] [US3] `services/lcm/ui` — views `issuers, certificates, requests, secrets, permissions, audit, dashboard`; delete `components/{AuditTable,CertificateDrawer,IssueDialog,IssuerDrawer,PermissionDrawer,SecretDrawer,StatsCard}.vue`; `HeaderCert.vue` rebuilt on kit primitives (module-unique, keeps `./header`).
- [X] T057 [P] [US3] `services/notification/ui` — views `inbox, messages, channels, templates, categories, log, permissions`; delete `components/{AuditTable,BackupDialog,ChannelDrawer,MessageDrawer,PermissionDrawer,TemplateDrawer,StatsCard}.vue`; `HeaderBell.vue` rebuilt (keeps `./header`).
- [X] T058 [US3] `services/warden/ui` — views `secrets, generator, permissions`; delete `components/{AuditTable,FolderTree,PermissionDrawer,SecretDrawer,ShareDialog,SharesPanel,StatsCard,VersionDrawer}.vue` (→ kit); keep `BitwardenImportDialog.vue`/`FolderActions.vue` only if module-unique after review; secrets rendered through `UiSecretField` (last, most review).
- [X] T059 [US3] Update each module's Dockerfile UI stage (`services/{asset,inventory,ipam,paperless,deployer,lcm,notification,warden}/Dockerfile`) to install the root workspace and build `ui/kit` before `npm run build`; verify `docker compose -p freya-stack … up -d --build` serves each migrated remote (quickstart §4).

## Phase 6: User Story 4 — Auth console migrated (Priority: P2)

**Goal**: standalone console on the kit. **Independent test**: quickstart §5.

### Tests
- [X] T060 [P] [US4] `services/auth/console/tests/unit/views.spec.ts` — `SignIn`, `MfaChallenge` (TOTP or recovery code → shared refusal message + refocus), `AcceptInvitation` (password rules inline, recovery codes legible/copyable, `UiSecretField` on password), `Account`, `Sessions`, admin `Users/UserDetail/InviteDialog/Roles/RoleEditor/Groups/GroupDetail/Policy/Clients/Audit`, operator `Tenants/TenantDetail` render on kit components with abilities gating.
- [X] T061 [P] [US4] `services/auth/console/tests/e2e/console-flow.spec.ts` — accept invitation → enrol MFA → sign out → sign in (TOTP, then recovery code) → change password → invite user → edit roles, at the three viewports; asserts no `securitypolicyviolation`, password/code fields have `autocomplete="off"`.

### Implementation
- [X] T062 [US4] `services/auth/console/package.json`, `vite.config.ts`, `src/main.css`, `src/main.ts` (`useTheme`, `UiAppShell` layout, kit `createApi({ base: '/api/v1' })`), delete Vuetify plugin/theme files.
- [X] T063 [US4] Rebuild `services/auth/console/src/views/{SignIn,MfaChallenge,MfaEnrol,AcceptInvitation,ForgotPassword,ResetPassword,Recovery,Account,Sessions,Home,Forbidden,NotFound,Outage}.vue` on kit components + T038 schemas.
- [X] T064 [US4] Rebuild `services/auth/console/src/views/admin/{Users,UserDetail,InviteDialog,Roles,RoleEditor,Groups,GroupDetail,Policy,Clients,Audit}.vue` and `src/views/operator/{Tenants,TenantDetail}.vue` (`UiDataTable`, `UiRecordDialog`, `UiAuditTable`, `UiPermissionDrawer`); delete `src/components/*` duplicates; mark `auth/console` `migrated`; update `services/auth/Dockerfile` console stage for the workspace.

## Phase 7: User Story 5 — Old toolkit removed, duplication eliminated (Priority: P3)

**Goal**: no Vuetify anywhere; checks enforce the rules. **Independent test**: quickstart §6.

### Tests
- [X] T065 [P] [US5] `ui/scripts/tests/checks.spec.mjs` — fixtures proving `check-duplicates` fails on a kit/module or module/module basename clash, `check-no-legacy` fails on `vuetify`/`@mdi/font`/`:rules=`/`style=` in a `migrated` front-end and ignores `legacy` ones, `check-bundle-size` fails above 75% of baseline.
- [X] T066 [P] [US5] `services/gateway/shell/tests/unit/shared-runtime.spec.ts` — the shared list contains exactly `vue, vue-router, pinia, @casl/ability, @casl/vue, zod, @freya/ui`; a remote requesting `vuetify` fails into the error boundary.

### Implementation
- [X] T067 [US5] Remove `vuetify`, `vite-plugin-vuetify`, `@mdi/font` from every `package.json` (shell, console, 8 remotes), from `services/gateway/shell/module-federation.config.ts` shared list and every remote's `module-federation.config.ts`; delete `services/gateway/shell/src/theme/materio.*`; `npm install` refreshes the lock file.
- [X] T068 [US5] Set every front-end to `migrated` in `ui/MIGRATION.md`; run all three checks, record post-migration sizes in `check-bundle-size.mjs` and assert ≤ 75% baseline (SC-007); `npm audit --audit-level=high --workspaces` clean.
- [X] T069 [US5] Rebuild all images and run every module's quickstart flow once more in `deploy/stack` (`sg docker -c 'deploy/stack/up.sh'`), confirming `registered:true` for each module and the shell nav lists all modules.

## Phase 8: Polish & cross-cutting

- [X] T070 [P] Accessibility sweep: axe run over every catalogue page and every migrated view (`ui/kit/catalogue/tests/a11y.spec.ts`, per-module `tests/e2e/a11y.spec.ts`) with 0 serious/critical findings in both themes (SC-006); reduced-motion and 200% zoom spot checks documented in `ui/MIGRATION.md`.
- [X] T071 [P] Docs: `ui/kit/README.md` (usage, adding a component, the "second consumer moves it to the kit" rule, form recipe), `docs/frontend.md` (architecture, migration order, checks), update `deploy/stack/README.md` and each `services/*/deploy/README.md` UI notes; CHANGELOG entry.
- [X] T072 [P] Security review checklist for the feature in `specs/013-flyonui-frontend-rework/checklists/security.md`: CSP unchanged (edge headers diff), no `unsafe-inline`, secret fields audit (`grep -rn autocomplete`), dependency review of `flyonui`, `tailwindcss`, `zod`, `@iconify/*` (licences, maintainers, postinstall scripts none), fixtures synthetic.
- [X] T073 Kit coverage gate: `ui/kit` ≥ 80% statements overall, `src/forms` and `src/api` 100% (`vitest --coverage` thresholds in `ui/kit/vite.config.ts`); wire into the CI kit job.

## Dependencies & sequencing

- Phase 1 → Phase 2 → US1 (shell) are strictly sequential prerequisites for every remote.
- US2 (schemas) depends only on Phase 2 and can run in parallel with US1; each module's
  schema file must exist before that module's US3 rebuild.
- US3 modules are independent of each other once US1 + their US2 schemas are done
  (asset first as the reference; warden last).
- US4 depends on Phase 2 + T038 only (standalone app, no federation).
- US5 depends on every front-end being `migrated` (US1, US3 all modules, US4).
- Phase 8 runs after US5 (a11y sweep needs the final views).

## Parallel execution examples

- Phase 2: T007–T016 (tests) in parallel, then T017–T022 in parallel, then T023, T024.
- After US1: T036–T039 (schemas) in parallel with T041–T049 (module tests); then T052–T057
  (six remotes) in parallel by different agents, T058 (warden) after review of T050/T051.
- Phase 8: T070–T072 in parallel.

## Implementation strategy

MVP = Phase 1 + Phase 2 + US1: the kit exists with its catalogue and checks, and the shell
runs on it while every legacy remote still loads. Then US2 + asset (T050/T051) proves the
full pattern on the richest remote; the remaining remotes and the console follow
independently; US5 removes Vuetify and locks the rules in CI.

## Summary

- **Total tasks**: 73 across 8 phases.
- **Per story**: US1 = 7 (T025–T031), US2 = 9 (T032–T040), US3 = 19 (T041–T059), US4 = 5 (T060–T064), US5 = 5 (T065–T069); Setup 6, Foundational 18, Polish 4.
- **Parallelizable**: 49 tasks marked `[P]`.
- **Independent test per story**: US1 quickstart §1–2 (shell at 3 viewports + failing remote); US2 §3 (one form per module: inline errors, no request while invalid, payload = schema output); US3 §4 (each module's quickstart flow at 3 viewports, read-only vs manage operator); US4 §5 (invite → MFA → sign-in with TOTP and recovery code); US5 §6 (checks + audit + no vuetify in any package).
