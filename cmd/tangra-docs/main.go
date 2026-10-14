// Command tangra-docs checks and packs a module's help pages (package
// helpdocs):
//
//	tangra-docs lint [dir]                          (default docs/help)
//	tangra-docs bundle [-module name] [-o file.json] [dir]
//
// lint prints every problem as "file:line: message" (file relative to dir)
// and exits 1 when there is any. bundle prints the digest, page count and
// size of the bundle a module would serve, and writes it with -o. CI runs
// lint through .github/actions/docs-lint.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-tangra/go-tangra/v4/helpdocs"
)

func main() { os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr)) }

func cli(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: tangra-docs lint|bundle [flags] [dir]")
		return 2
	}
	switch args[0] {
	case "lint":
		return lint(args[1:], stdout, stderr)
	case "bundle":
		return bundle(args[1:], stdout, stderr)
	}
	_, _ = fmt.Fprintf(stderr, "unknown command %q (lint, bundle)\n", args[0])
	return 2
}

// root parses flags (which may also follow the directory) and returns the
// docs directory, refusing one that does not exist.
func root(fs *flag.FlagSet, args []string, stderr io.Writer) (string, bool) {
	if err := fs.Parse(args); err != nil {
		return "", false
	}
	dir := helpdocs.DefaultDir
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil || fs.NArg() > 0 {
			_, _ = fmt.Fprintln(stderr, "expected at most one directory")
			return "", false
		}
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		_, _ = fmt.Fprintf(stderr, "%s: not a directory\n", dir)
		return "", false
	}
	return dir, true
}

// report prints err (Problems or another error) and returns the exit code.
func report(dir string, err error, stderr io.Writer) int {
	var ps helpdocs.Problems
	if !errors.As(err, &ps) {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	for _, p := range ps {
		if p.File != "" {
			p.File = filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(p.File)))
		}
		_, _ = fmt.Fprintln(stderr, p.String())
	}
	_, _ = fmt.Fprintf(stderr, "%d problem(s)\n", len(ps))
	return 1
}

func lint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir, ok := root(fs, args, stderr)
	if !ok {
		return 2
	}
	if err := helpdocs.Lint(os.DirFS(dir)); err != nil {
		return report(dir, err, stderr)
	}
	_, _ = fmt.Fprintf(stdout, "%s: ok\n", dir)
	return 0
}

func bundle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	module := fs.String("module", "", "module (service) name, e.g. ipam")
	out := fs.String("o", "", "write the bundle JSON to this file")
	dir, ok := root(fs, args, stderr)
	if !ok {
		return 2
	}
	if *module == "" {
		_, _ = fmt.Fprintln(stderr, "-module is required")
		return 2
	}
	b, err := helpdocs.Load(os.DirFS(dir), *module)
	if err != nil {
		return report(dir, err, stderr)
	}
	raw, err := b.Marshal()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if *out != "" {
		if err := os.WriteFile(*out, raw, 0o644); err != nil { // #nosec G306 -- public documentation
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
	}
	digest := b.Digest
	if digest == "" {
		digest = "(empty)"
	}
	_, _ = fmt.Fprintf(stdout, "%s: digest %s, %d pages, %d images, %d bytes\n", b.Module, digest, len(b.Pages), len(b.Assets), len(raw))
	return 0
}
