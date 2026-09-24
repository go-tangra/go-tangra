// Command orders is the caller in the two-services example: it reserves stock
// from inventory over the secure channel and exits.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-tangra/go-tangra/v4"
	"github.com/go-tangra/go-tangra/v4/config"
	inventoryv1 "github.com/go-tangra/go-tangra/v4/examples/two-services/api/inventory/v1"
	"github.com/go-tangra/go-tangra/v4/observe"
	"google.golang.org/grpc/metadata"
)

func main() {
	path := flag.String("config", "orders.yaml", "config file")
	inventory := flag.String("inventory", "", "inventory host:port (overrides discovery.static)")
	policy := flag.String("policy", "", "policy file (overrides config authz.path)")
	sku := flag.String("sku", "widget-42", "sku to reserve")
	loop := flag.Duration("loop", 0, "keep calling at this interval (0 = once)")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		fail(err)
	}
	if *inventory != "" {
		cfg.Discovery.Static = map[string][]string{"inventory": {*inventory}}
	}
	if *policy != "" {
		cfg.Authz.Source, cfg.Authz.Path = config.AuthzFile, *policy
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	app, err := freya.New(cfg, freya.WithLogger(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		fail(err)
	}
	defer app.Close()
	ctx := context.Background()
	for {
		if err := reserve(ctx, app, log, *sku); err != nil {
			log.Error("call failed", "err", err.Error())
			if *loop == 0 {
				os.Exit(2)
			}
		}
		if *loop == 0 {
			return
		}
		time.Sleep(*loop)
	}
}

func reserve(ctx context.Context, app *freya.App, log *slog.Logger, sku string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := app.Client(ctx, "inventory")
	if err != nil {
		return err
	}
	cid := observe.NewCorrelationID()
	ctx = observe.WithCorrelationID(ctx, cid)
	var hdr metadata.MD
	resp, err := inventoryv1.NewInventoryClient(conn).Reserve(ctx, &inventoryv1.ReserveRequest{Sku: sku, Quantity: 1}, grpcHeader(&hdr))
	if err != nil {
		return err
	}
	log.Info("call ok", "peer", "spiffe://"+app.TrustDomain()+"/svc/inventory", "op", "/inventory.v1.Inventory/Reserve",
		"x-request-id", cid, "reservation", resp.GetReservationId(), "reserved_by", resp.GetReservedBy())
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "orders:", err)
	os.Exit(1)
}
