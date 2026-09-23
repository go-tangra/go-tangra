# Contract change: Module Federation shared runtime (amends specs/003 contracts/federation.md)

- Shared singletons become: `vue ^3.5`, `vue-router ^5`, `pinia ^4`, `@casl/ability ^7`,
  `@casl/vue ^3`, **`zod ^4`**, **`@freya/ui ^1`** (`strictVersion: true`).
- Transitional: the shell additionally shares `vuetify ^4` until every remote is marked
  `migrated`; then it is removed and a remote still requesting it fails to load into
  the error boundary (by design).
- A remote MUST NOT bundle its own copy of any shared package; a shared-version
  mismatch is surfaced by `UiRemoteBoundary` as the module's error state with a retry
  action and a log line naming the package and versions (FR-011).
- The shell sets `data-theme` on `<html>` and mounts the single toast host and confirm
  host; remotes use `useToast()`/`useConfirm()` from the kit and never mount their own.
- `./routes`, `./nav`, `./header`, `./boot` exposes and the CASL ability distribution are
  unchanged. Icons in manifests keep the `mdi-*` names.
- CSS: each front-end ships its own compiled stylesheet; the kit's theme file is imported
  by every build so tokens are identical.
