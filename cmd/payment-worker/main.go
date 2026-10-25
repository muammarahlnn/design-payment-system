// Command payment-worker consumes payment.created from Kafka, submits each
// payment to the gateway, and moves it out of PENDING.
//
// The HTTP server exists purely so the worker exposes a health endpoint — it
// serves no business traffic.
package main

import (
	"context"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/muammarahlnn/design-payment-system/internal/config"
	"github.com/muammarahlnn/design-payment-system/internal/event"
	"github.com/muammarahlnn/design-payment-system/internal/gateway"
	"github.com/muammarahlnn/design-payment-system/internal/health"
	"github.com/muammarahlnn/design-payment-system/internal/inbox"
	"github.com/muammarahlnn/design-payment-system/internal/infra/kafka"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
	"github.com/muammarahlnn/design-payment-system/internal/payment"
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

	kafkaClient, err := kafka.New(cfg.Kafka,
		kgo.ConsumerGroup(payment.ConsumerName),
		kgo.ConsumeTopics(event.TopicPaymentCreated),
	)
	if err != nil {
		log.Fatalf("init kafka: %v", err)
	}
	defer kafkaClient.Close()

	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	// The DLQ is not automatic — the consumer produces to it, so it must exist.
	if err := kafka.EnsureTopic(context.Background(), kafkaClient, event.TopicPaymentCreatedDLQ, 1, 1); err != nil {
		log.Fatalf("ensure dlq topic: %v", err)
	}

	log.Printf("%s: gateway at %s (timeout %s)", cfg.App.Name, cfg.Gateway.URL, cfg.Gateway.Timeout)

	// Wire the consumer: gateway client + repositories -> processor -> consumer.
	consumer := payment.NewConsumer(
		kafkaClient,
		kafka.NewProducer(kafkaClient),
		payment.NewProcessor(
			db,
			payment.NewRepository(db),
			inbox.NewRepository(db),
			gateway.New(cfg.Gateway.URL, cfg.Gateway.Timeout),
		),
	)
	go consumer.Run(context.Background())

	// Reconciliation is the backstop. Every other path here is event-driven,
	// and event-driven systems lose events: a callback that never arrives, a
	// crash between charging and committing, a gateway that times out. This
	// loop depends on nothing being delivered — it polls our own database and
	// asks the provider directly.
	gatewayClient := gateway.New(cfg.Gateway.URL, cfg.Gateway.Timeout)
	reconciler := payment.NewReconciler(
		db,
		payment.NewRepository(db),
		inbox.NewRepository(db),
		gatewayClient,
		payment.ReconcilerConfig{
			Interval:    cfg.Reconcile.Interval,
			StaleAfter:  cfg.Reconcile.StaleAfter,
			ExpireAfter: cfg.Reconcile.ExpireAfter,
			BatchSize:   cfg.Reconcile.BatchSize,
		},
	)
	go reconciler.Run(context.Background())

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
