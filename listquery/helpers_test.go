package listquery

import "net/url"

// hostSpec is the fixture list used across the tests.
var hostSpec = Spec{
	Fields: map[string]Field{
		"hostname":  {Expr: "h.hostname", Text: true},
		"last_seen": {Expr: "h.last_seen", DefaultDir: Desc},
		"size":      {Expr: "h.size", DefaultDir: Asc},
		"created":   {Expr: "h.created_at", DefaultDir: Desc, NotNull: true},
		"name":      {Expr: "h.name", Text: true, NotNull: true},
	},
	Default:  "hostname",
	TieBreak: "h.id",
}

func values(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	return q
}
