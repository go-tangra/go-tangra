# Contract: Go package `github.com/go-tangra/go-tangra/v4/listquery`

Stdlib only. 100% statement coverage and a fuzz test on `Parse` (security
package, constitution IV).

```go
package listquery

type Dir string // "asc" | "desc"

const (
    Asc  Dir = "asc"
    Desc Dir = "desc"
    MaxPageSize     = 200
    DefaultPageSize = 25
)

// Field maps a public sort name to a constant SQL expression.
type Field struct {
    Expr       string // constant, e.g. "a.created_at"; never request data
    Text       bool   // order by lower(Expr)
    DefaultDir Dir    // direction when the request names the field without order
}

// Spec is one list's definition. Build once (package var); Validate() at init.
type Spec struct {
    Fields      map[string]Field
    Default     string // default sort field (must be in Fields)
    TieBreak    string // constant unique column, e.g. "a.id"
    DefaultSize int    // 0 → DefaultPageSize
    MaxSize     int    // 0 → MaxPageSize; must be ≤ MaxPageSize
}

func (s Spec) Validate() error // panics-free check used in module init tests

// Request is a validated page request.
type Request struct {
    Page, PageSize int
    Sort           string
    Order          Dir
}

// Error names the offending parameter only.
type Error struct{ Param string }
func (e *Error) Error() string

// Parse reads page, page_size, sort, order from q. Absent values take defaults;
// invalid values return *Error.
func Parse(q url.Values, s Spec) (Request, error)

// Legacy reports whether q uses the old cursor/limit style only; Parse returns
// *Error{Param:"cursor"} when both styles are mixed.
func Legacy(q url.Values) bool

// SQL helpers — output is built only from Spec constants and Dir.
func (r Request) OrderBy(s Spec) string // "lower(a.name) ASC NULLS LAST, a.id ASC"
func (r Request) Limit() int
func (r Request) Offset() int

// Clamp moves Page to the last page for total (page 1 when total == 0).
func (r Request) Clamp(total int) Request

// Page is the response shape.
type Page[T any] struct {
    Items    []T    `json:"items"`
    Total    int    `json:"total"`
    Page     int    `json:"page"`
    PageSize int    `json:"page_size"`
    Sort     string `json:"sort"`
    Order    Dir    `json:"order"`
}
func NewPage[T any](items []T, total int, r Request) Page[T] // nil items → []

// In-memory lists (memstores, PowerDNS records, registries, fleet views).
// key returns the sortable value of field for an item (string, int64,
// float64, time.Time or nil); tie returns the unique tie-breaker value.
func SortSlice[T any](items []T, r Request, key func(T, string) any, tie func(T) string)
func Window[T any](items []T, r Request) (page []T, total int, applied Request)
```

Module usage pattern (repo layer):

```go
var hostList = listquery.Spec{
    Fields: map[string]listquery.Field{
        "hostname":  {Expr: "h.hostname", Text: true, DefaultDir: listquery.Asc},
        "last_seen": {Expr: "h.last_seen", DefaultDir: listquery.Desc},
    },
    Default: "hostname", TieBreak: "h.id",
}
// count(*) WHERE … → total; req = req.Clamp(total);
// SELECT … WHERE … ORDER BY req.OrderBy(hostList) LIMIT $n OFFSET $m
```

HTTP handlers map `*listquery.Error` to the module's
`WriteDetail(w, 422, "validation_failed", map[string]any{"param": e.Param})`.
