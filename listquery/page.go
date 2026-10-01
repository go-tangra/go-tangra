package listquery

// Page is the list response shape shared by every module.
type Page[T any] struct {
	Items    []T    `json:"items"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Sort     string `json:"sort"`
	Order    Dir    `json:"order"`
}

// NewPage shapes a response; nil items encode as an empty array.
func NewPage[T any](items []T, total int, r Request) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Page: r.Page, PageSize: r.PageSize, Sort: r.Sort, Order: r.Order}
}
