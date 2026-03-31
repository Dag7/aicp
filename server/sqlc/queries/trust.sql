-- name: GetTrustScore :one
SELECT did, score, identity, account_age, transactions, endorsements, credentials, dispute_penalty, computed_at
FROM trust_scores
WHERE did = $1;

-- name: UpsertTrustScore :exec
INSERT INTO trust_scores (did, score, identity, account_age, transactions, endorsements, credentials, dispute_penalty, computed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (did) DO UPDATE SET
    score = EXCLUDED.score,
    identity = EXCLUDED.identity,
    account_age = EXCLUDED.account_age,
    transactions = EXCLUDED.transactions,
    endorsements = EXCLUDED.endorsements,
    credentials = EXCLUDED.credentials,
    dispute_penalty = EXCLUDED.dispute_penalty,
    computed_at = NOW();
