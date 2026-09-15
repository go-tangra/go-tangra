// Package valkey provides an authz.Source (policy document + pub/sub change
// notifications) and an identity.RevocationChecker (emergency denylist) backed
// by Valkey.
package valkey

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	valkey "github.com/valkey-io/valkey-go"
)

// KV is the minimal client surface used by this package (real client or fake).
type KV interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Exists(ctx context.Context, key string) (bool, error)
	Subscribe(ctx context.Context, channel string, onMessage func(msg string)) error
	Close()
}

// Config for the real client.
type Config struct {
	Addresses []string
	Username  string
	Password  string
	// TLS is required unless AllowPlaintext is set (development only).
	TLS            *tls.Config
	AllowPlaintext bool
}

type client struct{ c valkey.Client }

// NewClient connects to Valkey. TLS 1.3 is enforced on the supplied config.
func NewClient(cfg Config) (KV, error) {
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("policy-valkey: at least one address is required")
	}
	if cfg.TLS == nil && !cfg.AllowPlaintext {
		return nil, errors.New("policy-valkey: TLS config is required (set AllowPlaintext only for local development)")
	}
	if cfg.TLS != nil {
		cfg.TLS = cfg.TLS.Clone()
		if cfg.TLS.MinVersion < tls.VersionTLS13 {
			cfg.TLS.MinVersion = tls.VersionTLS13
		}
	}
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: cfg.Addresses, Username: cfg.Username, Password: cfg.Password, TLSConfig: cfg.TLS})
	if err != nil {
		return nil, fmt.Errorf("policy-valkey: %w", err)
	}
	return &client{c: c}, nil
}

func (c *client) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := c.c.Do(ctx, c.c.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (c *client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.c.Do(ctx, c.c.B().Exists().Key(key).Build()).AsInt64()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (c *client) Subscribe(ctx context.Context, channel string, onMessage func(string)) error {
	return c.c.Receive(ctx, c.c.B().Subscribe().Channel(channel).Build(), func(m valkey.PubSubMessage) { onMessage(m.Message) })
}

func (c *client) Close() { c.c.Close() }

// Keys for one trust domain.
func docKey(td string) string     { return "freya:policy:" + td + ":doc" }
func versionKey(td string) string { return "freya:policy:" + td + ":version" }

// ChangedChannel is the pub/sub channel carrying new policy versions.
const ChangedChannel = "freya:policy:changed"

func revokedKey(id, serial string) string {
	if serial == "" {
		return "freya:revoked:" + id
	}
	return "freya:revoked:" + id + "#" + serial
}
