package listquery

import "testing"

func TestOrderBy(t *testing.T) {
	cases := []struct {
		r    Request
		want string
	}{
		{Request{1, 25, "hostname", Asc}, "lower(h.hostname) ASC NULLS LAST, h.id ASC"},
		{Request{1, 25, "hostname", Desc}, "lower(h.hostname) DESC NULLS LAST, h.id DESC"},
		{Request{1, 25, "last_seen", Desc}, "h.last_seen DESC NULLS LAST, h.id DESC"},
		{Request{1, 25, "size", Asc}, "h.size ASC NULLS LAST, h.id ASC"},
		// A hand-built request with an unknown field or direction falls back
		// to the default field and ascending order — never to request text.
		{Request{1, 25, "x; drop", "sideways"}, "lower(h.hostname) ASC NULLS LAST, h.id ASC"},
	}
	for _, c := range cases {
		if got := c.r.OrderBy(hostSpec); got != c.want {
			t.Fatalf("%+v: got %q, want %q", c.r, got, c.want)
		}
	}
}

func TestLimitOffset(t *testing.T) {
	r := Request{Page: 3, PageSize: 50}
	if r.Limit() != 50 || r.Offset() != 100 {
		t.Fatalf("limit %d offset %d", r.Limit(), r.Offset())
	}
	if (Request{Page: 0, PageSize: 10}).Offset() != 0 {
		t.Fatal("page 0 must not produce a negative offset")
	}
}

func TestClamp(t *testing.T) {
	cases := []struct {
		page, size, total, want int
	}{
		{1, 50, 1234, 1},
		{25, 50, 1234, 25},
		{26, 50, 1234, 25},
		{999, 50, 1234, 25},
		{3, 50, 100, 2}, // exact multiple
		{2, 50, 100, 2},
		{5, 25, 0, 1}, // empty list → page 1
		{4, 0, 10, 4}, // nonsensical size leaves the page alone
	}
	for _, c := range cases {
		got := Request{Page: c.page, PageSize: c.size}.Clamp(c.total)
		if got.Page != c.want {
			t.Fatalf("%+v: page %d, want %d", c, got.Page, c.want)
		}
	}
}
