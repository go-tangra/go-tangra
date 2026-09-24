package contract

import (
	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/transport/edge"
	tgrpc "github.com/go-tangra/go-tangra/v4/transport/grpc"
	thttp "github.com/go-tangra/go-tangra/v4/transport/http"
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
		"NewClient":      thttp.NewClient,
		"RequestTimeout": thttp.RequestTimeout,
	}
}

func edgeExports() map[string]any {
	return map[string]any{
		"NewServer":       edge.NewServer,
		"WithMiddleware":  edge.WithMiddleware,
		"IssueCSRFCookie": edge.IssueCSRFCookie,
		"Nonce":           edge.Nonce,
		"WithNonce":       edge.WithNonce,
		"Server":          edge.Server{},
		"Config":          edge.Config{},
	}
}
