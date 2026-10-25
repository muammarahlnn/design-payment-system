package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DBTX is the subset of *sql.DB and *sql.Tx that repositories use. Taking a
// DBTX instead of a concrete *sql.DB lets the same repository run standalone
// or inside a transaction (see the WithTx methods on repositories).
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// RunInTx runs fn inside a single transaction: commit if fn returns nil, roll
// back on error or panic. This is how a caller writes to two repositories
// atomically (e.g. the payment row + its outbox row).
func RunInTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		// sql.ErrTxDone means the driver already finished the transaction —
		// a connection drop, for instance. The rollback is then a no-op, not
		// a second failure worth reporting alongside the real one.
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
