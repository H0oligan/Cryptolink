-- +migrate Up
-- Drip sequence orchestration for marketing campaigns + per-instance settings.

CREATE TABLE IF NOT EXISTS marketing_sequences (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID UNIQUE NOT NULL,
    name              VARCHAR(255) NOT NULL,
    audience          VARCHAR(50) NOT NULL DEFAULT 'contacts_opted_in',
    status            VARCHAR(50) NOT NULL DEFAULT 'draft',
    skip_if_converted BOOLEAN NOT NULL DEFAULT TRUE,
    start_at          TIMESTAMP,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at        TIMESTAMP,
    completed_at      TIMESTAMP,
    CONSTRAINT marketing_sequences_audience_chk
        CHECK (audience IN ('merchants','contacts_opted_in','all')),
    CONSTRAINT marketing_sequences_status_chk
        CHECK (status IN ('draft','running','paused','completed','cancelled'))
);

CREATE TABLE IF NOT EXISTS marketing_sequence_steps (
    id               BIGSERIAL PRIMARY KEY,
    sequence_id      BIGINT NOT NULL REFERENCES marketing_sequences(id) ON DELETE CASCADE,
    step_index       INT NOT NULL,
    template_id      VARCHAR(100),
    subject_override VARCHAR(500),
    offset_hours     INT NOT NULL DEFAULT 0,
    UNIQUE (sequence_id, step_index)
);

CREATE TABLE IF NOT EXISTS marketing_sequence_enrollments (
    id            BIGSERIAL PRIMARY KEY,
    sequence_id   BIGINT NOT NULL REFERENCES marketing_sequences(id) ON DELETE CASCADE,
    email         VARCHAR(255) NOT NULL,
    current_step  INT NOT NULL DEFAULT 0,
    next_send_at  TIMESTAMP NOT NULL,
    status        VARCHAR(50) NOT NULL DEFAULT 'active',
    attempt_count INT NOT NULL DEFAULT 0,
    last_error    TEXT,
    enrolled_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (sequence_id, email),
    CONSTRAINT marketing_sequence_enrollments_status_chk
        CHECK (status IN ('active','converted','unsubscribed','completed','failed'))
);

CREATE INDEX IF NOT EXISTS idx_mse_due
    ON marketing_sequence_enrollments (next_send_at)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS marketing_settings (
    id          INT PRIMARY KEY DEFAULT 1,
    daily_limit INT NOT NULL DEFAULT 250,
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT marketing_settings_singleton_chk CHECK (id = 1),
    CONSTRAINT marketing_settings_daily_limit_chk CHECK (daily_limit BETWEEN 50 AND 250)
);

INSERT INTO marketing_settings (id, daily_limit) VALUES (1, 250)
    ON CONFLICT (id) DO NOTHING;

UPDATE marketing_email_quota SET daily_limit = 250 WHERE daily_limit = 200;

-- +migrate Down
DROP TABLE IF EXISTS marketing_settings;
DROP INDEX IF EXISTS idx_mse_due;
DROP TABLE IF EXISTS marketing_sequence_enrollments;
DROP TABLE IF EXISTS marketing_sequence_steps;
DROP TABLE IF EXISTS marketing_sequences;
UPDATE marketing_email_quota SET daily_limit = 200 WHERE daily_limit = 250;
