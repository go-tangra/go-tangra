package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/authz"
	kerrors "github.com/go-kratos/kratos/v3/errors"
)

func TestErrorEncoderMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		reason string
	}{
		{authn.ErrNoIdentity, 401, "no_identity"},
		{authn.ErrRevoked, 401, "identity_revoked"},
		{authn.ErrNameMismatch, 401, "name_mismatch"},
		{authz.ErrDenied, 403, "denied"},
		{kerrors.New(413, "limit_exceeded", "body too large: 2MiB > 1MiB at /internal/path"), 413, "limit_exceeded"},
		{kerrors.New(431, "limit_exceeded", ""), 431, "limit_exceeded"},
		{kerrors.TooManyRequests("limit_exceeded", "x"), 429, "limit_exceeded"},
		{kerrors.GatewayTimeout("timeout", "x"), 504, "timeout"},
		{kerrors.InternalServer("db_conn_failed", "postgres://user:pw@10.0.0.9"), 500, "internal"},
		{errors.New("panic: runtime error at freya.go:42 v1.2.3"), 500, "internal"},
		{kerrors.NotFound("order_missing", "missing /internal"), 404, "order_missing"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		r, _ := http.NewRequest(http.MethodGet, "https://x/y", nil)
		ErrorEncoder(rec, r, tc.err)
		if rec.Code != tc.status {
			t.Errorf("%v: status %d want %d", tc.err, rec.Code, tc.status)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%v: body %q", tc.err, rec.Body.String())
		}
		if body["reason"] != tc.reason || len(body) != 1 {
			t.Errorf("%v: body %v", tc.err, body)
		}
		for _, leak := range []string{"internal/path", "10.0.0.9", "pw", "freya.go", "v1.2.3", "postgres"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Errorf("%v: leaked %q", tc.err, leak)
			}
		}
	}
}
