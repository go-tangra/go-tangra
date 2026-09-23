# Agent memory

> Maintained by spec-router. Decisions, interfaces and gotchas reported by coding
> agents after each integrated task, shared with every agent on later tasks.
> Humans may edit or prune entries; agents treat this file as read-only.

## Decisions

- [T001 claude] Pinned via `services/auth/tools/ldap.go` (`//go:build tools`, blank import), because `go mod tidy` removes a requirement nothing imports. This file isn't part of any binary.
- [T002 claude] The package comments follow the pattern of the existing `<domain>db` packages: `directorydb` is DB bindings only and is covered by the integration suite, and `directory` reaches storage through a Store interface (`directorydb` in production, `memstore` in tests).
- [T002 claude] The doc comments record security commitments that later tasks must honour: the bind password uses associated data `ldap-bind:<tenant_id>:<connection_id>` and is zeroed after Bind; there is no skip-verify field; `ldapfake` is never linked into the binary.
- [T005 claude] Sentinel bind-password values must start with `LDAP-MARKER-PW-` followed by at least one letter or digit (e.g. `LDAP-MARKER-PW-s3cret`). The scan pattern is `LDAP-MARKER-PW-[A-Za-z0-9]`, so the bare prefix in source code or test names never matches. This follows the `DNS-MARKER-KEY-*` / `LCM-MARKER-SECRET-*` convention.
- [T005 claude] Only `internal/ldapdir` is in the 100% gate, as the task asks. `internal/directory` is held to the 80% overall target only.
- [T004 kimi] Naming encodes grammar-level truth, not freya policy: valid-* (grammar-valid, in all caps), invalid-* (grammar-rejected), injection-* (fragments that must fail standalone and never escape Combine), policy-* (grammar-valid but refused by D6 caps), odd-* (grammar-valid edges like `(&)`, `(uid=)`, empty DN — acceptance is T044's call), everything else named by dimension. Table tests must not assert…
- [T004 kimi] Corpus files have no trailing newline (it changes parser input); .editorconfig scoped `[services/auth/tests/fuzz/testdata/ldap/**]` insert_final_newline=false / trim_trailing_whitespace=false to protect them.
- [T004 kimi] DN scoping expectations are relative to base dc=example,dc=test, case-insensitive RDN compare (matches people.ldif from T003).
- [T003 kimi] TLS contract for T064/T067: harness mounts server.crt + server.key at /tls (env TLS_DIR); entrypoint stages them ldap-owned into /run/openldap/tls and slapd.ldif points there — mounts of any mode/ownership work. StartTLS on 389, ldaps on 636, both requiring the per-run test CA.
- [T003 kimi] Fixture layout: no-mail (uid=eng4) and shared-mail twins (uid=eng6/eng7) live under ou=Engineering (the connection base) so quickstart Scenario 2's import math (created: 4) works; uid=eng-outlier has departmentNumber: 7 as the base-filter (departmentNumber=42) trap; the 1 MiB description is generated at build time and intentionally kept out of people.ldif.
- [T003 kimi] Bind DN for all tests: cn=reader,dc=example,dc=test / reader-password; cn=admin/admin-password (root DN) is debugging-only.
- [T006 claude] Bounds: `dial_timeout` in (0, 30s]; `max_size_limit` in [1, 1000]; `max_time_limit` in [1s, 60s] (matching the data-model CHECKs 1..1000 and 1..60); `rate_per_minute` and `max_connections_per_tenant` must be > 0, with no upper cap. An empty `allowed_ports` is refused, and ports must be 1..65535.
- [T006 claude] CIDRs must parse as prefixes (a bare IP like `10.0.0.1` is refused; `netip.ParsePrefix` behaves this way). Directory fields are validated even when `enabled: false`.
- [T006 claude] `allow_plaintext` is refused only when `env: production`. `Warnings()` must mention `directory`, `allow_plaintext`, `allow_cidrs` and each allow CIDR string verbatim. `deny_cidrs` produces no directory warning. With defaults, the production shape must give zero warnings.
- [T006 claude] Default `AllowCIDRs` is empty. The default `DenyCIDRs` is not asserted, so T007 may choose it.
- [T007 claude] `deny_cidrs` defaults to empty. The always-denied set (loopback, link-local/metadata, unspecified, multicast) must be enforced in code by the `ldapdir` target policy (D5), not through config. Private ranges are not denied by default because customer directories are often on private networks.
- [T007 claude] Validation order: `allow_plaintext` (production only), then deny CIDRs, allow CIDRs, ports, dial timeout, size limit, time limit, rate, connections. The first failure is returned.
- [T008 claude] The test uses raw SQL for the new tables, not store helpers, so it does not depend on T010. It runs as `auth_app` through `st.Tx` (RLS applies) and checks results through the admin connection.
- [T009 claude] No separate `(tenant_id, connection_id)` index on `user_directory_links`. The unique index `user_directory_links_source_uid (tenant_id, connection_id, directory_uid)` has those columns first, so it serves the same lookups.
- [T009 claude] `user_directory_links.tenant_id` has no FK to tenants, like `group_members`. `directory_connections.tenant_id` does reference `tenants(id)`.
- [T009 claude] The Down migration first runs `set_config('app.system','on',true)`, because `users` has FORCE RLS. It then deletes users with status `imported` (their dependent rows go too, via cascades), restores the three-value CHECK and drops both tables.
- [T010 claude] `ListUsers` signature is unchanged. It fills two new `store.User` fields, `Directory *DirectoryLink` and `InvitationID *string`, and no other query sets them. This avoids breaking memstore, userdb and admin before T011/T050.
- [T010 claude] A pending invitation means not accepted and not revoked, including expired ones, because Resend works on expired invitations. It is only looked up for `status='invited'`, and the most recent one wins.
- [T010 claude] `UpdateImportedUser` / `DeleteImportedUser` return `ErrNotFound` when the user is missing or is not `imported`. The service loads the user first to tell 404 from 409 (invalid_state).
- [T010 claude] `UpdateDirectoryConnection` keeps the stored password when `BindPasswordEnc` is empty. It does not change the test result, `created_by` or `created_at`. It sets `updated_by`.
- [T010 claude] Store functions pass structs by value, like the rest of the package (gocritic hugeParam warnings accepted).
- [T012 claude] No `user_activated` event type. Per research D14, activation is emitted as `audit.InviteCreated` with `Reason: "activation"` and `SubjectID` set to the user.
- [T013 claude] "Dummy verify executed" is checked in two ways. An imported row that has a stray `PasswordHash` still refuses the correct password, and an imported sign-in must cost 0.5–2× an unknown one (best of 3 runs, pad disabled), the same bound `password_test.go` uses.
- [T013 claude] The invite test inserts the invitation directly with `ms.InsertInvitation` rather than calling `CreateWith`. D10 will change `CreateWith` to turn an imported e-mail into an activation, and that would conflict with this test.

## Interfaces

- [T001 claude] Module version: `github.com/go-ldap/ldap/v3 v3.4.14`. go-ldap's own test dependencies (gokrb5, sspi) are in go.sum but not linked into the binary.
- [T002 claude] Package paths: `github.com/go-freya/freya/services/auth/internal/{ldapdir,ldapdir/ldapfake,directory,directory/directorydb}`.
- [T005 claude] `scripts/redaction-scan.sh` runs `go test -count=1 -v ./internal/ldapdir/... ./internal/directory/...` without build tags, with `FREYA_CAPTURE_DIR=$ARTIFACTS/capture` exported. It copies that log to `$FREYA_CAPTURE_DIR/ldap-suite.log`, and every file in the capture directory is scanned.
- [T004 kimi] Directory layout: testdata/ldap/{filters,dns,urls,objectguid}/; .bin = non-UTF-8/binary, .txt = UTF-8. Regenerate: `cd services/auth/tests/fuzz/testdata/ldap && go run gen.go`.
- [T004 kimi] Documented expected GUID decodes: valid-sequential.bin → 03020100-0504-0706-0809-0a0b0c0d0e0f, valid-ad-example.bin → 3f78f21c-a23e-4c71-9b4e-2d6f9c1a7b55.
- [T004 kimi] Consumers: T037–T039 table tests and T040 fuzz targets (FuzzCompileUserFilter, FuzzCombine, FuzzScopeBase, FuzzCheckURL, FuzzDecodeEntry) — load whole subdirs as f.Add seeds.
- [T003 kimi] Image path for testcontainers FromDockerfile (T064) and compose build (T067): services/auth/tests/integration/testdata/openldap/.
- [T003 kimi] Env knobs: TLS_DIR (default /tls), SLAPD_LOGLEVEL (default stats); container exposes 389/636, slapd runs as user ldap.
- [T003 kimi] Seed facts tests can rely on: base ou=Engineering,dc=example,dc=test; alias cn=eng-secret-alias → cn=hidden,ou=Secret; referral ou=Partners (ref ldap://directory.example.invalid); eng5 description = exactly 1048576 bytes.
- [T006 claude] `Config.Directory Directory` (yaml `directory`). `Directory{Enabled bool; AllowPlaintext bool; Targets DirectoryTargets; DialTimeout time.Duration; MaxSizeLimit int; MaxTimeLimit time.Duration; RatePerMinute int; MaxConnectionsPerTenant int}`, with yaml keys `enabled, allow_plaintext, targets, dial_timeout, max_size_limit, max_time_limit, rate_per_minute, max_connections_per_tenant`.
- [T006 claude] `DirectoryTargets{DenyCIDRs []string; AllowCIDRs []string; AllowedPorts []int}`, with yaml keys `deny_cidrs, allow_cidrs, allowed_ports`.
- [T006 claude] Validate error messages must contain the full dotted key, e.g. `directory.targets.allow_cidrs`, `directory.targets.allowed_ports`, `directory.dial_timeout`, `directory.max_size_limit`, `directory.max_time_limit`, `directory.rate_per_minute`, `directory.max_connections_per_tenant`, `directory.allow_plaintext`.
- [T007 claude] `config.Directory{Enabled, AllowPlaintext bool; Targets DirectoryTargets; DialTimeout time.Duration; MaxSizeLimit int; MaxTimeLimit time.Duration; RatePerMinute, MaxConnectionsPerTenant int}`
- [T007 claude] `config.DirectoryTargets{DenyCIDRs, AllowCIDRs []string; AllowedPorts []int}`. CIDRs are stored as strings that are known to parse, so consumers call `netip.ParsePrefix` again (it cannot fail after `Validate`).
- [T007 claude] Warning texts: `directory connections may use ldap:// without TLS (directory.allow_plaintext)` and `directory.targets.allow_cidrs overrides deny_cidrs for: <cidrs joined by ", ">`.
- [T008 claude] T009 must produce: constraint named exactly `users_status_check` (drop and re-add; only one status CHECK on users), index `users_imported_idx`, one policy named `tenant_isolation` per new table.
- [T008 claude] Minimum insert columns the test uses: `directory_connections(id, tenant_id, name, kind='openldap', url, tls_mode='ldaps', bind_dn, bind_password_enc, base_dn, attr_uid='entryUUID', attr_email='mail', attr_display_name='cn')`. Any other column needs a DEFAULT or a nullable type, and T009's CHECKs must accept these values (url `ldaps://h`, bind_dn `cn=a`, base_dn `dc=a` also appear).
- [T008 claude] `user_directory_links(user_id, tenant_id, connection_id, connection_name, directory_uid, directory_dn, first_imported_at, last_imported_at)`.
- [T008 claude] Added helper `sqlState(err) string` in the store package's integration test files.
- [T009 claude] Index and constraint names:
- [T009 claude] `directory_connections_tenant_name` (unique on `tenant_id, lower(name)`) → a duplicate name gives 23505.
- [T009 claude] `user_directory_links_source_uid` (unique) → a repeat import of the same source uid gives 23505.
- [T009 claude] `users_status_check`, `users_imported_idx`.
- [T009 claude] DB CHECKs, all giving 23514:
- [T010 claude] `InsertDirectoryConnection(ctx, tx, DirectoryConnection) error` (duplicate name → ErrConflict); `GetDirectoryConnection(ctx, tx, tenantID, id)`; `GetDirectoryConnectionAnyTenant(ctx, tx, id)` (system scope); `ListDirectoryConnections(ctx, tx, tenantID)` (ordered by lower(name)); `CountDirectoryConnections(ctx, tx, tenantID) (int, error)`; `UpdateDirectoryConnection(ctx, tx, DirectoryConnection) …
- [T010 claude] `UsersByEmails(ctx, tx, tenantID, []string) (map[string]User, error)`: matching is case-insensitive (citext) and the map is keyed by the stored e-mail. `LinksByUIDs(ctx, tx, tenantID, connID, []string) (map[string]DirectoryLink, error)` is keyed by directory uid.
- [T010 claude] `UpsertLink(ctx, tx, DirectoryLink) error`: upserts on user_id and keeps `first_imported_at`. The same uid already linked to another user gives ErrConflict.
- [T010 claude] `UpdateImportedUser(ctx, tx, tenantID, userID, ImportedProfile{Email, DisplayName, FirstName, LastName string; DisplayNameExplicit bool}) error`: an e-mail already in use gives ErrConflict. `DeleteImportedUser(ctx, tx, tenantID, userID) error`.
- [T010 claude] `DirectoryConnection.CAPEM` is "" for NULL. `LastTestOutcome`, `LastTestAt`, `CreatedBy`, `UpdatedBy` and `DirectoryLink.ConnectionID` / `ImportedBy` are pointers.
- [T012 claude] `audit.DirectoryConnectionCreated`, `audit.DirectoryConnectionUpdated`, `audit.DirectoryConnectionDeleted`, `audit.DirectoryConnectionTested`, `audit.DirectorySearched`, `audit.DirectoryImported`, `audit.ImportedUserDeleted` (all `audit.EventType`).
- [T013 claude] The sign-in test expects imported attempts to be recorded as `memstore.Attempt{UserID: "", Outcome: "refused", Reason: "unknown_account"}`. `signin_failed` audit rows must have `ActorUserID == nil` and `Reason == "unknown_account"`. There must be no `lockout` event and no `cache.RateKey("fail", uid)` key.

## Gotchas

- [T001 claude] `make vuln` runs `govulncheck ./...` without the `tools` tag, so it won't scan go-ldap until real code imports it. Use `govulncheck -tags tools ./tools/` in the meantime.
- [T001 claude] `make lint` already fails with 57 findings (revive/staticcheck/gosec/goimports) in files this task didn't touch. None are in tools/ or LDAP code.
- [T002 claude] The packages contain no statements yet, so the 100 % coverage gate for `ldapdir` (a later task adds it to SECURITY_PKGS) applies once real code lands.
- [T005 claude] The scan uses grep's basic regular expressions (`grep -rc`, no `-E`). Keep it that way: switching to `-E` would change how existing patterns like `+38591` and `\$argon2id\$` are read.
- [T005 claude] A test that fails in those packages makes `make redaction-scan` fail.
- [T005 claude] The sentinel value must never appear in `-v` test output (`t.Log`, failure messages). Assert on whether it is present, and don't print the value.
- [T004 kimi] Empty filter input is grammar-rejected by ldap.CompileFilter(""); D6 defaulting to (objectClass=*) happens in freya code before compilation — valid-empty.txt is 0 bytes by design.
- [T004 kimi] go-ldap's grammar accepts things policy must refuse: raw NUL in values, `(bad_attr=x)`, `(&)`, deep nesting >16, >64 components — hence the policy-* prefix.
- [T004 kimi] `ldap://::1` parses as hostname ":" port "1"; `ldap://host:389:636` as hostname "host:389" — parsing alone is never sufficient for CheckURL.
- [T004 kimi] policy-deep-nesting-16 vs -20 straddle the depth-16 cap; which is accepted depends on how T044 counts depth (fuzz-only seeds, no outcome claimed).
- [T003 kimi] docker on this machine: socket is root:docker and the login session is stale — use `sg docker -c '...'` (jadmin IS in the docker group per /etc/group).
- [T003 kimi] cn=config bootstrap: slaptest/slapadd -F need pre-created dirs; slaptest must run schema-only (a database section makes it try to open mdb and fail); slaptest-generated schema ldifs have relative DNs and no blank-line separators (the Dockerfile sed-rewrites the DN and inserts separators); slapadd rejects changetype: modify — use slapmodify; busybox awk has no paragraph mode.
- [T003 kimi] Entry timestamps are slapadd build time — tests must not assert on them; any people.ldif/slapd.ldif change requires an image rebuild (T064 FromDockerfile rebuilds automatically).
- [T006 claude] `AllowedPorts` must be `[]int`, because the tests compare it with `slices.Equal` against `[]int{...}`.
- [T007 claude] golangci-lint still reports `hugeParam` on the existing `Config.Validate` and `Config.Warnings` value receivers. That's not from this task, so I left it.
- [T008 claude] Run with `sg docker -c 'go test -tags integration -run TestMigration0008LDAPImport ./internal/store/'` (the docker group isn't active in the stale login session).
- [T008 claude] FK checks ignore RLS, so a cross-tenant link insert is refused only by the policy's WITH CHECK on `tenant_id`, which is how the test checks it.
- [T009 claude] The URL CHECK refuses anything with userinfo, a path, a query or a fragment, including a trailing `/`. The service should normalise the URL (e.g. strip a trailing slash) before inserting.
- [T009 claude] Run the integration tests with `sg docker -c 'go test -tags integration ./internal/store/'`.
- [T010 claude] citext comparisons with a Go `[]string` need `$n::text[]::citext[]`. A plain `text[]` compares case-sensitively.
- [T010 claude] `DirectoryConnection` includes `BindPasswordEnc`. API views must drop it.
- [T012 claude] `audit.Row` redacts any detail key whose name contains `password|secret|token|key|code|cookie|authorization|phone|first_name|last_name|display_name|email_address`. Avoid detail keys like `search_key`, `status_code` or `error_code`; use `reason` or plain names such as `connection_id`, `filter`, `count`, `truncated`, `created`, `updated`, `user_ids`.
- [T012 claude] There is no database CHECK on `event_type`, so the Go `known` map is the only thing that enforces the vocabulary.
- [T013 claude] In the unknown branch, `u` still holds the imported row after T014's fix. That is harmless only because the unknown branch never uses `u.ID`, so keep it that way.
