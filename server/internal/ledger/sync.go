package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// BlockSyncer synchronizes blocks between validator nodes.
// Remote validators can push blocks to the local node, or the local node
// can pull missing blocks from peers.
type BlockSyncer struct {
	backend   LedgerBackend
	consensus *ConsensusEngine
	logger    *slog.Logger
}

// NewBlockSyncer creates a new block syncer.
func NewBlockSyncer(backend LedgerBackend, consensus *ConsensusEngine, logger *slog.Logger) *BlockSyncer {
	return &BlockSyncer{
		backend:   backend,
		consensus: consensus,
		logger:    logger,
	}
}

// SyncFromPeer fetches and applies blocks from a remote peer starting from the given block number.
type BlockFetcher interface {
	// FetchBlocks retrieves blocks from a remote peer in ascending order.
	FetchBlocks(ctx context.Context, fromBlock uint64) ([]aicp.FinalizedBlock, error)
}

// SyncFromPeer synchronizes blocks from a remote peer starting from fromBlock.
func (bs *BlockSyncer) SyncFromPeer(ctx context.Context, fetcher BlockFetcher, fromBlock uint64) error {
	blocks, err := fetcher.FetchBlocks(ctx, fromBlock)
	if err != nil {
		return fmt.Errorf("fetching blocks from peer: %w", err)
	}

	for _, fb := range blocks {
		if err := bs.applyBlock(ctx, fb); err != nil {
			return fmt.Errorf("applying block %d: %w", fb.Block.Number, err)
		}
	}

	bs.logger.Info("block sync complete",
		"blocks_synced", len(blocks),
		"from", fromBlock,
	)
	return nil
}

// applyBlock validates and stores a finalized block from a peer.
func (bs *BlockSyncer) applyBlock(ctx context.Context, fb aicp.FinalizedBlock) error {
	// Verify the block hash.
	expectedHash := BuildBlockHash(
		fb.Block.Number,
		fb.Block.PreviousHash,
		fb.Block.MerkleRoot,
		fb.Block.ValidatorDID,
	)
	if fb.Block.Hash != expectedHash {
		return fmt.Errorf("block hash mismatch: expected %s, got %s", expectedHash, fb.Block.Hash)
	}

	// Verify we have the parent block (unless this is block 0 or 1).
	if fb.Block.Number > 1 {
		parent, err := bs.backend.GetBlock(ctx, fb.Block.Number-1)
		if err != nil {
			return fmt.Errorf("getting parent block: %w", err)
		}
		if parent.Hash != fb.Block.PreviousHash {
			return fmt.Errorf("parent hash mismatch: expected %s, got %s", parent.Hash, fb.Block.PreviousHash)
		}
	}

	// Verify signatures meet quorum.
	validSigs := 0
	for _, sig := range fb.Signatures {
		valid, err := bs.consensus.VerifyBlockSignature(ctx, sig, fb.Block.Hash)
		if err != nil {
			bs.logger.Warn("failed to verify block signature",
				"validator", sig.ValidatorDID,
				"error", err,
			)
			continue
		}
		if valid {
			validSigs++
		}
	}

	validators, err := bs.backend.ListValidators(ctx)
	if err != nil {
		return fmt.Errorf("listing validators for quorum check: %w", err)
	}
	required := quorum(len(validators))
	if validSigs < required {
		return fmt.Errorf("insufficient valid signatures: got %d, need %d", validSigs, required)
	}

	// Store the block.
	return bs.backend.StoreBlock(ctx, fb.Block)
}

// CatchUp checks the local chain height against the latest known block
// and fetches any missing blocks from a peer.
func (bs *BlockSyncer) CatchUp(ctx context.Context, fetcher BlockFetcher) error {
	latest, err := bs.backend.GetLatestBlock(ctx)
	if err != nil && err != aicp.ErrNotFound {
		return fmt.Errorf("getting latest block: %w", err)
	}

	var fromBlock uint64 = 1
	if latest != nil {
		fromBlock = latest.Number + 1
	}

	return bs.SyncFromPeer(ctx, fetcher, fromBlock)
}

// BlockSyncServer handles incoming block sync requests from peers.
type BlockSyncServer struct {
	backend LedgerBackend
	logger  *slog.Logger
}

// NewBlockSyncServer creates a server that responds to peer sync requests.
func NewBlockSyncServer(backend LedgerBackend, logger *slog.Logger) *BlockSyncServer {
	return &BlockSyncServer{
		backend: backend,
		logger:  logger,
	}
}

// GetBlocks returns a range of blocks for a requesting peer.
func (bss *BlockSyncServer) GetBlocks(ctx context.Context, fromBlock, toBlock uint64) ([]aicp.Block, error) {
	if toBlock < fromBlock {
		return nil, fmt.Errorf("invalid range: from=%d to=%d", fromBlock, toBlock)
	}

	const maxBatchSize = 100
	if toBlock-fromBlock > maxBatchSize {
		toBlock = fromBlock + maxBatchSize
	}

	var blocks []aicp.Block
	for num := fromBlock; num <= toBlock; num++ {
		block, err := bss.backend.GetBlock(ctx, num)
		if err != nil {
			if err == aicp.ErrNotFound {
				break // Reached the end of our chain.
			}
			return nil, fmt.Errorf("getting block %d: %w", num, err)
		}
		blocks = append(blocks, *block)
	}

	return blocks, nil
}

// LatestBlockNumber returns the number of the most recent block.
func (bss *BlockSyncServer) LatestBlockNumber(ctx context.Context) (uint64, error) {
	latest, err := bss.backend.GetLatestBlock(ctx)
	if err != nil {
		if err == aicp.ErrNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("getting latest block: %w", err)
	}
	return latest.Number, nil
}

// HealthCheck verifies the backend is responsive and the chain is consistent.
func (bss *BlockSyncServer) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return bss.backend.Healthy(ctx)
}
