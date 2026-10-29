-- +migrate Up
-- Durable webhook delivery state so failed merchant webhooks are retried
-- instead of being dropped, and an already-delivered status is never sent
-- again.
--
-- A delivery key is the payment status, or "partial:<fill idempotency key>"
-- for partial fills (one partial webhook per fill).
--   webhook_delivered_status: last key the merchant acknowledged with 2xx
--   webhook_pending_status:   key currently claimed / awaiting retry
--   webhook_next_attempt_at:  in-flight lease, then retry backoff
--   webhook_attempts:         attempts for webhook_pending_status
ALTER TABLE payments ADD COLUMN IF NOT EXISTS webhook_delivered_status varchar(128);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS webhook_pending_status varchar(128);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS webhook_next_attempt_at timestamp;

-- Existing payments were handled by the old fire-and-forget sender; treat
-- them as delivered so the retry job doesn't replay history.
UPDATE payments SET webhook_delivered_status = status WHERE webhook_delivered_status IS NULL;

-- +migrate Down
ALTER TABLE payments DROP COLUMN IF EXISTS webhook_next_attempt_at;
ALTER TABLE payments DROP COLUMN IF EXISTS webhook_pending_status;
ALTER TABLE payments DROP COLUMN IF EXISTS webhook_delivered_status;
