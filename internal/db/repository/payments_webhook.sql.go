// Hand-written queries (source: scripts/queries/payments.sql). Kept out of
// payments.sql.go so a sqlc regen can't silently drop them.

package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

const claimPaymentWebhook = `-- name: ClaimPaymentWebhook :one
UPDATE payments SET
    webhook_attempts = CASE WHEN webhook_pending_status IS DISTINCT FROM $4 THEN 1 ELSE webhook_attempts + 1 END,
    webhook_pending_status = $4,
    webhook_next_attempt_at = $5
WHERE id = $1 AND merchant_id = $2 AND status = $3
  AND webhook_delivered_status IS DISTINCT FROM $4
  AND (
    webhook_pending_status IS DISTINCT FROM $4
    OR ((webhook_next_attempt_at IS NULL OR webhook_next_attempt_at <= $6) AND webhook_attempts < $7)
  )
RETURNING webhook_attempts
`

type ClaimPaymentWebhookParams struct {
	ID          int64
	MerchantID  int64
	Status      string
	DeliveryKey string
	LeaseUntil  time.Time
	Now         time.Time
	MaxAttempts int32
}

// ClaimPaymentWebhook atomically takes the right to send DeliveryKey for a
// payment. It returns pgx.ErrNoRows when the key was already delivered, the
// payment status moved on, another sender holds the lease, or retries are
// exhausted. Only the claimer may send.
func (q *Queries) ClaimPaymentWebhook(ctx context.Context, arg ClaimPaymentWebhookParams) (int32, error) {
	row := q.db.QueryRow(ctx, claimPaymentWebhook,
		arg.ID,
		arg.MerchantID,
		arg.Status,
		arg.DeliveryKey,
		arg.LeaseUntil,
		arg.Now,
		arg.MaxAttempts,
	)
	var attempts int32
	err := row.Scan(&attempts)
	return attempts, err
}

const markPaymentWebhookDelivered = `-- name: MarkPaymentWebhookDelivered :exec
UPDATE payments SET
    webhook_delivered_status = $3,
    webhook_sent_at = COALESCE($4, webhook_sent_at),
    webhook_next_attempt_at = NULL
WHERE id = $1 AND merchant_id = $2 AND webhook_pending_status = $3
`

type MarkPaymentWebhookDeliveredParams struct {
	ID          int64
	MerchantID  int64
	DeliveryKey string
	SentAt      sql.NullTime
}

// MarkPaymentWebhookDelivered is a no-op when a newer key has been claimed in
// the meantime, so a slow stale send can't overwrite a newer delivery.
func (q *Queries) MarkPaymentWebhookDelivered(ctx context.Context, arg MarkPaymentWebhookDeliveredParams) error {
	_, err := q.db.Exec(ctx, markPaymentWebhookDelivered,
		arg.ID,
		arg.MerchantID,
		arg.DeliveryKey,
		arg.SentAt,
	)
	return err
}

const markPaymentWebhookFailed = `-- name: MarkPaymentWebhookFailed :exec
UPDATE payments SET webhook_next_attempt_at = $4
WHERE id = $1 AND merchant_id = $2 AND webhook_pending_status = $3
`

type MarkPaymentWebhookFailedParams struct {
	ID            int64
	MerchantID    int64
	DeliveryKey   string
	NextAttemptAt sql.NullTime
}

func (q *Queries) MarkPaymentWebhookFailed(ctx context.Context, arg MarkPaymentWebhookFailedParams) error {
	_, err := q.db.Exec(ctx, markPaymentWebhookFailed,
		arg.ID,
		arg.MerchantID,
		arg.DeliveryKey,
		arg.NextAttemptAt,
	)
	return err
}

const listPaymentsWithUndeliveredWebhook = `-- name: ListPaymentsWithUndeliveredWebhook :many
SELECT id, merchant_id FROM (
  SELECT p.id, p.merchant_id,
    COALESCE(p.webhook_next_attempt_at, p.updated_at) AS due_at,
    row_number() OVER (PARTITION BY p.merchant_id ORDER BY COALESCE(p.webhook_next_attempt_at, p.updated_at), p.id) AS merchant_rank
  FROM payments p
  JOIN merchants m ON m.id = p.merchant_id
  WHERE p.status NOT IN ('pending', 'locked')
    AND p.updated_at > $1 AND p.updated_at < $2
    AND m.deleted_at IS NULL
    AND COALESCE(m.settings->>'webhook.url', '') <> ''
    AND (
      split_part(COALESCE(p.webhook_delivered_status, ''), ':', 1) IS DISTINCT FROM p.status
      OR (p.webhook_pending_status IS NOT NULL AND p.webhook_pending_status IS DISTINCT FROM p.webhook_delivered_status)
    )
    AND (p.webhook_next_attempt_at IS NULL OR p.webhook_next_attempt_at <= $3)
    AND NOT (
      split_part(COALESCE(p.webhook_pending_status, ''), ':', 1) = p.status
      AND p.webhook_attempts >= $4
    )
) due
WHERE merchant_rank <= $5
ORDER BY due_at, id
LIMIT $6
`

type ListPaymentsWithUndeliveredWebhookParams struct {
	UpdatedAfter  time.Time
	UpdatedBefore time.Time
	Now           time.Time
	MaxAttempts   int32
	PerMerchant   int32
	RowLimit      int32
}

type ListPaymentsWithUndeliveredWebhookRow struct {
	ID         int64
	MerchantID int64
}

func (q *Queries) ListPaymentsWithUndeliveredWebhook(
	ctx context.Context,
	arg ListPaymentsWithUndeliveredWebhookParams,
) ([]ListPaymentsWithUndeliveredWebhookRow, error) {
	rows, err := q.db.Query(ctx, listPaymentsWithUndeliveredWebhook,
		arg.UpdatedAfter,
		arg.UpdatedBefore,
		arg.Now,
		arg.MaxAttempts,
		arg.PerMerchant,
		arg.RowLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListPaymentsWithUndeliveredWebhookRow
	for rows.Next() {
		var i ListPaymentsWithUndeliveredWebhookRow
		if err := rows.Scan(&i.ID, &i.MerchantID); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const paymentWebhookStateColumns = `p.id, p.public_id, p.merchant_id, p.status, p.webhook_attempts,
    p.webhook_sent_at, p.webhook_delivered_status, p.webhook_next_attempt_at, p.updated_at`

// PaymentWebhookState is the support view of a payment's webhook delivery.
type PaymentWebhookState struct {
	ID                     int64
	PublicID               uuid.UUID
	MerchantID             int64
	Status                 string
	WebhookAttempts        int32
	WebhookSentAt          sql.NullTime
	WebhookDeliveredStatus sql.NullString
	WebhookNextAttemptAt   sql.NullTime
	UpdatedAt              time.Time
}

type paymentWebhookStateScanner interface {
	Scan(dest ...any) error
}

func scanPaymentWebhookState(row paymentWebhookStateScanner) (PaymentWebhookState, error) {
	var i PaymentWebhookState
	err := row.Scan(
		&i.ID, &i.PublicID, &i.MerchantID, &i.Status, &i.WebhookAttempts,
		&i.WebhookSentAt, &i.WebhookDeliveredStatus, &i.WebhookNextAttemptAt, &i.UpdatedAt,
	)
	return i, err
}

func (q *Queries) GetPaymentWebhookStateByPublicID(ctx context.Context, publicID uuid.UUID) (PaymentWebhookState, error) {
	return scanPaymentWebhookState(q.db.QueryRow(ctx,
		`SELECT `+paymentWebhookStateColumns+` FROM payments p WHERE p.public_id = $1`, publicID,
	))
}

// ListPaymentsAwaitingWebhook returns payments the merchant has not acknowledged:
// those still retrying and those that gave up, which the retry job no longer picks up.
func (q *Queries) ListPaymentsAwaitingWebhook(ctx context.Context, rowLimit int32) ([]PaymentWebhookState, error) {
	rows, err := q.db.Query(ctx, `SELECT `+paymentWebhookStateColumns+`
FROM payments p
JOIN merchants m ON m.id = p.merchant_id
WHERE p.status NOT IN ('pending', 'locked')
  AND m.deleted_at IS NULL
  AND COALESCE(m.settings->>'webhook.url', '') <> ''
  AND (
    split_part(COALESCE(p.webhook_delivered_status, ''), ':', 1) IS DISTINCT FROM p.status
    OR (p.webhook_pending_status IS NOT NULL AND p.webhook_pending_status IS DISTINCT FROM p.webhook_delivered_status)
  )
ORDER BY p.updated_at DESC, p.id DESC
LIMIT $1`, rowLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []PaymentWebhookState{}
	for rows.Next() {
		i, err := scanPaymentWebhookState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// ResetPaymentWebhook makes a settled payment's webhook claimable again, with a
// fresh retry budget. Pending and locked payments have nothing to deliver.
func (q *Queries) ResetPaymentWebhook(ctx context.Context, merchantID, id int64) (int64, error) {
	tag, err := q.db.Exec(ctx, `UPDATE payments SET
    webhook_delivered_status = NULL,
    webhook_pending_status = NULL,
    webhook_next_attempt_at = NULL,
    webhook_attempts = 0
WHERE id = $1 AND merchant_id = $2 AND status NOT IN ('pending', 'locked')`, id, merchantID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
