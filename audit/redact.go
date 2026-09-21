package audit

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"log/slog"
	"strings"
)

// Redacted replaces any value the redacting handler refuses to log.
const Redacted = "[REDACTED]"

// forbiddenKeyParts are matched case-insensitively as substrings of attribute keys.
var forbiddenKeyParts = []string{"key", "private", "secret", "token", "password", "authorization", "phone"}

type redactingHandler struct{ inner slog.Handler }

// NewRedactingHandler wraps inner so that secrets can never reach it: attributes
// whose key looks sensitive, whose value is TLS/key material, whose value is a
// byte slice, or whose string contains a PEM header are replaced by Redacted.
// Wrapping an already redacting handler returns it unchanged.
func NewRedactingHandler(inner slog.Handler) slog.Handler {
	if _, ok := inner.(*redactingHandler); ok {
		return inner
	}
	return &redactingHandler{inner: inner}
}

func (h *redactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &redactingHandler{inner: h.inner.WithAttrs(out)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{inner: h.inner.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		g := a.Value.Group()
		out := make([]slog.Attr, len(g))
		for i, ga := range g {
			out[i] = redactAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	if forbiddenKey(a.Key) || sensitiveValue(a.Value) {
		return slog.String(a.Key, Redacted)
	}
	return a
}

func forbiddenKey(k string) bool {
	lk := strings.ToLower(k)
	for _, p := range forbiddenKeyParts {
		if strings.Contains(lk, p) {
			return true
		}
	}
	return false
}

func sensitiveValue(v slog.Value) bool {
	switch v.Kind() {
	case slog.KindString:
		return strings.Contains(v.String(), "-----BEGIN")
	case slog.KindAny:
		switch x := v.Any().(type) {
		case *tls.Config, tls.Config, tls.Certificate, *tls.Certificate,
			*rsa.PrivateKey, rsa.PrivateKey, *ecdsa.PrivateKey, ecdsa.PrivateKey,
			ed25519.PrivateKey, *ed25519.PrivateKey, crypto.Signer, crypto.Decrypter:
			return true
		case []byte:
			return true
		case string:
			return strings.Contains(x, "-----BEGIN")
		}
	}
	return false
}
