-- +migrate Up
CREATE TABLE usage_tracking (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    payment_count INTEGER DEFAULT 0,
    payment_volume_usd DECIMAL(20,2) DEFAULT 0,
    api_calls_count INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT usage_tracking_unique UNIQUE (merchant_id, period_start),
    CONSTRAINT fk_merchant_usage FOREIGN KEY (merchant_id) REFERENCES merchants(id) ON DELETE CASCADE
);

CREATE INDEX usage_tracking_merchant_id ON usage_tracking(merchant_id);
CREATE INDEX usage_tracking_period ON usage_tracking(merchant_id, period_start, period_end);

-- +migrate Down
DROP INDEX IF EXISTS usage_tracking_period;
DROP INDEX IF EXISTS usage_tracking_merchant_id;
DROP TABLE IF EXISTS usage_tracking;
