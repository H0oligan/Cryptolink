-- +migrate Up
-- Fix last_derived_index default from 0 to -1 so first derivation produces index 0
-- (matching BIP44/49/84 standard where first receive address is at index 0).
ALTER TABLE xpub_wallets ALTER COLUMN last_derived_index SET DEFAULT -1;
UPDATE xpub_wallets SET last_derived_index = -1 WHERE last_derived_index = 0;

-- +migrate Down
ALTER TABLE xpub_wallets ALTER COLUMN last_derived_index SET DEFAULT 0;
UPDATE xpub_wallets SET last_derived_index = 0 WHERE last_derived_index = -1;
