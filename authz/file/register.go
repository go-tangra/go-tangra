package file

import "github.com/go-tangra/go-tangra/v4/authz"

func init() {
	authz.RegisterFileSource(func(path string) (authz.Source, error) { return New(path) })
}
