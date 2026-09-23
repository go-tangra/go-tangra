# Front-end static checks

- `check-duplicates.mjs` — fails when a component basename exists both in the kit
  (`ui/kit/src/components`) and in any front-end, or in two or more front-ends. A
  component needed by a second front-end moves to the kit.
- `check-no-legacy.mjs` — for front-ends marked `migrated` in `ui/MIGRATION.md`, fails on
  imports of `vuetify`, `vite-plugin-vuetify`, `@mdi/font`, on `:rules=`/`v-form`
  bindings and on `style=`/`:style=` attributes (the edge CSP has no `unsafe-inline`).
- `check-bundle-size.mjs` — measures gzipped JS+CSS of the shell and asset `dist/` and
  fails above 75% of the baseline in `ui/MIGRATION.md`; `--record` prints the current
  numbers.

Run all: `npm run check` (root). Each script exits non-zero with the offending paths.
