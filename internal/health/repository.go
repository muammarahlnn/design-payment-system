package health

import (
	"context"
	"database/sql"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Repository is the data-access layer of the health feature: it reaches out to
// the infrastructure the service depends on and reports whether it responds.
//
// Either dependency may be nil, which means "this service does not use it" —
// the fake-gateway, for instance, talks to neither Postgres nor Kafka.
type Repository struct {
	db    *sql.DB
	kafka *kgo.Client
}

func NewRepository(db *sql.DB, kafka *kgo.Client) *Repository {
	return &Repository{db: db, kafka: kafka}
}

func (r *Repository) HasDatabase() bool { return r.db != nil }

func (r *Repository) HasKafka() bool { return r.kafka != nil }

func (r *Repository) PingDatabase(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *Repository) PingKafka(ctx context.Context) error {
	return r.kafka.Ping(ctx)
}
