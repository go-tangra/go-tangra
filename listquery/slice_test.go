package listquery

import (
	"fmt"
	"testing"
	"time"
)

type row struct {
	id    string
	name  any
	n     any
	when  any
	ratio any
	other any
}

func rowKey(r *row, field string) any {
	switch field {
	case "name":
		return r.name
	case "n":
		return r.n
	case "when":
		return r.when
	case "ratio":
		return r.ratio
	default:
		return r.other
	}
}

func rowTie(r *row) string { return r.id }

func ids(rs []*row) string {
	s := ""
	for _, r := range rs {
		s += r.id
	}
	return s
}

func TestSortSliceStrings(t *testing.T) {
	rs := []*row{{id: "a", name: "beta"}, {id: "b", name: "Alpha"}, {id: "c", name: nil}, {id: "d", name: "alpha"}}
	SortSlice(rs, Request{Sort: "name", Order: Asc}, rowKey, rowTie)
	if got := ids(rs); got != "bdac" { // case-insensitive, tie by id, nil last
		t.Fatalf("asc %s", got)
	}
	SortSlice(rs, Request{Sort: "name", Order: Desc}, rowKey, rowTie)
	if got := ids(rs); got != "adbc" { // nil still last
		t.Fatalf("desc %s", got)
	}
}

func TestSortSliceNumbersAndTimes(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rs := []*row{
		{id: "a", n: int64(3), when: t0.Add(time.Hour), ratio: 0.5},
		{id: "b", n: int64(1), when: t0, ratio: 1.5},
		{id: "c", n: nil, when: nil, ratio: nil},
		{id: "d", n: int64(1), when: t0.Add(2 * time.Hour), ratio: 0.25},
	}
	SortSlice(rs, Request{Sort: "n", Order: Asc}, rowKey, rowTie)
	if got := ids(rs); got != "bdac" {
		t.Fatalf("int64 %s", got)
	}
	SortSlice(rs, Request{Sort: "when", Order: Desc}, rowKey, rowTie)
	if got := ids(rs); got != "dabc" {
		t.Fatalf("time %s", got)
	}
	SortSlice(rs, Request{Sort: "ratio", Order: Asc}, rowKey, rowTie)
	if got := ids(rs); got != "dabc" {
		t.Fatalf("float %s", got)
	}
}

func TestSortSliceIntsAndMixed(t *testing.T) {
	rs := []*row{{id: "a", other: 2}, {id: "b", other: 1}, {id: "c", other: 2}}
	SortSlice(rs, Request{Sort: "other", Order: Desc}, rowKey, rowTie)
	if got := ids(rs); got != "cab" { // desc: equal values tie-break descending too
		t.Fatalf("int %s", got)
	}
	// Mixed or unsupported types compare by their text form.
	mixed := []*row{{id: "a", other: true}, {id: "b", other: "abc"}, {id: "c", other: 7}}
	SortSlice(mixed, Request{Sort: "other", Order: Asc}, rowKey, rowTie)
	if got := ids(mixed); got != "cba" { // "7" < "abc" < "true"
		t.Fatalf("mixed %s", got)
	}
}

func TestWindow(t *testing.T) {
	var rs []int
	for i := 1; i <= 1234; i++ {
		rs = append(rs, i)
	}
	page, total, applied := Window(rs, Request{Page: 999, PageSize: 50})
	if total != 1234 || applied.Page != 25 || len(page) != 34 || page[0] != 1201 {
		t.Fatalf("last page: total %d page %d len %d first %d", total, applied.Page, len(page), page[0])
	}
	// Every element exactly once across all pages at several sizes.
	for _, size := range []int{1, 7, 25, 200} {
		seen := map[int]int{}
		for p := 1; ; p++ {
			pg, _, a := Window(rs, Request{Page: p, PageSize: size})
			if a.Page != p {
				break
			}
			for _, v := range pg {
				seen[v]++
			}
		}
		for _, v := range rs {
			if seen[v] != 1 {
				t.Fatalf("size %d: %d seen %d times", size, v, seen[v])
			}
		}
	}
	empty, total, applied := Window([]int(nil), Request{Page: 3, PageSize: 10})
	if len(empty) != 0 || total != 0 || applied.Page != 1 {
		t.Fatalf("empty: %v %d %d", empty, total, applied.Page)
	}
}

func ExampleWindow() {
	page, total, applied := Window([]string{"a", "b", "c", "d", "e"}, Request{Page: 2, PageSize: 2})
	fmt.Println(page, total, applied.Page)
	// Output: [c d] 5 2
}
