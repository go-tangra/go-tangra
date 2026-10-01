package listquery

import (
	"encoding/json"
	"testing"
)

func TestSpecValidate(t *testing.T) {
	if err := hostSpec.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	bad := map[string]Spec{
		"no fields":        {Default: "a", TieBreak: "id"},
		"default missing":  {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "b", TieBreak: "id"},
		"no tie-breaker":   {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a"},
		"empty expr":       {Fields: map[string]Field{"a": {Expr: ""}}, Default: "a", TieBreak: "id"},
		"bad direction":    {Fields: map[string]Field{"a": {Expr: "a", DefaultDir: "up"}}, Default: "a", TieBreak: "id"},
		"max too large":    {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a", TieBreak: "id", MaxSize: 201},
		"negative max":     {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a", TieBreak: "id", MaxSize: -1},
		"default over max": {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a", TieBreak: "id", MaxSize: 10, DefaultSize: 20},
		"negative default": {Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a", TieBreak: "id", DefaultSize: -1},
	}
	for name, s := range bad {
		if err := s.Validate(); err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
	ok := Spec{Fields: map[string]Field{"a": {Expr: "a"}}, Default: "a", TieBreak: "id", MaxSize: 50, DefaultSize: 50}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if ok.defaultSize() != 50 || ok.maxSize() != 50 {
		t.Fatalf("sizes %d %d", ok.defaultSize(), ok.maxSize())
	}
	// A default larger than a small max is capped.
	capped := Spec{Fields: ok.Fields, Default: "a", TieBreak: "id", MaxSize: 10}
	if capped.defaultSize() != 10 {
		t.Fatalf("capped default %d", capped.defaultSize())
	}
}

func TestNewPage(t *testing.T) {
	r := Request{Page: 2, PageSize: 10, Sort: "hostname", Order: Desc}
	p := NewPage[string](nil, 12, r)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"items":[],"total":12,"page":2,"page_size":10,"sort":"hostname","order":"desc"}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
	if q := NewPage([]int{1, 2}, 2, r); len(q.Items) != 2 {
		t.Fatal("items dropped")
	}
}
