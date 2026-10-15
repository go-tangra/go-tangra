// Package helpdocs loads, validates and serves a module's user
// documentation: Markdown pages with YAML front-matter plus images, kept in
// the module repository under docs/help and embedded in the module binary.
//
// A module embeds the tree, loads it once at start-up and serves the result
// to the gateway on its Freya HTTP server:
//
//	//go:embed all:help
//	var helpFS embed.FS
//
//	sub, _ := fs.Sub(helpFS, "help")
//	b, err := helpdocs.Load(sub, "ipam")
//	app.HTTP().Handle(helpdocs.BundlePath, helpdocs.NewHandler(b, nil))
//
// The gateway fetches the bundle (GET /docs/v1/bundle) over the mTLS mesh,
// renders the Markdown itself and indexes it for search; this package never
// renders. The bundle digest (Bundle.Digest) is announced at registration so
// the gateway refetches only when the content changed. A module without docs
// gets an empty bundle whose digest is "".
//
// Everything is strict: unknown front-matter keys, unresolved relative links,
// SVG and unsniffable images, and oversize trees are refused. Load reports
// every problem it finds (as Problems), not just the first; cmd/tangra-docs
// prints them for CI.
package helpdocs
