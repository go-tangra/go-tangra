# Feature Specification: Platform Frontend Rework (FlyonUI + Zod)

**Feature Branch**: `013-flyonui-frontend-rework`

**Created**: 2026-09-21

**Status**: Draft

**Input**: User description: "Platform-wide frontend rework: replace Vuetify (Materio) with FlyonUI (https://flyonui.com/, Tailwind CSS + daisyUI-based component library) across ALL module UIs (gateway shell, auth console, warden, notification, lcm, deployer, paperless, inventory, ipam, asset remotes). Adopt Zod (https://zod.dev/) for all form validation (schemas shared between forms and API payload types). Every page must remain fully responsive (phone/tablet/desktop). Any component needed by more than one module must live in a single shared, reusable UI package consumed by all Module-Federation remotes and the shell — no duplicated components across modules. Preserve existing behaviour, routes, permissions/CASL abilities and the Module-Federation contract with the shell."

## Overview

The Freya platform ships ten browser front-ends: the **gateway shell** (the host
that signs operators in, renders navigation and mounts every module), the
**auth console** (a standalone operator/admin application), and eight
**module remotes** loaded into the shell at runtime (warden, notification, lcm,
deployer, paperless, inventory, ipam, asset). All of them are built on the same
component library today, and each module has grown its own copies of the same
building blocks (record dialogs, key/value tables, stat tiles, document lists,
tree views, confirm prompts, empty states).

This feature re-platforms every front-end onto **one shared design system**
built from the FlyonUI component library, introduces **one shared,
schema-based form-validation approach** (Zod) whose schemas are the single
source of truth for what a form accepts and what the API payload looks like,
and consolidates every cross-module building block into a **single reusable
UI package** so no component exists in more than one place. The rework is a
like-for-like replacement: every screen keeps its routes, its data, its
permission gating, its live updates and its behaviour; what changes is the
visual system, the form validation and the internal reuse. Every screen must
work on a phone, a tablet and a desktop.

The user-visible outcome is a consistent, faster, accessible interface across
all modules with immediate, uniform form feedback; the engineering outcome is
one place to fix a component or a validation rule for the whole platform and a
smaller, single copy of the UI toolkit delivered to the browser.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One shared UI kit and a re-platformed shell (Priority: P1)

An operator signs in at the platform and uses the shell — app bar, navigation
drawer, module areas, session/profile menu, error boundaries, theme (light/dark)
— rendered by the new design system, on a phone, a tablet and a desktop. The
building blocks the shell and the modules share (layout primitives, data table,
dialog, confirm prompt, form fields, select/autocomplete, file picker, tabs,
tree view, stat tile, key/value table, empty/error/loading states, toast,
badge/status chip, pagination) exist exactly once, in a shared package, with
documentation and tests.

**Why this priority**: Nothing else can be migrated until the shared kit and the
host exist; the shell also defines what every remote shares at runtime, so it
must move first.

**Independent Test**: Sign in, open every shell surface (nav, drawer, profile
menu, a module area with an intentionally failing remote) at 360 px, 768 px and
1280 px widths; every element is reachable, readable and operable without
horizontal scrolling; the shared package's component catalogue renders each
component in light and dark theme.

**Acceptance Scenarios**:

1. **Given** a signed-in operator on a 360 px wide phone, **When** they open the shell, **Then** navigation collapses into a drawer/menu, no content is clipped or requires horizontal scrolling, and every action remains reachable with touch.
2. **Given** the shell on a desktop, **When** a module remote fails to load, **Then** the module's area shows the shared retry/error state while the rest of the shell keeps working.
3. **Given** the shared UI package, **When** a component is added that already exists elsewhere in any module, **Then** the build's duplication check fails and names the existing component.
4. **Given** any shared component, **When** it is used by two different modules, **Then** both render identically and pass the same keyboard and screen-reader checks.

---

### User Story 2 - Uniform, schema-driven form validation (Priority: P1)

A user filling any form on the platform (creating an asset, inviting a user,
adding a subnet, uploading a document, configuring a deployment target) gets
immediate, consistent inline feedback: required fields, formats, ranges and
cross-field rules are checked before submission with the same wording and
placement everywhere, and the submitted payload always matches what the API
accepts because the same schema defines both.

**Why this priority**: Validation today is hand-written per form and drifts from
the API contract; a single schema per form removes a whole class of "the server
said 422" surprises and is required by every migrated screen.

**Independent Test**: For a representative form in each module, submit empty,
malformed and boundary values; every rule is reported inline before any request
is sent, the wording matches the shared vocabulary, and a valid submission
produces exactly the payload the API contract describes.

**Acceptance Scenarios**:

1. **Given** a form with a required field left blank, **When** the user submits, **Then** the field is marked invalid with the shared "required" message, focus moves to the first invalid field, and no request is made.
2. **Given** a field with a format rule (e-mail, CIDR, date, positive number), **When** the user types an invalid value and leaves the field, **Then** the specific message appears inline and clears as soon as the value is valid.
3. **Given** a valid form, **When** it is submitted, **Then** the request body contains only the schema's fields with the schema's types (numbers as numbers, dates in the API's format, blanks normalised as the schema defines).
4. **Given** the API refuses a submission (e.g. a duplicate name), **When** the response arrives, **Then** the refusal is shown in the same inline/banner style as client-side errors, using the shared reason vocabulary.

---

### User Story 3 - Module remotes migrated one by one (Priority: P2)

Each module's screens (asset, inventory, ipam, paperless, deployer, lcm,
notification, warden) are rebuilt on the shared kit and schemas, one module at
a time, each independently shippable: the module keeps every route, every
permission-gated control, its live (streamed) updates, its uploads/downloads
and its data, and reads well at every breakpoint.

**Why this priority**: This is the bulk of the work and the user-facing value;
each module is a self-contained increment that can be verified and released on
its own once the kit exists.

**Independent Test**: For one module, walk its primary flow (from the module's
quickstart) at the three breakpoints with an operator that holds all
permissions and one that holds read-only permissions; every control appears
only for the permitted operator, every route still resolves, and the flow
completes with the same results as before the migration.

**Acceptance Scenarios**:

1. **Given** a migrated module, **When** its routes are opened directly by URL, **Then** every previously existing route still resolves to the equivalent screen.
2. **Given** an operator without a module's manage permission, **When** they open the module, **Then** the create/edit/delete/assign controls are absent, exactly as before the migration.
3. **Given** a module with live updates (assignments, agent status, sync progress), **When** an event arrives, **Then** the screen updates in place as before.
4. **Given** a module list screen on a phone, **When** it is opened, **Then** tables reflow (stacked rows or horizontal scroll within the table only), actions stay reachable, and dialogs fit the viewport.

---

### User Story 4 - Auth console migrated (Priority: P2)

Operators and administrators use the auth console (sign-in, MFA challenge and
enrolment, invitation acceptance, password recovery, account settings, user and
role administration, audit) on the new design system with the shared kit and
schemas, on any device.

**Why this priority**: The console is the first screen every operator sees and
carries the most sensitive forms; it is a standalone application, so it can be
migrated and released independently of the shell and remotes.

**Independent Test**: Accept an invitation, enrol MFA, sign out, sign in with a
TOTP and with a recovery code, change the password, invite a user and edit
their roles — on a phone and a desktop — with identical outcomes to today.

**Acceptance Scenarios**:

1. **Given** a new operator, **When** they accept an invitation on a phone, **Then** the password rules are shown inline before submission and the recovery codes are displayed legibly and copyable.
2. **Given** the sign-in MFA step, **When** a wrong code is entered, **Then** the shared refusal message is shown inline and the field is refocused.

---

### User Story 5 - Old toolkit removed and duplication eliminated (Priority: P3)

Once every front-end is migrated, the previous component library is no longer
delivered to the browser, the shell shares only the new kit's runtime with the
remotes, and an automated check keeps the platform free of duplicated
components and of validation logic outside the shared schemas.

**Why this priority**: The cost and consistency benefits only materialise when
the old toolkit is gone and the "one copy" rule is enforced, not just followed.

**Independent Test**: Build every front-end; the old toolkit is absent from every
bundle and lock file; the duplication check and the "no ad-hoc validation"
check pass; the shell's shared runtime list contains only the new kit's
packages.

**Acceptance Scenarios**:

1. **Given** the final build, **When** bundles are inspected, **Then** no module ships its own copy of any shared runtime package and the previous toolkit is absent.
2. **Given** a developer adds a component under a module that duplicates a shared one, **When** the checks run, **Then** they fail with the path of the shared component to use instead.

---

### Edge Cases

- A remote built against an older kit version is loaded by a newer shell (or vice versa) during a rolling deployment: the shell must refuse an incompatible remote in its error boundary rather than render a broken screen.
- A user with a very narrow viewport (320 px) or a large font/zoom setting: content must still reflow without clipping; tables may scroll horizontally within their own container only.
- A form whose API payload has fields the user cannot see (server-set ids, computed values): the schema separates the editable input shape from the full payload so hidden fields are never required of the user.
- The API refuses with a reason the shared vocabulary does not know: a generic, safe message is shown and the raw reason is logged for the developer, never rendered as-is.
- Content-Security-Policy: the design system must work with the shell's nonce-based policy — no inline styles or scripts that the policy would block.
- Reduced-motion and high-contrast preferences: animations respect the user's reduced-motion setting; status is never conveyed by colour alone.
- Right-to-left or long translated labels (future): layouts must not depend on fixed label widths.
- Keyboard-only operation: every dialog traps focus, every menu and tree is arrow-key navigable, and closing a dialog returns focus to its trigger.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The platform MUST provide one shared front-end UI package, consumed by the shell, the auth console and every module remote, containing every component used by more than one front-end.
- **FR-002**: No component, composable, form-field wrapper, layout primitive or validation helper MAY exist in more than one front-end; an automated check MUST fail the build when a module defines a component whose name or purpose duplicates a shared one.
- **FR-003**: Every front-end (shell, auth console, eight module remotes) MUST be rebuilt on the new design system with no remaining dependency on the previous component library once the feature is complete.
- **FR-004**: Every existing route, screen, action, live-update behaviour, upload/download and data shown today MUST be preserved with equivalent behaviour (like-for-like migration, no functional regressions).
- **FR-005**: Every screen MUST be usable on phone (≥ 320 px), tablet (≥ 768 px) and desktop (≥ 1280 px) widths: no horizontal page scrolling, no clipped or unreachable controls, dialogs and menus fit the viewport, tables reflow or scroll within their own container.
- **FR-006**: Every form MUST be validated by a declared schema that is the single source of truth for the form's accepted input and for the type of the API payload it produces; hand-written per-form validation logic MUST NOT exist.
- **FR-007**: Validation feedback MUST be shown inline before submission (on blur and on submit), use a shared message vocabulary, move focus to the first invalid field on submit, and block the request while invalid.
- **FR-008**: API refusals MUST be surfaced through the same shared error presentation and the shared reason vocabulary already used across modules (e.g. `validation_failed`, `conflict`, `forbidden`), with unknown reasons mapped to a safe generic message.
- **FR-009**: Permission gating MUST continue to be driven by the abilities the shell distributes; every conditional control that maps to an API permission MUST remain gated exactly as today.
- **FR-010**: The shell/remote integration contract (runtime-registered remotes, `./routes`, `./nav`, optional `./header`/`./boot`, shared singleton runtimes, error boundaries) MUST be preserved; the set of shared runtime packages MUST be updated to the new kit and remotes MUST NOT bundle their own copies.
- **FR-011**: The shell MUST detect a remote built against an incompatible shared-kit version and show the module's error state instead of rendering it.
- **FR-012**: Light and dark themes MUST both be supported, follow the operator's preference as today, and apply consistently across the shell, console and every remote.
- **FR-013**: Every shared component MUST be keyboard operable, expose accessible names/roles/states, trap and restore focus for overlays, meet WCAG 2.2 AA colour contrast in both themes, and respect the reduced-motion preference.
- **FR-014**: The shared package MUST ship a browsable component catalogue (every component, its variants and both themes) used for review and visual regression.
- **FR-015**: Each front-end MUST keep its automated checks (type check, lint, unit tests, build) green, and the shared package MUST have unit tests for every component and schema helper.
- **FR-016**: Each module's primary user flow (per its quickstart) MUST have an end-to-end check that runs at the three breakpoints after migration.
- **FR-017**: The migration MUST be deliverable incrementally (shell/kit first, then one front-end at a time), with the platform remaining fully usable after each increment.

### Security Requirements *(mandatory — Constitution: Development Workflow)*

- **Trust boundaries crossed**: none new — the browser ↔ gateway boundary and the gateway ↔ module boundaries are unchanged; this feature changes only browser-side code.
- **Data classification**: the front-ends display tenant business data, PII (names, e-mails, avatars), and one-time secrets (recovery codes, enrolment tokens, minted credentials) exactly as today.
- **Authentication/Authorization**: unchanged — session cookie + CSRF at the gateway edge, platform token forwarded to modules, CASL abilities distributed by the shell; client-side gating is a UX convenience and the API remains authoritative.
- **Threat scenarios**: client-side validation mistaken for enforcement; a new third-party UI/validation dependency introducing a supply-chain vulnerability; CSP weakened to accommodate the design system; validation messages or catalogue examples leaking real data; component defaults (e.g. autocomplete on secret fields) exposing secrets.
- **SR-001**: Client-side schema validation MUST be treated as user-experience only; servers MUST continue to validate every payload and the front-ends MUST handle server refusals gracefully.
- **SR-002**: The Content-Security-Policy MUST NOT be relaxed for the new design system; no inline scripts, no `unsafe-inline`/`unsafe-eval`, styles delivered as static assets or nonce-approved.
- **SR-003**: New browser dependencies MUST be pinned by lock file, audited at CI (no known high/critical vulnerabilities) and limited to the design system, its styling toolchain and the schema library; transitive additions MUST be reviewed.
- **SR-004**: Fields that hold secrets or one-time codes MUST disable browser autocomplete/autofill and MUST NOT be persisted in local storage, drafts or logs by any shared component.
- **SR-005**: The component catalogue and tests MUST use synthetic data only; no real tenant data, tokens or credentials may appear in fixtures.
- **SR-006**: Validation and error messages MUST never echo server-internal detail beyond the shared reason vocabulary.

### Key Entities

- **Shared UI package**: the single library of components, layout primitives, theme tokens, composables and validation helpers consumed by every front-end; versioned, with a component catalogue and tests.
- **Form schema**: a declared description of a form's accepted input (fields, types, formats, ranges, cross-field rules) from which both the user-facing validation and the API payload type derive; one per form, owned by the module that owns the form; shared schemas (e.g. e-mail, CIDR, tag maps, money) live in the shared package.
- **Shared error/reason vocabulary**: the closed set of API refusal reasons and their user-facing wording, defined once and reused by every front-end.
- **Front-end**: one of the shell, the auth console, or a module remote; each has routes, permission-gated controls, forms and (for remotes) an integration contract with the shell.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the platform's front-ends (10 of 10) run on the new design system with zero dependency on the previous component library in their builds.
- **SC-002**: Zero duplicated components across front-ends — an automated check reports 0 duplicates, and the shared package is the only location of every component used by ≥ 2 front-ends.
- **SC-003**: 100% of forms (every screen that submits user input) validate through a declared schema; 0 forms with hand-written validation logic.
- **SC-004**: Every screen passes the responsiveness check at 320/768/1280 px widths with no horizontal page scrolling and no unreachable controls.
- **SC-005**: Every route that existed before the migration still resolves and every module's primary flow completes end to end with the same outcome as before (0 functional regressions found by the per-module flow checks).
- **SC-006**: All shared components meet WCAG 2.2 AA (contrast, keyboard operability, focus management) in both themes, verified by automated accessibility checks with 0 serious/critical findings.
- **SC-007**: The total JavaScript/CSS delivered to an operator opening the shell plus one module is at least 25% smaller than today.
- **SC-008**: A user submitting an invalid form sees inline feedback within 100 ms of blur/submit and no network request is made until the form is valid.
- **SC-009**: A change to a shared component or a shared validation rule takes effect in every front-end after a single change in one place (verified by changing one message and observing it in all front-ends).
- **SC-010**: The platform remains fully usable after each migration increment (no increment leaves a front-end broken or missing a screen).

## Assumptions

- The migration is incremental: the shared kit and the shell move first; the shell temporarily provides both the old and the new shared runtimes so already-migrated and not-yet-migrated remotes coexist; the old runtime is removed with the last remote (User Story 5). If a big-bang cutover is preferred instead, the plan changes but the requirements do not.
- The design system's default look (with the platform's theme tokens for brand colour, light/dark) replaces the current admin-template look; no pixel-for-pixel reproduction of the current visuals is required — behavioural and information parity is.
- The auth console is in scope even though it is a standalone application (it is listed explicitly); it consumes the shared package like the remotes do.
- Existing API contracts, routes, permissions and manifests are unchanged; only browser-side code, the shell's shared-runtime list and front-end build tooling change.
- Existing end-to-end checks that require a signed-in operator keep their current skip-without-credentials behaviour; the per-module flow checks are added alongside them.
- Icons continue to come from the current icon set (or an equivalent bundled set) so nav entries declared by module manifests keep rendering.
- Browser support stays as today (current evergreen browsers); no legacy-browser work is in scope.
- Translations/localisation are out of scope, but layouts must not rely on fixed label widths so they do not block a later i18n effort.
