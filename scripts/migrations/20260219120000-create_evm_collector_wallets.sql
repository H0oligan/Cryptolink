-- +migrate Up

-- EVM smart contract collector wallets (one per merchant per EVM chain)
CREATE TABLE IF NOT EXISTS evm_collector_wallets (
    id                    bigserial PRIMARY KEY,
    uuid                  uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    merchant_id           bigint NOT NULL REFERENCES merchants(id),
    blockchain            varchar(16) NOT NULL,
    chain_id              int NOT NULL,
    contract_address      varchar(64) NOT NULL,
    owner_address         varchar(64) NOT NULL,
    factory_address       varchar(64) NOT NULL,
    tatum_subscription_id varchar(32) NULL,
    is_active             bool NOT NULL DEFAULT true,
    created_at            timestamp(0) NOT NULL,
    updated_at            timestamp(0) NOT NULL,
    CONSTRAINT evm_collectors_merchant_blockchain UNIQUE (merchant_id, blockchain)
);

CREATE INDEX IF NOT EXISTS evm_collector_wallets_merchant_id ON evm_collector_wallets (merchant_id);
CREATE INDEX IF NOT EXISTS evm_collector_wallets_contract_address ON evm_collector_wallets (contract_address);
CREATE INDEX IF NOT EXISTS evm_collector_wallets_uuid ON evm_collector_wallets (uuid);

-- Enrich transactions table with additional blockchain data
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS block_number bigint NULL;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS confirmations int NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS raw_network_fee_amount numeric(36,18) NULL;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS raw_network_fee_ticker varchar(16) NULL;

-- +migrate Down
DROP TABLE IF EXISTS evm_collector_wallets;
ALTER TABLE transactions DROP COLUMN IF EXISTS block_number;
ALTER TABLE transactions DROP COLUMN IF EXISTS confirmations;
ALTER TABLE transactions DROP COLUMN IF EXISTS raw_network_fee_amount;
ALTER TABLE transactions DROP COLUMN IF EXISTS raw_network_fee_ticker;
