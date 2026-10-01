// Package listquery is the platform's list contract: numbered pages with an
// exact total, and server-side sorting restricted to a per-list allow-list.
//
// A list declares a Spec once (sortable fields mapped to constant SQL
// expressions, a default sort and a unique tie-breaker). Parse turns the
// untrusted page, page_size, sort and order query parameters into a validated
// Request or an *Error naming the offending parameter. OrderBy builds the
// ORDER BY clause only from the Spec's constants and a closed direction enum,
// so request text never reaches SQL. Repositories count the matching rows,
// Clamp the request to the last page, then fetch with Limit and Offset;
// NewPage shapes the response. SortSlice and Window give in-memory lists the
// same semantics.
package listquery
