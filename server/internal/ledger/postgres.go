package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// PostgresBackend implements LedgerBackend using PostgreSQL.
type PostgresBackend struct {
	pool *pgxpool.Pool
}

// NewPostgresBackend creates a new PostgreSQL-backed ledger.
func NewPostgresBackend(pool *pgxpool.Pool) *PostgresBackend {
	return &PostgresBackend{pool: pool}
}

// Initialize runs any required setup (pool is expected to be already connected).
func (p *PostgresBackend) Initialize(ctx context.Context) error {
	return p.Healthy(ctx)
}

// Close closes the connection pool.
func (p *PostgresBackend) Close() error {
	p.pool.Close()
	return nil
}

// Healthy checks database connectivity.
func (p *PostgresBackend) Healthy(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// ── DID Operations ──────────────────────────────────────────────────────────

func (p *PostgresBackend) AnchorDID(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error) {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling DID document: %w", err)
	}
	docHash := crypto.SHA256Hex(docJSON)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO did_documents (did, controller, document, document_hash, version_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 1, NOW(), NOW())`,
		did, doc.Controller, docJSON, docHash,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting DID document: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO did_document_versions (did, version_id, document, document_hash, proof_type, proof_value, proof_method, created_at)
		 VALUES ($1, 1, $2, $3, $4, $5, $6, NOW())`,
		did, docJSON, docHash, proof.Type, proof.ProofValue, proof.VerificationMethod,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting DID document version: %w", err)
	}

	// Queue ledger entry.
	entryHash := crypto.SHA256Hex([]byte(did + ":create:" + docHash))
	var entryID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
		 VALUES ('did_create', $1, $2, NOW()) RETURNING id`,
		entryHash, did,
	).Scan(&entryID)
	if err != nil {
		return nil, fmt.Errorf("inserting ledger entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &aicp.AnchorReceipt{
		DID:          did,
		EntryHash:    entryHash,
		DocumentHash: docHash,
		AnchoredAt:   time.Now().UTC(),
	}, nil
}

func (p *PostgresBackend) ResolveDID(ctx context.Context, did string) (*aicp.DIDResolutionResult, error) {
	var (
		docJSON     json.RawMessage
		docHash     string
		deactivated bool
		versionID   int
		createdAt   time.Time
		updatedAt   time.Time
	)

	err := p.pool.QueryRow(ctx,
		`SELECT document, document_hash, deactivated, version_id, created_at, updated_at
		 FROM did_documents WHERE did = $1`,
		did,
	).Scan(&docJSON, &docHash, &deactivated, &versionID, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying DID document: %w", err)
	}

	var doc aicp.DIDDocument
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return nil, fmt.Errorf("unmarshaling DID document: %w", err)
	}

	return &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:      createdAt,
			Updated:      updatedAt,
			VersionID:    versionID,
			Deactivated:  deactivated,
			DocumentHash: docHash,
		},
	}, nil
}

func (p *PostgresBackend) UpdateDIDDocument(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error) {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling DID document: %w", err)
	}
	docHash := crypto.SHA256Hex(docJSON)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var newVersion int
	err = tx.QueryRow(ctx,
		`UPDATE did_documents SET document = $2, document_hash = $3, version_id = version_id + 1, updated_at = NOW()
		 WHERE did = $1 AND NOT deactivated RETURNING version_id`,
		did, docJSON, docHash,
	).Scan(&newVersion)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("updating DID document: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO did_document_versions (did, version_id, document, document_hash, proof_type, proof_value, proof_method, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		did, newVersion, docJSON, docHash, proof.Type, proof.ProofValue, proof.VerificationMethod,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting DID document version: %w", err)
	}

	entryHash := crypto.SHA256Hex([]byte(did + ":update:" + docHash))
	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
		 VALUES ('did_update', $1, $2, NOW())`,
		entryHash, did,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting ledger entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &aicp.AnchorReceipt{
		DID:          did,
		EntryHash:    entryHash,
		DocumentHash: docHash,
		AnchoredAt:   time.Now().UTC(),
	}, nil
}

func (p *PostgresBackend) DeactivateDID(ctx context.Context, did string, proof aicp.Proof) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE did_documents SET deactivated = TRUE, updated_at = NOW() WHERE did = $1 AND NOT deactivated`,
		did,
	)
	if err != nil {
		return fmt.Errorf("deactivating DID: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return aicp.ErrNotFound
	}

	entryHash := crypto.SHA256Hex([]byte(did + ":deactivate"))
	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
		 VALUES ('did_deactivate', $1, $2, NOW())`,
		entryHash, did,
	)
	if err != nil {
		return fmt.Errorf("inserting ledger entry: %w", err)
	}

	return tx.Commit(ctx)
}

func (p *PostgresBackend) ListAgentDIDs(ctx context.Context, principalDID string) ([]string, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT did FROM did_documents WHERE controller = $1 AND did != $1 AND NOT deactivated`,
		principalDID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing agent DIDs: %w", err)
	}
	defer rows.Close()

	var dids []string
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, fmt.Errorf("scanning agent DID: %w", err)
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

// ── Credential Operations ───────────────────────────────────────────────────

func (p *PostgresBackend) StoreCredential(ctx context.Context, cred aicp.VerifiableCredential) error {
	credJSON, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshaling credential: %w", err)
	}

	var subjectDID string
	var subjectMap map[string]any
	if err := json.Unmarshal(cred.CredentialSubject, &subjectMap); err == nil {
		if id, ok := subjectMap["id"].(string); ok {
			subjectDID = id
		}
	}

	_, err = p.pool.Exec(ctx,
		`INSERT INTO credentials (credential_id, issuer_did, subject_did, credential_type, credential, issued_at, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		cred.ID, cred.Issuer, subjectDID, cred.Type, credJSON, cred.IssuanceDate, cred.ExpirationDate,
	)
	if err != nil {
		return fmt.Errorf("storing credential: %w", err)
	}
	return nil
}

func (p *PostgresBackend) GetCredential(ctx context.Context, credentialID string) (*aicp.VerifiableCredential, error) {
	var credJSON json.RawMessage
	err := p.pool.QueryRow(ctx,
		`SELECT credential FROM credentials WHERE credential_id = $1`,
		credentialID,
	).Scan(&credJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying credential: %w", err)
	}

	var cred aicp.VerifiableCredential
	if err := json.Unmarshal(credJSON, &cred); err != nil {
		return nil, fmt.Errorf("unmarshaling credential: %w", err)
	}
	return &cred, nil
}

func (p *PostgresBackend) RevokeCredential(ctx context.Context, credentialID string, proof aicp.Proof) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE credentials SET revoked = TRUE, revoked_at = NOW() WHERE credential_id = $1 AND NOT revoked`,
		credentialID,
	)
	if err != nil {
		return fmt.Errorf("revoking credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return aicp.ErrNotFound
	}

	entryHash := crypto.SHA256Hex([]byte("credential_revoke:" + credentialID))
	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
		 VALUES ('credential_revoke', $1, $2, NOW())`,
		entryHash, credentialID,
	)
	if err != nil {
		return fmt.Errorf("inserting ledger entry: %w", err)
	}

	return tx.Commit(ctx)
}

func (p *PostgresBackend) IsCredentialRevoked(ctx context.Context, credentialID string) (bool, error) {
	var revoked bool
	err := p.pool.QueryRow(ctx,
		`SELECT revoked FROM credentials WHERE credential_id = $1`,
		credentialID,
	).Scan(&revoked)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, aicp.ErrNotFound
		}
		return false, fmt.Errorf("checking credential revocation: %w", err)
	}
	return revoked, nil
}

func (p *PostgresBackend) GetRevocationList(ctx context.Context, listID string) (*aicp.RevocationList, error) {
	var (
		issuerDID     string
		encodedList   string
		totalCapacity int
		nextFreeIndex int
		versionID     int
		updatedAt     time.Time
	)

	err := p.pool.QueryRow(ctx,
		`SELECT issuer_did, encoded_list, total_capacity, next_free_index, version_id, updated_at
		 FROM revocation_lists WHERE list_id = $1`,
		listID,
	).Scan(&issuerDID, &encodedList, &totalCapacity, &nextFreeIndex, &versionID, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying revocation list: %w", err)
	}

	return &aicp.RevocationList{
		ListID:        listID,
		IssuerDID:     issuerDID,
		EncodedList:   encodedList,
		TotalCapacity: totalCapacity,
		NextFreeIndex: nextFreeIndex,
		VersionID:     versionID,
		UpdatedAt:     updatedAt,
	}, nil
}

func (p *PostgresBackend) CreateRevocationList(ctx context.Context, list aicp.RevocationList) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO revocation_lists (list_id, issuer_did, encoded_list, total_capacity, next_free_index, version_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 1, NOW(), NOW())`,
		list.ListID, list.IssuerDID, list.EncodedList, list.TotalCapacity, list.NextFreeIndex,
	)
	if err != nil {
		return fmt.Errorf("creating revocation list: %w", err)
	}
	return nil
}

func (p *PostgresBackend) UpdateRevocationList(ctx context.Context, list aicp.RevocationList) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE revocation_lists SET encoded_list = $2, next_free_index = $3, version_id = version_id + 1, updated_at = NOW()
		 WHERE list_id = $1`,
		list.ListID, list.EncodedList, list.NextFreeIndex,
	)
	if err != nil {
		return fmt.Errorf("updating revocation list: %w", err)
	}
	return nil
}

// ── Receipt Operations ──────────────────────────────────────────────────────

func (p *PostgresBackend) StoreReceipt(ctx context.Context, receipt aicp.SignedPaymentReceipt) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO payment_receipts (receipt_id, token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, settled_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
		receipt.ReceiptID, receipt.TokenJTI, receipt.PayerDID, receipt.PayeeDID,
		receipt.Amount.Value, receipt.Amount.Currency, receipt.ReceiptHash, receipt.Status, receipt.SettledAt,
	)
	if err != nil {
		return fmt.Errorf("storing receipt: %w", err)
	}

	entryHash := crypto.SHA256Hex([]byte("receipt:" + receipt.ReceiptID + ":" + receipt.ReceiptHash))
	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at)
		 VALUES ('payment_receipt', $1, $2, NOW())`,
		entryHash, receipt.ReceiptID,
	)
	if err != nil {
		return fmt.Errorf("inserting ledger entry: %w", err)
	}

	return tx.Commit(ctx)
}

func (p *PostgresBackend) GetReceipt(ctx context.Context, receiptID string) (*aicp.SignedPaymentReceipt, error) {
	var (
		tokenJTI    string
		payerDID    string
		payeeDID    string
		amountVal   string
		amountCur   string
		receiptHash string
		status      string
		settledAt   time.Time
	)

	err := p.pool.QueryRow(ctx,
		`SELECT token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, settled_at
		 FROM payment_receipts WHERE receipt_id = $1`,
		receiptID,
	).Scan(&tokenJTI, &payerDID, &payeeDID, &amountVal, &amountCur, &receiptHash, &status, &settledAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying receipt: %w", err)
	}

	return &aicp.SignedPaymentReceipt{
		ReceiptID:   receiptID,
		TokenJTI:    tokenJTI,
		PayerDID:    payerDID,
		PayeeDID:    payeeDID,
		Amount:      aicp.MoneyAmount{Value: amountVal, Currency: amountCur},
		ReceiptHash: receiptHash,
		Status:      status,
		SettledAt:   settledAt,
	}, nil
}

func (p *PostgresBackend) ListReceiptsByAgent(ctx context.Context, agentDID string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := 0
	if opts.Cursor != "" {
		parsed, err := strconv.Atoi(opts.Cursor)
		if err == nil {
			offset = parsed
		}
	}

	rows, err := p.pool.Query(ctx,
		`SELECT receipt_id, token_jti, payer_did, payee_did, amount_value, amount_currency, receipt_hash, status, settled_at
		 FROM payment_receipts
		 WHERE payer_did = $1 OR payee_did = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		agentDID, limit+1, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing receipts: %w", err)
	}
	defer rows.Close()

	var receipts []aicp.SignedPaymentReceipt
	for rows.Next() {
		var r aicp.SignedPaymentReceipt
		var amountVal, amountCur string
		if err := rows.Scan(&r.ReceiptID, &r.TokenJTI, &r.PayerDID, &r.PayeeDID, &amountVal, &amountCur, &r.ReceiptHash, &r.Status, &r.SettledAt); err != nil {
			return nil, fmt.Errorf("scanning receipt: %w", err)
		}
		r.Amount = aicp.MoneyAmount{Value: amountVal, Currency: amountCur}
		receipts = append(receipts, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(receipts) > limit
	if hasMore {
		receipts = receipts[:limit]
	}

	result := &aicp.PaginatedResult[aicp.SignedPaymentReceipt]{
		Items:   receipts,
		HasMore: hasMore,
	}
	if hasMore {
		result.NextCursor = strconv.Itoa(offset + limit)
	}
	if result.Items == nil {
		result.Items = []aicp.SignedPaymentReceipt{}
	}
	return result, nil
}

func (p *PostgresBackend) IsTokenUsed(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM used_tokens WHERE jti = $1)`,
		jti,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking token usage: %w", err)
	}
	return exists, nil
}

func (p *PostgresBackend) MarkTokenUsed(ctx context.Context, jti string) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO used_tokens (jti, used_at) VALUES ($1, NOW()) ON CONFLICT (jti) DO NOTHING`,
		jti,
	)
	if err != nil {
		return fmt.Errorf("marking token used: %w", err)
	}
	return nil
}

// ── Block / Consensus Operations ────────────────────────────────────────────

func (p *PostgresBackend) AppendEntry(ctx context.Context, entry aicp.LedgerEntry) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO ledger_entries (entry_type, entry_hash, entry_data_ref, recorded_at) VALUES ($1, $2, $3, NOW())`,
		entry.EntryType, entry.EntryHash, entry.EntryDataRef,
	)
	if err != nil {
		return fmt.Errorf("appending ledger entry: %w", err)
	}
	return nil
}

func (p *PostgresBackend) GetPendingEntries(ctx context.Context) ([]aicp.LedgerEntry, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, entry_type, entry_hash, entry_data_ref, recorded_at
		 FROM ledger_entries WHERE block_number IS NULL ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying pending entries: %w", err)
	}
	defer rows.Close()

	var entries []aicp.LedgerEntry
	for rows.Next() {
		var e aicp.LedgerEntry
		if err := rows.Scan(&e.ID, &e.EntryType, &e.EntryHash, &e.EntryDataRef, &e.RecordedAt); err != nil {
			return nil, fmt.Errorf("scanning ledger entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (p *PostgresBackend) StoreBlock(ctx context.Context, block aicp.Block) error {
	sigsJSON, err := json.Marshal(block.Signatures)
	if err != nil {
		return fmt.Errorf("marshaling signatures: %w", err)
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_blocks (number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		block.Number, block.Hash, block.PreviousHash, block.MerkleRoot,
		block.EntryCount, block.ValidatorDID, block.ValidatorSig, sigsJSON, block.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting block: %w", err)
	}

	// Assign pending entries to this block. We use entry_count to determine how many
	// of the oldest pending entries belong to this block.
	_, err = tx.Exec(ctx,
		`UPDATE ledger_entries SET block_number = $1
		 WHERE id IN (
		     SELECT id FROM ledger_entries WHERE block_number IS NULL ORDER BY id ASC LIMIT $2
		 )`,
		block.Number, block.EntryCount,
	)
	if err != nil {
		return fmt.Errorf("assigning entries to block: %w", err)
	}

	return tx.Commit(ctx)
}

func (p *PostgresBackend) GetLatestBlock(ctx context.Context) (*aicp.Block, error) {
	return p.scanBlock(ctx,
		`SELECT number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at
		 FROM ledger_blocks ORDER BY number DESC LIMIT 1`,
	)
}

func (p *PostgresBackend) GetBlock(ctx context.Context, number uint64) (*aicp.Block, error) {
	return p.scanBlock(ctx,
		`SELECT number, hash, previous_hash, merkle_root, entry_count, validator_did, validator_sig, signatures, created_at
		 FROM ledger_blocks WHERE number = $1`,
		number,
	)
}

func (p *PostgresBackend) scanBlock(ctx context.Context, query string, args ...any) (*aicp.Block, error) {
	var (
		block   aicp.Block
		sigsRaw json.RawMessage
	)

	err := p.pool.QueryRow(ctx, query, args...).Scan(
		&block.Number, &block.Hash, &block.PreviousHash, &block.MerkleRoot,
		&block.EntryCount, &block.ValidatorDID, &block.ValidatorSig, &sigsRaw, &block.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("scanning block: %w", err)
	}

	if err := json.Unmarshal(sigsRaw, &block.Signatures); err != nil {
		return nil, fmt.Errorf("unmarshaling block signatures: %w", err)
	}
	return &block, nil
}

// ── Trust Operations ────────────────────────────────────────────────────────

func (p *PostgresBackend) GetTrustScore(ctx context.Context, did string) (*aicp.TrustScore, error) {
	var ts aicp.TrustScore
	err := p.pool.QueryRow(ctx,
		`SELECT did, score, identity, account_age, transactions, endorsements, credentials, dispute_penalty, computed_at
		 FROM trust_scores WHERE did = $1`,
		did,
	).Scan(&ts.DID, &ts.Score, &ts.Components.Identity, &ts.Components.AccountAge,
		&ts.Components.Transactions, &ts.Components.Endorsements, &ts.Components.Credentials,
		&ts.Components.DisputePenalty, &ts.ComputedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying trust score: %w", err)
	}
	return &ts, nil
}

func (p *PostgresBackend) UpdateTrustScore(ctx context.Context, score aicp.TrustScore) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO trust_scores (did, score, identity, account_age, transactions, endorsements, credentials, dispute_penalty, computed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (did) DO UPDATE SET
		     score = EXCLUDED.score, identity = EXCLUDED.identity, account_age = EXCLUDED.account_age,
		     transactions = EXCLUDED.transactions, endorsements = EXCLUDED.endorsements, credentials = EXCLUDED.credentials,
		     dispute_penalty = EXCLUDED.dispute_penalty, computed_at = NOW()`,
		score.DID, score.Score, score.Components.Identity, score.Components.AccountAge,
		score.Components.Transactions, score.Components.Endorsements, score.Components.Credentials,
		score.Components.DisputePenalty,
	)
	if err != nil {
		return fmt.Errorf("upserting trust score: %w", err)
	}
	return nil
}

// ── ANS Operations ──────────────────────────────────────────────────────────

func (p *PostgresBackend) StoreANSRecord(ctx context.Context, record aicp.ANSRecord) error {
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling ANS record: %w", err)
	}

	_, err = p.pool.Exec(ctx,
		`INSERT INTO ans_records (service_did, aicp_version, record, service_name, categories, discovery_tags, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		 ON CONFLICT (service_did) DO UPDATE SET
		     aicp_version = EXCLUDED.aicp_version, record = EXCLUDED.record,
		     service_name = EXCLUDED.service_name, categories = EXCLUDED.categories,
		     discovery_tags = EXCLUDED.discovery_tags, updated_at = NOW()`,
		record.Service.DID, record.AICPVersion, recordJSON, record.Service.Name,
		record.Service.Category, record.DiscoveryTags,
	)
	if err != nil {
		return fmt.Errorf("storing ANS record: %w", err)
	}
	return nil
}

func (p *PostgresBackend) GetANSRecord(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	var recordJSON json.RawMessage
	err := p.pool.QueryRow(ctx,
		`SELECT record FROM ans_records WHERE service_did = $1`,
		serviceDID,
	).Scan(&recordJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying ANS record: %w", err)
	}

	var record aicp.ANSRecord
	if err := json.Unmarshal(recordJSON, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling ANS record: %w", err)
	}
	return &record, nil
}

func (p *PostgresBackend) SearchANSRecords(ctx context.Context, query string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := 0
	if opts.Cursor != "" {
		parsed, err := strconv.Atoi(opts.Cursor)
		if err == nil {
			offset = parsed
		}
	}

	rows, err := p.pool.Query(ctx,
		`SELECT record FROM ans_records
		 WHERE search_vector @@ plainto_tsquery('english', $1)
		 ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC
		 LIMIT $2 OFFSET $3`,
		query, limit+1, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("searching ANS records: %w", err)
	}
	defer rows.Close()

	var records []aicp.ANSRecord
	for rows.Next() {
		var recordJSON json.RawMessage
		if err := rows.Scan(&recordJSON); err != nil {
			return nil, fmt.Errorf("scanning ANS record: %w", err)
		}
		var r aicp.ANSRecord
		if err := json.Unmarshal(recordJSON, &r); err != nil {
			return nil, fmt.Errorf("unmarshaling ANS record: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	result := &aicp.PaginatedResult[aicp.ANSRecord]{
		Items:   records,
		HasMore: hasMore,
	}
	if hasMore {
		result.NextCursor = strconv.Itoa(offset + limit)
	}
	if result.Items == nil {
		result.Items = []aicp.ANSRecord{}
	}
	return result, nil
}

func (p *PostgresBackend) DeleteANSRecord(ctx context.Context, serviceDID string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM ans_records WHERE service_did = $1`, serviceDID)
	if err != nil {
		return fmt.Errorf("deleting ANS record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return aicp.ErrNotFound
	}
	return nil
}

// ── Validator Operations ────────────────────────────────────────────────────

func (p *PostgresBackend) RegisterValidator(ctx context.Context, v aicp.Validator) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO validators (did, public_key, grpc_address, active, joined_at)
		 VALUES ($1, $2, $3, TRUE, NOW())
		 ON CONFLICT (did) DO UPDATE SET public_key = EXCLUDED.public_key, grpc_address = EXCLUDED.grpc_address, active = TRUE`,
		v.DID, v.PublicKey, v.GRPCAddress,
	)
	if err != nil {
		return fmt.Errorf("registering validator: %w", err)
	}
	return nil
}

func (p *PostgresBackend) RemoveValidator(ctx context.Context, validatorDID string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE validators SET active = FALSE WHERE did = $1 AND active`,
		validatorDID,
	)
	if err != nil {
		return fmt.Errorf("removing validator: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return aicp.ErrNotFound
	}
	return nil
}

func (p *PostgresBackend) ListValidators(ctx context.Context) ([]aicp.Validator, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT did, public_key, grpc_address, active, joined_at FROM validators WHERE active ORDER BY joined_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing validators: %w", err)
	}
	defer rows.Close()

	var validators []aicp.Validator
	for rows.Next() {
		var v aicp.Validator
		if err := rows.Scan(&v.DID, &v.PublicKey, &v.GRPCAddress, &v.Active, &v.JoinedAt); err != nil {
			return nil, fmt.Errorf("scanning validator: %w", err)
		}
		validators = append(validators, v)
	}
	return validators, rows.Err()
}

func (p *PostgresBackend) GetValidator(ctx context.Context, validatorDID string) (*aicp.Validator, error) {
	var v aicp.Validator
	err := p.pool.QueryRow(ctx,
		`SELECT did, public_key, grpc_address, active, joined_at FROM validators WHERE did = $1`,
		validatorDID,
	).Scan(&v.DID, &v.PublicKey, &v.GRPCAddress, &v.Active, &v.JoinedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("querying validator: %w", err)
	}
	return &v, nil
}

// ── Spending Tracking ───────────────────────────────────────────────────────

func (p *PostgresBackend) GetDailySpending(ctx context.Context, did string, currency string) (string, error) {
	var total string
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_value::NUMERIC), 0)::TEXT
		 FROM spending_records
		 WHERE did = $1 AND amount_currency = $2
		   AND recorded_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC')`,
		did, currency,
	).Scan(&total)
	if err != nil {
		return "0", fmt.Errorf("querying daily spending: %w", err)
	}
	return total, nil
}

func (p *PostgresBackend) GetMonthlySpending(ctx context.Context, did string, currency string) (string, error) {
	var total string
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_value::NUMERIC), 0)::TEXT
		 FROM spending_records
		 WHERE did = $1 AND amount_currency = $2
		   AND recorded_at >= DATE_TRUNC('month', NOW() AT TIME ZONE 'UTC')`,
		did, currency,
	).Scan(&total)
	if err != nil {
		return "0", fmt.Errorf("querying monthly spending: %w", err)
	}
	return total, nil
}

func (p *PostgresBackend) RecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO spending_records (did, amount_value, amount_currency, recorded_at) VALUES ($1, $2, $3, NOW())`,
		did, amount.Value, amount.Currency,
	)
	if err != nil {
		return fmt.Errorf("recording spending: %w", err)
	}
	return nil
}

// Compile-time interface satisfaction check.
var _ LedgerBackend = (*PostgresBackend)(nil)
