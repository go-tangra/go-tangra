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
