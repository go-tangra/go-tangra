package helpdocs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// Problem is one validation finding. Line is 1-based; 0 means the problem
// concerns the whole file (or, with File "", the whole tree).
type Problem struct {
	File    string
	Line    int
	Message string
}

// String formats the problem as "file:line: message".
func (p Problem) String() string {
	switch {
	case p.File == "":
		return p.Message
	case p.Line > 0:
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Message)
	default:
		return p.File + ": " + p.Message
	}
}

// Problems is every finding of a Load or Lint, sorted by file and line. It
// is returned as the error; use errors.As to get the list.
type Problems []Problem

func (ps Problems) Error() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

var (
	segmentRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	assetSegRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)
)

// imageTypes maps the allowed image extensions to the sniffed MIME type the
// content must have. SVG is deliberately absent: it can carry script.
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

func validSlug(s string) bool {
	if s == "" || len(s) > 255 {
		return false
	}
	for seg := range strings.SplitSeq(s, "/") {
		if !segmentRE.MatchString(seg) {
			return false
		}
	}
	return true
}

func validAssetPath(p string) bool {
	if p == "" || len(p) > 255 {
		return false
	}
	for seg := range strings.SplitSeq(p, "/") {
		if !assetSegRE.MatchString(seg) || strings.Contains(seg, "..") {
			return false
		}
	}
	return true
}

// slugOf maps a page file (relative, forward slashes, ending in .md) to its
// slug: "index.md" -> "index", "guides/index.md" -> "guides",
// "guides/setup.md" -> "guides/setup".
func slugOf(file string) string {
	s := strings.TrimSuffix(file, ".md")
	if s != "index" {
		s = strings.TrimSuffix(s, "/index")
	}
	return s
}

// imageType returns the MIME type an image file must have, by extension.
func imageType(name string) (string, error) {
	ext := strings.ToLower(path.Ext(name))
	if ext == ".svg" || ext == ".svgz" {
		return "", errors.New("SVG images are not allowed (they can carry script); use png, jpg, webp or gif")
	}
	if want, ok := imageTypes[ext]; ok {
		return want, nil
	}
	return "", fmt.Errorf("unsupported file type %q (pages are .md; images png, jpg, webp or gif)", ext)
}

// sniffImage returns the MIME type of an allowed image, judged by its
// extension and confirmed by content sniffing.
func sniffImage(name string, data []byte) (string, error) {
	want, err := imageType(name)
	if err != nil {
		return "", err
	}
	if got := http.DetectContentType(data); got != want {
		return "", fmt.Errorf("content is %s, not %s", got, want)
	}
	return want, nil
}

// Load reads a docs tree (the contents of docs/help: fsys is rooted there),
// validates it and returns the bundle for module. A nil fsys, a missing root
// or a tree with no files yields an empty bundle with digest "". On any
// problem the error is Problems.
func Load(fsys fs.FS, module string) (*Bundle, error) {
	b, ps := load(fsys, module, true)
	if len(ps) > 0 {
		return nil, ps
	}
	return b, nil
}

// Lint validates a docs tree like Load, without a module name. It returns
// nil or Problems.
func Lint(fsys fs.FS) error {
	if _, ps := load(fsys, "", false); len(ps) > 0 {
		return ps
	}
	return nil
}

type body struct {
	text string
	line int // 1-based file line of the first Markdown line
}

type loader struct {
	fsys   fs.FS
	ps     Problems
	pages  map[string]*Page // valid pages, by file
	files  map[string]bool  // every page file with a valid slug (link targets)
	assets map[string]*Asset
	bodies map[string]body // every page's Markdown, for link checks
	total  int64
}

func (l *loader) add(file string, line int, format string, a ...any) {
	l.ps = append(l.ps, Problem{File: file, Line: line, Message: fmt.Sprintf(format, a...)})
}

func load(fsys fs.FS, module string, checkModule bool) (*Bundle, Problems) {
	empty := &Bundle{Schema: Schema, Module: module, Pages: []Page{}, Assets: []Asset{}}
	if checkModule && !identity.ValidServiceName(module) {
		return nil, Problems{{Message: fmt.Sprintf("module name %q is not a valid service name", module)}}
	}
	if fsys == nil {
		return empty, nil
	}
	l := &loader{fsys: fsys, pages: map[string]*Page{}, files: map[string]bool{}, assets: map[string]*Asset{}, bodies: map[string]body{}}
	files := 0
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == "." && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			l.add(p, 0, "%v", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		files++
		if !d.Type().IsRegular() {
			l.add(p, 0, "not a regular file (symlinks are not allowed)")
			return nil
		}
		l.file(p, d)
		return nil
	})
	if err != nil {
		l.add("", 0, "walk: %v", err)
	}
	if files == 0 && len(l.ps) == 0 {
		return empty, nil
	}
	if !l.files["index.md"] {
		l.add("index.md", 0, "a root index.md is required")
	}
	l.checkSlugs()
	l.checkLinks()
	if len(l.files) > MaxPages {
		l.add("", 0, "%d pages exceed the limit of %d", len(l.files), MaxPages)
	}
	if l.total > MaxTotalBytes {
		l.add("", 0, "%d bytes of pages and images exceed the limit of %d", l.total, MaxTotalBytes)
	}
	if len(l.ps) > 0 {
		sort.SliceStable(l.ps, func(i, j int) bool {
			if l.ps[i].File != l.ps[j].File {
				return l.ps[i].File < l.ps[j].File
			}
			return l.ps[i].Line < l.ps[j].Line
		})
		return nil, l.ps
	}
	b := &Bundle{Schema: Schema, Module: module, Pages: make([]Page, 0, len(l.pages)), Assets: make([]Asset, 0, len(l.assets))}
	for _, p := range l.pages {
		b.Pages = append(b.Pages, *p)
	}
	for _, a := range l.assets {
		b.Assets = append(b.Assets, *a)
	}
	sort.Slice(b.Pages, func(i, j int) bool { return b.Pages[i].Slug < b.Pages[j].Slug })
	sort.Slice(b.Assets, func(i, j int) bool { return b.Assets[i].Path < b.Assets[j].Path })
	b.Digest = ComputeDigest(b)
	return b, nil
}

// file reads and validates one file of the tree.
func (l *loader) file(p string, d fs.DirEntry) {
	info, err := d.Info()
	if err != nil {
		l.add(p, 0, "%v", err)
		return
	}
	isPage := strings.HasSuffix(p, ".md")
	limit := int64(MaxAssetBytes)
	if isPage {
		limit = MaxPageBytes
	}
	if info.Size() > limit {
		l.add(p, 0, "%d bytes exceed the per-file limit of %d", info.Size(), limit)
		return
	}
	if !isPage {
		if _, err := imageType(p); err != nil {
			l.add(p, 0, "%v", err) // refuse by extension before reading
			return
		}
	}
	data, err := fs.ReadFile(l.fsys, p)
	if err != nil {
		l.add(p, 0, "%v", err)
		return
	}
	if int64(len(data)) > limit {
		l.add(p, 0, "%d bytes exceed the per-file limit of %d", len(data), limit)
		return
	}
	l.total += int64(len(data))
	if isPage {
		l.page(p, data)
		return
	}
	if !validAssetPath(p) {
		l.add(p, 0, "image path segments must be lowercase [a-z0-9_.-]")
		return
	}
	mime, err := sniffImage(p, data)
	if err != nil {
		l.add(p, 0, "%v", err)
		return
	}
	sum := sha256.Sum256(data)
	l.assets[p] = &Asset{Path: p, MIME: mime, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Data: data}
}

func (l *loader) page(p string, data []byte) {
	slug := slugOf(p)
	if !validSlug(slug) {
		l.add(p, 0, "page path segments must be lowercase [a-z0-9-] (slug %q)", slug)
		return
	}
	l.files[p] = true
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	pg, bodyLine, ps := parsePage(p, src)
	l.ps = append(l.ps, ps...)
	if pg == nil {
		return
	}
	pg.Slug = slug
	l.bodies[p] = body{pg.Markdown, bodyLine}
	if len(ps) == 0 {
		l.pages[p] = pg
	}
}

// checkSlugs reports pages that map to the same slug ("a.md" and "a/index.md").
func (l *loader) checkSlugs() {
	by := map[string][]string{}
	for f := range l.files {
		by[slugOf(f)] = append(by[slugOf(f)], f)
	}
	for slug, files := range by {
		if len(files) > 1 {
			sort.Strings(files)
			for _, f := range files[1:] {
				l.add(f, 0, "duplicate slug %q (also %s)", slug, files[0])
			}
		}
	}
}

func (l *loader) checkLinks() {
	for f, b := range l.bodies {
		for _, r := range scanRefs(b.text, b.line) {
			if msg := l.resolve(f, r); msg != "" {
				l.add(f, r.line, "%s", msg)
			}
		}
	}
}
