-- name: GetPaymentByMerchantIDAndOrderUUID :one
SELECT * FROM payments
WHERE merchant_id = $1 and merchant_order_uuid = $2
LIMIT 1;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1
and (CASE WHEN @filter_by_merchant_id::boolean THEN merchant_id = $2 ELSE true END)
limit 1;

-- name: GetPaymentByPublicID :one
SELECT * FROM payments
WHERE public_id = $1
LIMIT 1;

-- name: GetPaymentByMerchantIDs :one
SELECT * FROM payments WHERE merchant_id = $1 and merchant_order_uuid = $2
LIMIT 1;

-- name: PaginatePaymentsAsc :many
SELECT * from payments
WHERE merchant_id = $1 and id >= $2
AND (CASE WHEN @filter_by_types::boolean THEN type = any(sqlc.arg(type)::varchar[]) ELSE true END)
ORDER BY id LIMIT $3;

-- name: PaginatePaymentsDesc :many
SELECT * from payments
WHERE merchant_id = $1 and id <= $2
AND (CASE WHEN @filter_by_types::boolean THEN type = any(sqlc.arg(type)::varchar[]) ELSE true END)
ORDER BY id desc LIMIT $3;

-- name: GetPaymentsByType :many
SELECT * from payments
where type = $1 and status = $2
and (CASE WHEN @filter_by_ids::boolean THEN id = any(sqlc.arg(id)::int[]) ELSE true END)
order by id limit $3;

-- name: GetBatchExpiredPayments :many
SELECT * from payments
where (
  (expires_at is not null and expires_at < $2)
  or (expires_at is null and created_at < $3)
)
and type = $4
and status = any(sqlc.arg(status)::varchar[])
order by id limit $1;

-- name: CreatePayment :one
INSERT INTO payments (
public_id,
created_at,
updated_at,
type,
status,
merchant_id,
merchant_order_uuid,
merchant_order_id,
expires_at,
price,
decimals,
currency,
description,
redirect_url,
metadata,
is_test
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: UpdatePaymentCustomerID :exec
UPDATE payments set customer_id = $1 where id = $2;

-- name: UpdatePayment :one
UPDATE payments
set status = $3,
updated_at = $4,
expires_at = (CASE WHEN @set_expires_at::boolean THEN $5 ELSE payments.expires_at END)
WHERE id = $1 and merchant_id = $2
returning *;

-- name: UpdatePaymentWebhookInfo :exec
UPDATE payments set webhook_sent_at = $3, updated_at = $4
WHERE id = $1 and merchant_id = $2;

-- Hand-written in payments_webhook.sql.go (see that file for the claim semantics).

-- name: ClaimPaymentWebhook :one
UPDATE payments SET
    webhook_attempts = CASE WHEN webhook_pending_status IS DISTINCT FROM @delivery_key THEN 1 ELSE webhook_attempts + 1 END,
    webhook_pending_status = @delivery_key,
    webhook_next_attempt_at = @lease_until
WHERE id = @id AND merchant_id = @merchant_id AND status = @status
  AND webhook_delivered_status IS DISTINCT FROM @delivery_key
  AND (
    webhook_pending_status IS DISTINCT FROM @delivery_key
    OR ((webhook_next_attempt_at IS NULL OR webhook_next_attempt_at <= @now) AND webhook_attempts < @max_attempts)
  )
RETURNING webhook_attempts;

-- name: MarkPaymentWebhookDelivered :exec
UPDATE payments SET
    webhook_delivered_status = @delivery_key,
    webhook_sent_at = COALESCE(@sent_at, webhook_sent_at),
    webhook_next_attempt_at = NULL
WHERE id = @id AND merchant_id = @merchant_id AND webhook_pending_status = @delivery_key;

-- name: MarkPaymentWebhookFailed :exec
UPDATE payments SET webhook_next_attempt_at = @next_attempt_at
WHERE id = @id AND merchant_id = @merchant_id AND webhook_pending_status = @delivery_key;

-- name: ListPaymentsWithUndeliveredWebhook :many
SELECT id, merchant_id FROM (
  SELECT p.id, p.merchant_id,
    COALESCE(p.webhook_next_attempt_at, p.updated_at) AS due_at,
    row_number() OVER (PARTITION BY p.merchant_id ORDER BY COALESCE(p.webhook_next_attempt_at, p.updated_at), p.id) AS merchant_rank
  FROM payments p
  JOIN merchants m ON m.id = p.merchant_id
  WHERE p.status NOT IN ('pending', 'locked')
    AND p.updated_at > @updated_after AND p.updated_at < @updated_before
    AND m.deleted_at IS NULL
    AND COALESCE(m.settings->>'webhook.url', '') <> ''
    AND (
      split_part(COALESCE(p.webhook_delivered_status, ''), ':', 1) IS DISTINCT FROM p.status
      OR (p.webhook_pending_status IS NOT NULL AND p.webhook_pending_status IS DISTINCT FROM p.webhook_delivered_status)
    )
    AND (p.webhook_next_attempt_at IS NULL OR p.webhook_next_attempt_at <= @now)
    AND NOT (
      split_part(COALESCE(p.webhook_pending_status, ''), ':', 1) = p.status
      AND p.webhook_attempts >= @max_attempts
    )
) due
WHERE merchant_rank <= @per_merchant
ORDER BY due_at, id
LIMIT @row_limit;
