package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"qr-feedback-api/internal/models"
)

func insertOutbox(ctx context.Context, tx sqlTx, delivery models.IntakeDelivery) error {
	payload, err := json.Marshal(delivery)
	if err != nil {
		return fmt.Errorf("encode intake delivery: %w", err)
	}
	const q = `
INSERT INTO intake_delivery_outbox (receipt_id, intake_type, payload)
VALUES ($1::uuid, $2, $3::jsonb)`
	if _, err := tx.ExecContext(ctx, q, delivery.ReceiptID, delivery.IntakeType, string(payload)); err != nil {
		return fmt.Errorf("queue intake delivery: %w", err)
	}
	return nil
}

// sqlTx is the transaction surface used by raw receipt and outbox writes.
type sqlTx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r *Repo) ListPendingDeliveries(ctx context.Context, limit int) ([]models.PendingDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
SELECT id, payload
FROM intake_delivery_outbox
WHERE delivered_at IS NULL AND next_attempt_at <= now()
ORDER BY id
LIMIT $1`
	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.PendingDelivery, 0)
	for rows.Next() {
		var item models.PendingDelivery
		if err := rows.Scan(&item.ID, &item.Payload); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repo) MarkDeliveryComplete(ctx context.Context, id int64) error {
	const q = `
UPDATE intake_delivery_outbox
SET delivered_at = now(), payload = '{}'::jsonb, last_error_code = NULL
WHERE id = $1 AND delivered_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

func (r *Repo) MarkDeliveryFailed(ctx context.Context, id int64, errorCode string) error {
	const q = `
UPDATE intake_delivery_outbox
SET attempts = attempts + 1,
    next_attempt_at = now() + LEAST(interval '15 minutes', interval '15 seconds' * power(2, LEAST(attempts, 6))),
    last_error_code = $2
WHERE id = $1 AND delivered_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id, trimErrorCode(errorCode))
	return err
}

func trimErrorCode(value string) string {
	if len(value) > 80 {
		return value[:80]
	}
	return value
}
