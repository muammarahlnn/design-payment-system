// Command fake-gateway stands in for a real payment provider (Stripe, Adyen,
// Midtrans). The worker charges against it, so it is the place to simulate
// latency, declines, timeouts, and duplicate-charge protection.
//
// It depends on no infrastructure and keeps charges in memory. Charging twice
// with the same Idempotency-Key replays the first charge, which is what makes
// the worker safe to retry.
package main

import (
	"log"

	"github.com/muammarahlnn/design-payment-system/internal/config"
	"github.com/muammarahlnn/design-payment-system/internal/fakegateway"
	"github.com/muammarahlnn/design-payment-system/internal/health"
	"github.com/muammarahlnn/design-payment-system/internal/server"
)

func main() {
	cfg, err := config.Load(config.Options{
		ServiceName: "fake-gateway",
		DefaultPort: "8082",
	})
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// No Postgres, no Kafka: the gateway is a standalone HTTP stub, so the
	// health check has nothing external to ping.
	healthHandler := health.NewHandler(
		health.NewService(
			health.NewRepository(nil, nil),
		),
	)

	// POST /charges. Idempotent by Idempotency-Key, like a real provider.
	chargeHandler := fakegateway.NewHandler(
		cfg.Gateway.FailMode,
		cfg.Gateway.StorePath,
		fakegateway.NewNotifier(cfg.Gateway.WebhookURL, cfg.Gateway.WebhookSecret),
	)

	srv := server.New(cfg.App)
	healthHandler.RegisterRoutes(srv.Router())
	chargeHandler.RegisterRoutes(srv.Router())

	log.Printf("%s listening on :%s", cfg.App.Name, cfg.App.Port)
	if err := srv.Run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
