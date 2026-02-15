-- name: GetMerchantSubscriptionByID :one
SELECT * FROM merchant_subscriptions
WHERE id = $1
LIMIT 1;

-- name: GetMerchantSubscriptionByUUID :one
SELECT * FROM merchant_subscriptions
WHERE uuid = $1
LIMIT 1;

-- name: GetActiveSubscriptionByMerchantID :one
SELECT * FROM merchant_subscriptions
WHERE merchant_id = $1
AND status IN ('active', 'pending_payment')
ORDER BY id DESC
LIMIT 1;

-- name: ListMerchantSubscriptionsByMerchantID :many
SELECT * FROM merchant_subscriptions
WHERE merchant_id = $1
ORDER BY created_at DESC;

-- name: ListExpiringSubscriptions :many
SELECT * FROM merchant_subscriptions
WHERE status = 'active'
AND current_period_end <= $1
AND auto_renew = true
ORDER BY current_period_end ASC;

-- name: CreateMerchantSubscription :one
INSERT INTO merchant_subscriptions (
    uuid,
    merchant_id,
    plan_id,
    status,
    current_period_start,
    current_period_end,
    payment_id,
    auto_renew,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateMerchantSubscription :one
UPDATE merchant_subscriptions
SET
    status = $2,
    current_period_start = $3,
    current_period_end = $4,
    payment_id = $5,
    auto_renew = $6,
    updated_at = $7
WHERE id = $1
RETURNING *;

-- name: CancelMerchantSubscription :exec
UPDATE merchant_subscriptions
SET
    auto_renew = false,
    cancelled_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkSubscriptionExpired :exec
UPDATE merchant_subscriptions
SET
    status = 'expired',
    updated_at = NOW()
WHERE id = $1;

-- name: LinkPaymentToSubscription :exec
UPDATE merchant_subscriptions
SET
    payment_id = $2,
    updated_at = NOW()
WHERE id = $1;
