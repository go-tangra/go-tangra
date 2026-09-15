package identity

import "testing"

func TestZeroIDString(t *testing.T) {
	if (SPIFFEID{}).String() != "" {
		t.Fatal("zero id must print empty")
	}
	if !ValidTrustDomain("example.org") || ValidTrustDomain("") || !ValidServiceName("a") || ValidServiceName("-") {
		t.Fatal("validators broken")
	}
	e := &VerifyError{Reason: ReasonExpired}
	if e.Error() != "identity: peer refused: identity_expired" {
		t.Fatalf("got %q", e.Error())
	}
	e.Detail = "skew"
	if e.Error() != "identity: peer refused: identity_expired (skew)" {
		t.Fatalf("got %q", e.Error())
	}
}
