package listquery

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// SortSlice orders items in place with the same semantics as OrderBy: key
// returns an item's value for the requested field (string, int, int64,
// float64, time.Time or nil; other types compare by their text form), strings
// compare case-insensitively, nil values come last in both directions, and tie
// (a unique value per item) breaks equal keys in the requested direction.
// A NotNull field never yields nil, so its order matches OrderBy's (which then
// has no NULLS clause) exactly.
func SortSlice[T any](items []T, r Request, key func(T, string) any, tie func(T) string) {
	desc := r.Order == Desc
	slices.SortStableFunc(items, func(a, b T) int {
		ka, kb := key(a, r.Sort), key(b, r.Sort)
		switch {
		case ka == nil && kb == nil:
		case ka == nil:
			return 1
		case kb == nil:
			return -1
		default:
			if c := compareAny(ka, kb); c != 0 {
				if desc {
					return -c
				}
				return c
			}
		}
		c := cmp.Compare(tie(a), tie(b))
		if desc {
			return -c
		}
		return c
	})
}

// Window returns the requested page of already-sorted items, the total and
// the request clamped to the last page.
func Window[T any](items []T, r Request) (page []T, total int, applied Request) {
	total = len(items)
	applied = r.Clamp(total)
	start := min(applied.Offset(), total)
	end := min(start+applied.Limit(), total)
	return items[start:end], total, applied
}

func compareAny(a, b any) int {
	switch x := a.(type) {
	case string:
		if y, ok := b.(string); ok {
			return cmp.Compare(strings.ToLower(x), strings.ToLower(y))
		}
	case int:
		if y, ok := b.(int); ok {
			return cmp.Compare(x, y)
		}
	case int64:
		if y, ok := b.(int64); ok {
			return cmp.Compare(x, y)
		}
	case float64:
		if y, ok := b.(float64); ok {
			return cmp.Compare(x, y)
		}
	case time.Time:
		if y, ok := b.(time.Time); ok {
			return x.Compare(y)
		}
	}
	return cmp.Compare(strings.ToLower(fmt.Sprint(a)), strings.ToLower(fmt.Sprint(b)))
}
