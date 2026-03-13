-- +migrate Up
-- Drop legacy custodial tables no longer used in non-custodial architecture.
-- wallet_locks: Hot wallet locking for inbound payments (replaced by smart contracts/xpub).
-- merchant_addresses: Merchant withdrawal addresses (merchants withdraw directly from their contracts).
-- wallets: System-managed hot wallets (replaced by merchant-deployed smart contracts and xpub derivation).

DROP TABLE IF EXISTS wallet_locks;
DROP TABLE IF EXISTS merchant_addresses;

-- +migrate Down
-- Recreate the legacy tables (schema only, data is unrecoverable).

CREATE TABLE IF NOT EXISTS wallet_locks (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL,
    merchant_id BIGINT NOT NULL,
    currency VARCHAR(64) NOT NULL,
    network_id VARCHAR(64) NOT NULL,
    locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS merchant_addresses (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    merchant_id BIGINT NOT NULL,
    name VARCHAR(256) NOT NULL,
    blockchain VARCHAR(64) NOT NULL,
    address VARCHAR(256) NOT NULL
);
