package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/catalogue"
)

const descriptor = `schema: 1
module: sms-gw
display_name: SMS Gateway
category: Communications
summary: SMS API.
image: ghcr.io/go-tangra/go-tangra-sms-gw
routes: { prefixes: [/api/sms-gw, /m/sms-gw], names: [sms-gw] }
bundle: { dir: deploy/bundle, templates: [compose.yaml] }
`

func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(p, s string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tangra-module.yaml", descriptor)
	write("deploy/bundle/compose.yaml", "services: {}\n")
	write("perms.json", `["providers:read","messages:send"]`)
	return dir
}

func run(args ...string) (int, string) {
	var out bytes.Buffer
	code := cli(args, &out, &out)
	return code, out.String()
}

func TestValidate(t *testing.T) {
	dir := repo(t)
	if code, out := run("validate", "-f", filepath.Join(dir, "tangra-module.yaml")); code != 0 || !strings.Contains(out, "sms-gw: valid") {
		t.Fatalf("%d %s", code, out)
	}
	_ = os.WriteFile(filepath.Join(dir, "tangra-module.yaml"), []byte(descriptor+"oops: 1\n"), 0o644)
	if code, out := run("validate", "-f", filepath.Join(dir, "tangra-module.yaml")); code != 1 || !strings.Contains(out, "oops") {
		t.Fatalf("%d %s", code, out)
	}
	// A template the bundle lacks fails validation too.
	_ = os.WriteFile(filepath.Join(dir, "tangra-module.yaml"), []byte(strings.Replace(descriptor, "[compose.yaml]", "[missing.yaml]", 1)), 0o644)
	if code, out := run("validate", "-f", filepath.Join(dir, "tangra-module.yaml")); code != 1 || !strings.Contains(out, "missing.yaml") {
		t.Fatalf("%d %s", code, out)
	}
}

func TestBuild(t *testing.T) {
	dir := repo(t)
	out := filepath.Join(dir, "dist")
	code, msg := run("build", "-f", filepath.Join(dir, "tangra-module.yaml"), "-version", "4.3.0", "-repository", "go-tangra/go-tangra-sms-gw",
		"-permissions-file", filepath.Join(dir, "perms.json"), "-out", out)
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	entryRaw, _ := os.ReadFile(filepath.Join(out, "catalogue-entry.json"))
	bundle, _ := os.ReadFile(filepath.Join(out, "bundle.zip"))
	e, err := catalogue.ParseEntry(entryRaw)
	if err != nil {
		t.Fatal(err)
	}
	if e.Version != "4.3.0" || len(e.Permissions) != 2 || e.CheckBundle(bundle) != nil {
		t.Fatalf("%+v", e)
	}
	// A v-prefixed tag is accepted and normalised (release tags are vX.Y.Z).
	if code, msg := run("build", "-f", filepath.Join(dir, "tangra-module.yaml"), "-version", "v4.3.1", "-repository", "go-tangra/go-tangra-sms-gw",
		"-permissions-file", filepath.Join(dir, "perms.json"), "-out", out); code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	if code, _ := run("build", "-f", filepath.Join(dir, "tangra-module.yaml"), "-version", "4.3", "-repository", "go-tangra/x", "-out", out); code != 1 {
		t.Fatal("bad version built")
	}
	_ = os.WriteFile(filepath.Join(dir, "perms.json"), []byte(`not json`), 0o644)
	if code, _ := run("build", "-f", filepath.Join(dir, "tangra-module.yaml"), "-version", "4.3.0", "-repository", "go-tangra/x",
		"-permissions-file", filepath.Join(dir, "perms.json"), "-out", out); code != 1 {
		t.Fatal("bad permissions file accepted")
	}
	if code, _ := run("nope"); code != 2 {
		t.Fatal("unknown command")
	}
}
