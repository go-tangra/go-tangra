package helpdocs

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ref is a link or image target found in a page's Markdown.
type ref struct {
	target string
	line   int
	image  bool
}

var (
	// [text](target) not preceded by "!"; text may hold one level of brackets
	// (so [![alt](img.png)](page.md) is the outer link).
	linkRE  = regexp.MustCompile(`(^|[^!\\])\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^\s)]*)`)
	imageRE = regexp.MustCompile(`!\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^\s)]*)`)
	// [label]: target
	refDefRE = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)`)
	// <scheme:...>
	autoRE = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9+.-]{1,31}:[^\s<>]*)>`)
	// raw HTML href="..." / src="..."
	attrRE   = regexp.MustCompile(`(?i)\b(href|src)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	codeRE   = regexp.MustCompile("``.*?``|`[^`]*`")
	schemeRE = regexp.MustCompile(`^([a-z][a-z0-9+.-]*):`)
)

// scanRefs returns the link and image targets of body (whose first line is
// file line first), skipping fenced code blocks and inline code spans.
func scanRefs(body string, first int) []ref {
	var out []ref
	fence := ""
	for i, line := range strings.Split(body, "\n") {
		n := first + i
		t := strings.TrimLeft(line, " ")
		if len(line)-len(t) <= 3 {
			if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
				fence = t[:3]
				continue
			}
			if fence != "" && strings.HasPrefix(t, fence) {
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		line = codeRE.ReplaceAllString(line, "")
		for _, m := range imageRE.FindAllStringSubmatch(line, -1) {
			out = append(out, ref{target: unangle(m[1]), line: n, image: true})
		}
		for _, m := range linkRE.FindAllStringSubmatch(line, -1) {
			out = append(out, ref{target: unangle(m[2]), line: n})
		}
		if m := refDefRE.FindStringSubmatch(line); m != nil {
			out = append(out, ref{target: unangle(m[1]), line: n})
		}
		for _, m := range autoRE.FindAllStringSubmatch(line, -1) {
			out = append(out, ref{target: m[1], line: n})
		}
		for _, m := range attrRE.FindAllStringSubmatch(line, -1) {
			v := strings.Trim(m[2], `"'`)
			out = append(out, ref{target: v, line: n, image: strings.EqualFold(m[1], "src")})
		}
	}
	return out
}

func unangle(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">")
}

// schemeOf returns the lower-cased URL scheme of target, ignoring the ASCII
// whitespace and control characters browsers strip ("java\tscript:").
func schemeOf(target string) string {
	var b strings.Builder
	for _, r := range target {
		if r <= ' ' || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	m := schemeRE.FindStringSubmatch(strings.ToLower(b.String()))
	if m == nil {
		return ""
	}
	return m[1]
}

// resolve checks one reference of page file and returns a problem message,
// or "" when the reference is fine.
func (l *loader) resolve(file string, r ref) string {
	t := strings.TrimSpace(r.target)
	what := "link"
	if r.image {
		what = "image"
	}
	if t == "" {
		return "empty " + what + " target"
	}
	if s := schemeOf(t); s != "" {
		if r.image {
			return "image " + t + ": images must be bundled under docs/help (relative path)"
		}
		switch s {
		case "http", "https", "mailto":
			return ""
		}
		return "link " + t + ": scheme " + s + ": is not allowed (http, https, mailto or a relative path)"
	}
	if strings.HasPrefix(t, "//") || strings.HasPrefix(t, `\\`) {
		return what + " " + t + ": protocol-relative URLs are not allowed"
	}
	if strings.HasPrefix(t, "/") {
		if r.image {
			return "image " + t + ": images must be referenced by a relative path"
		}
		return "" // a portal route, e.g. /ipam/subnets
	}
	p := t
	if i := strings.IndexAny(p, "#?"); i >= 0 {
		p = p[:i]
	}
	if p == "" {
		if r.image {
			return "image " + t + ": not a file"
		}
		return "" // same-page anchor
	}
	p, err := url.PathUnescape(p)
	if err != nil {
		return what + " " + t + ": bad escape"
	}
	resolved := path.Join(path.Dir(file), p)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return what + " " + t + ": points outside docs/help"
	}
	if r.image {
		if _, ok := l.assets[resolved]; ok {
			return ""
		}
		return "image " + t + ": " + resolved + " not found"
	}
	if l.files[resolved] {
		return ""
	}
	if _, ok := l.assets[resolved]; ok {
		return ""
	}
	return "link " + t + ": " + resolved + " not found (link to pages by their .md file)"
}
