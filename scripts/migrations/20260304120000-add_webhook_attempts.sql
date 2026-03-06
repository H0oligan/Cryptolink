-- +migrate Up
ALTER TABLE payments ADD COLUMN IF NOT EXISTS webhook_attempts integer NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE payments DROP COLUMN IF EXISTS webhook_attempts;
