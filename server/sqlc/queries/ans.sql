-- name: StoreANSRecord :exec
INSERT INTO ans_records (service_did, aicp_version, record, service_name, categories, discovery_tags, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (service_did) DO UPDATE SET
    aicp_version = EXCLUDED.aicp_version,
    record = EXCLUDED.record,
    service_name = EXCLUDED.service_name,
    categories = EXCLUDED.categories,
    discovery_tags = EXCLUDED.discovery_tags,
    updated_at = NOW();

-- name: GetANSRecord :one
SELECT service_did, aicp_version, record, service_name, categories, discovery_tags, created_at, updated_at
FROM ans_records
WHERE service_did = $1;

-- name: SearchANSRecords :many
SELECT service_did, aicp_version, record, service_name, categories, discovery_tags, created_at, updated_at
FROM ans_records
WHERE search_vector @@ plainto_tsquery('english', $1)
ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC
LIMIT $2 OFFSET $3;

-- name: CountANSSearchResults :one
SELECT COUNT(*)
FROM ans_records
WHERE search_vector @@ plainto_tsquery('english', $1);

-- name: DeleteANSRecord :exec
DELETE FROM ans_records WHERE service_did = $1;

-- name: SearchANSByCategory :many
SELECT service_did, aicp_version, record, service_name, categories, discovery_tags, created_at, updated_at
FROM ans_records
WHERE categories && $1::TEXT[]
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;
