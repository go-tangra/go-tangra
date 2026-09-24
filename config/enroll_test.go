package config

import (
	"strings"
	"testing"
)

func TestEnrollTLSValidate(t *testing.T) {
	const td = "example.org"
	cases := []struct {
		name string
		e    EnrollTLS
		prod bool
		want string // substring of the error; "" = valid
	}{
		{name: "public edge (zero value) dev", e: EnrollTLS{}},
		{name: "public edge (zero value) production", e: EnrollTLS{}, prod: true},
		{name: "mesh ca dev", e: EnrollTLS{CAFile: "/certs/ca.pem"}},
		{name: "mesh ca production", e: EnrollTLS{CAFile: "/certs/ca.pem"}, prod: true},
		{name: "mesh ca explicit id", e: EnrollTLS{CAFile: "/certs/ca.pem", ServerSPIFFEID: "spiffe://example.org/svc/lcm"}, prod: true},
		{name: "insecure dev", e: EnrollTLS{Insecure: true}},
		{name: "insecure production", e: EnrollTLS{Insecure: true}, prod: true, want: "enroll.insecure is refused in production"},
		{name: "insecure with ca", e: EnrollTLS{Insecure: true, CAFile: "/certs/ca.pem"}, want: "enroll.insecure and enroll.ca_file are mutually exclusive"},
		{name: "id without ca", e: EnrollTLS{ServerSPIFFEID: "spiffe://example.org/svc/lcm"}, want: "enroll.server_spiffe_id requires enroll.ca_file"},
		{name: "malformed id", e: EnrollTLS{CAFile: "/c", ServerSPIFFEID: "https://lcm"}, want: "enroll.server_spiffe_id"},
		{name: "foreign trust domain", e: EnrollTLS{CAFile: "/c", ServerSPIFFEID: "spiffe://other.org/svc/lcm"}, want: "outside trust domain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.e.Validate(td, tc.prod)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestEnrollTLSServerID(t *testing.T) {
	id, err := EnrollTLS{CAFile: "/c"}.ServerID("example.org")
	if err != nil || id.String() != "spiffe://example.org/svc/lcm" {
		t.Fatalf("default = %v, %v", id, err)
	}
	id, err = EnrollTLS{CAFile: "/c", ServerSPIFFEID: "spiffe://example.org/svc/lcm-edge"}.ServerID("example.org")
	if err != nil || id.String() != "spiffe://example.org/svc/lcm-edge" {
		t.Fatalf("explicit = %v, %v", id, err)
	}
	if _, err = (EnrollTLS{CAFile: "/c"}).ServerID("NOT A DOMAIN"); err == nil {
		t.Fatal("invalid trust domain accepted")
	}
}

func TestEnrollTLSWarnings(t *testing.T) {
	if w := (EnrollTLS{}).Warnings(); len(w) != 0 {
		t.Fatalf("zero value warns: %v", w)
	}
	w := EnrollTLS{Insecure: true}.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "enroll.insecure") {
		t.Fatalf("warnings = %v", w)
	}
}
