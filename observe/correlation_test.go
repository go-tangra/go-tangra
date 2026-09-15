package observe

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/transport"
)

type hdr map[string][]string

func (h hdr) Get(k string) string {
	if v := h[strings.ToLower(k)]; len(v) > 0 {
		return v[0]
	}
	return ""
}
func (h hdr) Set(k, v string) { h[strings.ToLower(k)] = []string{v} }
func (h hdr) Add(k, v string) { h[strings.ToLower(k)] = append(h[strings.ToLower(k)], v) }
func (h hdr) Keys() []string {
	out := []string{}
	for k := range h {
		out = append(out, k)
	}
	return out
}
func (h hdr) Values(k string) []string { return h[strings.ToLower(k)] }

type tr struct {
	req, rep hdr
	kind     transport.Kind
}

func (t tr) Kind() transport.Kind            { return t.kind }
func (t tr) Endpoint() string                { return "grpc://x" }
func (t tr) Operation() string               { return "/a.B/C" }
func (t tr) RequestHeader() transport.Header { return t.req }
func (t tr) ReplyHeader() transport.Header   { return t.rep }

var uuidV7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewCorrelationIDIsUUIDv7(t *testing.T) {
	a, b := NewCorrelationID(), NewCorrelationID()
	if !uuidV7.MatchString(a) || !uuidV7.MatchString(b) || a == b {
		t.Fatalf("bad ids %q %q", a, b)
	}
}

func TestValidCorrelationID(t *testing.T) {
	ok := []string{"a", "abc-123_x.y", strings.Repeat("z", 128), NewCorrelationID()}
	bad := []string{"", "has space", "tab\t", strings.Repeat("z", 129), "é", "a/b", "a:b"}
	for _, s := range ok {
		if !ValidCorrelationID(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if ValidCorrelationID(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestServerMiddlewareGeneratesAndPreserves(t *testing.T) {
	mw := ServerCorrelation()
	var seen string
	h := mw(func(ctx context.Context, req any) (any, error) {
		seen = CorrelationID(ctx)
		return nil, nil
	})
	// Absent → generated and echoed in reply header.
	t1 := tr{req: hdr{}, rep: hdr{}, kind: transport.KindGRPC}
	if _, err := h(transport.NewServerContext(context.Background(), t1), nil); err != nil {
		t.Fatal(err)
	}
	if !uuidV7.MatchString(seen) || t1.rep.Get(HeaderCorrelationID) != seen {
		t.Fatalf("generated id not propagated: %q %q", seen, t1.rep.Get(HeaderCorrelationID))
	}
	// Valid incoming → preserved.
	t2 := tr{req: hdr{HeaderCorrelationID: {"client-42"}}, rep: hdr{}}
	_, _ = h(transport.NewServerContext(context.Background(), t2), nil)
	if seen != "client-42" || t2.rep.Get(HeaderCorrelationID) != "client-42" {
		t.Fatalf("valid id not preserved: %q", seen)
	}
	// Invalid incoming → replaced, not echoed.
	t3 := tr{req: hdr{HeaderCorrelationID: {"bad id!"}}, rep: hdr{}}
	_, _ = h(transport.NewServerContext(context.Background(), t3), nil)
	if seen == "bad id!" || !uuidV7.MatchString(seen) || t3.rep.Get(HeaderCorrelationID) == "bad id!" {
		t.Fatalf("invalid id echoed: %q", seen)
	}
	// No transport in context → still generates.
	_, _ = h(context.Background(), nil)
	if !uuidV7.MatchString(seen) {
		t.Fatalf("no id without transport: %q", seen)
	}
	// Explicit context value wins.
	if got := CorrelationID(WithCorrelationID(context.Background(), "explicit")); got != "explicit" {
		t.Fatalf("got %q", got)
	}
	if got := CorrelationID(context.Background()); got != "" {
		t.Fatalf("expected empty without id, got %q", got)
	}
}

func TestClientMiddlewareInjects(t *testing.T) {
	mw := ClientCorrelation()
	h := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })
	tc := tr{req: hdr{}, rep: hdr{}}
	ctx := transport.NewClientContext(WithCorrelationID(context.Background(), "up-1"), tc)
	_, _ = h(ctx, nil)
	if tc.req.Get(HeaderCorrelationID) != "up-1" {
		t.Fatalf("outgoing header %q", tc.req.Get(HeaderCorrelationID))
	}
	// Without an id in context a new one is generated and injected.
	tc2 := tr{req: hdr{}, rep: hdr{}}
	_, _ = h(transport.NewClientContext(context.Background(), tc2), nil)
	if !uuidV7.MatchString(tc2.req.Get(HeaderCorrelationID)) {
		t.Fatalf("generated outgoing header %q", tc2.req.Get(HeaderCorrelationID))
	}
	// No transport: passthrough without panic.
	if _, err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}
