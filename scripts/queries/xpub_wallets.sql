-- name: CreateXpubWallet :one
INSERT INTO xpub_wallets (
    uuid, merchant_id, blockchain, xpub, derivation_path,
    created_at, updated_at, last_derived_index, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetXpubWalletByID :one
SELECT * FROM xpub_wallets WHERE id = $1 LIMIT 1;

-- name: GetXpubWalletByUUID :one
SELECT * FROM xpub_wallets WHERE uuid = $1 LIMIT 1;

-- name: GetXpubWalletByMerchantAndBlockchain :one
SELECT * FROM xpub_wallets
WHERE merchant_id = $1 AND blockchain = $2 AND is_active = true
LIMIT 1;

-- name: ListXpubWalletsByMerchantID :many
SELECT * FROM xpub_wallets
WHERE merchant_id = $1 AND is_active = true
ORDER BY created_at DESC;

-- name: UpdateXpubWalletLastIndex :one
UPDATE xpub_wallets
SET last_derived_index = $2, updated_at = $3
WHERE id = $1
RETURNING *;

-- name: DeactivateXpubWallet :exec
UPDATE xpub_wallets
SET is_active = false, updated_at = $2
WHERE id = $1;

-- name: UpdateXpubWalletTatumSubscription :one
UPDATE xpub_wallets
SET tatum_subscription_id = $2, updated_at = $3
WHERE id = $1
RETURNING *;
