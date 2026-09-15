package contract

import (
	"crypto"
	"crypto/tls"
	"reflect"
	"strings"
	"testing"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/identity"
)

var keyish = []reflect.Type{
	reflect.TypeOf((*crypto.PrivateKey)(nil)).Elem(),
	reflect.TypeOf((*crypto.Signer)(nil)).Elem(),
	reflect.TypeOf(tls.Certificate{}),
	reflect.TypeOf((*tls.Certificate)(nil)),
	reflect.TypeOf([]byte(nil)),
}

func assertNoKeyAccess(t *testing.T, name string, rt reflect.Type) {
	t.Helper()
	for i := 0; i < rt.NumMethod(); i++ {
		m := rt.Method(i)
		if strings.Contains(strings.ToLower(m.Name), "key") || strings.Contains(strings.ToLower(m.Name), "credential") {
			t.Errorf("%s.%s looks like a key accessor", name, m.Name)
		}
		for j := 0; j < m.Type.NumOut(); j++ {
			for _, k := range keyish {
				if m.Type.Out(j) == k {
					t.Errorf("%s.%s returns %s", name, m.Name, k)
				}
			}
		}
	}
	if rt.Kind() == reflect.Struct {
		for i := 0; i < rt.NumField(); i++ {
			for _, k := range keyish {
				if rt.Field(i).Type == k {
					t.Errorf("%s.%s is %s", name, rt.Field(i).Name, k)
				}
			}
		}
	}
}

func TestIdentityHasNoKeyAccessor(t *testing.T) {
	assertNoKeyAccess(t, "identity.Identity", reflect.TypeOf((*identity.Identity)(nil)).Elem())
	assertNoKeyAccess(t, "identity.Provider", reflect.TypeOf((*identity.Provider)(nil)).Elem())
	assertNoKeyAccess(t, "authn.PeerIdentity", reflect.TypeOf(authn.PeerIdentity{}))
	assertNoKeyAccess(t, "freya.App", reflect.TypeOf(&freya.App{}))
	m, ok := reflect.TypeOf(&freya.App{}).MethodByName("Identity")
	if !ok || m.Type.NumOut() != 1 || m.Type.Out(0) != reflect.TypeOf((*identity.Identity)(nil)).Elem() {
		t.Fatal("App.Identity must return identity.Identity only")
	}
}
