package edge

import (
	"net/http"
	"strings"
	"testing"
)

func TestCSRFExemptHook(t *testing.T) {
	srv, client, base := devServer(t, Config{CSRFExempt: func(r *http.Request) bool {
		_, hasCookie := r.Header["Cookie"]
		return !hasCookie && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
	}})
	srv.HandleFunc("/api/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	req, _ := http.NewRequest(http.MethodPost, base+"/api/x", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("bearer client without cookies must bypass CSRF: %v %v", resp, err)
	}
	resp.Body.Close()
	// The same request with a cookie is still subject to the check.
	req, _ = http.NewRequest(http.MethodPost, base+"/api/x", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "__Host-session=abc")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 403 {
		t.Fatalf("cookie-bearing request must be checked: %v %v", resp, err)
	}
	resp.Body.Close()
}
