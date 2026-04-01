package ledger

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ConsensusEngine implements round-robin leader election with 2f+1 signature
// collection for block finalization across a set of consortium validators.
type ConsensusEngine struct {
	backend  LedgerBackend
	localDID string
	localKey ed25519.PrivateKey
	logger   *slog.Logger

	mu         sync.RWMutex
	validators []aicp.Validator
	signers    map[string]ValidatorSigner
}

// ValidatorSigner is an abstraction for collecting a remote validator's signature.
// In production, this is implemented via gRPC; for testing, it can be a local callback.
type ValidatorSigner interface {
	// RequestSignature asks a remote validator to sign a block hash.
	RequestSignature(ctx context.Context, blockHash []byte, proposed *aicp.ProposedBlock) (*aicp.ValidatorSig, error)
}

// NewConsensusEngine creates a new consensus engine for the local validator.
func NewConsensusEngine(backend LedgerBackend, localDID string, localKey ed25519.PrivateKey, logger *slog.Logger) *ConsensusEngine {
	return &ConsensusEngine{
		backend:  backend,
		localDID: localDID,
		localKey: localKey,
		logger:   logger,
		signers:  make(map[string]ValidatorSigner),
	}
}

// LocalValidatorDID returns the DID of the local validator node.
func (ce *ConsensusEngine) LocalValidatorDID() string {
	return ce.localDID
}

// RefreshValidators reloads the active validator set from the ledger backend.
func (ce *ConsensusEngine) RefreshValidators(ctx context.Context) error {
	validators, err := ce.backend.ListValidators(ctx)
	if err != nil {
		return fmt.Errorf("listing validators: %w", err)
	}

	ce.mu.Lock()
	ce.validators = validators
	ce.mu.Unlock()

	return nil
}

// RegisterSigner registers a ValidatorSigner for a remote validator.
func (ce *ConsensusEngine) RegisterSigner(validatorDID string, signer ValidatorSigner) {
	ce.mu.Lock()
	ce.signers[validatorDID] = signer
	ce.mu.Unlock()
}

// IsLeader determines whether the local node is the leader for a given block number.
// Uses round-robin: leader = validators[blockNumber % len(validators)].
func (ce *ConsensusEngine) IsLeader(ctx context.Context, blockNumber uint64) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.validators) == 0 {
		// Single node mode: always the leader.
		return true
	}

	idx := blockNumber % uint64(len(ce.validators))
	return ce.validators[idx].DID == ce.localDID
}

// CurrentLeader returns the validator that should propose the given block number.
func (ce *ConsensusEngine) CurrentLeader(ctx context.Context, blockNumber uint64) (*aicp.Validator, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.validators) == 0 {
		return nil, fmt.Errorf("no validators registered")
	}

	idx := blockNumber % uint64(len(ce.validators))
	v := ce.validators[idx]
	return &v, nil
}

// SignBlock signs a block hash with the local validator's private key.
func (ce *ConsensusEngine) SignBlock(ctx context.Context, blockHash []byte) (*aicp.ValidatorSig, error) {
	sig, err := crypto.SignJSON(ce.localKey, blockHash)
	if err != nil {
		return nil, fmt.Errorf("signing block hash: %w", err)
	}
	return &aicp.ValidatorSig{
		ValidatorDID: ce.localDID,
		Signature:    sig,
	}, nil
}

// FinalizeBlock collects 2f+1 signatures from validators to finalize a proposed block.
// In single-node mode, only the local signature is required.
func (ce *ConsensusEngine) FinalizeBlock(ctx context.Context, proposed *aicp.ProposedBlock) (*aicp.FinalizedBlock, error) {
	blockHash := BuildBlockHash(proposed.Number, proposed.PreviousHash, proposed.MerkleRoot, proposed.ProposerDID)

	// Start with the proposer's own signature.
	localSig, err := ce.SignBlock(ctx, []byte(blockHash))
	if err != nil {
		return nil, fmt.Errorf("signing proposed block: %w", err)
	}

	signatures := []aicp.ValidatorSig{*localSig}

	ce.mu.RLock()
	validators := make([]aicp.Validator, len(ce.validators))
	copy(validators, ce.validators)
	ce.mu.RUnlock()

	requiredSigs := quorum(len(validators))

	if len(validators) <= 1 {
		// Single-node mode: one signature suffices.
		return ce.buildFinalized(proposed, blockHash, signatures), nil
	}

	// Collect signatures from remote validators in parallel.
	type sigResult struct {
		sig *aicp.ValidatorSig
		err error
	}
	results := make(chan sigResult, len(validators))

	ce.mu.RLock()
	for _, v := range validators {
		if v.DID == ce.localDID {
			continue // Already have our own signature.
		}
		signer, ok := ce.signers[v.DID]
		if !ok {
			ce.logger.Warn("no signer registered for validator", "did", v.DID)
			continue
		}
		go func(s ValidatorSigner) {
			sig, err := s.RequestSignature(ctx, []byte(blockHash), proposed)
			results <- sigResult{sig: sig, err: err}
		}(signer)
	}
	ce.mu.RUnlock()

	// Collect results.
	remoteCount := len(validators) - 1
	for i := 0; i < remoteCount; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-results:
			if r.err != nil {
				ce.logger.Warn("failed to collect signature", "error", r.err)
				continue
			}
			signatures = append(signatures, *r.sig)
			if len(signatures) >= requiredSigs {
				return ce.buildFinalized(proposed, blockHash, signatures), nil
			}
		}
	}

	if len(signatures) >= requiredSigs {
		return ce.buildFinalized(proposed, blockHash, signatures), nil
	}

	return nil, fmt.Errorf("insufficient signatures: got %d, need %d", len(signatures), requiredSigs)
}

// VerifyBlockSignature verifies that a validator's signature over a block hash is valid.
func (ce *ConsensusEngine) VerifyBlockSignature(ctx context.Context, sig aicp.ValidatorSig, blockHash string) (bool, error) {
	validator, err := ce.backend.GetValidator(ctx, sig.ValidatorDID)
	if err != nil {
		return false, fmt.Errorf("getting validator %s: %w", sig.ValidatorDID, err)
	}

	pubKey, err := crypto.PublicKeyFromMultibase(validator.PublicKey)
	if err != nil {
		return false, fmt.Errorf("decoding validator public key: %w", err)
	}

	return crypto.VerifyJSON(pubKey, []byte(blockHash), sig.Signature)
}

func (ce *ConsensusEngine) buildFinalized(proposed *aicp.ProposedBlock, blockHash string, signatures []aicp.ValidatorSig) *aicp.FinalizedBlock {
	return &aicp.FinalizedBlock{
		Block: aicp.Block{
			Number:       proposed.Number,
			Hash:         blockHash,
			PreviousHash: proposed.PreviousHash,
			MerkleRoot:   proposed.MerkleRoot,
			EntryCount:   len(proposed.Entries),
			ValidatorDID: proposed.ProposerDID,
			ValidatorSig: proposed.ProposerSig,
			Signatures:   signatures,
			CreatedAt:    proposed.Timestamp,
		},
		Signatures: signatures,
	}
}

// quorum returns the minimum number of signatures needed for consensus (2f+1).
// f = (n-1)/3 where n is the total number of validators.
func quorum(n int) int {
	if n <= 1 {
		return 1
	}
	f := (n - 1) / 3
	return 2*f + 1
}
