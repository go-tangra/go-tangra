package audit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func capture(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	h := NewRedactingHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return slog.New(h), &buf
}

func TestRedactsForbiddenKeys(t *testing.T) {
	log, buf := capture(t)
	log.Info("x",
		"api_key", "AKIA123", "Private_Key", "pk", "client_secret", "s3", "TOKEN", "tok",
		"password", "pw", "Authorization", "Bearer abc", "x-request-id", "keep-me", "peer", "keep-too")
	out := buf.String()
	for _, leaked := range []string{"AKIA123", `"pk"`, `"s3"`, `"tok"`, `"pw"`, "Bearer abc"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaked %q in %s", leaked, out)
		}
	}
	for _, kept := range []string{"keep-me", "keep-too"} {
		if !strings.Contains(out, kept) {
			t.Errorf("wrongly redacted %q in %s", kept, out)
		}
	}
	if strings.Count(out, Redacted) < 6 {
		t.Errorf("expected at least 6 redactions: %s", out)
	}
}

func TestRedactsSensitiveValueTypes(t *testing.T) {
	log, buf := capture(t)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalECPrivateKey(key)
	pemText := "-----BEGIN EC PRIVATE KEY-----\nAAAA\n-----END EC PRIVATE KEY-----"
	log.Info("x",
		"cfg", &tls.Config{ServerName: "leak-servername"},
		"cert", tls.Certificate{Certificate: [][]byte{[]byte("leak-cert")}},
		"certp", &tls.Certificate{Certificate: [][]byte{[]byte("leak-certp")}},
		"signer", key,
		"pem", pemText,
		"pemb", []byte(pemText),
		"der", der,
		"plain", "visible")
	out := buf.String()
	for _, leaked := range []string{"leak-servername", "leak-cert", "AAAA", "BEGIN"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaked %q in %s", leaked, out)
		}
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("plain value lost: %s", out)
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"cfg", "cert", "certp", "signer", "pem", "pemb", "der"} {
		if rec[k] != Redacted {
			t.Errorf("%s = %v, want %s", k, rec[k], Redacted)
		}
	}
}

func TestRedactsNestedGroupsAndWithAttrs(t *testing.T) {
	log, buf := capture(t)
	log = log.With("token", "with-tok", "ok", "with-ok").WithGroup("g")
	log.Info("x", slog.Group("inner", "password", "nested-pw", "name", "nested-ok"))
	out := buf.String()
	if strings.Contains(out, "with-tok") || strings.Contains(out, "nested-pw") {
		t.Fatalf("leak: %s", out)
	}
	if !strings.Contains(out, "with-ok") || !strings.Contains(out, "nested-ok") {
		t.Fatalf("over-redaction: %s", out)
	}
}

func TestRedactingHandlerEnabledPassthrough(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled must delegate to inner handler")
	}
	if NewRedactingHandler(h) != h {
		t.Fatal("wrapping a redacting handler must be idempotent")
	}
}
