# Contract: `@freya/ui` public API (v1)

Semver: the kit is `1.x`; shell and remotes declare `"@freya/ui": "workspace:*"` and
share it as a Module-Federation singleton with `requiredVersion: "^1.0.0"`,
`strictVersion: true`. A breaking change bumps the major and requires all front-ends to
move together.

## Components (all `Ui*`, `<script setup lang="ts">`, typed props, `defineSlots`)

- Every interactive component: keyboard operable, ARIA roles/states, `aria-label` or
  visible label, focus trap + restore for overlays, honours `prefers-reduced-motion`.
- Every component accepts `class` passthrough; none renders a `style` attribute.
- Sizes: `size?: 'sm' | 'md' | 'lg'` where meaningful; density is not a global mode.
- Data table: `items`, `columns: {key,label,sortable?,align?,width?,cell?: slot}`,
  `rowKey`, `loading`, `empty` slot, `actions` slot, `cursor`/`onLoadMore`,
  `responsive: 'stack' | 'scroll'` (default `stack` below `md`).
- Dialog: `modelValue`, `title`, `size`, `persistent`, slots `default`/`actions`;
  full-screen below `md`.
- Record dialog: `schema` (Zod object), `fields: FieldDef[]` (`key,label,type,options,
  hint,cols`) or auto-derived from the schema (`zodToFields`), `initial`, `submit(values)`
  → emits `saved`; shows server refusals inline.
- Tree: `items: {id,label,children?,meta?}`, `expanded` v-model, `selected`, arrow-key
  navigation, slots `label`/`actions`.
- Toast: `useToast().show({ kind, title, text?, timeout? })` — shell mounts one host.
- Confirm: `await useConfirm().ask({ title, text, danger? })` → boolean.

## Forms (`@freya/ui/forms`)

```ts
const form = useZodForm(AssetInput, { initial, onSubmit: (payload) => api.create(payload) })
form.values      // reactive input model
form.errors      // Record<path, string> (shared vocabulary)
form.touched     // blur tracking; validate on blur and on submit
form.submit()    // validates → focuses first invalid → calls onSubmit → maps ApiError
form.serverError // banner text for non-field refusals
form.reset(initial?)
```
- `messages`: `{ reason: Record<ApiReason, string>, issue: (issue: ZodIssue) => string }`.
- Shared schemas: `email, uuid, cidr, isoDate, money, nonEmpty(max), tagMap(maxKeys), slug`.

## API client (`@freya/ui/api`)

`api<T>(method, path, body?, { query?, signal? })`, `upload<T>(path, file, fields?)`,
`fileUrl(path)`, `csrfToken()`, `describe(err)`, `class ApiError { status, reason }` —
identical semantics to the per-module copies today; `BASE` is passed at module boot
(`createApi({ base: '/api/asset/v1' })`).

## Theme (`@freya/ui/theme.css`)

Themes `freya-light` (default) and `freya-dark`, selected by `data-theme` on `<html>`;
brand tokens `--color-primary` etc. defined once. Consumers import this file after
`@plugin "flyonui"`.

## Static checks (`ui/scripts`)

- `check-duplicates.mjs`: fails when a component basename exists in `ui/kit/src/components`
  and any module, or in two or more modules.
- `check-no-legacy.mjs`: for front-ends marked `migrated` in `ui/MIGRATION.md`, fails on
  imports of `vuetify`, `@mdi/font`, `vite-plugin-vuetify`, on `:rules=` bindings and on
  `style=`/`:style=` attributes.
- `check-bundle-size.mjs`: fails when shell + asset remote gzipped JS+CSS exceed 75% of
  the baseline recorded in the script.
