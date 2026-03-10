-- +migrate Up

CREATE TABLE IF NOT EXISTS collector_factories (
    id                     BIGSERIAL PRIMARY KEY,
    blockchain             VARCHAR(32) NOT NULL UNIQUE,
    implementation_address VARCHAR(128) NOT NULL,
    factory_address        VARCHAR(128) NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +migrate Down

DROP TABLE IF EXISTS collector_factories;
