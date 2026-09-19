// Command payment-service is the API tier of the payment system. It accepts
// payment requests, and (once implemented) writes the payment row and its
// outbox row to Postgres in one transaction, leaving a relay to publish to
// Kafka.
//
// Scaffolding only for now: config, infrastructure clients, and /health.
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
		ServiceName: "payment-service",
		DefaultPort: "8080",
	})
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}
	defer db.Close()

	kafkaClient, err := kafka.New(cfg.Kafka)
	if err != nil {
		log.Fatalf("init kafka: %v", err)
	}
	defer kafkaClient.Close()

	// TODO: migrations, the payments + outbox tables, the HTTP payment API,
	// and the outbox relay that drains to Kafka.

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
