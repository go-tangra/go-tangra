package listquery

import (
	"math"
	"net/url"
	"strconv"
)

// Request is a validated page request.
type Request struct {
	Page     int
	PageSize int
	Sort     string
	Order    Dir
}

// Error names the request parameter that failed validation. It never carries
// the submitted value.
type Error struct{ Param string }

func (e *Error) Error() string { return "listquery: invalid " + e.Param }

// Parse reads page, page_size, sort and order from q against s. Absent values
// take the Spec's defaults; any present but invalid value returns an *Error.
// Mixing the legacy cursor/limit parameters with the page parameters is an
// error on "cursor".
func Parse(q url.Values, s Spec) (Request, error) {
	if hasLegacy(q) && hasPage(q) {
		return Request{}, &Error{Param: "cursor"}
	}
	page, size := 0, 0
	if q.Has("page") {
		n, err := strconv.Atoi(q.Get("page"))
		if err != nil || n < 1 || n > math.MaxInt32 {
			return Request{}, &Error{Param: "page"}
		}
		page = n
	}
	if q.Has("page_size") {
		n, err := strconv.Atoi(q.Get("page_size"))
		if err != nil || n < 1 {
			return Request{}, &Error{Param: "page_size"}
		}
		size = n
	}
	sort := s.Default
	if q.Has("sort") {
		sort = q.Get("sort")
		if _, ok := s.Fields[sort]; !ok {
			return Request{}, &Error{Param: "sort"}
		}
	}
	var order Dir
	if q.Has("order") {
		order = Dir(q.Get("order"))
		if order != Asc && order != Desc {
			return Request{}, &Error{Param: "order"}
		}
	}
	return New(page, size, sort, order, s)
}

// New builds a Request from already-typed values (gRPC or internal callers).
// Zero page, size, sort or order take the Spec's defaults; other invalid values
// return an *Error.
func New(page, pageSize int, sort string, order Dir, s Spec) (Request, error) {
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return Request{}, &Error{Param: "page"}
	}
	if pageSize == 0 {
		pageSize = s.defaultSize()
	}
	if pageSize < 1 || pageSize > s.maxSize() {
		return Request{}, &Error{Param: "page_size"}
	}
	if sort == "" {
		sort = s.Default
	}
	f, ok := s.Fields[sort]
	if !ok {
		return Request{}, &Error{Param: "sort"}
	}
	if order == "" {
		order = f.dirOf()
	}
	if order != Asc && order != Desc {
		return Request{}, &Error{Param: "order"}
	}
	return Request{Page: page, PageSize: pageSize, Sort: sort, Order: order}, nil
}

// Legacy reports whether q uses only the old cursor/limit style, so a handler
// can keep serving it for one release.
func Legacy(q url.Values) bool { return hasLegacy(q) && !hasPage(q) }

func hasLegacy(q url.Values) bool { return q.Has("cursor") || q.Has("limit") }

func hasPage(q url.Values) bool {
	return q.Has("page") || q.Has("page_size") || q.Has("sort") || q.Has("order")
}
