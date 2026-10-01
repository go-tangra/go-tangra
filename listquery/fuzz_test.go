package listquery

import (
	"errors"
	"net/url"
	"testing"
)

// FuzzParse checks that no input can make OrderBy emit anything but the
// Spec's constants, and that errors only ever name a known parameter.
func FuzzParse(f *testing.F) {
	allowed := map[string]bool{}
	for name := range hostSpec.Fields {
		for _, d := range []Dir{Asc, Desc} {
			allowed[Request{Sort: name, Order: d}.OrderBy(hostSpec)] = true
		}
	}
	params := map[string]bool{"page": true, "page_size": true, "sort": true, "order": true, "cursor": true}
	for _, seed := range [][4]string{
		{"1", "25", "hostname", "asc"},
		{"0", "201", "bogus", "up"},
		{"", "", "hostname;drop table h", "desc, (select 1)"},
		{"99999999999", "-5", "last_seen", "DESC"},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3], "")
	}
	f.Fuzz(func(t *testing.T, page, size, sort, order, cursor string) {
		q := url.Values{}
		for k, v := range map[string]string{"page": page, "page_size": size, "sort": sort, "order": order, "cursor": cursor} {
			if v != "" {
				q.Set(k, v)
			}
		}
		r, err := Parse(q, hostSpec)
		if err != nil {
			var e *Error
			if !errors.As(err, &e) || !params[e.Param] {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		if ob := r.OrderBy(hostSpec); !allowed[ob] {
			t.Fatalf("OrderBy produced %q", ob)
		}
		if r.Page < 1 || r.PageSize < 1 || r.PageSize > MaxPageSize || r.Offset() < 0 {
			t.Fatalf("out of bounds %+v", r)
		}
	})
}
