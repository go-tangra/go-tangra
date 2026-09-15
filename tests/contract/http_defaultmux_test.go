package contract

import (
	"io"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof on http.DefaultServeMux on purpose
	"strings"
	"testing"

	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"
	thttp "github.com/go-freya/freya/transport/http"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestHTTPServerNeverServesDefaultMux(t *testing.T) {
	ca := testutil.MustCA("example.org")
	rt := testrt.New(t, ca, "inventory")
	srv, err := thttp.NewServer(rt, thttp.WithAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	srv.Route("/").GET("/hello", func(ctx khttp.Context) error { return ctx.String(200, "hi") })
	stop := testrt.StartServer(t, srv)
	defer stop()
	client := testrt.HTTPClient(t, ca, "orders", "inventory")
	ep, _ := srv.Endpoint()
	base := "https://" + ep.Host

	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/nope"} {
		resp, err := client.Get(base + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d body %q — DefaultServeMux leaked", path, resp.StatusCode, body)
		}
		if strings.Contains(string(body), "pprof") {
			t.Errorf("%s: body mentions pprof: %q", path, body)
		}
	}
	resp, err := client.Post(base+"/hello", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /hello: status %d, want 405", resp.StatusCode)
	}
}
