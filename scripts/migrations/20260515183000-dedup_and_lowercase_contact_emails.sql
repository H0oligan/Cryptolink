-- +migrate Up
-- Dedup case-different contact rows (e.g. John@x.com vs john@x.com), lowercase
-- every remaining email, and add CHECK constraints so future inserts cannot
-- regress. The application layer now lowercases at the API boundary; this
-- migration guarantees the same outcome for restored backups and fresh DBs.

-- Drop the unique index temporarily so we can lowercase + merge without
-- colliding with an existing row that already holds the lowercase form.
DROP INDEX IF EXISTS contacts_email_unique;

-- Step 1: merge each duplicate group into the oldest row.
-- Upgrade-only consent (bool_or), earliest terms_accepted_at, earliest
-- source_merchant_id, earliest created_at. Lowercasing happens in step 3.
WITH groups AS (
    SELECT
        LOWER(email) AS lower_email,
        MIN(id) AS keep_id,
        bool_or(marketing_consent) AS merged_consent,
        MIN(terms_accepted_at) AS earliest_terms,
        (array_agg(source_merchant_id ORDER BY created_at)
            FILTER (WHERE source_merchant_id IS NOT NULL))[1] AS earliest_merchant,
        MIN(created_at) AS earliest_created
    FROM contacts
    GROUP BY LOWER(email)
    HAVING COUNT(*) > 1
)
UPDATE contacts c
SET marketing_consent  = g.merged_consent,
    terms_accepted_at  = g.earliest_terms,
    source_merchant_id = g.earliest_merchant,
    created_at         = g.earliest_created,
    updated_at         = NOW()
FROM groups g
WHERE c.id = g.keep_id;

-- Step 2: delete every duplicate row except the kept one.
DELETE FROM contacts c
USING (
    SELECT LOWER(email) AS lower_email, MIN(id) AS keep_id
    FROM contacts
    GROUP BY LOWER(email)
    HAVING COUNT(*) > 1
) g
WHERE LOWER(c.email) = g.lower_email AND c.id <> g.keep_id;

-- Step 3: lowercase every remaining email (covers both formerly-duplicated
-- rows and any single-row mixed-case addresses).
UPDATE contacts SET email = LOWER(email) WHERE email <> LOWER(email);

-- Step 4: recreate the unique index now that all values are lowercase.
CREATE UNIQUE INDEX IF NOT EXISTS contacts_email_unique ON contacts (email);

-- Step 5: enforce lowercase going forward. The CHECK fails loudly if any
-- caller forgets to normalize, surfacing the bug instead of silently storing
-- a mixed-case row that escapes the application-layer LOWER() filter.
-- Wrapped in DO so re-running the migration is a clean no-op.
DO $$ BEGIN
    ALTER TABLE contacts
        ADD CONSTRAINT contacts_email_lowercase_chk
        CHECK (email = LOWER(email));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Same hygiene for marketing_unsubscribes. The original unique was a column
-- constraint (marketing_unsubscribes_email_key); we keep that name so the
-- ON CONFLICT (email) clauses in marketing/service.go and sequences.go
-- continue to resolve to it after the index is recreated.
UPDATE marketing_unsubscribes SET email = LOWER(email) WHERE email <> LOWER(email);

DO $$ BEGIN
    ALTER TABLE marketing_unsubscribes
        ADD CONSTRAINT marketing_unsubscribes_email_lowercase_chk
        CHECK (email = LOWER(email));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- +migrate Down
ALTER TABLE marketing_unsubscribes
    DROP CONSTRAINT IF EXISTS marketing_unsubscribes_email_lowercase_chk;
ALTER TABLE contacts
    DROP CONSTRAINT IF EXISTS contacts_email_lowercase_chk;
-- Note: dedup + lowercasing are not reversed (the original case is lost).
