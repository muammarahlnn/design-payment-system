// Command payment-worker consumes payment events from Kafka and (once
// implemented) charges them against the payment gateway, writing the outcome
// back to Postgres.
//
// Scaffolding only for now: config, infrastructure clients, and /health. The
// HTTP server exists purely so the worker exposes a health endpoint — it
// serves no business traffic.
package main

import (
	"log"

	"github.com/muammarahlnn/design-payment-system/internal/config"
	"github.com/muammarahlnn/design-payment-system/internal/health"
	"github.com/muammarahlnn/design-payment-system/internal/infra/kafka"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
	"github.com/muammarahlnn/design-payment-system/internal/server"
)

func main() {
	cfg, err := config.Load(config.Options{
		ServiceName: "payment-worker",
		DefaultPort: "8081",
	})
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}
	defer db.Close()

	// A plain client for now. The consumer group and topic subscription get
	// added with the consumer itself, once the event stream exists.
	kafkaClient, err := kafka.New(cfg.Kafka)
	if err != nil {
		log.Fatalf("init kafka: %v", err)
	}
	defer kafkaClient.Close()

	// TODO: consume the payment event topic, charge cfg.Gateway.URL, persist
	// the result, and handle retries + DLQ.
	log.Printf("%s: gateway configured at %s", cfg.App.Name, cfg.Gateway.URL)

	healthHandler := health.NewHandler(
		health.NewService(
			health.NewRepository(db, kafkaClient),
		),
	)

	srv := server.New(cfg.App)
	healthHandler.RegisterRoutes(srv.Router())

	log.Printf("%s listening on :%s", cfg.App.Name, cfg.App.Port)
	if err := srv.Run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
