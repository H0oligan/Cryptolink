-- name: GetUsageTrackingByID :one
SELECT * FROM usage_tracking
WHERE id = $1
LIMIT 1;

-- name: GetUsageTrackingByMerchantAndPeriod :one
SELECT * FROM usage_tracking
WHERE merchant_id = $1
AND period_start = $2
LIMIT 1;

-- name: GetCurrentPeriodUsage :one
SELECT * FROM usage_tracking
WHERE merchant_id = $1
AND period_start <= $2
AND period_end >= $2
ORDER BY period_start DESC
LIMIT 1;

-- name: ListUsageTrackingByMerchantID :many
SELECT * FROM usage_tracking
WHERE merchant_id = $1
ORDER BY period_start DESC
LIMIT $2;

-- name: CreateUsageTracking :one
INSERT INTO usage_tracking (
    merchant_id,
    period_start,
    period_end,
    payment_count,
    payment_volume_usd,
    api_calls_count,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: IncrementPaymentUsage :exec
UPDATE usage_tracking
SET
    payment_count = payment_count + 1,
    payment_volume_usd = payment_volume_usd + $2,
    updated_at = NOW()
WHERE merchant_id = $1
AND period_start <= NOW()
AND period_end >= NOW();

-- name: IncrementAPIUsage :exec
UPDATE usage_tracking
SET
    api_calls_count = api_calls_count + $2,
    updated_at = NOW()
WHERE merchant_id = $1
AND period_start <= NOW()
AND period_end >= NOW();

-- name: UpdateUsageTracking :exec
UPDATE usage_tracking
SET
    payment_count = $2,
    payment_volume_usd = $3,
    api_calls_count = $4,
    updated_at = NOW()
WHERE id = $1;
