# Quickstart: Platform Frontend Rework (FlyonUI + Zod)

Validation scenarios proving the feature. Prerequisites: Node 22, the repo root npm
workspace installed (`npm ci` at the root), and for live checks the `deploy/stack`
platform up with a provisioned operator (`E2E_OPERATOR_PASSWORD`, TOTP secret).

## 1. Shared kit (US1)
```sh
npm run -w ui/kit test          # every component + schema helper unit/a11y tests pass
npm run -w ui/kit build          # dist + types
npm run -w ui/kit catalogue      # open the catalogue; toggle theme; every component renders in both
node ui/scripts/check-duplicates.mjs   # exits 0
```
Expected: 0 test failures, 0 axe violations, catalogue lists every component in the
inventory table of data-model.md.

## 2. Shell (US1)
```sh
cd services/gateway/shell && npm run lint && npm run build
```
Sign in at https://localhost:8443 on 320/768/1280 px viewports: drawer collapses,
no horizontal page scroll, profile menu and module areas operable; stop the `asset`
container → its area shows the retry card, the rest of the shell keeps working.
Toggle dark mode → shell and a migrated remote switch together.

## 3. Forms (US2)
For one form per module (asset create, ipam subnet, deployer target, auth invite):
submit empty → inline "required" messages, focus on first invalid field, no network
request (DevTools); enter an invalid e-mail/CIDR/date → specific inline message on
blur; valid submit → request body equals `z.output` of the schema (compare in DevTools);
server `conflict` (duplicate name) → inline/banner with the shared wording.

## 4. Each migrated remote (US3)
```sh
cd services/<module>/ui && npm run lint && npm run test:unit && npm run build
E2E_VIEWPORTS=320x640,768x1024,1280x800 npx playwright test   # skips without operator creds
```
Walk the module's own quickstart primary flow as an all-permissions operator and a
read-only operator: routes resolve, manage controls absent for read-only, live updates
still arrive, uploads/downloads work.

## 5. Auth console (US4)
Accept a fresh invitation on a phone viewport, enrol MFA (codes legible/copyable),
sign out, sign in with TOTP and with a recovery code, change password, invite a user.

## 6. Removal + gates (US5)
```sh
node ui/scripts/check-no-legacy.mjs     # every front-end marked migrated; 0 findings
node ui/scripts/check-bundle-size.mjs   # shell + asset ≤ 75% of baseline
grep -rl vuetify services/*/ui/package.json services/gateway/shell/package.json services/auth/console/package.json  # no output
npm audit --audit-level=high --workspaces
```
