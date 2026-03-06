-- +migrate Up
CREATE TABLE merchant_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT gen_random_uuid(),
    merchant_id BIGINT NOT NULL,
    plan_id VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    current_period_start TIMESTAMP NOT NULL,
    current_period_end TIMESTAMP NOT NULL,
    payment_id BIGINT,
    auto_renew BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    cancelled_at TIMESTAMP,
    CONSTRAINT fk_merchant FOREIGN KEY (merchant_id) REFERENCES merchants(id) ON DELETE CASCADE,
    CONSTRAINT fk_plan FOREIGN KEY (plan_id) REFERENCES subscription_plans(id),
    CONSTRAINT fk_payment FOREIGN KEY (payment_id) REFERENCES payments(id)
);

CREATE UNIQUE INDEX merchant_subscriptions_uuid_key ON merchant_subscriptions(uuid);
CREATE UNIQUE INDEX merchant_subscriptions_active_unique ON merchant_subscriptions(merchant_id) WHERE status IN ('active', 'pending_payment');
CREATE INDEX merchant_subscriptions_merchant_id ON merchant_subscriptions(merchant_id);
CREATE INDEX merchant_subscriptions_status ON merchant_subscriptions(status);
CREATE INDEX merchant_subscriptions_period ON merchant_subscriptions(current_period_start, current_period_end);

-- +migrate Down
DROP INDEX IF EXISTS merchant_subscriptions_period;
DROP INDEX IF EXISTS merchant_subscriptions_status;
DROP INDEX IF EXISTS merchant_subscriptions_merchant_id;
DROP INDEX IF EXISTS merchant_subscriptions_active_unique;
DROP INDEX IF EXISTS merchant_subscriptions_uuid_key;
DROP TABLE IF EXISTS merchant_subscriptions;
