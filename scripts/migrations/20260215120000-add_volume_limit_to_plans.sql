-- +migrate Up
ALTER TABLE subscription_plans ADD COLUMN max_volume_monthly_usd DECIMAL(20,2);
-- NULL = unlimited, otherwise the USD volume cap per month

-- +migrate Down
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS max_volume_monthly_usd;
