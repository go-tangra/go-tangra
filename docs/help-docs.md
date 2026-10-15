# Module help pages

Every go-tangra module can ship user documentation inside its binary. The
module serves the pages as one **bundle** to the gateway over the mTLS mesh;
the gateway renders the Markdown to sanitized HTML, indexes it for full-text
search and shows it in the portal. Modules never render: they keep the
Markdown source, validated at build time by `tangra-docs lint`.

Package: `github.com/go-tangra/go-tangra/v4/helpdocs`. CLI:
`cmd/tangra-docs`. CI: `.github/actions/docs-lint`.

## Layout

```
docs/help/
  index.md                 slug "index" (required: the module's start page)
  getting-started.md       slug "getting-started"
  guides/
    index.md               slug "guides"
    subnets.md             slug "guides/subnets"
  images/
    subnet-form.png        referenced as ![...](images/subnet-form.png)
```

- **Pages** are `*.md` files anywhere under `docs/help`. The slug is the path
  without `.md`; every segment must be lowercase `[a-z0-9-]` (no underscores,
  no capitals). `x/index.md` has slug `x`, so `x.md` and `x/index.md` together
  are a duplicate. A root `index.md` is required. Slugs `assets` and
  `assets/...` are reserved (the gateway serves images there); images
  themselves may live in an `assets/` folder.
- **Images** are png, jpg/jpeg, webp or gif, anywhere under `docs/help`, path
  segments lowercase `[a-z0-9_.-]`. The type is checked by content sniffing,
  so a renamed file is caught. **SVG is not allowed** (it can carry script).
- Nothing else may live in the tree: other files (including dotfiles and
  symlinks) are lint errors.

## Front-matter

Every page starts with YAML front-matter between two `---` lines:

```markdown
---
title: Allocate an address
summary: Reserve the next free IP in a subnet.
kind: guide
order: 20
tags: [ipam, addresses]
requires: addresses:write
audience: user
---
# Allocate an address
...
```

| Field      | Required | Meaning |
|------------|----------|---------|
| `title`    | yes      | One line, at most 200 characters. |
| `summary`  | no       | At most 500 characters; shown in search results and lists. |
| `kind`     | yes      | `guide` (task-oriented how-to), `example` (worked example), `reference` (fields, limits, API), `faq`. |
| `order`    | no       | Integer sort key within the page's section (default 0). |
| `tags`     | no       | Up to 20 lowercase `[a-z0-9-]` tags (32 chars max), no duplicates. |
| `requires` | no       | A permission of this module in its own `resource:action` form, the same as a manifest nav entry's `requires`. The gateway shows the page (and returns it in search) only to users holding it. |
| `audience` | no       | `user` (default) or `operator` (installation, configuration, troubleshooting). |
| `lang`     | no       | `en` (default; the only language accepted for now). |

Unknown fields are errors, so typos do not silently drop metadata.

`requires` and `audience` are filters, not secrets: anything in the bundle is
readable by the gateway. Do not put credentials or tenant data in docs.

## Links

- Link to other pages by their file, relatively: `[subnets](guides/subnets.md)`,
  `[back](../index.md)`, `[field list](reference.md#fields)`. The target file
  must exist (anchors are not checked). The gateway rewrites these to portal
  URLs.
- Images are relative and must exist in the tree: `![form](images/form.png)`.
  Remote images are refused; bundle them.
- External links may use `http`, `https` or `mailto`. Root-relative links
  (`/ipam/subnets`) are portal routes and are left alone. Every other scheme
  (`javascript:`, `file:`, `data:`, ...) and protocol-relative `//host` URLs
  are refused, including in raw HTML `href`/`src` attributes.
- Links inside code spans and fenced code blocks are not checked.

## Limits

| Limit | Value |
|-------|-------|
| pages | 500 |
| page source | 256 KiB |
| image | 512 KiB |
| pages + images | 4 MiB |

## Lint and inspect

```sh
go run github.com/go-tangra/go-tangra/v4/cmd/tangra-docs@v4.X.Y lint            # docs/help
go run github.com/go-tangra/go-tangra/v4/cmd/tangra-docs@v4.X.Y bundle -module ipam -o /tmp/bundle.json
```

`lint` prints every problem as `file:line: message` and exits 1 if there is
any. `bundle` prints the digest, page and image counts and the bundle size.

In CI (after `actions/checkout` and `actions/setup-go`):

```yaml
- uses: go-tangra/go-tangra/.github/actions/docs-lint@v4.X.Y
  with:
    tool-version: v4.X.Y
```

The step passes when the module has no `docs/help` directory, and annotates
the pull request with each problem otherwise.

## Embedding and serving

Put an `embed.go` next to the tree (in `docs/`), so the module binary carries
it:

```go
// Package docs embeds the module's help pages.
package docs

import (
	"embed"
	"io/fs"
)

//go:embed all:help
var files embed.FS

// Help is the docs/help tree.
func Help() fs.FS {
	sub, err := fs.Sub(files, "help")
	if err != nil {
		panic(err)
	}
	return sub
}
```

At start-up, load it (refuse to start on problems: CI should have caught them)
and mount the handler on the module's **Freya HTTP server**, `app.HTTP()`:

```go
b, err := helpdocs.Load(docs.Help(), cfg.ServiceName) // e.g. "ipam"
if err != nil {
	return nil, fmt.Errorf("help docs: %w", err)
}
// Before any catch-all: HandlePrefix("/") registered first would shadow it.
app.HTTP().Handle(helpdocs.BundlePath, helpdocs.NewHandler(b, helpdocs.AllowServices(cfg.Gateway.Service)))
app.HTTP().HandlePrefix("/", api.Handler())
```

`GET /docs/v1/bundle` returns the bundle JSON with `ETag: "<digest>"` and
`Cache-Control: no-cache`; `If-None-Match` with the current digest gets 304.
Other methods get 405.

### Who may fetch it

The handler is for the gateway only. Two layers decide:

1. The Freya HTTP server's security chain runs before any route: the peer must
   present a valid SVID of the trust domain (mTLS, re-verified by
   `authn.Middleware`), not revoked, and the service-to-service policy
   (`authz`, operation `GET /docs/v1/bundle`) must allow it. The usual module
   policy rule (`from: [spiffe://<td>/svc/gateway]`, `operations: ["*"]`)
   already covers it.
2. The handler then checks the verified peer itself (`authn.FromContext`):
   `NewHandler(b, nil)` admits only the service `gateway`;
   `AllowServices(names...)` names others (use the module's configured gateway
   service name). Any other caller, even one the policy lets through, gets 403.
   A custom `helpdocs.Authorizer` (`func(*http.Request) bool`) can replace it.

Do not mount the handler on the module's public edge listener or behind the
user-token middleware: the gateway calls it with its service identity, not a
user token.

### Announcing the digest

Pass `b.Digest` to the gateway client before it registers: call
`gatewayclient.SetDocsDigest(b.Digest)` before `gatewayclient.New`, or set
`gatewayclient.Options.DocsDigest` (portal SDK v4.2.0 or later; the value
travels as `RegisterRequest.docs_digest`). The gateway fetches the bundle when the announced digest
differs from the one it indexed. A module without docs announces `""`:
`helpdocs.Load(nil, name)` (or a tree with no files) returns an empty bundle
with an empty digest, and the handler serves it without an ETag.

## Bundle format (schema 1)

```json
{
  "schema": 1,
  "module": "ipam",
  "digest": "<sha256 hex>",
  "pages": [{"slug": "index", "title": "...", "summary": "...", "kind": "guide",
             "order": 0, "tags": ["..."], "requires": "subnets:read",
             "audience": "user", "lang": "en", "markdown": "# ..."}],
  "assets": [{"path": "images/form.png", "mime": "image/png", "size": 1234,
              "sha256": "<hex>", "data": "<base64>"}]
}
```

Pages are sorted by slug and assets by path. `markdown` is the page without
its front-matter. The digest is the sha256 of a canonical, length-prefixed
encoding of the schema, module name and every page and asset field (assets
by their sha256), in sorted order: the same content always gives the same
digest, whatever the build machine, walk order or JSON formatting
(`helpdocs.ComputeDigest`). The gateway checks a fetched bundle with
`helpdocs.Decode`, which re-verifies schema, limits, image types, asset hashes
and the digest.
