// Command tangra-catalogue validates a module's tangra-module.yaml and builds
// the release assets the gateway's module catalogue reads (spec 035):
//
//	tangra-catalogue validate [-f tangra-module.yaml]
//	tangra-catalogue build -version 4.3.0 -repository owner/repo
//	    [-f tangra-module.yaml] [-permissions-file perms.json] [-out dist]
//
// build writes <out>/catalogue-entry.json and <out>/bundle.zip; the release
// workflow then attests and attaches them (.github/actions/catalogue-entry).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-tangra/go-tangra/v4/catalogue"
)

func main() { os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr)) }

func cli(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: tangra-catalogue validate|build [flags]")
		return 2
	}
	switch args[0] {
	case "validate":
		return validate(args[1:], stdout, stderr)
	case "build":
		return build(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown command %q (validate, build)\n", args[0])
	return 2
}

// load parses the descriptor and packs its bundle directory (relative to the
// descriptor's directory), checking the declared templates exist.
func load(file string) (catalogue.Descriptor, []byte, error) {
	raw, err := os.ReadFile(file) // #nosec G304 -- operator-supplied path
	if err != nil {
		return catalogue.Descriptor{}, nil, err
	}
	d, err := catalogue.ParseDescriptor(raw)
	if err != nil {
		return d, nil, err
	}
	root := filepath.Join(filepath.Dir(file), filepath.FromSlash(d.Bundle.Dir))
	bundle, err := catalogue.PackBundle(os.DirFS(root))
	if err != nil {
		return d, nil, err
	}
	files, err := catalogue.ReadBundle(bundle)
	if err != nil {
		return d, nil, err
	}
	for _, t := range d.Bundle.Templates {
		if _, ok := files[t]; !ok {
			return d, nil, fmt.Errorf("%w: bundle.templates: %s is not in %s", catalogue.ErrInvalid, t, d.Bundle.Dir)
		}
	}
	return d, bundle, nil
}

func validate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "tangra-module.yaml", "module descriptor")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, bundle, err := load(*file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: valid (bundle %d bytes, %d templates)\n", d.Module, len(bundle), len(d.Bundle.Templates))
	return 0
}

func build(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "tangra-module.yaml", "module descriptor")
	version := fs.String("version", "", "release version (X.Y.Z; a leading v is dropped)")
	repository := fs.String("repository", "", "GitHub owner/repo of the module")
	permsFile := fs.String("permissions-file", "", "JSON array of the module's permissions (resource:action)")
	out := fs.String("out", "dist", "output directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, bundle, err := load(*file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var perms []string
	if *permsFile != "" {
		raw, err := os.ReadFile(*permsFile) // #nosec G304 -- operator-supplied path
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.Unmarshal(raw, &perms); err != nil {
			fmt.Fprintf(stderr, "permissions file: %v\n", err)
			return 1
		}
	}
	e, err := catalogue.BuildEntry(d, strings.TrimPrefix(*version, "v"), *repository, perms, bundle)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	raw, err := e.Marshal()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.MkdirAll(*out, 0o755); err != nil { // #nosec G301 -- release output, public content
		fmt.Fprintln(stderr, err)
		return 1
	}
	for name, data := range map[string][]byte{"catalogue-entry.json": raw, "bundle.zip": bundle} {
		if err := os.WriteFile(filepath.Join(*out, name), data, 0o644); err != nil { // #nosec G306 -- public release assets
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "%s %s: wrote %s and %s\n", e.Module, e.Version, filepath.Join(*out, "catalogue-entry.json"), filepath.Join(*out, "bundle.zip"))
	return 0
}
