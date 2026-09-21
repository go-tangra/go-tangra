package edge

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-freya/freya/audit"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// CSRF double-submit contract (contracts/token.md).
const (
	CSRFCookie = "__Host-csrf"
	CSRFHeader = "X-CSRF-Token"
)

// IssueCSRFCookie sets a fresh CSRF cookie (readable by the console script).
func IssueCSRFCookie(w http.ResponseWriter) string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	v := base64.RawURLEncoding.EncodeToString(b[:])
	http.SetCookie(w, &http.Cookie{Name: CSRFCookie, Value: v, Path: "/", Secure: true, HttpOnly: false, SameSite: http.SameSiteStrictMode})
	return v
}

func safeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func (s *Server) csrfFilter() khttp.FilterFunc {
	allowed := map[string]bool{}
	for _, o := range s.cfg.AllowedOrigins {
		allowed[strings.TrimRight(strings.ToLower(o), "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethod(r.Method) || (s.cfg.CSRFExempt != nil && s.cfg.CSRFExempt(r)) {
				next.ServeHTTP(w, r)
				return
			}
			reason := s.checkCSRF(r, allowed)
			if reason != "" {
				auditRefusal(s.rt, r, audit.ReasonCSRFRefused, map[string]string{"detail": reason})
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"reason":"csrf"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// checkCSRF returns "" when the request passes, else a fixed-vocabulary detail.
func (s *Server) checkCSRF(r *http.Request, allowed map[string]bool) string {
	c, err := r.Cookie(CSRFCookie)
	if err != nil || len(c.Value) < 32 {
		return "missing_cookie"
	}
	h := r.Header.Get(CSRFHeader)
	if h == "" {
		return "missing_header"
	}
	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(h)) != 1 {
		return "mismatch"
	}
	origin := strings.TrimRight(strings.ToLower(r.Header.Get("Origin")), "/")
	site := r.Header.Get("Sec-Fetch-Site")
	switch {
	case site == "cross-site":
		return "cross_site"
	case origin != "":
		if len(allowed) == 0 {
			self := strings.ToLower("https://" + r.Host)
			if origin != self {
				return "origin"
			}
		} else if !allowed[origin] {
			return "origin"
		}
	case site == "same-origin" || site == "none":
	default:
		return "no_origin"
	}
	return ""
}
