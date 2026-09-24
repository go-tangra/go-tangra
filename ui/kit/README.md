# `@go-tangra/ui` — the Freya front-end kit

One component library, one theme and one form/API recipe for every Freya
front-end (the gateway shell, the auth console and the eight module remotes).
Built on **FlyonUI 2** (Tailwind 4), **Zod 4** and Vue 3.5; ships as a workspace
package (`ui/kit`) that every front-end consumes through Module Federation as a
shared singleton.

```
@go-tangra/ui            components + composables (UiButton, UiDataTable, useToast …)
@go-tangra/ui/forms      useZodForm, zodToFields, messages, shared schema primitives
@go-tangra/ui/api        createApi (fetch transport: CSRF, request ids, ApiError)
@go-tangra/ui/theme.css  the two platform themes (freya-light / freya-dark)
@go-tangra/ui/sources.css  `@source "./dist"` — makes Tailwind scan the kit's components
@go-tangra/ui/eslint     shared lint rules (`import { freyaRules } from '@go-tangra/ui/eslint'`)
@go-tangra/ui/vite       breakpointSpecificity() Vite plugin
bin go-tangra-ui-check-no-legacy   legacy/CSP guard run by every front-end's `npm run lint`
```

## Using the kit

```sh
npm ci                     # root workspace: installs the kit + every front-end
npm run kit                # builds ui/kit/dist (types included) — needed before any front-end build
npm run -w ui/kit dev      # component catalogue on http://127.0.0.1:5199 (every component, both themes, synthetic data)
```

Outside this monorepo the kit is installed from GitHub Packages
(`@go-tangra:registry=https://npm.pkg.github.com` in `.npmrc`, plus
`//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}` supplied by CI or the
Docker build secret — never committed). A stylesheet that needs the kit's
classes imports the theme and the source entry; the `@source` path resolves
relative to the kit itself, so it works hoisted, symlinked or installed:

```css
@import "tailwindcss";
@import "@go-tangra/ui/theme.css";
@import "@go-tangra/ui/sources.css";
```

Publishing: `npm publish -w ui/kit` (registry from `publishConfig`); the tarball
carries only `dist/`, `build/`, `bin/`, `src/theme.css`, `sources.css` and
`eslint.rules.js`.

A module remote imports the kit like any package; it never bundles it — the
shell provides `@go-tangra/ui`, `@go-tangra/ui/forms`, `@go-tangra/ui/api`, `zod`, `vue`,
`vue-router`, `pinia` and `@casl/*` as strict-version singletons
(`module-federation.config.ts`, mirrored in the shell). Production remote builds
set `import: false` for those, so a module bundle is only its own code.

```vue
<script setup lang="ts">
import { UiPage, UiCard, UiDataTable, UiRecordDialog, useToast, type Column } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { api } from '@/api/client'          // createApi({ base: '/api/<module>/v1' }) re-export
import { supplierSchema } from '@/schemas'   // every write payload has a Zod schema under src/schemas/
</script>
```

Styling: a remote's `src/main.css` compiles only Tailwind utilities + FlyonUI
component classes (`themes: false`); the theme itself comes from the shell.
Standalone development (`vite`) uses `src/dev.css`, which imports the full theme.
No inline styles — the edge CSP has no `unsafe-inline`; `check-no-legacy` fails
the build on `style=`.

## The form recipe

1. Describe the payload once, in `src/schemas/<record>.ts`, with Zod. The schema
   **output** is the API payload (`z.output<typeof schema>`): trims, coercions,
   blank-to-undefined and defaults live in the schema, never in the view.
2. Bind it with `useZodForm(schema, { initial, onSubmit })` and spread
   `form.field('name')` onto a kit field (`UiInput`, `UiSelect`, `UiSecretField`
   …). Errors render inline under the control (`role="alert"`, `aria-invalid`,
   `aria-describedby`); the first invalid field is focused on submit; a server
   refusal shows as a banner with the kit's neutral wording (`describeReason`)
   or the module's own (`registerReasons({ vault_unavailable: '…' })`).
3. For a plain create/edit record, skip the template: `zodToFields(schema, overrides)`
   derives the field list and `UiRecordDialog` / `UiRecordDrawer` render it.

```ts
const form = useZodForm(subnetSchema, {
  initial: { name: '', cidr: '' },
  onSubmit: (v) => api('POST', 'subnets', v),    // v is z.output — already the payload
  onSuccess: () => toast.success('Subnet created.'),
})
```

Secret material (`UiSecretField`) is masked, `autocomplete="off"`, never written
to storage, and shown once when the API returns it once (client secrets, enrol
tokens, recovery codes).

## Adding a component

- A component belongs in the kit when a **second** front-end needs it. The
  first consumer may keep a module-local component under `src/components/`;
  the moment another front-end copies it, it moves to `ui/kit/src/components/`
  and both import it. `node ui/scripts/check-duplicates.mjs` fails the build when
  the same basename exists in two places (kit/module or module/module).
- Every kit component: `Ui` prefix, props typed with `defineProps<…>()`,
  FlyonUI classes only (no `style`), keyboard and screen-reader behaviour
  (labels, roles, focus), works at 320 px, both themes.
- Export it from `src/index.ts`, add it to the catalogue page that fits
  (`catalogue/src/pages/*.vue`, synthetic data only), write its unit test under
  `tests/components/` (`expectA11y` from `tests/helpers.ts` runs axe) and refresh
  the screenshot baseline:
  `PW_CHANNEL=chrome npx playwright test -c catalogue/playwright.config.ts --update-snapshots`.
- Icons are Material Design Icons by name (`mdi-laptop`). Tailwind only emits
  the classes it can see, so a name used at runtime (manifest nav entries!) must
  be in `src/icons.ts`; `tests/icons.spec.ts` verifies every name exists in the
  icon set and that the class safelist mirrors the list.

## Quality gates

| Command | Gate |
|---|---|
| `npm run -w ui/kit test:coverage` | ≥ 80 % statements overall; `src/forms` and `src/api` 100 % |
| `npm run -w ui/kit lint` | ESLint (shared rules in `eslint.rules.js`) + `vue-tsc` |
| `npx playwright test -c catalogue/playwright.config.ts` | screenshots at 320/768/1280 in both themes, CSP listener, no `[style]`, axe (`tests/a11y.spec.ts`) |
| `npm run check` (root) | `check-duplicates`, `check-no-legacy` (+ `check-bundle-size` after building shell + asset) |
| `npm run check:self` (root) | the checks' own fixtures (`ui/scripts/tests/checks.spec.mjs`) |

See `docs/frontend.md` for the platform-level picture (shell, remotes, shared
scope, migration status) and `ui/MIGRATION.md` for the per-front-end status
table the checks read.
