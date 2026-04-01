// Package transport implements HTTP-based JSON RPC for validator-to-validator
// communication. This provides block signature collection (consensus) and
// block synchronization between consortium validators using the existing
// chi router infrastructure.
package transport

import (
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ── Request Types ────────────────────────────────────────────────────────────

// SignatureRequest is the body of POST /validator/sign.
type SignatureRequest struct {
	BlockHash     string              `json:"block_hash"`
	ProposedBlock ProposedBlockWire   `json:"proposed_block"`
}

// FetchBlocksRequest is the body of POST /validator/blocks.
type FetchBlocksRequest struct {
	FromBlock uint64 `json:"from_block"`
}

// ── Response Types ───────────────────────────────────────────────────────────

// SignatureResponse is returned from POST /validator/sign.
type SignatureResponse struct {
	ValidatorDID string `json:"validator_did"`
	Signature    string `json:"signature"`
}

// FetchBlocksResponse is returned from POST /validator/blocks.
type FetchBlocksResponse struct {
	Blocks []FinalizedBlockWire `json:"blocks"`
}

// LatestBlockResponse is returned from GET /validator/latest.
type LatestBlockResponse struct {
	BlockNumber uint64 `json:"block_number"`
}

// HealthResponse is returned from GET /validator/health.
type HealthResponse struct {
	Healthy bool `json:"healthy"`
}

// ── Wire Types ───────────────────────────────────────────────────────────────
//
// These mirror the aicp domain types but use int64 for time fields to ensure
// unambiguous serialization across language boundaries.

// ProposedBlockWire is the JSON-safe wire format for a ProposedBlock.
type ProposedBlockWire struct {
	Number       uint64              `json:"number"`
	PreviousHash string              `json:"previous_hash"`
	MerkleRoot   string              `json:"merkle_root"`
	Entries      []aicp.LedgerEntry  `json:"entries"`
	ProposerDID  string              `json:"proposer_did"`
	ProposerSig  string              `json:"proposer_sig"`
	Timestamp    int64               `json:"timestamp"`
}

// FinalizedBlockWire is the JSON-safe wire format for a FinalizedBlock.
type FinalizedBlockWire struct {
	Block      BlockWire            `json:"block"`
	Signatures []ValidatorSigWire   `json:"signatures"`
}

// BlockWire is the JSON-safe wire format for a Block.
type BlockWire struct {
	Number       uint64             `json:"number"`
	Hash         string             `json:"hash"`
	PreviousHash string             `json:"previous_hash"`
	MerkleRoot   string             `json:"merkle_root"`
	EntryCount   int                `json:"entry_count"`
	ValidatorDID string             `json:"validator_did"`
	ValidatorSig string             `json:"validator_sig"`
	Signatures   []ValidatorSigWire `json:"signatures,omitempty"`
	CreatedAt    int64              `json:"created_at"`
}

// ValidatorSigWire is the JSON-safe wire format for a ValidatorSig.
type ValidatorSigWire struct {
	ValidatorDID string `json:"validator_did"`
	Signature    string `json:"signature"`
}

// ── Conversion Helpers ───────────────────────────────────────────────────────

// ProposedBlockToWire converts a domain ProposedBlock to its wire format.
func ProposedBlockToWire(pb *aicp.ProposedBlock) ProposedBlockWire {
	return ProposedBlockWire{
		Number:       pb.Number,
		PreviousHash: pb.PreviousHash,
		MerkleRoot:   pb.MerkleRoot,
		Entries:      pb.Entries,
		ProposerDID:  pb.ProposerDID,
		ProposerSig:  pb.ProposerSig,
		Timestamp:    pb.Timestamp.UnixMilli(),
	}
}

// ProposedBlockFromWire converts a wire ProposedBlock to the domain type.
func ProposedBlockFromWire(w ProposedBlockWire) *aicp.ProposedBlock {
	return &aicp.ProposedBlock{
		Number:       w.Number,
		PreviousHash: w.PreviousHash,
		MerkleRoot:   w.MerkleRoot,
		Entries:      w.Entries,
		ProposerDID:  w.ProposerDID,
		ProposerSig:  w.ProposerSig,
		Timestamp:    time.UnixMilli(w.Timestamp).UTC(),
	}
}

// BlockToWire converts a domain Block to its wire format.
func BlockToWire(b aicp.Block) BlockWire {
	sigs := make([]ValidatorSigWire, len(b.Signatures))
	for i, s := range b.Signatures {
		sigs[i] = ValidatorSigWire{
			ValidatorDID: s.ValidatorDID,
			Signature:    s.Signature,
		}
	}
	return BlockWire{
		Number:       b.Number,
		Hash:         b.Hash,
		PreviousHash: b.PreviousHash,
		MerkleRoot:   b.MerkleRoot,
		EntryCount:   b.EntryCount,
		ValidatorDID: b.ValidatorDID,
		ValidatorSig: b.ValidatorSig,
		Signatures:   sigs,
		CreatedAt:    b.CreatedAt.UnixMilli(),
	}
}

// BlockFromWire converts a wire Block to the domain type.
func BlockFromWire(w BlockWire) aicp.Block {
	sigs := make([]aicp.ValidatorSig, len(w.Signatures))
	for i, s := range w.Signatures {
		sigs[i] = aicp.ValidatorSig{
			ValidatorDID: s.ValidatorDID,
			Signature:    s.Signature,
		}
	}
	return aicp.Block{
		Number:       w.Number,
		Hash:         w.Hash,
		PreviousHash: w.PreviousHash,
		MerkleRoot:   w.MerkleRoot,
		EntryCount:   w.EntryCount,
		ValidatorDID: w.ValidatorDID,
		ValidatorSig: w.ValidatorSig,
		Signatures:   sigs,
		CreatedAt:    time.UnixMilli(w.CreatedAt).UTC(),
	}
}

// FinalizedBlockToWire converts a domain FinalizedBlock to its wire format.
func FinalizedBlockToWire(fb aicp.FinalizedBlock) FinalizedBlockWire {
	sigs := make([]ValidatorSigWire, len(fb.Signatures))
	for i, s := range fb.Signatures {
		sigs[i] = ValidatorSigWire{
			ValidatorDID: s.ValidatorDID,
			Signature:    s.Signature,
		}
	}
	return FinalizedBlockWire{
		Block:      BlockToWire(fb.Block),
		Signatures: sigs,
	}
}

// FinalizedBlockFromWire converts a wire FinalizedBlock to the domain type.
func FinalizedBlockFromWire(w FinalizedBlockWire) aicp.FinalizedBlock {
	sigs := make([]aicp.ValidatorSig, len(w.Signatures))
	for i, s := range w.Signatures {
		sigs[i] = aicp.ValidatorSig{
			ValidatorDID: s.ValidatorDID,
			Signature:    s.Signature,
		}
	}
	return aicp.FinalizedBlock{
		Block:      BlockFromWire(w.Block),
		Signatures: sigs,
	}
}
