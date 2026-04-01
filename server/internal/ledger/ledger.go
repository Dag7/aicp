// Package ledger implements the AICP consortium ledger with PostgreSQL-backed
// storage, Merkle-root block production, and multi-node consensus.
package ledger

import (
	"context"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// LedgerBackend is the primary storage abstraction for all AICP ledger operations.
// Implementations must be safe for concurrent use.
type LedgerBackend interface {
	// Lifecycle
	Initialize(ctx context.Context) error
	Close() error
	Healthy(ctx context.Context) error

	// ── DID Operations ──────────────────────────────────────────────────

	// AnchorDID anchors a new DID document on the ledger and returns an anchor receipt.
	AnchorDID(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error)

	// ResolveDID returns the latest DID document for a given DID.
	ResolveDID(ctx context.Context, did string) (*aicp.DIDResolutionResult, error)

	// UpdateDIDDocument updates an existing DID document and returns an anchor receipt.
	UpdateDIDDocument(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error)

	// DeactivateDID marks a DID as permanently deactivated.
	DeactivateDID(ctx context.Context, did string, proof aicp.Proof) error

	// ListAgentDIDs lists all agent DIDs controlled by the given principal DID.
	ListAgentDIDs(ctx context.Context, principalDID string) ([]string, error)

	// ── Credential Operations ───────────────────────────────────────────

	// StoreCredential persists a verifiable credential.
	StoreCredential(ctx context.Context, cred aicp.VerifiableCredential) error

	// GetCredential retrieves a credential by its ID.
	GetCredential(ctx context.Context, credentialID string) (*aicp.VerifiableCredential, error)

	// RevokeCredential marks a credential as revoked on the ledger.
	RevokeCredential(ctx context.Context, credentialID string, proof aicp.Proof) error

	// IsCredentialRevoked checks whether a credential has been revoked.
	IsCredentialRevoked(ctx context.Context, credentialID string) (bool, error)

	// GetRevocationList returns a revocation list by ID.
	GetRevocationList(ctx context.Context, listID string) (*aicp.RevocationList, error)

	// CreateRevocationList creates a new revocation list for an issuer.
	CreateRevocationList(ctx context.Context, list aicp.RevocationList) error

	// UpdateRevocationList updates an existing revocation list.
	UpdateRevocationList(ctx context.Context, list aicp.RevocationList) error

	// ── Receipt Operations ──────────────────────────────────────────────

	// StoreReceipt persists a signed payment receipt on the ledger.
	StoreReceipt(ctx context.Context, receipt aicp.SignedPaymentReceipt) error

	// GetReceipt retrieves a receipt by ID.
	GetReceipt(ctx context.Context, receiptID string) (*aicp.SignedPaymentReceipt, error)

	// ListReceiptsByAgent returns paginated receipts for a given agent DID.
	ListReceiptsByAgent(ctx context.Context, agentDID string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error)

	// IsTokenUsed checks if a payment token JTI has already been redeemed (replay prevention).
	IsTokenUsed(ctx context.Context, jti string) (bool, error)

	// MarkTokenUsed records a payment token JTI as redeemed.
	MarkTokenUsed(ctx context.Context, jti string) error

	// ── Block / Consensus Operations ────────────────────────────────────

	// AppendEntry queues a new entry for inclusion in the next block.
	AppendEntry(ctx context.Context, entry aicp.LedgerEntry) error

	// GetPendingEntries returns entries that have not yet been included in a finalized block.
	GetPendingEntries(ctx context.Context) ([]aicp.LedgerEntry, error)

	// StoreBlock persists a finalized block and marks its entries as included.
	StoreBlock(ctx context.Context, block aicp.Block) error

	// GetLatestBlock returns the most recently finalized block.
	GetLatestBlock(ctx context.Context) (*aicp.Block, error)

	// GetBlock returns a block by its number.
	GetBlock(ctx context.Context, number uint64) (*aicp.Block, error)

	// ── Trust Operations ────────────────────────────────────────────────

	// GetTrustScore returns the current trust score for a DID.
	GetTrustScore(ctx context.Context, did string) (*aicp.TrustScore, error)

	// UpdateTrustScore creates or updates the trust score for a DID.
	UpdateTrustScore(ctx context.Context, score aicp.TrustScore) error

	// ── ANS Operations ──────────────────────────────────────────────────

	// StoreANSRecord persists or updates an ANS record.
	StoreANSRecord(ctx context.Context, record aicp.ANSRecord) error

	// GetANSRecord retrieves an ANS record by service DID.
	GetANSRecord(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error)

	// SearchANSRecords performs a capability/text search over ANS records.
	SearchANSRecords(ctx context.Context, query string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error)

	// DeleteANSRecord removes an ANS record.
	DeleteANSRecord(ctx context.Context, serviceDID string) error

	// ── Validator Operations ────────────────────────────────────────────

	// RegisterValidator adds a new validator to the consortium.
	RegisterValidator(ctx context.Context, v aicp.Validator) error

	// RemoveValidator removes a validator from the consortium.
	RemoveValidator(ctx context.Context, validatorDID string) error

	// ListValidators returns all active validators.
	ListValidators(ctx context.Context) ([]aicp.Validator, error)

	// GetValidator returns a single validator by DID.
	GetValidator(ctx context.Context, validatorDID string) (*aicp.Validator, error)

	// ── Spending Tracking ───────────────────────────────────────────────

	// GetDailySpending returns the total spending for a DID on a given day (UTC).
	GetDailySpending(ctx context.Context, did string, currency string) (string, error)

	// GetMonthlySpending returns the total spending for a DID in the current month (UTC).
	GetMonthlySpending(ctx context.Context, did string, currency string) (string, error)

	// RecordSpending records a spending event for limit tracking.
	RecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount) error

	// CheckAndRecordSpending atomically verifies that the given amount does not
	// exceed the daily or monthly spending limit and, if within limits, records
	// the spending in a single transaction. This eliminates the race condition
	// between separate CheckLimits and RecordSpending calls where two concurrent
	// requests could both pass the check before either records. Returns true if
	// the spending was within limits and recorded; false if the limit would be
	// exceeded (no record is written in that case).
	CheckAndRecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount, dailyLimit, monthlyLimit string) (bool, error)
}
