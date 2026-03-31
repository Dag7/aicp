-- name: StoreCredential :exec
INSERT INTO credentials (credential_id, issuer_did, subject_did, credential_type, credential, issued_at, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW());

-- name: GetCredential :one
SELECT credential_id, issuer_did, subject_did, credential_type, credential, revoked, revoked_at, issued_at, expires_at, created_at
FROM credentials
WHERE credential_id = $1;

-- name: RevokeCredential :exec
UPDATE credentials
SET revoked = TRUE, revoked_at = NOW()
WHERE credential_id = $1 AND NOT revoked;

-- name: IsCredentialRevoked :one
SELECT revoked FROM credentials WHERE credential_id = $1;

-- name: CreateRevocationList :exec
INSERT INTO revocation_lists (list_id, issuer_did, encoded_list, total_capacity, next_free_index, version_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 1, NOW(), NOW());

-- name: GetRevocationList :one
SELECT list_id, issuer_did, encoded_list, total_capacity, next_free_index, version_id, created_at, updated_at
FROM revocation_lists
WHERE list_id = $1;

-- name: UpdateRevocationList :exec
UPDATE revocation_lists
SET encoded_list = $2, next_free_index = $3, version_id = version_id + 1, updated_at = NOW()
WHERE list_id = $1;

-- name: CreateRevocationEvent :exec
INSERT INTO revocation_events (list_id, credential_id, list_index, proof_type, proof_value, revoked_at)
VALUES ($1, $2, $3, $4, $5, NOW());
