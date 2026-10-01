package listquery

import (
	"errors"
	"fmt"
)

// Dir is a sort direction.
type Dir string

// Sort directions.
const (
	Asc  Dir = "asc"
	Desc Dir = "desc"
)

// Page-size bounds shared by every list.
const (
	MaxPageSize     = 200
	DefaultPageSize = 25
)

// Field maps a public sort name to a constant SQL expression.
type Field struct {
	// Expr is the column or expression to order by, e.g. "h.created_at". It
	// must be a constant in module code, never request data.
	Expr string
	// Text orders case-insensitively, by lower(Expr).
	Text bool
	// DefaultDir is the direction used when a request names the field without
	// an order; empty means ascending.
	DefaultDir Dir
	// NotNull declares that Expr never yields NULL (a NOT NULL column, or an
	// expression over NOT NULL columns). OrderBy then omits "NULLS LAST", so a
	// plain btree index on the column serves both directions: a backward index
	// scan yields DESC NULLS FIRST, which the planner cannot match to
	// DESC NULLS LAST even when the column has no NULLs. Set it only when the
	// schema guarantees it; a NULL in a NotNull field would sort first in
	// descending order instead of last. The zero value keeps NULLS LAST.
	NotNull bool
}

// Spec is one list's definition: its sortable fields, default sort and unique
// tie-breaker. Declare it once as a package value and check it with Validate
// in a test.
type Spec struct {
	Fields map[string]Field
	// Default is the field used when no sort is requested.
	Default string
	// TieBreak is a constant unique column appended to every ORDER BY so that
	// equal values page deterministically, e.g. "h.id".
	TieBreak string
	// DefaultSize is the page size when none is requested (0 → 25, capped to
	// MaxSize). MaxSize caps the page size (0 → 200; never above 200).
	DefaultSize int
	MaxSize     int
}

// Validate reports a malformed Spec.
func (s Spec) Validate() error {
	if len(s.Fields) == 0 {
		return errors.New("listquery: spec has no fields")
	}
	if _, ok := s.Fields[s.Default]; !ok {
		return fmt.Errorf("listquery: default sort %q is not a field", s.Default)
	}
	if s.TieBreak == "" {
		return errors.New("listquery: spec has no tie-breaker")
	}
	for name, f := range s.Fields {
		if f.Expr == "" {
			return fmt.Errorf("listquery: field %q has no expression", name)
		}
		if f.DefaultDir != "" && f.DefaultDir != Asc && f.DefaultDir != Desc {
			return fmt.Errorf("listquery: field %q has an invalid default direction", name)
		}
	}
	if s.MaxSize < 0 || s.MaxSize > MaxPageSize {
		return fmt.Errorf("listquery: max size must be 0..%d", MaxPageSize)
	}
	if s.DefaultSize < 0 || (s.MaxSize > 0 && s.DefaultSize > s.MaxSize) {
		return errors.New("listquery: default size must be 0..max size")
	}
	return nil
}

func (s Spec) maxSize() int {
	if s.MaxSize > 0 {
		return s.MaxSize
	}
	return MaxPageSize
}

func (s Spec) defaultSize() int {
	d := s.DefaultSize
	if d == 0 {
		d = DefaultPageSize
	}
	return min(d, s.maxSize())
}

// dirOf returns the field's default direction.
func (f Field) dirOf() Dir {
	if f.DefaultDir == Desc {
		return Desc
	}
	return Asc
}
