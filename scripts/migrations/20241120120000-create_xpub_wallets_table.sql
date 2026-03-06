-- +migrate Up
create table if not exists xpub_wallets (
    id                    bigserial primary key,
    uuid                  uuid not null unique,
    merchant_id           bigint not null references merchants(id),
    blockchain            varchar(16) not null,

    xpub                  text not null,
    derivation_path       varchar(64) not null,

    created_at            timestamp(0) not null,
    updated_at            timestamp(0) not null,

    tatum_subscription_id varchar(32) null,

    last_derived_index    int default 0,
    is_active             bool default true,

    constraint xpub_wallets_merchant_blockchain unique (merchant_id, blockchain)
);

create index xpub_wallets_merchant_id on xpub_wallets (merchant_id);
create index xpub_wallets_blockchain on xpub_wallets (blockchain);

-- +migrate Down
drop table if exists xpub_wallets;
