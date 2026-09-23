# Phase 1 Data Model: Platform Frontend Rework

This feature stores nothing server-side; its "entities" are the shared package's
contracts.

## Shared UI package (`@freya/ui`, `ui/kit`)

| Entry point | Contents |
|---|---|
| `@freya/ui` | Components (below), composables (`useBreakpoint`, `useToast`, `useConfirm`, `useTheme`, `useFocusTrap`), `UiIcon` name map |
| `@freya/ui/forms` | `useZodForm`, `messages` (reason/validation vocabulary), shared schemas (`email`, `uuid`, `cidr`, `isoDate`, `money`, `nonEmpty`, `tagMap`, `slug`), `zodToFields` helper, `ApiError` mapping |
| `@freya/ui/api` | `api()`, `upload()`, `fileUrl()`, `csrfToken()`, `describe()` — the fetch client every module currently duplicates |
| `@freya/ui/theme.css` | `freya-light` / `freya-dark` theme definitions, brand tokens, base layer |

### Component inventory (one home each; replaces per-module copies)

| Kit component | Replaces today | Used by |
|---|---|---|
| `UiPage`, `UiToolbar`, `UiGrid`, `UiCard`, `UiSection` | ad-hoc `v-container/v-row/v-card` layouts | all |
| `UiDataTable` (responsive, sortable, cursor pagination, row actions, empty/loading states) | `v-table` + per-module list markup | all |
| `UiDialog`, `UiConfirm` (`useConfirm`) | 56 `v-dialog` usages, per-module confirm prompts | all |
| `UiForm`, `UiField` (label/hint/error), `UiInput`, `UiTextarea`, `UiSelect`, `UiCombobox`, `UiCheckbox`, `UiSwitch`, `UiDateInput`, `UiNumberInput`, `UiFilePicker`, `UiSecretField`, `UiTagEditor` | `v-form`/`v-text-field`/`v-select`/`v-autocomplete`/`v-file-input` + `RecordDialog` variants (asset, ipam, deployer, lcm) | all |
| `UiRecordDialog` (schema-driven create/edit dialog) | `RecordDialog.vue` (asset), `EntityDialog` variants elsewhere | asset, ipam, deployer, lcm, notification, warden |
| `UiKeyValueTable` | `KeyValueTable.vue` (inventory, ipam, lcm) | inventory, ipam, lcm, asset |
| `UiStatTile`, `UiStatGrid` | `StatsCard.vue` (inventory, asset, ipam, paperless) | dashboards |
| `UiTree` (keyboard navigable) | `TreeNodes.vue` (asset), ipam subnet tree | asset, ipam, paperless categories |
| `UiDocumentList` (list/upload/download/delete) | `EntityDocuments.vue` (asset), paperless attachments | asset, paperless |
| `UiStatusChip`, `UiBadge` | inline `v-chip` colour maps | all |
| `UiTabs`, `UiAccordion`, `UiDropdownMenu`, `UiTooltip`, `UiToast` (`useToast`), `UiAlert`, `UiEmptyState`, `UiErrorState`, `UiSkeleton`, `UiPagination`, `UiAvatar`, `UiCopyButton` | scattered Vuetify equivalents | all |
| `UiAppShell`, `UiNavDrawer`, `UiAppBar`, `UiUserMenu`, `UiRemoteBoundary` | shell layouts + `RemoteBoundary.vue` | shell (console reuses `UiAppShell`) |
| `UiLiveIndicator` | "live" chip in inventory/asset/ipam | stream-backed modules |

Rule: a component enters the kit when a second front-end needs it; a module may keep
a component only while it is used by that module alone (checked by `check-duplicates`).

## Form schema

- One schema per form under `services/<module>/ui/src/schemas/<form>.ts` (module-owned) or
  `ui/kit/src/forms/schemas/` (shared primitives).
- Shape: `const AssetInput = z.object({...})`; `type AssetPayload = z.output<typeof AssetInput>`
  is the API body type; UI types import the payload type instead of hand-written `*Input`
  interfaces.
- Rules expressible: required, min/max length, format (e-mail, uuid, cidr, date), numeric
  range, enum, cross-field (`refine`: `valid_to ≥ valid_from`), conditional requirement.
- Normalisation belongs in the schema (`trim`, empty string → `undefined`/`null`, date →
  ISO) so the payload never needs post-processing in a view.

## Error / reason vocabulary

`messages.ts` maps the closed API reason set already used by every module (`forbidden`,
`not_found`, `conflict`, `validation_failed`, `malformed_body`, `rate_limited`,
`temporarily_unavailable`, `unauthenticated`, `body_too_large`, `unsupported_media_type`,
`network`) and the validation issue codes (Zod issue codes → wording) to user text;
unknown reasons render the generic fallback and are logged with `console.warn`.

## Front-end

| Front-end | Kind | Remote name | Notes |
|---|---|---|---|
| gateway shell | host | — | provides shared singletons, theme, `RemoteBoundary` |
| auth console | standalone app | — | consumes kit directly; no federation |
| warden, notification, lcm, deployer, paperless, inventory, ipam, asset | remotes | `<module>` | expose `./routes`, `./nav` (+ optional `./header`, `./boot`) unchanged |

State per front-end: `legacy` → `migrated` (tracked in `ui/MIGRATION.md` and enforced by
`check-no-legacy` once marked migrated).
