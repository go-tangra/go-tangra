# Front-end migration status (feature 013 — FlyonUI + Zod)

`ui/scripts/check-no-legacy.mjs` reads this table: a front-end marked `migrated` must not
import Vuetify/`@mdi/font`, bind `:rules`, or use `style=` attributes.

| Front-end | Path | Status |
|---|---|---|
| gateway shell (host) | services/gateway/shell | migrated |
| auth console | services/auth/console | migrated |
| asset | services/asset/ui | migrated |
| inventory | services/inventory/ui | migrated |
| ipam | services/ipam/ui | migrated |
| paperless | services/paperless/ui | migrated |
| deployer | services/deployer/ui | migrated |
| lcm | services/lcm/ui | migrated |
| notification | services/notification/ui | migrated |
| warden | services/warden/ui | migrated |

## Bundle baseline (gzipped JS+CSS of `dist/`, measured 2026-09-21 before migration)

| Build | Bytes |
|---|---|
| services/gateway/shell | 314225 |
| services/asset/ui | 320096 |
| shell + asset | 634321 |

Target after migration (SC-007): shell + asset ≤ 75% of baseline = 475740 bytes.

## Post-migration sizes (2026-09-22, `node ui/scripts/check-bundle-size.mjs`)

| Build | Bytes gz | vs baseline |
|---|---|---|
| services/gateway/shell | 269038 | 86 % |
| services/asset/ui | 96134 | 30 % |
| shell + asset | 365172 | 58 % (limit 75 % = 475740) |

Remotes declare the shared singletons `import: false` in production builds, so a
module bundle contains no fallback copies of vue / zod / the kit.

## Notes

- Accessibility sweep (2026-09-22): catalogue, both themes, three widths —
  0 serious/critical axe findings after (a) soft badge/alert ink mixed towards
  black/white (`theme.css`), (b) muted text raised from `/60` to `/70` opacity,
  (c) light `error` darkened to oklch(55 %). Per-module `tests/e2e/a11y.spec.ts`
  runs the same sweep over every view inside the shell against a live stack.
- Reduced motion: `theme.css` collapses every animation/transition to 0.01 ms
  under `prefers-reduced-motion`; asserted in `ui/kit/catalogue/tests/a11y.spec.ts`.
- 200 % zoom: forms and data pages keep a single column with no horizontal scroll
  at half the viewport (floor 320 CSS px per WCAG 1.4.10); same spec.
