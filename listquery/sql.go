package listquery

// OrderBy returns the ORDER BY clause body for r, built only from s's constant
// expressions and the direction enum: the sort expression (lower() for text
// fields) with NULLS LAST, then the tie-breaker in the same direction. A
// hand-built Request naming an unknown field falls back to the default field.
func (r Request) OrderBy(s Spec) string {
	f, ok := s.Fields[r.Sort]
	if !ok {
		f = s.Fields[s.Default]
	}
	expr := f.Expr
	if f.Text {
		expr = "lower(" + expr + ")"
	}
	dir := "ASC"
	if r.Order == Desc {
		dir = "DESC"
	}
	return expr + " " + dir + " NULLS LAST, " + s.TieBreak + " " + dir
}

// Limit is the page size, for LIMIT.
func (r Request) Limit() int { return r.PageSize }

// Offset is the number of rows before the page, for OFFSET.
func (r Request) Offset() int {
	if r.Page < 1 {
		return 0
	}
	return (r.Page - 1) * r.PageSize
}

// Clamp moves a page beyond the end of total records to the last page (page 1
// for an empty list). Call it after counting and before fetching.
func (r Request) Clamp(total int) Request {
	if r.PageSize < 1 {
		return r
	}
	last := max((total+r.PageSize-1)/r.PageSize, 1)
	if r.Page > last {
		r.Page = last
	}
	return r
}
