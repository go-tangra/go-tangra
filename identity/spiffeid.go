package identity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	scheme    = "spiffe://"
	svcPrefix = "/svc/"
)

var (
	trustDomainRE = regexp.MustCompile(`^[a-z0-9._-]{1,255}$`)
	serviceNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// SPIFFEID is a validated, canonical service identity of the form
// spiffe://<trust-domain>/svc/<service-name>. The zero value is invalid.
type SPIFFEID struct {
	td   string
	name string
}

// ParseSPIFFEID parses and validates s. Only the canonical lower-case form is
// accepted; the parser never normalises, so String() always round-trips.
func ParseSPIFFEID(s string) (SPIFFEID, error) {
	if s == "" {
		return SPIFFEID{}, errors.New("spiffe id: empty")
	}
	if !strings.HasPrefix(s, scheme) {
		return SPIFFEID{}, errors.New("spiffe id: scheme must be spiffe://")
	}
	rest := s[len(scheme):]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return SPIFFEID{}, errors.New("spiffe id: path must start with /svc/")
	}
	td, path := rest[:slash], rest[slash:]
	if !trustDomainRE.MatchString(td) {
		return SPIFFEID{}, errors.New("spiffe id: invalid trust domain")
	}
	if !strings.HasPrefix(path, svcPrefix) {
		return SPIFFEID{}, errors.New("spiffe id: path must start with /svc/")
	}
	name := path[len(svcPrefix):]
	if !serviceNameRE.MatchString(name) {
		return SPIFFEID{}, errors.New("spiffe id: invalid service name")
	}
	return SPIFFEID{td: td, name: name}, nil
}

// NewSPIFFEID builds an ID from its parts, validating both.
func NewSPIFFEID(trustDomain, serviceName string) (SPIFFEID, error) {
	return ParseSPIFFEID(scheme + trustDomain + svcPrefix + serviceName)
}

// ForService is NewSPIFFEID for inputs known to be valid; it panics otherwise.
func ForService(trustDomain, serviceName string) SPIFFEID {
	id, err := NewSPIFFEID(trustDomain, serviceName)
	if err != nil {
		panic(fmt.Sprintf("identity.ForService(%q, %q): %v", trustDomain, serviceName, err))
	}
	return id
}

// ValidTrustDomain reports whether td is a syntactically valid trust domain.
func ValidTrustDomain(td string) bool { return trustDomainRE.MatchString(td) }

// ValidServiceName reports whether name is a syntactically valid service name.
func ValidServiceName(name string) bool { return serviceNameRE.MatchString(name) }

// String returns the canonical URI form.
func (id SPIFFEID) String() string {
	if id.IsZero() {
		return ""
	}
	return scheme + id.td + svcPrefix + id.name
}

// TrustDomain returns the trust domain part.
func (id SPIFFEID) TrustDomain() string { return id.td }

// ServiceName returns the service name part.
func (id SPIFFEID) ServiceName() string { return id.name }

// IsZero reports whether id is the zero (invalid) value.
func (id SPIFFEID) IsZero() bool { return id.td == "" && id.name == "" }

// Equal reports whether both IDs are identical.
func (id SPIFFEID) Equal(o SPIFFEID) bool { return id == o }
