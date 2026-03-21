-- Migration: Enhance merchant registration with business fields + email verification
-- Add contacts table for GDPR-compliant invoice payer consent tracking

-- 1. Merchant registration: add business fields + email verification + consent
ALTER TABLE users ADD COLUMN IF NOT EXISTS company_name VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS address TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS website VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token_expires TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS marketing_consent BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted_at TIMESTAMP;

-- Mark existing admin as verified
UPDATE users SET email_verified = TRUE WHERE is_super_admin = TRUE;

-- 2. Contacts table: global CryptoLink consent for invoice payers
-- Separate from per-merchant customers table (which remains unchanged)
CREATE TABLE IF NOT EXISTS contacts (
    id                  bigserial PRIMARY KEY,
    uuid                UUID UNIQUE NOT NULL,
    email               VARCHAR(255) NOT NULL,
    marketing_consent   BOOLEAN DEFAULT FALSE,
    terms_accepted_at   TIMESTAMP,
    source_merchant_id  bigint,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS contacts_email_unique ON contacts (email);
CREATE INDEX IF NOT EXISTS contacts_marketing_consent_idx ON contacts (marketing_consent);
