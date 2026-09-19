package postgres

import (
	"database/sql"

	_ "github.com/lib/pq"

	"github.com/muammarahlnn/design-payment-system/internal/config"
)

// New opens a Postgres connection pool from the injected config. It does not
// verify connectivity; callers (such as the health check) ping when needed.
func New(cfg config.PostgresConfig) (*sql.DB, error) {
	return sql.Open("postgres", cfg.DSN())
}
