// Command payment-service is the API tier of the payment system. It accepts
// payment requests, records them as PENDING keyed by the client's idempotency
// key, and queues a payment.created event in the same transaction. A relay
// goroutine ships queued events to Kafka.
package main

import (
	"context"
	"log"

	"github.com/muammarahlnn/design-payment-system/internal/config"
	"github.com/muammarahlnn/design-payment-system/internal/event"
	"github.com/muammarahlnn/design-payment-system/internal/health"
	"github.com/muammarahlnn/design-payment-system/internal/idempotency"
	"github.com/muammarahlnn/design-payment-system/internal/inbox"
	"github.com/muammarahlnn/design-payment-system/internal/infra/kafka"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
	"github.com/muammarahlnn/design-payment-system/internal/outbox"
	"github.com/muammarahlnn/design-payment-system/internal/payment"
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

	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	kafkaClient, err := kafka.New(cfg.Kafka)
	if err != nil {
		log.Fatalf("init kafka: %v", err)
	}
	defer kafkaClient.Close()

	// payment-service owns the payment.created stream, so it declares the
	// topic (1 partition, replication 1 — single-broker dev setup).
	if err := kafka.EnsureTopic(context.Background(), kafkaClient, event.TopicPaymentCreated, 1, 1); err != nil {
		log.Fatalf("ensure kafka topic: %v", err)
	}

	outboxRepo := outbox.NewRepository(db)

	healthHandler := health.NewHandler(
		health.NewService(
			health.NewRepository(db, kafkaClient),
		),
	)

	// Wire the payment feature: repositories -> service -> handler. The
	// service writes the payment row and its idempotency key in one
	// transaction, so it needs the pool as well as the repositories.
	paymentHandler := payment.NewHandler(
		payment.NewService(
			db,
			payment.NewRepository(db),
			idempotency.NewRepository(db),
			outboxRepo,
		),
	)

	// Gateway callbacks land here: POST /webhook/payment.
	webhookHandler := payment.NewWebhookHandler(
		payment.NewWebhookService(db, payment.NewRepository(db), inbox.NewRepository(db)),
		cfg.Gateway.WebhookSecret,
	)

	// The relay drains the outbox table to Kafka (at-least-once delivery).
	relay := outbox.NewRelay(db, outboxRepo, kafka.NewProducer(kafkaClient))
	go relay.Run(context.Background())

	srv := server.New(cfg.App)
	healthHandler.RegisterRoutes(srv.Router())
	paymentHandler.RegisterRoutes(srv.Router())
	webhookHandler.RegisterRoutes(srv.Router())
	payment.NewQueryHandler(payment.NewRepository(db)).RegisterRoutes(srv.Router())

	log.Printf("%s listening on :%s", cfg.App.Name, cfg.App.Port)
	if err := srv.Run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
