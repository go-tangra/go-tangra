package fuzz

import (
	"testing"

	"github.com/go-tangra/go-tangra/v4/identity"
)

func FuzzParseSPIFFEID(f *testing.F) {
	for _, s := range []string{
		"spiffe://example.org/svc/orders", "spiffe://", "spiffe://x/svc/", "spiffe://例え.jp/svc/a",
		"spiffe://example.org/svc/a-b-c", "SPIFFE://EXAMPLE.ORG/SVC/ORDERS", "spiffe://a/svc/a/../b",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		id, err := identity.ParseSPIFFEID(in)
		if err != nil {
			if !id.IsZero() {
				t.Fatalf("error with non-zero id for %q", in)
			}
			return
		}
		again, err := identity.ParseSPIFFEID(id.String())
		if err != nil {
			t.Fatalf("round trip failed for %q -> %q: %v", in, id.String(), err)
		}
		if !again.Equal(id) {
			t.Fatalf("round trip mismatch %q vs %q", again, id)
		}
		if id.String() != in {
			t.Fatalf("parser must be canonical: %q parsed to %q", in, id.String())
		}
	})
}
