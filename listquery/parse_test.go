package listquery

import (
	"errors"
	"net/url"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	r, err := Parse(url.Values{}, hostSpec)
	if err != nil {
		t.Fatal(err)
	}
	want := Request{Page: 1, PageSize: DefaultPageSize, Sort: "hostname", Order: Asc}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}
}

func TestParseValid(t *testing.T) {
	cases := []struct {
		q    url.Values
		want Request
	}{
		{values("page", "3", "page_size", "50", "sort", "last_seen", "order", "asc"), Request{3, 50, "last_seen", Asc}},
		{values("sort", "last_seen"), Request{1, 25, "last_seen", Desc}}, // field default direction
		{values("sort", "size"), Request{1, 25, "size", Asc}},
		{values("sort", "hostname", "order", "desc"), Request{1, 25, "hostname", Desc}},
		{values("page_size", "200"), Request{1, 200, "hostname", Asc}},
		{values("page_size", "1", "page", "2147483647"), Request{2147483647, 1, "hostname", Asc}},
		{values("order", "desc"), Request{1, 25, "hostname", Desc}}, // direction for the default field
	}
	for _, c := range cases {
		got, err := Parse(c.q, hostSpec)
		if err != nil {
			t.Fatalf("%v: %v", c.q, err)
		}
		if got != c.want {
			t.Fatalf("%v: got %+v, want %+v", c.q, got, c.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	small := hostSpec
	small.MaxSize = 50
	cases := []struct {
		q     url.Values
		spec  Spec
		param string
	}{
		{values("page", "0"), hostSpec, "page"},
		{values("page", "-1"), hostSpec, "page"},
		{values("page", "abc"), hostSpec, "page"},
		{values("page", ""), hostSpec, "page"},
		{values("page", "2147483648"), hostSpec, "page"},
		{values("page_size", "0"), hostSpec, "page_size"},
		{values("page_size", "201"), hostSpec, "page_size"},
		{values("page_size", "abc"), hostSpec, "page_size"},
		{values("page_size", "51"), small, "page_size"},
		{values("sort", "bogus"), hostSpec, "sort"},
		{values("sort", ""), hostSpec, "sort"},
		{values("sort", "hostname;drop table x"), hostSpec, "sort"},
		{values("order", "up"), hostSpec, "order"},
		{values("order", "DESC"), hostSpec, "order"},
		{values("cursor", "x", "page", "2"), hostSpec, "cursor"},
		{values("limit", "10", "sort", "hostname"), hostSpec, "cursor"},
	}
	for _, c := range cases {
		_, err := Parse(c.q, c.spec)
		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("%v: want *Error, got %v", c.q, err)
		}
		if e.Param != c.param {
			t.Fatalf("%v: param %q, want %q", c.q, e.Param, c.param)
		}
		if e.Error() != "listquery: invalid "+c.param {
			t.Fatalf("%v: message %q", c.q, e.Error())
		}
	}
}

func TestLegacy(t *testing.T) {
	cases := []struct {
		q    url.Values
		want bool
	}{
		{url.Values{}, false},
		{values("cursor", "abc"), true},
		{values("limit", "10"), true},
		{values("cursor", "abc", "limit", "10", "status", "active"), true},
		{values("cursor", "abc", "page", "1"), false},
		{values("limit", "10", "order", "asc"), false},
		{values("page", "1"), false},
	}
	for _, c := range cases {
		if got := Legacy(c.q); got != c.want {
			t.Fatalf("%v: got %v", c.q, got)
		}
	}
}

func TestNew(t *testing.T) {
	r, err := New(0, 0, "", "", hostSpec)
	if err != nil || r != (Request{1, 25, "hostname", Asc}) {
		t.Fatalf("zero values: %+v %v", r, err)
	}
	r, err = New(4, 10, "last_seen", "", hostSpec)
	if err != nil || r != (Request{4, 10, "last_seen", Desc}) {
		t.Fatalf("explicit: %+v %v", r, err)
	}
	for _, c := range []struct {
		page, size  int
		sort, param string
		order       Dir
	}{
		{-1, 0, "", "page", ""},
		{0, -1, "", "page_size", ""},
		{0, 201, "", "page_size", ""},
		{0, 0, "nope", "sort", ""},
		{0, 0, "", "order", "sideways"},
	} {
		_, err := New(c.page, c.size, c.sort, c.order, hostSpec)
		var e *Error
		if !errors.As(err, &e) || e.Param != c.param {
			t.Fatalf("%+v: got %v, want param %s", c, err, c.param)
		}
	}
}
