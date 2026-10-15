package helpdocs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// goldenDigest pins the canonical encoding: if it changes, every module's
// announced digest changes with it, so a change here must be deliberate.
const goldenDigest = "48a94e4946d1c99c25f4b6d58c5de3560ee8242c33a73fff52a31452781b995b"

func TestDigestStable(t *testing.T) {
	b1, err := Load(base(), "ipam")
	if err != nil {
		t.Fatal(err)
	}
	if b1.Digest != goldenDigest {
		t.Fatalf("digest = %s, want %s", b1.Digest, goldenDigest)
	}

	// The same tree on disk, written in reverse order.
	dir := t.TempDir()
	files := base()
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	for i := len(names) - 1; i >= 0; i-- {
		p := filepath.Join(dir, filepath.FromSlash(names[i]))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, files[names[i]].Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b2, err := Load(os.DirFS(dir), "ipam")
	if err != nil {
		t.Fatal(err)
	}
	if b2.Digest != b1.Digest {
		t.Fatalf("disk digest %s != %s", b2.Digest, b1.Digest)
	}

	// Input order of pages/assets does not matter.
	shuffled := *b1
	shuffled.Pages = append([]Page(nil), b1.Pages...)
	shuffled.Pages[0], shuffled.Pages[3] = shuffled.Pages[3], shuffled.Pages[0]
	if ComputeDigest(&shuffled) != b1.Digest {
		t.Fatal("digest depends on page order")
	}
}

func TestDigestChanges(t *testing.T) {
	ref, _ := Load(base(), "ipam")
	edits := map[string]func(fstest.MapFS){
		"page text": func(f fstest.MapFS) {
			f["guides/setup.md"] = page("title: Setup\nkind: example\nsummary: Set it up.\nrequires: subnets:write\naudience: operator\nlang: en", "Steps!\n")
		},
		"title": func(f fstest.MapFS) {
			f["guides/index.md"] = page("title: Guide\nkind: guide", "All guides. Back [home](../index.md).\n")
		},
		"image":     func(f fstest.MapFS) { f["img/logo.png"] = &fstest.MapFile{Data: []byte(pngData + "x")} },
		"new image": func(f fstest.MapFS) { f["img/b.png"] = &fstest.MapFile{Data: []byte(pngData)} },
	}
	for name, edit := range edits {
		f := base()
		edit(f)
		b, err := Load(f, "ipam")
		if err != nil {
			t.Fatal(name, err)
		}
		if b.Digest == ref.Digest {
			t.Errorf("%s: digest unchanged", name)
		}
	}
	other, _ := Load(base(), "dns")
	if other.Digest == ref.Digest {
		t.Error("module name not in digest")
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	b, err := Load(base(), "ipam")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != b.Digest || len(got.Pages) != 4 || string(got.Assets[0].Data) != pngData {
		t.Fatalf("round trip: %+v", got)
	}

	tamper := []func(*Bundle){
		func(x *Bundle) { x.Pages[0].Markdown += "x" },
		func(x *Bundle) { x.Assets[0].Data = append([]byte(nil), "\x89PNG\r\n\x1a\nzz"...) },
		func(x *Bundle) { x.Schema = 2 },
		func(x *Bundle) { x.Pages[1].Slug = x.Pages[0].Slug },
		func(x *Bundle) { x.Assets[0].MIME = "image/svg+xml" },
	}
	for i, f := range tamper {
		x, _ := Decode(raw)
		f(x)
		r, _ := x.Marshal()
		if _, err := Decode(r); !errors.Is(err, ErrInvalidBundle) {
			t.Errorf("tamper %d: %v", i, err)
		}
	}
	if _, err := Decode([]byte(`{"schema":1,"module":"ipam","digest":"","pages":[],"assets":[],"extra":1}`)); err == nil {
		t.Error("unknown field accepted")
	}
	if _, err := Decode([]byte(`{"schema":1,"module":"ipam","digest":"","pages":[],"assets":[]}`)); err != nil {
		t.Errorf("empty bundle: %v", err)
	}
}
