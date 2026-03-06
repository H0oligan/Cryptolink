-- +migrate Up
CREATE TABLE subscription_plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_usd DECIMAL(10,2) NOT NULL,
    billing_period VARCHAR(20) NOT NULL DEFAULT 'monthly',
    max_payments_monthly INTEGER,
    max_merchants INTEGER DEFAULT 1,
    max_api_calls_monthly INTEGER,
    features JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +migrate Down
DROP TABLE IF EXISTS subscription_plans;
