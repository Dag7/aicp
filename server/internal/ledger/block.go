package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// BlockProducer batches pending ledger entries into blocks at a configured interval.
type BlockProducer struct {
	backend   LedgerBackend
	consensus *ConsensusEngine
	interval  time.Duration
	logger    *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewBlockProducer creates a new block producer.
func NewBlockProducer(backend LedgerBackend, consensus *ConsensusEngine, interval time.Duration, logger *slog.Logger) *BlockProducer {
	return &BlockProducer{
		backend:   backend,
		consensus: consensus,
		interval:  interval,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the block production loop. Blocks until Stop is called.
func (bp *BlockProducer) Start(ctx context.Context) {
	bp.wg.Add(1)
	go bp.run(ctx)
}

// Stop signals the producer to stop and waits for it to finish.
func (bp *BlockProducer) Stop() {
	close(bp.stopCh)
	bp.wg.Wait()
}

func (bp *BlockProducer) run(ctx context.Context) {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-bp.stopCh:
			return
		case <-ticker.C:
			if err := bp.produceBlock(ctx); err != nil {
				bp.logger.Error("block production failed", "error", err)
			}
		}
	}
}

// produceBlock collects pending entries, builds a block, and runs consensus.
func (bp *BlockProducer) produceBlock(ctx context.Context) error {
	// Only the current leader produces blocks.
	latestBlock, err := bp.backend.GetLatestBlock(ctx)
	if err != nil && err != aicp.ErrNotFound {
		return fmt.Errorf("getting latest block: %w", err)
	}

	var nextNumber uint64 = 1
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if latestBlock != nil {
		nextNumber = latestBlock.Number + 1
		previousHash = latestBlock.Hash
	}

	if !bp.consensus.IsLeader(ctx, nextNumber) {
		return nil // Not our turn.
	}

	entries, err := bp.backend.GetPendingEntries(ctx)
	if err != nil {
		return fmt.Errorf("getting pending entries: %w", err)
	}
	if len(entries) == 0 {
		return nil // Nothing to include.
	}

	// Build Merkle root from entry hashes.
	hashes := make([]string, len(entries))
	for i, e := range entries {
		hashes[i] = e.EntryHash
	}
	merkleRoot, err := crypto.MerkleRoot(hashes)
	if err != nil {
		return fmt.Errorf("computing merkle root: %w", err)
	}

	// Build the proposed block.
	proposed := &aicp.ProposedBlock{
		Number:       nextNumber,
		PreviousHash: previousHash,
		MerkleRoot:   merkleRoot,
		Entries:      entries,
		ProposerDID:  bp.consensus.LocalValidatorDID(),
		Timestamp:    time.Now().UTC(),
	}

	// Sign the block hash.
	blockHashInput := fmt.Sprintf("%d:%s:%s:%s", proposed.Number, proposed.PreviousHash, proposed.MerkleRoot, proposed.ProposerDID)
	blockHash := crypto.SHA256Hex([]byte(blockHashInput))

	sig, err := bp.consensus.SignBlock(ctx, []byte(blockHash))
	if err != nil {
		return fmt.Errorf("signing block: %w", err)
	}
	proposed.ProposerSig = sig.Signature

	// Collect signatures from other validators.
	finalized, err := bp.consensus.FinalizeBlock(ctx, proposed)
	if err != nil {
		return fmt.Errorf("finalizing block: %w", err)
	}

	// Persist the finalized block.
	block := aicp.Block{
		Number:       finalized.Block.Number,
		Hash:         finalized.Block.Hash,
		PreviousHash: finalized.Block.PreviousHash,
		MerkleRoot:   finalized.Block.MerkleRoot,
		EntryCount:   finalized.Block.EntryCount,
		ValidatorDID: finalized.Block.ValidatorDID,
		ValidatorSig: finalized.Block.ValidatorSig,
		Signatures:   finalized.Signatures,
		CreatedAt:    time.Now().UTC(),
	}

	if err := bp.backend.StoreBlock(ctx, block); err != nil {
		return fmt.Errorf("storing block: %w", err)
	}

	bp.logger.Info("block produced",
		"number", block.Number,
		"entries", block.EntryCount,
		"hash", block.Hash,
	)

	return nil
}

// BuildBlockHash computes the canonical hash of a proposed block.
func BuildBlockHash(number uint64, previousHash, merkleRoot, proposerDID string) string {
	input := fmt.Sprintf("%d:%s:%s:%s", number, previousHash, merkleRoot, proposerDID)
	return crypto.SHA256Hex([]byte(input))
}

// BuildGenesisBlock creates the genesis block (block 0).
func BuildGenesisBlock(validatorDID, validatorSig string) aicp.Block {
	genesisData, _ := json.Marshal(map[string]string{
		"type":    "genesis",
		"message": "AICP Consortium Ledger Genesis Block",
	})
	genesisHash := crypto.SHA256Hex(genesisData)

	return aicp.Block{
		Number:       0,
		Hash:         genesisHash,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		MerkleRoot:   crypto.SHA256Hex([]byte("genesis")),
		EntryCount:   0,
		ValidatorDID: validatorDID,
		ValidatorSig: validatorSig,
		CreatedAt:    time.Now().UTC(),
	}
}
