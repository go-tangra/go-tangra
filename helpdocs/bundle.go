package helpdocs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"sort"
	"strconv"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// Schema is the bundle wire format version.
const Schema = 1

// DefaultDir is where a module repository keeps its help pages.
const DefaultDir = "docs/help"

// Limits.
const (
	MaxPages      = 500
	MaxPageBytes  = 256 << 10
	MaxAssetBytes = 512 << 10
	MaxTotalBytes = 4 << 20
)

// Kind classifies a page.
type Kind string

// Kinds.
const (
	KindGuide     Kind = "guide"
	KindExample   Kind = "example"
	KindReference Kind = "reference"
	KindFAQ       Kind = "faq"
)

// Audience says who a page is written for.
type Audience string

// Audiences.
const (
	AudienceUser     Audience = "user"
	AudienceOperator Audience = "operator"
)

// Page is one help page. Markdown is the page source without its
// front-matter; the gateway renders and sanitizes it.
type Page struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary,omitempty"`
	Kind     Kind     `json:"kind"`
	Order    int      `json:"order"`
	Tags     []string `json:"tags,omitempty"`
	Requires string   `json:"requires,omitempty"`
	Audience Audience `json:"audience"`
	Lang     string   `json:"lang"`
	Markdown string   `json:"markdown"`
}

// Asset is an image referenced by pages. Path is relative to the docs root
// (forward slashes); Data is base64 on the wire.
type Asset struct {
	Path   string `json:"path"`
	MIME   string `json:"mime"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Data   []byte `json:"data"`
}

// Bundle is a module's complete help content, as served to the gateway.
// Pages are sorted by slug and assets by path.
type Bundle struct {
	Schema int     `json:"schema"`
	Module string  `json:"module"`
	Digest string  `json:"digest"`
	Pages  []Page  `json:"pages"`
	Assets []Asset `json:"assets"`
}

// Empty reports whether the bundle has no pages (a module without docs).
func (b *Bundle) Empty() bool { return b == nil || len(b.Pages) == 0 }

// Marshal encodes the bundle as JSON.
func (b *Bundle) Marshal() ([]byte, error) { return json.Marshal(b) }

// ComputeDigest returns the sha256 (hex) of the bundle's canonical encoding:
// schema, module, then the pages sorted by slug and the assets sorted by
// path, every field length-prefixed in a fixed order (asset bytes enter via
// their sha256). It does not depend on the input order of Pages/Assets, on
// how the tree was walked or on JSON formatting. An empty bundle's digest is
// "".
func ComputeDigest(b *Bundle) string {
	if b.Empty() {
		return ""
	}
	pages := append([]Page(nil), b.Pages...)
	sort.Slice(pages, func(i, j int) bool { return pages[i].Slug < pages[j].Slug })
	assets := append([]Asset(nil), b.Assets...)
	sort.Slice(assets, func(i, j int) bool { return assets[i].Path < assets[j].Path })

	h := sha256.New()
	w := canon{h}
	w.str("tangra-helpdocs")
	w.str(strconv.Itoa(b.Schema))
	w.str(b.Module)
	w.num(len(pages))
	for i := range pages {
		p := &pages[i]
		w.str(p.Slug)
		w.str(p.Title)
		w.str(p.Summary)
		w.str(string(p.Kind))
		w.str(strconv.Itoa(p.Order))
		w.num(len(p.Tags))
		for _, t := range p.Tags {
			w.str(t)
		}
		w.str(p.Requires)
		w.str(string(p.Audience))
		w.str(p.Lang)
		w.str(p.Markdown)
	}
	w.num(len(assets))
	for i := range assets {
		a := &assets[i]
		sum := sha256.Sum256(a.Data)
		w.str(a.Path)
		w.str(a.MIME)
		w.str(hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// canon writes length-prefixed fields, so no two field sequences collide.
type canon struct{ h hash.Hash }

func (c canon) num(n int) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n)) // #nosec G115 -- lengths/counts are non-negative
	c.h.Write(b[:])
}

func (c canon) str(s string) {
	c.num(len(s))
	c.h.Write([]byte(s))
}

// ErrInvalidBundle wraps every refusal of a received bundle.
var ErrInvalidBundle = errors.New("helpdocs: invalid bundle")

// Decode parses a bundle received over the wire and checks it: schema,
// limits, asset sizes and hashes, sniffed image types, and that Digest is
// the digest of the content. It is what the gateway uses on a fetched
// bundle; it does not re-run the authoring lint (links, front-matter).
func Decode(data []byte) (*Bundle, error) {
	if len(data) > 2*MaxTotalBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrInvalidBundle, len(data))
	}
	var b Bundle
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBundle, err)
	}
	if b.Schema != Schema {
		return nil, fmt.Errorf("%w: schema %d (want %d)", ErrInvalidBundle, b.Schema, Schema)
	}
	if len(b.Pages) > MaxPages {
		return nil, fmt.Errorf("%w: %d pages (max %d)", ErrInvalidBundle, len(b.Pages), MaxPages)
	}
	if !identity.ValidServiceName(b.Module) {
		return nil, fmt.Errorf("%w: module %q", ErrInvalidBundle, b.Module)
	}
	var total int64
	slugs := make(map[string]bool, len(b.Pages))
	for i := range b.Pages {
		p := &b.Pages[i]
		if !validSlug(p.Slug) || slugs[p.Slug] || p.Title == "" || !validKind(p.Kind) {
			return nil, fmt.Errorf("%w: page %d (%q) is malformed or duplicate", ErrInvalidBundle, i, p.Slug)
		}
		slugs[p.Slug] = true
		if len(p.Markdown) > MaxPageBytes {
			return nil, fmt.Errorf("%w: page %s exceeds %d bytes", ErrInvalidBundle, p.Slug, MaxPageBytes)
		}
		total += int64(len(p.Markdown))
	}
	for i := range b.Assets {
		a := &b.Assets[i]
		if !validAssetPath(a.Path) || int64(len(a.Data)) != a.Size || a.Size > MaxAssetBytes {
			return nil, fmt.Errorf("%w: asset %q is malformed", ErrInvalidBundle, a.Path)
		}
		sum := sha256.Sum256(a.Data)
		if hex.EncodeToString(sum[:]) != a.SHA256 {
			return nil, fmt.Errorf("%w: asset %s: sha256 mismatch", ErrInvalidBundle, a.Path)
		}
		if mime, err := sniffImage(a.Path, a.Data); err != nil || mime != a.MIME {
			return nil, fmt.Errorf("%w: asset %s: not an allowed image", ErrInvalidBundle, a.Path)
		}
		total += a.Size
	}
	if total > MaxTotalBytes {
		return nil, fmt.Errorf("%w: %d bytes of content (max %d)", ErrInvalidBundle, total, MaxTotalBytes)
	}
	if got := ComputeDigest(&b); got != b.Digest {
		return nil, fmt.Errorf("%w: digest mismatch", ErrInvalidBundle)
	}
	return &b, nil
}
