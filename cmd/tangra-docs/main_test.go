package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, s := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(args ...string) (int, string) {
	var out bytes.Buffer
	code := cli(args, &out, &out)
	return code, out.String()
}

func TestLint(t *testing.T) {
	good := tree(t, map[string]string{"index.md": "---\ntitle: Home\nkind: guide\n---\nHi [a](a.md)\n", "a.md": "---\ntitle: A\nkind: faq\n---\nA\n"})
	if code, out := run("lint", good); code != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("%d %s", code, out)
	}
	bad := tree(t, map[string]string{"index.md": "---\ntitle: Home\nkind: guide\nfoo: 1\n---\nHi [a](b.md)\n"})
	code, out := run("lint", bad)
	if code != 1 || !strings.Contains(out, filepath.ToSlash(bad)+"/index.md:4: unknown front-matter field") ||
		!strings.Contains(out, "index.md:6: link b.md") || !strings.Contains(out, "2 problem(s)") {
		t.Fatalf("%d %s", code, out)
	}
	if code, _ := run("lint", filepath.Join(good, "missing")); code != 2 {
		t.Fatal("missing dir must fail")
	}
	if code, _ := run("nope"); code != 2 {
		t.Fatal("unknown command")
	}
}

func TestBundle(t *testing.T) {
	dir := tree(t, map[string]string{"index.md": "---\ntitle: Home\nkind: guide\n---\nHi\n"})
	if code, _ := run("bundle", dir); code != 2 {
		t.Fatal("-module required")
	}
	out := filepath.Join(t.TempDir(), "b.json")
	code, msg := run("bundle", "-module", "ipam", "-o", out, dir)
	if code != 0 || !strings.Contains(msg, "ipam: digest ") || !strings.Contains(msg, "1 pages") {
		t.Fatalf("%d %s", code, msg)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		Schema int    `json:"schema"`
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(raw, &b); err != nil || b.Schema != 1 || !strings.Contains(msg, b.Digest) {
		t.Fatalf("%v %+v", err, b)
	}
	// Flags after the directory work too.
	if code, msg := run("bundle", dir, "-module", "ipam"); code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
}
