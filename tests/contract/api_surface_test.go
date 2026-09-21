package contract

import (
	"crypto/tls"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// forbidden types that must never appear in any exported signature of the
// packages below: a caller could otherwise supply or extract transport security.
var forbidden = []reflect.Type{
	reflect.TypeOf((*tls.Config)(nil)),
	reflect.TypeOf(tls.Config{}),
	reflect.TypeOf(tls.Certificate{}),
	reflect.TypeOf((*tls.Certificate)(nil)),
	reflect.TypeOf((*grpc.DialOption)(nil)).Elem(),
	reflect.TypeOf((*credentials.TransportCredentials)(nil)).Elem(),
}

func mentionsForbidden(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	for _, f := range forbidden {
		if t == f {
			return true
		}
	}
	switch t.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Chan:
		return mentionsForbidden(t.Elem(), seen)
	case reflect.Map:
		return mentionsForbidden(t.Key(), seen) || mentionsForbidden(t.Elem(), seen)
	case reflect.Func:
		for i := 0; i < t.NumIn(); i++ {
			if mentionsForbidden(t.In(i), seen) {
				return true
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if mentionsForbidden(t.Out(i), seen) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).IsExported() && mentionsForbidden(t.Field(i).Type, seen) {
				return true
			}
		}
	}
	return false
}

func checkValues(t *testing.T, pkg string, vals map[string]any) {
	t.Helper()
	for name, v := range vals {
		rt := reflect.TypeOf(v)
		if mentionsForbidden(rt, map[reflect.Type]bool{}) {
			t.Errorf("%s.%s exposes transport security in its signature: %s", pkg, name, rt)
		}
		// Methods of returned types.
		for i := 0; i < rt.NumMethod(); i++ {
			m := rt.Method(i)
			if mentionsForbidden(m.Type, map[reflect.Type]bool{}) {
				t.Errorf("%s.%s.%s exposes transport security: %s", pkg, name, m.Name, m.Type)
			}
		}
		if rt.Kind() == reflect.Func {
			for i := 0; i < rt.NumOut(); i++ {
				out := rt.Out(i)
				for j := 0; j < out.NumMethod(); j++ {
					m := out.Method(j)
					if strings.HasPrefix(m.Name, "TLS") || mentionsForbidden(m.Type, map[reflect.Type]bool{}) {
						t.Errorf("%s.%s returns %s with method %s exposing transport security", pkg, name, out, m.Name)
					}
				}
			}
		}
	}
}

func TestNoPlaintextConstructor(t *testing.T) {
	checkValues(t, "freya", freyaExports())
	checkValues(t, "transport/grpc", grpcExports())
	checkValues(t, "transport/http", httpExports())
	checkValues(t, "transport/edge", edgeExports())
}
