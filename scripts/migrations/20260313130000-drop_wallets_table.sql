-- +migrate Up
-- Drop the wallets table. Hot wallets have been fully removed from CryptoLink
-- (non-custodial architecture). All wallet-related Go code and SQL queries have
-- been deleted. The table is empty in production.

DROP TABLE IF EXISTS wallets;

-- +migrate Down
-- Recreate the wallets table (schema only, data is unrecoverable).

CREATE TABLE IF NOT EXISTS wallets (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uuid UUID NOT NULL UNIQUE,
    address VARCHAR(256) NOT NULL,
    blockchain VARCHAR(64) NOT NULL,
    type VARCHAR(64) NOT NULL DEFAULT 'inbound',
    confirmed_mainnet_transactions BIGINT NOT NULL DEFAULT 0,
    pending_mainnet_transactions BIGINT NOT NULL DEFAULT 0,
    confirmed_testnet_transactions BIGINT NOT NULL DEFAULT 0,
    pending_testnet_transactions BIGINT NOT NULL DEFAULT 0
);
