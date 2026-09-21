package edge

import (
	"context"
	"net/http/httptest"
	"testing"
)

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func TestClientIPContext(t *testing.T) {
	if ClientIP(context.Background()) != "" {
		t.Fatal("empty outside a request")
	}
	if got := ClientIP(WithClientIP(context.Background(), "203.0.113.9")); got != "203.0.113.9" {
		t.Fatal(got)
	}
}
