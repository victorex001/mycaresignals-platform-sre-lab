package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrInvalidTransaction = errors.New("repository database and transaction callback are required")

// DBTX is the query surface shared by *sql.DB and *sql.Tx. Phase 019
// authorization and audit operations use it so a future protected handler can
// perform checks, mutation, and audit through the same transaction.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Repo is the central data access object used by handlers.
type Repo struct {
	db *sql.DB
}

// NewRepo constructs a Repo from a *sql.DB.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// TxRepo exposes transaction-scoped Phase 019 storage methods. TOCTOU
// protection is only complete when a protected handler performs relationship
// checks, mutation, and audit using the same transaction.
type TxRepo struct {
	q DBTX
}

func (r *Repo) WithTx(ctx context.Context, opts *sql.TxOptions, fn func(*TxRepo) error) error {
	if r == nil || r.db == nil || fn == nil {
		return ErrInvalidTransaction
	}
	tx, err := r.db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&TxRepo{q: tx}); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
