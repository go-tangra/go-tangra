package identity

import (
	"strings"
	"testing"
)

func TestParseSPIFFEIDValid(t *testing.T) {
	id, err := ParseSPIFFEID("spiffe://example.org/svc/orders")
	if err != nil {
		t.Fatal(err)
	}
	if id.TrustDomain() != "example.org" || id.ServiceName() != "orders" {
		t.Fatalf("unexpected parts: %q %q", id.TrustDomain(), id.ServiceName())
	}
	if id.String() != "spiffe://example.org/svc/orders" {
		t.Fatalf("round trip: %q", id.String())
	}
	if id.IsZero() {
		t.Fatal("parsed id must not be zero")
	}
	if (SPIFFEID{}).IsZero() == false {
		t.Fatal("zero value must be zero")
	}
}

func TestParseSPIFFEIDInvalid(t *testing.T) {
	long := strings.Repeat("a", 64)
	cases := map[string]string{
		"":                                                "empty",
		"http://example.org/svc/orders":                   "scheme",
		"spiffe://example.org/orders":                     "/svc/",
		"spiffe://example.org":                            "/svc/",
		"spiffe://example.org/svc/":                       "service name",
		"spiffe://example.org/svc/Orders":                 "service name",
		"spiffe://example.org/svc/-orders":                "service name",
		"spiffe://example.org/svc/orders-":                "service name",
		"spiffe://example.org/svc/or_ders":                "service name",
		"spiffe://example.org/svc/" + long:                "service name",
		"spiffe://Example.org/svc/orders":                 "trust domain",
		"spiffe:///svc/orders":                            "trust domain",
		"spiffe://exa mple.org/svc/orders":                "trust domain",
		"spiffe://example.org/svc/orders/extra":           "service name",
		"spiffe://example.org/svc/orders?x=1":             "service name",
		"spiffe://example.org/svc/orders#frag":            "service name",
		"spiffe://example.org:8080/svc/orders":            "trust domain",
		"spiffe://user@example.org/svc/orders":            "trust domain",
		"spiffe://" + strings.Repeat("a", 256) + "/svc/x": "trust domain",
	}
	for in, want := range cases {
		if _, err := ParseSPIFFEID(in); err == nil {
			t.Errorf("%q: expected error", in)
		} else if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q does not mention %q", in, err, want)
		}
	}
}

func TestMustSPIFFEID(t *testing.T) {
	id := ForService("example.org", "orders")
	if id.String() != "spiffe://example.org/svc/orders" {
		t.Fatalf("got %q", id)
	}
	if !id.Equal(ForService("example.org", "orders")) || id.Equal(ForService("example.org", "billing")) {
		t.Fatal("Equal broken")
	}
	if _, err := NewSPIFFEID("bad domain", "orders"); err == nil {
		t.Fatal("expected error for bad trust domain")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("ForService must panic on invalid input")
		}
	}()
	ForService("example.org", "BAD")
}
