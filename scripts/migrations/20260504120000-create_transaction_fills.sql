-- +migrate Up
-- Partial-payment support: track each on-chain transfer that contributes to
-- an invoice. Allows a customer to top up an underpayment in one or more
-- subsequent transactions to the SAME destination address, possibly from a
-- DIFFERENT sender wallet, until the cumulative confirmed amount reaches the
-- invoice's expected amount.
--
-- Idempotent on (transaction_id, network_id, transaction_hash, vout_or_logidx)
-- so re-detection by the watcher cannot double-count, and a single batch payer
-- tx with multiple outputs to the same address counts each output once.
CREATE TABLE IF NOT EXISTS transaction_fills (
    id BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    network_id VARCHAR(16) NOT NULL,
    transaction_hash VARCHAR(128) NOT NULL,
    vout_or_logidx INTEGER NOT NULL DEFAULT 0,
    amount NUMERIC(64,0) NOT NULL,
    sender_address VARCHAR(128),
    block_number BIGINT,
    confirmations INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'observed', -- observed | confirmed | reorged
    observed_at TIMESTAMP NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMP,
    CONSTRAINT transaction_fills_unique UNIQUE (transaction_id, network_id, transaction_hash, vout_or_logidx)
);

CREATE INDEX IF NOT EXISTS transaction_fills_tx ON transaction_fills (transaction_id);
CREATE INDEX IF NOT EXISTS transaction_fills_hash ON transaction_fills (network_id, transaction_hash);

-- Track the original expiry so per-fill extensions can be hard-capped (+24h).
ALTER TABLE payments ADD COLUMN IF NOT EXISTS original_expires_at TIMESTAMP;
UPDATE payments SET original_expires_at = expires_at WHERE original_expires_at IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS transaction_fills_hash;
DROP INDEX IF EXISTS transaction_fills_tx;
DROP TABLE IF EXISTS transaction_fills;
ALTER TABLE payments DROP COLUMN IF EXISTS original_expires_at;
