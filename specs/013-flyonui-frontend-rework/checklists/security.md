# Security review — feature 013 (FlyonUI + Zod front-end rework)

Reviewed 2026-09-22 against the implementation on `master`.

## Content Security Policy

- [x] Edge CSP headers unchanged: `services/gateway` still serves the feature-003
  policy (no `unsafe-inline` for `style-src` or `script-src`; `font-src 'self'`).
  Verified by diffing `services/gateway/internal/edge` headers — no change in this
  feature; the kit compiles all CSS at build time and ships no runtime `<style>`.
- [x] No inline styles: `check-no-legacy` refuses `style=` / `:style` in every
  migrated front-end; the catalogue and every e2e flow assert `[style]` count 0
  and register a `securitypolicyviolation` listener that fails the run.
- [x] Third-party CSS/JS is bundled, never loaded from a CDN (FlyonUI, Tailwind,
  Iconify icons are build-time; Inter is `@fontsource-variable/inter`).

## Secret handling in the UI

- [x] `grep -rn autocomplete services/*/ui/src services/auth/console/src ui/kit/src`:
  every password / token / code field is a `UiSecretField` (masked, `autocomplete="off"`
  or `new-password` / `current-password` / `one-time-code` where the browser flow
  needs it, `data-lpignore`) — no secret is rendered as plain text, written to
  `localStorage`/`sessionStorage` (kit test `secret field … nothing persisted`) or
  put in a URL.
- [x] One-time material (client secrets, enrolment tokens, recovery codes, warden
  reveals) is shown once from the API response and dropped on dismiss; never kept in
  a store.
- [x] Deployer configuration credentials are write-only: the schema never echoes a
  credential value in an issue message (`deployer/ui/tests/unit/schemas.spec.ts`).
- [x] API refusals map to fixed wording (`describeReason`, module `registerReasons`);
  unknown reasons and server `detail` strings are never echoed (kit test
  `never echoes a server detail string`). Sign-in shows one message for every
  credential failure (no account enumeration).

## Dependency review

| Package | Version | Licence | Maintainer | postinstall | Notes |
|---|---|---|---|---|---|
| flyonui | 2.4.1 | MIT | Themeselection | none | CSS + a small JS plugin; only the Tailwind plugin is used (no `flyonui.js` runtime) |
| tailwindcss / @tailwindcss/vite | 4.3.3 | MIT | Tailwind Labs | none | build-time only |
| zod | 4.6.5 | MIT | Colin McDonnell | none | shared singleton, strict version |
| @iconify/tailwind4 1.2.3 + @iconify-json/mdi 1.2.3 | — | MIT / Apache-2.0 (MDI) | Iconify | none | icons compiled to CSS at build time; no runtime fetch |
| @module-federation/vite | 1.22.1 | MIT | ByteDance MF team | none | `adm-zip` (dev-only, DTS plugin) pinned to 0.6.1 via root `overrides`; `npm audit --audit-level=high` clean |

`npm ls --all | grep -i postinstall` — none of the above declare install scripts.
Lockfile contains no `vuetify`, `vite-plugin-vuetify` or `@mdi/font`.

## Fixtures and tests

- [x] Catalogue pages and unit tests use synthetic data only (no exports from a
  live system; `ui/kit/catalogue` footer states it).
- [x] e2e flows create their own records with timestamped names and never print
  credentials; operator credentials come from the environment.

## Residual risks

- Module Federation loads remote code at runtime from the same origin only
  (`registerModules` refuses foreign entries); a compromised module image could
  still ship arbitrary UI code — unchanged from feature 003.
- `zod` 4.6 ships a JIT compiler and, by default, probes `new Function` on the
  first object parse — under the edge CSP the browser reported that probe as a
  `script-src` violation on the live sign-in page. The kit now sets
  `z.config({ jitless: true })` in `@freya/ui/forms` (a shared singleton, so it
  applies to every remote); asserted by `ui/kit/tests/forms/useZodForm.spec.ts`
  and by the live check (no `securitypolicyviolation` on `/console/signin`).
