package transport

import (
	"net"
	"strings"
	"testing"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	return lis
}

func TestResolveEndpoint(t *testing.T) {
	lis := listen(t)
	_, port, _ := net.SplitHostPort(lis.Addr().String())

	// A concrete configured host is advertised as-is (the override never applies).
	t.Setenv(AdvertiseHostEnv, "ignored.example")
	if got := ResolveEndpoint("10.1.2.3:9999", lis); got != net.JoinHostPort("10.1.2.3", port) {
		t.Fatalf("concrete host: %s", got)
	}

	// Unspecified host + override: the override is advertised with the bound port.
	for _, configured := range []string{"0.0.0.0:9544", ":9544", "[::]:9544"} {
		if got := ResolveEndpoint(configured, lis); got != net.JoinHostPort("ignored.example", port) {
			t.Fatalf("%s with override: %s", configured, got)
		}
	}

	// Unspecified host, no override: an interface address (or loopback), never empty.
	t.Setenv(AdvertiseHostEnv, "")
	got := ResolveEndpoint("0.0.0.0:9544", lis)
	if !strings.HasSuffix(got, ":"+port) || strings.HasPrefix(got, ":") {
		t.Fatalf("default: %s", got)
	}
}
