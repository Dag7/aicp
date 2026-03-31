-- name: StoreReceipt :exec
INSERT INTO payment_receipts (receipt_id, token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, adapter_name, external_id, settled_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW());

-- name: GetReceipt :one
SELECT receipt_id, token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, adapter_name, external_id, block_number, settled_at, created_at
FROM payment_receipts
WHERE receipt_id = $1;

-- name: ListReceiptsByAgent :many
SELECT receipt_id, token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, adapter_name, external_id, block_number, settled_at, created_at
FROM payment_receipts
WHERE payer_did = $1 OR payee_did = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReceiptsByAgent :one
SELECT COUNT(*) FROM payment_receipts
WHERE payer_did = $1 OR payee_did = $1;

-- name: IsTokenUsed :one
SELECT EXISTS(SELECT 1 FROM used_tokens WHERE jti = $1);

-- name: MarkTokenUsed :exec
INSERT INTO used_tokens (jti, used_at) VALUES ($1, NOW())
ON CONFLICT (jti) DO NOTHING;

-- name: RecordSpending :exec
INSERT INTO spending_records (did, amount_value, amount_currency, recorded_at)
VALUES ($1, $2, $3, NOW());

-- name: GetDailySpending :one
SELECT COALESCE(SUM(amount_value::NUMERIC), 0)::TEXT AS total
FROM spending_records
WHERE did = $1 AND amount_currency = $2
  AND recorded_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC');

-- name: GetMonthlySpending :one
SELECT COALESCE(SUM(amount_value::NUMERIC), 0)::TEXT AS total
FROM spending_records
WHERE did = $1 AND amount_currency = $2
  AND recorded_at >= DATE_TRUNC('month', NOW() AT TIME ZONE 'UTC');
