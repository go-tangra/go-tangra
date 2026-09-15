package http

import (
	"encoding/json"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v3/errors"
)

// ErrorEncoder writes {"reason":"<identifier>"} and nothing else: no messages,
// metadata, stack traces or versions (SR-007). Framework refusals carry the
// closed audit vocabulary; application 4xx errors keep their reason identifier;
// every 5xx (and any non-Kratos error) collapses to "internal".
func ErrorEncoder(w http.ResponseWriter, _ *http.Request, err error) {
	se := kerrors.FromError(err)
	reason := se.Reason
	code := int(se.Code)
	switch {
	case code == http.StatusGatewayTimeout:
		reason = "timeout"
	case code >= 500 || code < 400 || reason == "":
		reason, code = "internal", http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"reason": reason})
}
