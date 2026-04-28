-- +migrate Up
-- Speeds up the address watcher's dedup lookup
-- (network_id, transaction_hash) used in GetTransactionByHashAndNetworkID and
-- GetTransactionByHashNetworkAndRecipient. Partial index — transaction_hash is
-- NULL for the entire pending window.
CREATE INDEX CONCURRENTLY IF NOT EXISTS transactions_network_hash
  ON transactions (network_id, transaction_hash)
  WHERE transaction_hash IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS transactions_network_hash;
