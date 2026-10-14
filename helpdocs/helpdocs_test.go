package helpdocs

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

const pngData = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func page(fm, body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("---\n" + fm + "\n---\n" + body)}
}

func base() fstest.MapFS {
	return fstest.MapFS{
		"index.md":           page("title: Home\nkind: guide\norder: 1\ntags: [start]", "# Home\n\nSee [setup](guides/setup.md#first) and ![logo](img/logo.png).\n"),
		"guides/index.md":    page("title: Guides\nkind: guide", "All guides. Back [home](../index.md).\n"),
		"guides/setup.md":    page("title: Setup\nkind: example\nsummary: Set it up.\nrequires: subnets:write\naudience: operator\nlang: en", "Steps.\n"),
		"img/logo.png":       {Data: []byte(pngData)},
		"faq/common-errs.md": page("title: FAQ\nkind: faq", "Q and A <https://example.org/x>.\n"),
	}
}

func problems(t *testing.T, err error) Problems {
	t.Helper()
	var ps Problems
	if !errors.As(err, &ps) {
		t.Fatalf("want Problems, got %v", err)
	}
	return ps
}

func TestLoadValid(t *testing.T) {
	b, err := Load(base(), "ipam")
	if err != nil {
		t.Fatal(err)
	}
	if b.Schema != Schema || b.Module != "ipam" || len(b.Digest) != 64 {
		t.Fatalf("bundle header: %+v", b)
	}
	var slugs []string
	for _, p := range b.Pages {
		slugs = append(slugs, p.Slug)
	}
	if got := strings.Join(slugs, ","); got != "faq/common-errs,guides,guides/setup,index" {
		t.Fatalf("slugs = %s", got)
	}
	setup := b.Pages[2]
	if setup.Kind != KindExample || setup.Requires != "subnets:write" || setup.Audience != AudienceOperator || setup.Lang != "en" || setup.Markdown != "Steps.\n" {
		t.Fatalf("setup page: %+v", setup)
	}
	if home := b.Pages[3]; home.Audience != AudienceUser || home.Lang != "en" || home.Order != 1 || home.Tags[0] != "start" {
		t.Fatalf("defaults: %+v", home)
	}
	if len(b.Assets) != 1 || b.Assets[0].MIME != "image/png" || b.Assets[0].Size != int64(len(pngData)) || len(b.Assets[0].SHA256) != 64 {
		t.Fatalf("assets: %+v", b.Assets)
	}
	if err := Lint(base()); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEmpty(t *testing.T) {
	for name, fsys := range map[string]fs.FS{"nil": nil, "empty": fstest.MapFS{}, "missing": mustSub(t, fstest.MapFS{"x/y": {}}, "help")} {
		b, err := Load(fsys, "ipam")
		if err != nil || !b.Empty() || b.Digest != "" || b.Pages == nil {
			t.Fatalf("%s: %+v %v", name, b, err)
		}
	}
	if _, err := Load(nil, "Not_Valid"); err == nil {
		t.Fatal("invalid module name accepted")
	}
}

func mustSub(t *testing.T, f fs.FS, dir string) fs.FS {
	t.Helper()
	s, err := fs.Sub(f, dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProblems(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(fstest.MapFS)
		wants []string // substrings of "file:line: message"
	}{
		{"no front-matter", func(f fstest.MapFS) { f["a.md"] = &fstest.MapFile{Data: []byte("# hi\n")} }, []string{"a.md:1: a page must start"}},
		{"unclosed", func(f fstest.MapFS) { f["a.md"] = &fstest.MapFile{Data: []byte("---\ntitle: x\n")} }, []string{"a.md:1: front-matter is not closed"}},
		{"unknown key", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\ncolour: red", "x") }, []string{`a.md:4: unknown front-matter field "colour"`}},
		{"missing title and kind", func(f fstest.MapFS) { f["a.md"] = page("order: 2", "x") }, []string{"a.md:1: title is required", "a.md:1: kind is required"}},
		{"bad kind", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: tutorial", "x") }, []string{`a.md:3: kind "tutorial"`}},
		{"bad order type", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\norder: first", "x") }, []string{"a.md:4: front-matter: order has the wrong type"}},
		{"bad yaml", func(f fstest.MapFS) { f["a.md"] = page("title: [A\nkind: guide", "x") }, []string{"a.md:2: front-matter:"}},
		{"bad tags", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\ntags: [Ok, ok, ok]", "x") }, []string{`tag "Ok"`, `duplicate tag "ok"`}},
		{"bad requires", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\nrequires: admin", "x") }, []string{`a.md:4: requires "admin"`}},
		{"bad audience", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\naudience: admin", "x") }, []string{`audience "admin"`}},
		{"bad lang", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide\nlang: de", "x") }, []string{`lang "de" is not supported`}},
		{"empty body", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "  \n") }, []string{"a.md:5: page has no content"}},
		{"no index", func(f fstest.MapFS) { delete(f, "index.md") }, []string{"index.md: a root index.md is required"}},
		{"uppercase slug", func(f fstest.MapFS) { f["Guide.md"] = page("title: A\nkind: guide", "x") }, []string{"Guide.md: page path segments"}},
		{"underscore slug", func(f fstest.MapFS) { f["my_page.md"] = page("title: A\nkind: guide", "x") }, []string{"my_page.md: page path segments"}},
		{"reserved assets slug", func(f fstest.MapFS) { f["assets/intro.md"] = page("title: A\nkind: guide", "x") }, []string{`assets/intro.md: slug "assets/intro" is reserved`}},
		{"duplicate slug", func(f fstest.MapFS) { f["guides.md"] = page("title: A\nkind: guide", "x") }, []string{`guides/index.md: duplicate slug "guides" (also guides.md)`}},
		{"broken page link", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "line\n[x](nope.md)\n") }, []string{"a.md:6: link nope.md: nope.md not found"}},
		{"link outside", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "[x](../../etc.md)") }, []string{"points outside docs/help"}},
		{"broken image", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "![x](img/none.png)") }, []string{"image img/none.png: img/none.png not found"}},
		{"external image", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "![x](https://e.org/a.png)") }, []string{"images must be bundled"}},
		{"javascript link", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "[x](javascript:alert(1))") }, []string{"scheme javascript: is not allowed"}},
		{"obfuscated scheme", func(f fstest.MapFS) {
			f["a.md"] = page("title: A\nkind: guide", "<a href=\" Java\tScript:alert(1)\">x</a>")
		}, []string{"scheme javascript: is not allowed"}},
		{"file link", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "[x](file:///etc/passwd)") }, []string{"scheme file: is not allowed"}},
		{"html href", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", `<a href="javascript:x">y</a>`) }, []string{"scheme javascript: is not allowed"}},
		{"ref definition", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "[x][r]\n\n[r]: data:text/html,hi") }, []string{"a.md:7: link data:text/html,hi: scheme data:"}},
		{"autolink", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "<vbscript:x>") }, []string{"scheme vbscript:"}},
		{"protocol relative", func(f fstest.MapFS) { f["a.md"] = page("title: A\nkind: guide", "[x](//evil.org)") }, []string{"protocol-relative"}},
		{"svg", func(f fstest.MapFS) { f["img/x.svg"] = &fstest.MapFile{Data: []byte("<svg/>")} }, []string{"img/x.svg: SVG images are not allowed"}},
		{"svg named png", func(f fstest.MapFS) {
			f["img/x.png"] = &fstest.MapFile{Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>1</script></svg>`)}
		}, []string{"img/x.png: content is text/plain"}},
		{"jpeg named png", func(f fstest.MapFS) { f["img/x.png"] = &fstest.MapFile{Data: []byte("\xff\xd8\xff\xe0")} }, []string{"content is image/jpeg, not image/png"}},
		{"other file", func(f fstest.MapFS) { f["notes.txt"] = &fstest.MapFile{Data: []byte("x")} }, []string{`notes.txt: unsupported file type ".txt"`}},
		{"page too big", func(f fstest.MapFS) {
			f["big.md"] = page("title: A\nkind: guide", strings.Repeat("x", MaxPageBytes))
		}, []string{"big.md: ", "exceed the per-file limit of 262144"}},
		{"image too big", func(f fstest.MapFS) {
			f["img/big.png"] = &fstest.MapFile{Data: []byte(pngData + strings.Repeat("\x00", MaxAssetBytes))}
		}, []string{"img/big.png: ", "per-file limit of 524288"}},
		{"total too big", func(f fstest.MapFS) {
			for i := range 9 {
				f["img/"+string(rune('a'+i))+".png"] = &fstest.MapFile{Data: []byte(pngData + strings.Repeat("\x00", MaxAssetBytes-100))}
			}
		}, []string{"exceed the limit of 4194304"}},
		{"too many pages", func(f fstest.MapFS) {
			for i := range MaxPages {
				f["p/"+itoa(i)+".md"] = page("title: P\nkind: reference", "x")
			}
		}, []string{"504 pages exceed the limit of 500"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := base()
			c.edit(f)
			_, err := Load(f, "ipam")
			ps := problems(t, err)
			all := ps.Error()
			for _, w := range c.wants {
				if !strings.Contains(all, w) {
					t.Errorf("missing %q in:\n%s", w, all)
				}
			}
		})
	}
}

func itoa(i int) string {
	const d = "0123456789"
	if i < 10 {
		return d[i : i+1]
	}
	return itoa(i/10) + d[i%10:i%10+1]
}

func TestAllProblemsReported(t *testing.T) {
	f := base()
	f["a.md"] = page("title: A\nkind: nope", "[x](missing.md)")
	f["b.md"] = page("kind: guide", "x")
	f["img/x.svg"] = &fstest.MapFile{Data: []byte("<svg/>")}
	ps := problems(t, Lint(f))
	if len(ps) < 4 {
		t.Fatalf("want every problem, got:\n%s", ps.Error())
	}
	for i := 1; i < len(ps); i++ {
		if ps[i-1].File > ps[i].File {
			t.Fatalf("not sorted:\n%s", ps.Error())
		}
	}
}

func TestLinksIgnoreCode(t *testing.T) {
	f := base()
	f["a.md"] = page("title: A\nkind: reference", "```\n[x](javascript:1)\n```\n\nInline `[y](nope.md)` too.\n\n~~~md\n![z](missing.png)\n~~~\n[ok](index.md) [also](https://x.org) [anchor](#top) [route](/ipam/subnets) [mail](mailto:a@b.c) [![logo](img/logo.png)](guides/setup.md)\n")
	if err := Lint(f); err != nil {
		t.Fatal(err)
	}
}

func TestImageTypes(t *testing.T) {
	ok := map[string]string{
		"a.png": pngData, "a.jpg": "\xff\xd8\xff\xe0", "a.JPEG": "\xff\xd8\xff\xe0", "a.gif": "GIF89a....",
		"a.webp": "RIFF\x00\x00\x00\x00WEBPVP8 ",
	}
	for name, data := range ok {
		if _, err := sniffImage(name, []byte(data)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, data := range map[string]string{"a.svg": "<svg/>", "a.bmp": "BM", "a.png": "GIF89a", "a.webp": pngData} {
		if _, err := sniffImage(name, []byte(data)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSlugOf(t *testing.T) {
	for in, want := range map[string]string{"index.md": "index", "guides/index.md": "guides", "guides/setup.md": "guides/setup", "a/b/index.md": "a/b"} {
		if got := slugOf(in); got != want {
			t.Errorf("slugOf(%s) = %s, want %s", in, got, want)
		}
	}
}
