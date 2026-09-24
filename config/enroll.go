package config

import (
	"errors"
	"fmt"

	"github.com/go-tangra/go-tangra/v4/identity"
)

// DefaultEnrollServerService is the service whose SVID a workload expects when
// it enrolls at a mesh-certificate endpoint (lcm's keyless enroll listener)
// and enroll.server_spiffe_id is not set.
const DefaultEnrollServerService = "lcm"

// EnrollTLS is the server verification of a workload's FIRST network
// enrollment (the join-token HTTPS call to lcm; renewals are mTLS and always
// SPIFFE-verified). Services embed it inline in their enroll/mesh_enroll block.
// Exactly one mode applies:
//
//   - public (zero value): system roots + the host name of the enroll URL.
//     For enrolling through the gateway edge, which presents a public
//     (or operator-provided) certificate.
//   - mesh (ca_file set): the server must chain to the PEM roots in ca_file
//     (the mesh trust bundle) and present an SVID whose single URI SAN equals
//     server_spiffe_id (default spiffe://<trust-domain>/svc/lcm). No host name
//     check. For enrolling directly at lcm's keyless listener, which presents
//     lcm's own SVID (the gateway, which cannot enroll through itself).
//   - insecure: no server verification. Development only; production refuses it.
type EnrollTLS struct {
	Insecure       bool   `json:"insecure" yaml:"insecure"`
	CAFile         string `json:"ca_file" yaml:"ca_file"`
	ServerSPIFFEID string `json:"server_spiffe_id" yaml:"server_spiffe_id"`
}

// Validate refuses contradictory settings, and insecure enrollment when
// production is true.
func (e EnrollTLS) Validate(trustDomain string, production bool) error {
	switch {
	case e.Insecure && e.CAFile != "":
		return errors.New("config: enroll.insecure and enroll.ca_file are mutually exclusive")
	case e.ServerSPIFFEID != "" && e.CAFile == "":
		return errors.New("config: enroll.server_spiffe_id requires enroll.ca_file (the mesh trust bundle)")
	case e.Insecure && production:
		return errors.New("config: enroll.insecure is refused in production; set enroll.ca_file to the mesh trust bundle " +
			"(enrolling at lcm's keyless listener) or enroll through the gateway edge with a verifiable certificate")
	}
	if e.CAFile != "" {
		if _, err := e.ServerID(trustDomain); err != nil {
			return err
		}
	}
	return nil
}

// ServerID returns the SPIFFE ID the enroll server must present in mesh mode:
// server_spiffe_id, or spiffe://<trustDomain>/svc/lcm when it is empty.
func (e EnrollTLS) ServerID(trustDomain string) (identity.SPIFFEID, error) {
	if e.ServerSPIFFEID == "" {
		id, err := identity.NewSPIFFEID(trustDomain, DefaultEnrollServerService)
		if err != nil {
			return identity.SPIFFEID{}, fmt.Errorf("config: enroll server identity: %w", err)
		}
		return id, nil
	}
	id, err := identity.ParseSPIFFEID(e.ServerSPIFFEID)
	if err != nil {
		return identity.SPIFFEID{}, fmt.Errorf("config: enroll.server_spiffe_id: %w", err)
	}
	if id.TrustDomain() != trustDomain {
		return identity.SPIFFEID{}, fmt.Errorf("config: enroll.server_spiffe_id %s is outside trust domain %s", id, trustDomain)
	}
	return id, nil
}

// Warnings lists accepted insecure enrollment settings (logged at start).
func (e EnrollTLS) Warnings() []string {
	if e.Insecure {
		return []string{"enroll.insecure: first enrollment does not verify the server (development only)"}
	}
	return nil
}
