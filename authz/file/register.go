package file

import "github.com/go-freya/freya/authz"

func init() {
	authz.RegisterFileSource(func(path string) (authz.Source, error) { return New(path) })
}
