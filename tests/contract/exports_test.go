package contract

import (
	"github.com/go-freya/freya"
	tgrpc "github.com/go-freya/freya/transport/grpc"
	thttp "github.com/go-freya/freya/transport/http"
)

// The export lists are maintained by hand: adding a public constructor or option
// to these packages requires adding it here (and thereby to the contract check).

func freyaExports() map[string]any {
	return map[string]any{
		"New":                   freya.New,
		"WithIdentityProvider":  freya.WithIdentityProvider,
		"WithPolicySource":      freya.WithPolicySource,
		"WithDiscovery":         freya.WithDiscovery,
		"WithAuditSink":         freya.WithAuditSink,
		"WithLogger":            freya.WithLogger,
		"WithInsecureLocalDev":  freya.WithInsecureLocalDev,
		"WithAllowAllPolicy":    freya.WithAllowAllPolicy,
		"WithRevocationChecker": freya.WithRevocationChecker,
		"WithTracerProvider":    freya.WithTracerProvider,
		"App":                   freya.App{},
	}
}

func grpcExports() map[string]any {
	return map[string]any{
		"NewServer":      tgrpc.NewServer,
		"NewPool":        tgrpc.NewPool,
		"WithAddress":    tgrpc.WithAddress,
		"WithNetwork":    tgrpc.WithNetwork,
		"WithMiddleware": tgrpc.WithMiddleware,
		"WithTimeout":    tgrpc.WithTimeout,
		"Server":         tgrpc.Server{},
		"Pool":           tgrpc.Pool{},
	}
}

func httpExports() map[string]any {
	return map[string]any{
		"NewServer":      thttp.NewServer,
		"WithAddress":    thttp.WithAddress,
		"WithNetwork":    thttp.WithNetwork,
		"WithMiddleware": thttp.WithMiddleware,
		"WithTimeout":    thttp.WithTimeout,
		"Server":         thttp.Server{},
	}
}
