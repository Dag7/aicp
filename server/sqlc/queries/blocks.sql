-- name: StoreBlock :exec
INSERT INTO ledger_blocks (number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW());

-- name: GetLatestBlock :one
SELECT number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at
FROM ledger_blocks
ORDER BY number DESC
LIMIT 1;

-- name: GetBlock :one
SELECT number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at
FROM ledger_blocks
WHERE number = $1;

-- name: AppendEntry :one
INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
VALUES ($1, $2, $3, NOW())
RETURNING id;

-- name: GetPendingEntries :many
SELECT id, block_number, entry_type, entry_hash, entry_data_ref, recorded_at
FROM ledger_entries
WHERE block_number IS NULL
ORDER BY id ASC;

-- name: AssignEntriesToBlock :exec
UPDATE ledger_entries
SET block_number = $1
WHERE block_number IS NULL AND id <= $2;
