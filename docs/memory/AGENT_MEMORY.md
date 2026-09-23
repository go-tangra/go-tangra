# Agent memory

> Maintained by spec-router. Decisions, interfaces and gotchas reported by coding
> agents after each integrated task, shared with every agent on later tasks.
> Humans may edit or prune entries; agents treat this file as read-only.

## Decisions

- [T001 claude] Pinned via `services/auth/tools/ldap.go` (`//go:build tools`, blank import), because `go mod tidy` removes a requirement nothing imports. This file isn't part of any binary.
- [T002 claude] The package comments follow the pattern of the existing `<domain>db` packages: `directorydb` is DB bindings only and is covered by the integration suite, and `directory` reaches storage through a Store interface (`directorydb` in production, `memstore` in tests).
- [T002 claude] The doc comments record security commitments that later tasks must honour: the bind password uses associated data `ldap-bind:<tenant_id>:<connection_id>` and is zeroed after Bind; there is no skip-verify field; `ldapfake` is never linked into the binary.

## Interfaces

- [T001 claude] Module version: `github.com/go-ldap/ldap/v3 v3.4.14`. go-ldap's own test dependencies (gokrb5, sspi) are in go.sum but not linked into the binary.
- [T002 claude] Package paths: `github.com/go-freya/freya/services/auth/internal/{ldapdir,ldapdir/ldapfake,directory,directory/directorydb}`.

## Gotchas

- [T001 claude] `make vuln` runs `govulncheck ./...` without the `tools` tag, so it won't scan go-ldap until real code imports it. Use `govulncheck -tags tools ./tools/` in the meantime.
- [T001 claude] `make lint` already fails with 57 findings (revive/staticcheck/gosec/goimports) in files this task didn't touch. None are in tools/ or LDAP code.
- [T002 claude] The packages contain no statements yet, so the 100 % coverage gate for `ldapdir` (a later task adds it to SECURITY_PKGS) applies once real code lands.
