# Phase 0 Research: Platform Frontend Rework (FlyonUI + Zod)

All Technical Context unknowns are resolved below. Each entry: Decision / Rationale /
Alternatives considered.

## R1. Component library and styling toolchain

- **Decision**: FlyonUI 2.x (current 2.4.1) on Tailwind CSS 4 via `@tailwindcss/vite`;
  every front-end's `main.css` is `@import "tailwindcss"; @plugin "flyonui"; @import
  "flyonui/variants.css"; @source "<kit dist>";` plus the shared theme file from the kit.
- **Rationale**: FlyonUI is a Tailwind plugin (daisyUI-based semantic classes + Tailwind
  utilities), so styling is static, build-time CSS delivered from `'self'` — compatible
  with the edge CSP (`style-src 'self' 'nonce-…'`, no `unsafe-inline`). Tailwind 4 is the
  only version FlyonUI 2 supports; its Vite plugin needs no PostCSS config.
- **Alternatives**: daisyUI alone (fewer components: no tree/advanced form patterns);
  PrimeVue/Naive UI (full Vue libraries, but the user chose FlyonUI); keeping Vuetify
  (rejected by the request).
- **Sources**: FlyonUI quick start and Vue guide — https://flyonui.com/docs/getting-started/quick-start/ , https://flyonui.com/docs/framework-integrations/vuejs/ .

## R2. Interactive behaviour: Vue-owned, not FlyonUI's DOM plugins

- **Decision**: The shared kit implements interactive components (dialog, dropdown,
  tabs, accordion, tooltip, toast, combobox, tree) as Vue components that own their
  state and ARIA wiring and use FlyonUI classes for presentation. The FlyonUI JavaScript
  bundle (`flyonui/flyonui`, `window.HSStaticMethods.autoInit()`) is **not** loaded.
- **Rationale**: The FlyonUI JS layer initialises by scanning the DOM (`autoInit` after
  each route change) — fragile across ten independently built Module-Federation remotes
  whose DOM the shell does not control, and a global-singleton contract between shell
  and remotes we would have to version. Vue-owned state also makes focus trapping,
  keyboard navigation and reduced-motion behaviour testable with `@vue/test-utils`
  and deterministic under `@axe-core`. One fewer global script for the CSP to cover.
- **Alternatives**: load FlyonUI JS in the shell and call `autoInit` in `router.afterEach`
  (FlyonUI's own Vue recipe) — rejected for the reasons above; headless UI libraries
  (Headless UI, Radix Vue/reka-ui) — viable, but an extra dependency for behaviour the
  kit needs to own anyway; kept as a fallback for the combobox if the home-grown one
  proves costly.

## R3. Form validation

- **Decision**: Zod 4 schemas as the single source of truth. Per form: an *input schema*
  (what the user edits) and a derived *payload* (`z.output`) that is the API body type;
  shared primitives (`email`, `cidr`, `uuid`, `money`, `tagMap`, `isoDate`, `nonEmpty`)
  live in `@freya/ui/forms`. A small kit composable `useZodForm(schema, options)` binds
  fields, validates on blur and on submit, focuses the first invalid field, maps server
  `validation_failed`/`conflict` refusals onto fields/banner, and exposes `values`,
  `errors`, `submit`, `reset`. Message vocabulary is one map in the kit.
- **Rationale**: Zod is the user's choice; `z.infer`/`z.output` give payload types for
  free; a ~150-line composable avoids a further dependency and gives exact control of
  focus/blur/submit semantics required by FR-007 and SC-008.
- **Alternatives**: vee-validate + `@vee-validate/zod` (mature, but another shared
  singleton across remotes and a second validation vocabulary); per-form hand-written
  rules (the status quo, rejected by FR-006).

## R4. Where the shared package lives and how it is consumed

- **Decision**: A root npm workspace (`package.json` with `workspaces: ["ui/kit",
  "services/gateway/shell", "services/auth/console", "services/*/ui"]`) and the package
  `@freya/ui` at `ui/kit` with subpath exports: `@freya/ui` (components + composables),
  `@freya/ui/forms` (Zod helpers, `useZodForm`, shared schemas, error vocabulary),
  `@freya/ui/theme.css` (theme tokens / daisyUI theme definitions), `@freya/ui/api`
  (fetch client with CSRF + `describe()`, currently copied in every module). Consumed as a
  workspace dependency (`"@freya/ui": "workspace:*"`), built to `ui/kit/dist` (ESM +
  types) by `npm run build -w ui/kit` before any front-end build. Dockerfiles already use
  the repo root as build context, so the workspace resolves inside images.
- **Rationale**: One copy, one version, no publishing step; TypeScript project
  references keep type-checking strict; Tailwind `@source` in each front-end points at
  the kit dist so the kit's classes are generated in that front-end's CSS.
- **Alternatives**: a git submodule or a published private registry package (extra
  infrastructure); copying components into each module (rejected: FR-002).

## R5. Module Federation shared runtime and the transitional period

- **Decision**: Shell and remotes share these singletons: `vue`, `vue-router`, `pinia`,
  `@casl/ability`, `@casl/vue`, `zod`, `@freya/ui`. During migration the shell ALSO keeps
  `vuetify` shared until the last remote is migrated (User Story 5 removes it). Each
  remote declares `@freya/ui` with the same `requiredVersion` range; the shell's
  `RemoteBoundary` treats a shared-version mismatch (MF runtime `shareStrategy`
  "version-first" + `strictVersion`) as a load failure and shows the retry card (FR-011).
  CSS is not shared: each build ships its own compiled Tailwind CSS (deduplication
  across remotes is unnecessary — a remote's CSS is a few KB gzipped; the shell's CSS
  carries the theme tokens and the kit's base classes).
- **Rationale**: keeps the federation contract intact (only the `shared` list changes),
  lets modules migrate one at a time, and makes the version guard automatic.
- **Alternatives**: big-bang cutover (all ten front-ends in one release — too large to
  verify); sharing CSS through the shell (would couple remote class usage to the
  shell's build; rejected).

## R6. Theming, dark mode and CSP

- **Decision**: Two daisyUI/FlyonUI themes defined once in `@freya/ui/theme.css`
  (`freya-light`, `freya-dark` with the platform brand colour) selected by
  `data-theme` on `<html>`, set by the shell from the operator's preference (current
  `theme` store, persisted as today). No `style=` attributes anywhere in kit or module
  templates (CSP `style-src` has no `unsafe-inline`); dynamic sizes use classes, CSS
  variables declared in the static CSS, or native elements (`<progress>` for bars).
  An ESLint rule (`vue/no-restricted-static-attribute`/custom) forbids `style` bindings.
- **Rationale**: preserves today's nonce-based CSP with zero relaxation (SR-002).
- **Alternatives**: `unsafe-inline` for styles (rejected); Vuetify's runtime style
  injection with nonce (goes away with Vuetify).

## R7. Icons

- **Decision**: Keep Material Design Icons names as the manifest/nav contract
  (`mdi-…`), rendered through `@iconify/tailwind4` with the `mdi` Iconify set
  (`icon-[mdi--laptop]`); the kit's `<UiIcon name="mdi-laptop">` maps the existing
  names, so module manifests and Go code stay unchanged.
- **Rationale**: manifests already declare `mdi-*` icons (gateway contract); Iconify's
  Tailwind plugin emits static CSS (CSP-safe) and only the icons actually used.
- **Alternatives**: the FlyonUI default Tabler set (would change every manifest);
  `@mdi/font` webfont (ships the full font, ~300 KB).

## R8. Responsiveness strategy

- **Decision**: Kit layout primitives (`UiPage`, `UiToolbar`, `UiGrid`, `UiCard`) use
  Tailwind responsive utilities with the breakpoints `sm 640 / md 768 / lg 1024 / xl
  1280`; `UiDataTable` renders a stacked card list below `md` and a horizontally
  scrollable table (scroll contained in the table wrapper) from `md`; dialogs are
  full-screen sheets below `md` and centred modals above; the nav drawer is an overlay
  below `lg` and a rail/persistent drawer above.
- **Rationale**: matches FR-005 and the 320/768/1280 verification points.
- **Alternatives**: container queries only (not yet worth the complexity); separate
  mobile screens (duplication).

## R9. Testing and quality gates

- **Decision**: Kit: Vitest + `@vue/test-utils` + `vitest-axe` for every component
  (behaviour, keyboard, a11y) and every schema/helper; a visual catalogue app
  (`ui/kit/catalogue`, plain Vite + Vue, one page per component, both themes) used for
  review and Playwright screenshot regression. Front-ends: existing `npm run lint`
  (eslint + vue-tsc strict) and unit tests, plus a Playwright flow per module run at
  viewports 320×640, 768×1024, 1280×800 (skips without `E2E_OPERATOR_PASSWORD`, as the
  existing e2e suites do). Static checks in `ui/scripts/`: `check-duplicates` (fails on a
  component name present in the kit and any module, or in ≥ 2 modules),
  `check-no-legacy` (fails on any `vuetify`/`@mdi/font` import and on `:rules=` /
  hand-written validators outside `@freya/ui/forms` and module `schemas/`),
  `check-bundle-size` (shell + one module ≤ 75% of the pre-migration baseline recorded
  in the script). All wired into each front-end's `npm run lint` and the CI jobs.
- **Rationale**: Constitution IV (tests first) applied to a UI feature; SC-002/003/004/
  006/007 need automated evidence.
- **Alternatives**: Storybook/Histoire for the catalogue (heavier dependency; a plain
  Vite page is enough); Cypress (the repo already uses Playwright).

## R10. Migration order

- **Decision**: (1) kit foundations + catalogue + checks; (2) shell (layouts, drawer,
  app bar, RemoteBoundary, theme) keeping `vuetify` shared; (3) asset (newest, already
  schema-driven forms, best test coverage) as the reference remote; (4) inventory, ipam,
  paperless, deployer, lcm, notification, warden (smallest surface last is not required —
  ordered by risk: warden's 10 components include secret-handling fields and go last with
  the most review); (5) auth console; (6) remove `vuetify`/`@mdi/font`, tighten checks.
- **Rationale**: FR-017 and SC-010 — the platform stays usable after each step; asset
  proves the kit against a full CRUD/tree/upload/stream surface before others follow.

## Threat model (STRIDE, browser-side only)

| Threat | Scenario | Mitigation |
|---|---|---|
| Spoofing | none new (session/CSRF/token unchanged) | — |
| Tampering | client bypasses Zod and posts a bad payload | servers validate (SR-001); OpenAPI validation at the module edge is unchanged |
| Repudiation | none new | audit unchanged |
| Information disclosure | secrets autofilled/persisted by shared inputs; real data in fixtures; server detail echoed | `UiSecretField` (`autocomplete="off"`, no draft persistence) (SR-004); synthetic fixtures (SR-005); reason vocabulary only (SR-006) |
| Denial of service | huge client-side lists | `UiDataTable` virtualises/paginates beyond 200 rows |
| Elevation of privilege | UI shows a control the API refuses | CASL gating preserved; `TestAbilitiesMatchDecisions` unchanged |
| Supply chain | flyonui / tailwindcss / zod / @iconify pulled compromised | lock-file pinning, `npm audit --audit-level=high` in CI (SR-003), no postinstall scripts (`ignore-scripts` in `.npmrc`) |
