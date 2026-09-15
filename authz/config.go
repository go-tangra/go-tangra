package authz

import (
	"fmt"

	"github.com/go-freya/freya/config"
)

// fileSourceFactory is set by authz/file's init to avoid an import cycle.
var fileSourceFactory func(path string) (Source, error)

// RegisterFileSource is called by authz/file.
func RegisterFileSource(f func(path string) (Source, error)) { fileSourceFactory = f }

// NewSourceFromConfig builds the policy source named in cfg. The valkey source
// lives in contrib/policy-valkey and must be passed via freya.WithPolicySource.
func NewSourceFromConfig(cfg config.Authz) (Source, error) {
	switch cfg.Source {
	case config.AuthzFile:
		if fileSourceFactory == nil {
			return nil, fmt.Errorf("authz: file source not linked; import github.com/go-freya/freya/authz/file")
		}
		return fileSourceFactory(cfg.Path)
	case config.AuthzValkey:
		return nil, fmt.Errorf("authz: valkey source is provided by contrib/policy-valkey; pass it with freya.WithPolicySource")
	default:
		return nil, fmt.Errorf("authz: unsupported policy source %q", cfg.Source)
	}
}
