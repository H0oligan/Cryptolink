-- name: GetSubscriptionPlanByID :one
SELECT * FROM subscription_plans
WHERE id = $1 AND is_active = true
LIMIT 1;

-- name: ListSubscriptionPlans :many
SELECT * FROM subscription_plans
WHERE is_active = true
ORDER BY price_usd ASC;

-- name: ListAllSubscriptionPlans :many
SELECT * FROM subscription_plans
ORDER BY price_usd ASC;

-- name: UpdateSubscriptionPlan :exec
UPDATE subscription_plans
SET
    name = $2,
    description = $3,
    price_usd = $4,
    max_payments_monthly = $5,
    max_merchants = $6,
    max_api_calls_monthly = $7,
    features = $8,
    is_active = $9,
    updated_at = NOW()
WHERE id = $1;
