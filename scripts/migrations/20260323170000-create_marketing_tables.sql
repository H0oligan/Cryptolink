-- +migrate Up
-- Marketing campaigns system with queue-based email delivery and unsubscribe support

CREATE TABLE IF NOT EXISTS marketing_campaigns (
    id               bigserial PRIMARY KEY,
    uuid             UUID UNIQUE NOT NULL,
    name             VARCHAR(255) NOT NULL,
    subject          VARCHAR(500) NOT NULL,
    body_html        TEXT NOT NULL,
    template_id      VARCHAR(50),
    audience         VARCHAR(50) NOT NULL DEFAULT 'contacts_opted_in',
    status           VARCHAR(50) NOT NULL DEFAULT 'draft',
    total_recipients INTEGER NOT NULL DEFAULT 0,
    sent_count       INTEGER NOT NULL DEFAULT 0,
    failed_count     INTEGER NOT NULL DEFAULT 0,
    pending_count    INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at       TIMESTAMP,
    completed_at     TIMESTAMP
);

CREATE TABLE IF NOT EXISTS marketing_campaign_recipients (
    id             bigserial PRIMARY KEY,
    campaign_id    bigint NOT NULL REFERENCES marketing_campaigns(id) ON DELETE CASCADE,
    email          VARCHAR(255) NOT NULL,
    status         VARCHAR(50) NOT NULL DEFAULT 'pending',
    sent_at        TIMESTAMP,
    error_message  TEXT,
    UNIQUE(campaign_id, email)
);

CREATE INDEX IF NOT EXISTS idx_mcr_campaign_status ON marketing_campaign_recipients(campaign_id, status);
CREATE INDEX IF NOT EXISTS idx_mcr_pending ON marketing_campaign_recipients(status) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS marketing_unsubscribes (
    id              bigserial PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    token           VARCHAR(255) NOT NULL UNIQUE,
    unsubscribed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS marketing_email_quota (
    id          bigserial PRIMARY KEY,
    quota_date  DATE NOT NULL UNIQUE,
    sent_count  INTEGER NOT NULL DEFAULT 0,
    daily_limit INTEGER NOT NULL DEFAULT 200,
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +migrate Down
DROP TABLE IF EXISTS marketing_email_quota;
DROP TABLE IF EXISTS marketing_unsubscribes;
DROP TABLE IF EXISTS marketing_campaign_recipients;
DROP TABLE IF EXISTS marketing_campaigns;
