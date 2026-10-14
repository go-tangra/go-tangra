package helpdocs

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxTitleRunes   = 200
	maxSummaryRunes = 500
	maxTags         = 20
)

var (
	tagRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	// permRE is the gateway manifest's bare "resource:action" grammar.
	permRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}:[a-z0-9][a-z0-9_.-]{0,63}$`)
)

func validKind(k Kind) bool {
	switch k {
	case KindGuide, KindExample, KindReference, KindFAQ:
		return true
	}
	return false
}

// parsePage splits src into front-matter and Markdown and validates the
// front-matter. It returns the page (nil only when there is no
// front-matter to split off; with problems its fields may be partial), the
// 1-based line where the Markdown starts, and the problems.
func parsePage(file, src string) (*Page, int, Problems) {
	var ps Problems
	add := func(line int, format string, a ...any) {
		ps = append(ps, Problem{File: file, Line: line, Message: fmt.Sprintf(format, a...)})
	}
	if !utf8.ValidString(src) {
		add(0, "not valid UTF-8")
		return nil, 0, ps
	}
	lines := strings.SplitAfter(src, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\n") != "---" {
		add(1, "a page must start with YAML front-matter (a line \"---\")")
		return nil, 0, ps
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		add(1, "front-matter is not closed (no second \"---\" line)")
		return nil, 0, ps
	}
	yamlSrc := strings.Join(lines[1:end], "")
	body := strings.Join(lines[end+1:], "")
	bodyLine := end + 2 // 1-based line of lines[end+1]

	pg := &Page{Audience: AudienceUser, Lang: "en", Markdown: body}
	keyLine := map[string]int{}
	fmLine := func(yl int) int { return yl + 1 } // YAML line 1 is file line 2

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlSrc), &doc); err != nil {
		add(2, "front-matter: %s", strings.TrimPrefix(err.Error(), "yaml: "))
		return pg, bodyLine, ps
	}
	if len(doc.Content) > 0 {
		m := doc.Content[0]
		if m.Kind != yaml.MappingNode {
			add(fmLine(m.Line), "front-matter must be a mapping of fields")
			return pg, bodyLine, ps
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			line := fmLine(k.Line)
			keyLine[k.Value] = line
			var err error
			switch k.Value {
			case "title":
				err = v.Decode(&pg.Title)
			case "summary":
				err = v.Decode(&pg.Summary)
			case "kind":
				err = v.Decode(&pg.Kind)
			case "order":
				err = v.Decode(&pg.Order)
			case "tags":
				err = v.Decode(&pg.Tags)
			case "requires":
				err = v.Decode(&pg.Requires)
			case "audience":
				err = v.Decode(&pg.Audience)
			case "lang":
				err = v.Decode(&pg.Lang)
			default:
				add(line, "unknown front-matter field %q (title, summary, kind, order, tags, requires, audience, lang)", k.Value)
				continue
			}
			if err != nil || v.Kind == yaml.MappingNode || (k.Value != "tags" && v.Kind == yaml.SequenceNode) {
				add(line, "front-matter: %s has the wrong type", k.Value)
			}
		}
	}
	at := func(key string) int {
		if l, ok := keyLine[key]; ok {
			return l
		}
		return 1
	}

	pg.Title = strings.TrimSpace(pg.Title)
	switch {
	case pg.Title == "":
		add(at("title"), "title is required")
	case utf8.RuneCountInString(pg.Title) > maxTitleRunes || strings.ContainsAny(pg.Title, "\r\n"):
		add(at("title"), "title must be one line of at most %d characters", maxTitleRunes)
	}
	pg.Summary = strings.TrimSpace(pg.Summary)
	if utf8.RuneCountInString(pg.Summary) > maxSummaryRunes {
		add(at("summary"), "summary exceeds %d characters", maxSummaryRunes)
	}
	switch {
	case pg.Kind == "":
		add(at("kind"), "kind is required (guide, example, reference, faq)")
	case !validKind(pg.Kind):
		add(at("kind"), "kind %q is not one of guide, example, reference, faq", pg.Kind)
	}
	if len(pg.Tags) > maxTags {
		add(at("tags"), "at most %d tags", maxTags)
	}
	seen := map[string]bool{}
	for _, t := range pg.Tags {
		if !tagRE.MatchString(t) {
			add(at("tags"), "tag %q must be lowercase [a-z0-9-], at most 32 characters", t)
		}
		if seen[t] {
			add(at("tags"), "duplicate tag %q", t)
		}
		seen[t] = true
	}
	if pg.Requires != "" && !permRE.MatchString(pg.Requires) {
		add(at("requires"), "requires %q is not a resource:action permission", pg.Requires)
	}
	if pg.Audience != AudienceUser && pg.Audience != AudienceOperator {
		add(at("audience"), "audience %q is not one of user, operator", pg.Audience)
	}
	if pg.Lang != "en" {
		add(at("lang"), "lang %q is not supported (only en)", pg.Lang)
	}
	if strings.TrimSpace(body) == "" {
		add(bodyLine, "page has no content after the front-matter")
	}
	return pg, bodyLine, ps
}
