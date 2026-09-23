# Front-end architecture

Freya's web UI is one application composed at runtime: the **gateway shell**
(`services/gateway/shell`) is the host; every module ships a **federated remote**
(`services/<module>/ui`, plus the auth console at `services/auth/console`) that the
shell loads from `/m/<module>/mf-manifest.json` after the gateway announces it
(`GET /gateway/v1/me/modules`). Feature 013 replaced the Vuetify-era front-ends
with a single kit, `@freya/ui` (`ui/kit`), on FlyonUI + Tailwind 4 with Zod for
every form.

```
ui/kit                      @freya/ui — components, forms (Zod), api client, theme, catalogue
ui/scripts                  static checks (duplicates, legacy, bundle size) + their self-tests
ui/MIGRATION.md             per-front-end status table read by the checks; bundle baseline
services/gateway/shell      host: layouts, navigation, ops views, module boundaries
services/auth/console       standalone console (/console/) AND a remote (VITE_REMOTE=1)
services/<module>/ui        remotes: src/remote/{routes,nav,header,boot}.ts exposes
```

## Runtime contract

- **Shared scope** (`specs/003-application-gateway/contracts/federation.md`, amended
  by `specs/013-flyonui-frontend-rework/contracts/federation-changes.md`): `vue`,
  `vue-router`, `pinia`, `@casl/ability`, `@casl/vue`, `zod` (strict), `@freya/ui`,
  `@freya/ui/forms`, `@freya/ui/api` (strict). The shell provides all of them; a
  remote built against another kit major fails into its own error boundary
  (`ModuleBoundary` → `UiRemoteBoundary`) with a retry, the rest of the shell keeps
  working. Production remote builds declare these `import: false` so a module
  bundle contains only module code (`shell + asset` ≈ 58 % of the pre-migration
  baseline; `ui/scripts/check-bundle-size.mjs` enforces ≤ 75 %).
- **Theme**: `useTheme()` sets `data-theme` (`freya-light` / `freya-dark`) on
  `<html>`; the shell compiles `@freya/ui/theme.css`, remotes compile utilities
  only. Both themes pass axe (WCAG 2.1 AA, zero serious/critical) on the
  catalogue and on every module view (`tests/e2e/a11y.spec.ts`).
- **CSP**: unchanged from feature 003 — no `unsafe-inline`, no inline `style`.
  Every e2e flow registers a `securitypolicyviolation` listener and fails on any
  report; `check-no-legacy` refuses `style=` in source.
- **Forms**: one Zod schema per write payload under `src/schemas/`, bound with
  `useZodForm`; the schema output is the request body. Refusal wording comes from
  the kit (`describeReason`) plus module vocabularies (`registerReasons`) — server
  detail strings are never echoed.
- **Per-module API client**: `src/api/client.ts` = `createApi({ base })`
  (CSRF header from `__Host-csrf`, request ids, `ApiError { status, reason, detail }`,
  `upload`, `fileUrl`). The shell and console wrap it with outage / unauthenticated
  events.

## Migration order (done)

kit → shell (host, transitional Vuetify) → asset (reference module) → inventory,
ipam, paperless, deployer, lcm, notification, warden → auth console → Vuetify
removed from the shell and the lockfile. `ui/MIGRATION.md` records the status;
`check-no-legacy` fails a `migrated` front-end that imports Vuetify/`@mdi/font`,
binds `:rules`, keeps an inline validator outside `src/schemas/`, or sets `style=`.

## Checks and where they run

| Check | Local | CI |
|---|---|---|
| `node ui/scripts/check-duplicates.mjs` | `npm run check` | `ui-kit` job |
| `node ui/scripts/check-no-legacy.mjs` | every front-end `npm run lint` | every front-end job |
| `node ui/scripts/check-bundle-size.mjs` | after `npm run build` | `gateway-shell` job |
| `node --test ui/scripts/tests/checks.spec.mjs` | `npm run check:self` | `ui-kit` job |
| kit coverage thresholds | `npm run -w ui/kit test:coverage` | `ui-kit` job |
| catalogue screenshots + axe | `npx playwright test -c catalogue/playwright.config.ts` | `ui-kit` job |
| module e2e flows (`tests/e2e/*-flow.spec.ts`, `a11y.spec.ts`) | need a running stack + `E2E_OPERATOR_EMAIL/PASSWORD` (`PW_CHANNEL=chrome` on dev boxes) | manual / nightly |

## Building images

Every service Dockerfile has a workspace UI stage: it copies the root
`package.json` + `package-lock.json` and each workspace `package.json`, runs
`npm ci`, builds `ui/kit`, then builds the module UI (`npm run -w services/<m>/ui build`).
`deploy/stack/compose.yaml` builds from the repo root so the stage sees the
workspace. Use `sg docker -c 'docker compose -p freya-stack build <service>'` on
hosts where the user is not in the docker group.
