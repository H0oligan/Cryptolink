-- +migrate Up

-- Remove Tatum subscription columns from wallets table
ALTER TABLE wallets DROP COLUMN IF EXISTS tatum_mainnet_subscription_id;
ALTER TABLE wallets DROP COLUMN IF EXISTS tatum_testnet_subscription_id;

-- Remove Tatum subscription column from xpub_wallets table
ALTER TABLE xpub_wallets DROP COLUMN IF EXISTS tatum_subscription_id;

-- Remove Tatum subscription columns from derived_addresses table
ALTER TABLE derived_addresses DROP COLUMN IF EXISTS tatum_mainnet_subscription_id;
ALTER TABLE derived_addresses DROP COLUMN IF EXISTS tatum_testnet_subscription_id;

-- Remove Tatum subscription column from evm_collector_wallets table
ALTER TABLE evm_collector_wallets DROP COLUMN IF EXISTS tatum_subscription_id;

-- +migrate Down

ALTER TABLE wallets
    ADD COLUMN tatum_mainnet_subscription_id varchar(32) NULL,
    ADD COLUMN tatum_testnet_subscription_id varchar(32) NULL;

ALTER TABLE xpub_wallets
    ADD COLUMN tatum_subscription_id varchar(32) NULL;

ALTER TABLE derived_addresses
    ADD COLUMN tatum_mainnet_subscription_id varchar(32) NULL,
    ADD COLUMN tatum_testnet_subscription_id varchar(32) NULL;

ALTER TABLE evm_collector_wallets
    ADD COLUMN tatum_subscription_id varchar(32) NULL;
