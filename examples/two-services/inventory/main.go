// Command inventory is the callee in the two-services example.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/authn"
	"github.com/go-freya/freya/config"
	inventoryv1 "github.com/go-freya/freya/examples/two-services/api/inventory/v1"
	"github.com/go-freya/freya/observe"
)

type server struct {
	inventoryv1.UnimplementedInventoryServer
	log *slog.Logger
}

func (s *server) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	// The framework guarantees a verified peer here; no security code needed.
	peer, _ := authn.FromContext(ctx)
	s.log.Info("reserve", "sku", req.GetSku(), "qty", req.GetQuantity(),
		"peer", peer.ID.String(), "x-request-id", observe.CorrelationID(ctx))
	return &inventoryv1.ReserveResponse{
		ReservationId: "rsv-" + observe.CorrelationID(ctx),
		ReservedBy:    peer.ServiceName,
	}, nil
}

func main() {
	path := flag.String("config", "inventory.yaml", "config file")
	policy := flag.String("policy", "", "policy file (overrides config authz.path)")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		fail(err)
	}
	if *policy != "" {
		cfg.Authz.Source, cfg.Authz.Path = config.AuthzFile, *policy
	}
	app, err := freya.New(cfg, freya.WithLogger(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		fail(err)
	}
	defer app.Close()
	inventoryv1.RegisterInventoryServer(app.GRPC(), &server{log: slog.New(slog.NewJSONHandler(os.Stdout, nil))})
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if ep, err := app.GRPC().Endpoint(); err == nil {
		fmt.Fprintln(os.Stderr, "listening", ep.String())
	}
	if err := app.Run(ctx); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "inventory:", err)
	os.Exit(1)
}
