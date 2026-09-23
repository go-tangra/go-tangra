# Implementation Plan: Platform Frontend Rework (FlyonUI + Zod)

**Branch**: `013-flyonui-frontend-rework` | **Date**: 2026-09-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/013-flyonui-frontend-rework/spec.md`

## Summary

Re-platform all ten Freya front-ends (gateway shell host, auth console, and the
warden/notification/lcm/deployer/paperless/inventory/ipam/asset Module-Federation
remotes) from Vuetify/Materio onto FlyonUI 2 + Tailwind CSS 4, with one shared,
versioned UI package (`@freya/ui` at `ui/kit`) that owns every cross-module component,
the API client, theme tokens and a Zod-4-based form layer (`useZodForm`, shared
schemas, the reason/validation vocabulary). Forms declare one Zod schema each that is
the single source of truth for validation and for the API payload type. Interactive
behaviour is Vue-owned (FlyonUI's DOM-scanning JS is not loaded), so the nonce-based
CSP stays untouched. Migration is incremental — kit → shell (keeping `vuetify` shared
transitionally) → remotes one at a time (asset first as the reference) → auth console →
Vuetify removal — enforced by static checks for duplication, legacy imports/ad-hoc
validation and bundle size, plus per-component a11y tests and per-module Playwright
flows at 320/768/1280 px.

## Technical Context

**Language/Version**: TypeScript ~5.9 (strict), Vue 3.5, Node 22 (build); Go services untouched.

**Primary Dependencies**: `flyonui` 2.x, `tailwindcss` 4 + `@tailwindcss/vite`,
`@iconify/tailwind4` + `@iconify-json/mdi`, `zod` 4, existing `vue-router` 5, `pinia` 4,
`@casl/ability`/`@casl/vue`, `@module-federation/vite`; removed at the end: `vuetify`,
`vite-plugin-vuetify`, `@mdi/font`. Kit tests: `vitest`, `@vue/test-utils`,
`vitest-axe`; e2e: Playwright (already present). See research.md R1–R9.

**Storage**: N/A (browser-only; theme preference persisted where it is today).

**Testing**: kit unit + a11y tests (every component/schema helper); each front-end's
`lint` (eslint + vue-tsc) and unit tests; Playwright flow per module at three viewports
(skips without operator credentials); static checks `check-duplicates`,
`check-no-legacy`, `check-bundle-size`; catalogue screenshot regression.

**Target Platform**: evergreen browsers (Chrome/Firefox/Safari current), phone ≥ 320 px
to desktop; served by the gateway shell and module Freya HTTP servers under the
existing edge CSP.

**Project Type**: web front-ends (one library package + ten Vite apps) inside a Go
monorepo; a root npm workspace is introduced.

**Performance Goals**: shell + one module ≤ 75% of today's gzipped JS+CSS (SC-007);
inline validation feedback < 100 ms (SC-008); first render of a migrated module ≤ today.

**Constraints**: like-for-like behaviour (routes, CASL gating, live updates, uploads);
Module-Federation contract preserved (only the shared list changes); CSP unchanged (no
`style=` attributes, no inline scripts); no component in more than one place; every
form schema-validated; WCAG 2.2 AA; incremental releases keep the platform usable.

**Scale/Scope**: 10 front-ends, ~82 views, ~45 module-local components today (→ ~35
kit components), ~60 forms; 56 dialog usages and 32 forms to migrate.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Secure by Default**: No new runtime options. The kit's secret field defaults to
      `autocomplete="off"`/no persistence; nothing weakens edge defaults. PASS.
- [x] **II. Zero Trust**: No new endpoints or calls; session/CSRF/platform-token flows and
      CASL distribution unchanged. N/A (browser-only), verified by the unchanged
      `TestAbilitiesMatchDecisions`.
- [x] **III. Boundary Validation**: Client Zod validation is UX only; the OpenAPI-validated
      module edges remain authoritative (SR-001). No `style=`/inline script; CSP unchanged
      (SR-002). PASS.
- [x] **IV. Test-First (NON-NEGOTIABLE)**: tasks will list kit component/schema tests,
      a11y tests and per-module flow tests before the corresponding implementation;
      negative tests: invalid/boundary form inputs, incompatible-remote load, secret-field
      autofill, CSP violation check in Playwright. Coverage: kit ≥ 80% statements;
      `forms/` and `api/` 100% (they decide what reaches the server). PASS.
- [x] **V. Observability**: No audit changes; shared-version mismatches and unknown API
      reasons are logged client-side (`console.warn`) without secrets; correlation id
      header passthrough kept in `@freya/ui/api`. PASS.
- [x] **VI. Supply Chain**: New npm deps justified in research.md R1/R3/R7/R9, pinned by the
      workspace lock file, `npm audit --audit-level=high` in CI, `ignore-scripts` in
      `.npmrc`; no Go dependency changes (`govulncheck` unaffected). PASS.
- [x] **VII. Simplicity**: One package, one form composable (no vee-validate), no FlyonUI
      JS layer, explicit `createApi({ base })`; complexity recorded below. PASS.
- [x] **Threat Model**: STRIDE table in research.md (browser-side). PASS.

Post-design re-check (after Phase 1): unchanged — all gates PASS.

## Project Structure

### Documentation (this feature)

```text
specs/013-flyonui-frontend-rework/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R10 + threat model
├── data-model.md        # Phase 1: kit inventory, schema model, vocabulary, front-end states
├── quickstart.md        # Phase 1: validation scenarios
├── contracts/
│   ├── ui-kit-api.md          # @freya/ui public API + static checks
│   └── federation-changes.md  # amendment to specs/003 federation contract
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
package.json                      # npm workspaces: ui/kit, services/gateway/shell, services/auth/console, services/*/ui
.npmrc                            # ignore-scripts=true, save-exact
ui/
├── MIGRATION.md                  # per-front-end status: legacy | migrated (drives check-no-legacy)
├── scripts/
│   ├── check-duplicates.mjs
│   ├── check-no-legacy.mjs
│   └── check-bundle-size.mjs     # records/compares gzipped baseline
└── kit/                          # @freya/ui
    ├── package.json  vite.config.ts  tsconfig.json  eslint.config.js
    ├── src/
    │   ├── index.ts              # components + composables
    │   ├── theme.css             # freya-light / freya-dark tokens
    │   ├── components/           # Ui*.vue (see data-model inventory)
    │   ├── composables/          # useBreakpoint, useToast, useConfirm, useTheme, useFocusTrap
    │   ├── forms/                # useZodForm.ts, messages.ts, schemas/*.ts, zodToFields.ts
    │   ├── api/                  # client.ts (createApi, upload, fileUrl, describe, ApiError)
    │   └── icons.ts              # mdi-* name → iconify class map
    ├── catalogue/                # Vite app rendering every component in both themes
    └── tests/                    # vitest + @vue/test-utils + vitest-axe
services/gateway/shell/           # host: layouts on UiAppShell, RemoteBoundary → UiRemoteBoundary, theme via data-theme,
                                  # module-federation.config.ts shared list (+zod, +@freya/ui; vuetify until US5)
services/auth/console/            # standalone app on the kit
services/<module>/ui/             # remotes: src/schemas/*.ts (Zod per form), views on kit components,
                                  # local components only when module-unique; module-federation.config.ts shared list
services/*/Dockerfile             # UI stage builds the workspace (root context already) and the kit before the app
.github/workflows/ci.yml          # kit job + static checks + per-front-end lint/test/build/audit
```

**Structure Decision**: a root npm workspace with one library package (`ui/kit`) consumed
by every front-end; module remotes keep their existing directories and federation setup,
adding a `schemas/` folder; the shell remains the host. Go services, manifests and
Docker build contexts are unchanged apart from the UI build stage building the kit first.

## Migration order and increments

1. Foundations: workspace, `ui/kit` (theme, primitives, forms, api, icons, catalogue,
   tests, static checks), `ui/MIGRATION.md`.
2. Shell on the kit (`vuetify` still shared) — platform fully usable with legacy remotes.
3. asset remote (reference migration: CRUD, trees, uploads, stream, dashboard).
4. inventory, ipam, paperless, deployer, lcm, notification, warden (warden last: secret
   fields, most review).
5. auth console.
6. Drop `vuetify`/`@mdi/font` from the shell shared list and every package; enable
   strict `check-no-legacy` for all; record bundle baseline vs result.

## Complexity Tracking

| Added complexity | Why it is needed | Simpler alternative rejected because |
|---|---|---|
| Root npm workspace + one library package | One copy of every shared component/schema (FR-001/002) | copying components per module violates the no-duplication requirement; a registry adds infra |
| Transitional dual shared runtime (`vuetify` + `@freya/ui`) in the shell | Lets remotes migrate one at a time with the platform usable throughout (FR-017) | big-bang cutover of ten apps is too large to verify safely |
| Home-grown `useZodForm` + Vue-owned interactive components | Exact focus/blur/submit semantics, testable a11y, no DOM auto-init across remotes, CSP-safe | vee-validate / FlyonUI JS add shared singletons and DOM-scanning behaviour the shell cannot control |
| Three static checks | Machine-enforced "one copy", "no legacy", "smaller bundle" success criteria | code review alone does not scale across ten front-ends |
