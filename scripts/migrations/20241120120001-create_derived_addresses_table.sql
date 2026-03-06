-- +migrate Up
create table if not exists derived_addresses (
    id                    bigserial primary key,
    uuid                  uuid not null unique,

    xpub_wallet_id        bigint not null references xpub_wallets(id),
    merchant_id           bigint not null references merchants(id),
    blockchain            varchar(16) not null,

    address               text not null,
    derivation_path       varchar(128) not null,
    derivation_index      int not null,

    public_key            text null,

    is_used               bool default false,
    payment_id            bigint null,

    tatum_mainnet_subscription_id    varchar(32) null,
    tatum_testnet_subscription_id    varchar(32) null,

    created_at            timestamp(0) not null,
    updated_at            timestamp(0) not null,

    constraint derived_addresses_unique unique (xpub_wallet_id, derivation_index),
    constraint derived_addresses_address_unique unique (blockchain, address)
);

create index derived_addresses_merchant_id on derived_addresses (merchant_id);
create index derived_addresses_xpub_wallet_id on derived_addresses (xpub_wallet_id);
create index derived_addresses_blockchain on derived_addresses (blockchain);
create index derived_addresses_is_used on derived_addresses (is_used);
create index derived_addresses_address on derived_addresses (address);

-- +migrate Down
drop table if exists derived_addresses;
