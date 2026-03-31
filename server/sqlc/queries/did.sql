-- name: CreateDIDDocument :exec
INSERT INTO did_documents (did, controller, document, document_hash, version_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, 1, NOW(), NOW());

-- name: GetDIDDocument :one
SELECT did, controller, document, document_hash, deactivated, version_id, created_at, updated_at
FROM did_documents
WHERE did = $1;

-- name: UpdateDIDDocument :exec
UPDATE did_documents
SET document = $2, document_hash = $3, version_id = version_id + 1, updated_at = NOW()
WHERE did = $1 AND NOT deactivated;

-- name: DeactivateDIDDocument :exec
UPDATE did_documents
SET deactivated = TRUE, updated_at = NOW()
WHERE did = $1 AND NOT deactivated;

-- name: ListAgentDIDs :many
SELECT did
FROM did_documents
WHERE controller = $1 AND did != $1 AND NOT deactivated;

-- name: CreateDIDDocumentVersion :exec
INSERT INTO did_document_versions (did, version_id, document, document_hash, proof_type, proof_value, proof_method, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW());

-- name: GetDIDDocumentVersion :one
SELECT id, did, version_id, document, document_hash, proof_type, proof_value, proof_method, created_at
FROM did_document_versions
WHERE did = $1 AND version_id = $2;

-- name: DIDExists :one
SELECT EXISTS(SELECT 1 FROM did_documents WHERE did = $1);
