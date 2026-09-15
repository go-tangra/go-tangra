// Package cred is the only path through which private key material leaves an
// identity provider. It is internal so that nothing outside this module can obtain
// a service's key; transport/tlsconf type-asserts providers to Source.
package cred

import "crypto/tls"

// Source yields the current TLS certificate (leaf, chain, private key).
type Source interface {
	Credential() (*tls.Certificate, error)
}
