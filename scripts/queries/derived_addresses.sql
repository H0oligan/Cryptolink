-- name: CreateDerivedAddress :one
INSERT INTO derived_addresses (
    uuid, xpub_wallet_id, merchant_id, blockchain,
    address, derivation_path, derivation_index, public_key,
    is_used, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetDerivedAddressByID :one
SELECT * FROM derived_addresses WHERE id = $1 LIMIT 1;

-- name: GetDerivedAddressByUUID :one
SELECT * FROM derived_addresses WHERE uuid = $1 LIMIT 1;

-- name: GetDerivedAddressByAddress :one
SELECT * FROM derived_addresses
WHERE blockchain = $1 AND address = $2
LIMIT 1;

-- name: GetNextUnusedAddress :one
SELECT * FROM derived_addresses
WHERE xpub_wallet_id = $1 AND is_used = false
ORDER BY derivation_index ASC
LIMIT 1;

-- name: ListDerivedAddressesByWalletID :many
SELECT * FROM derived_addresses
WHERE xpub_wallet_id = $1
ORDER BY derivation_index ASC;

-- name: ListDerivedAddressesByMerchantID :many
SELECT * FROM derived_addresses
WHERE merchant_id = $1
ORDER BY created_at DESC;

-- name: MarkAddressAsUsed :one
UPDATE derived_addresses
SET is_used = true, payment_id = $2, updated_at = $3
WHERE id = $1
RETURNING *;

-- name: UpdateDerivedAddressTatumSubscription :one
UPDATE derived_addresses
SET tatum_mainnet_subscription_id = $2, tatum_testnet_subscription_id = $3, updated_at = $4
WHERE id = $1
RETURNING *;

-- name: CountUnusedAddresses :one
SELECT COUNT(*) FROM derived_addresses
WHERE xpub_wallet_id = $1 AND is_used = false;

-- name: GetLastDerivedIndex :one
SELECT COALESCE(MAX(derivation_index), -1) as last_index
FROM derived_addresses
WHERE xpub_wallet_id = $1;
