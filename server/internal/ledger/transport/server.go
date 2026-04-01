package transport

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Server exposes validator-to-validator communication endpoints over HTTP.
// It delegates signing to the consensus engine and block retrieval to the
// block sync server, keeping itself free of business logic.
type Server struct {
	consensus *ledger.ConsensusEngine
	syncSrv   *ledger.BlockSyncServer
	logger    *slog.Logger
}

// NewServer creates a validator transport server.
func NewServer(consensus *ledger.ConsensusEngine, syncSrv *ledger.BlockSyncServer, logger *slog.Logger) *Server {
	return &Server{
		consensus: consensus,
		syncSrv:   syncSrv,
		logger:    logger,
	}
}

// Router returns a chi.Router with all validator communication endpoints
// mounted. The caller should mount this under a protected path, e.g.
// r.Mount("/internal/validator", transportServer.Router()).
func (s *Server) Router() chi.Router {
	r := chi.NewRouter()

	r.Post("/sign", s.handleSign)
	r.Post("/blocks", s.handleFetchBlocks)
	r.Get("/latest", s.handleLatestBlock)
	r.Get("/health", s.handleHealth)

	return r
}

// handleSign processes a signature request from a peer validator.
// It verifies the proposed block and signs the block hash with the local key.
func (s *Server) handleSign(w http.ResponseWriter, r *http.Request) {
	var req SignatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed request body")
		return
	}
	defer func() { _ = r.Body.Close() }()

	if len(req.BlockHash) == 0 {
		s.writeError(w, http.StatusBadRequest, "MISSING_BLOCK_HASH", "block_hash is required")
		return
	}

	sig, err := s.consensus.SignBlock(r.Context(), []byte(req.BlockHash))
	if err != nil {
		s.logger.Error("failed to sign block",
			"error", err,
			"block_number", req.ProposedBlock.Number,
		)
		s.writeError(w, http.StatusInternalServerError, "SIGN_FAILED", "failed to sign block")
		return
	}

	s.writeJSON(w, http.StatusOK, SignatureResponse{
		ValidatorDID: sig.ValidatorDID,
		Signature:    sig.Signature,
	})
}

// handleFetchBlocks returns a range of finalized blocks starting from the
// requested block number. The server caps the batch at 100 blocks per request
// (enforced by BlockSyncServer.GetBlocks).
func (s *Server) handleFetchBlocks(w http.ResponseWriter, r *http.Request) {
	var req FetchBlocksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed request body")
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Determine the upper bound for the range.
	latestNum, err := s.syncSrv.LatestBlockNumber(r.Context())
	if err != nil {
		s.logger.Error("failed to get latest block number", "error", err)
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to determine chain height")
		return
	}

	if req.FromBlock > latestNum {
		// Nothing to return -- the peer is ahead or at the same height.
		s.writeJSON(w, http.StatusOK, FetchBlocksResponse{
			Blocks: []FinalizedBlockWire{},
		})
		return
	}

	blocks, err := s.syncSrv.GetBlocks(r.Context(), req.FromBlock, latestNum)
	if err != nil {
		s.logger.Error("failed to fetch blocks",
			"error", err,
			"from", req.FromBlock,
			"to", latestNum,
		)
		s.writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "failed to fetch blocks")
		return
	}

	wireBlocks := make([]FinalizedBlockWire, len(blocks))
	for i, b := range blocks {
		wireBlocks[i] = FinalizedBlockWire{
			Block:      BlockToWire(b),
			Signatures: blockSigsToWire(b.Signatures),
		}
	}

	s.writeJSON(w, http.StatusOK, FetchBlocksResponse{Blocks: wireBlocks})
}

// handleLatestBlock returns the number of the most recent finalized block.
func (s *Server) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	num, err := s.syncSrv.LatestBlockNumber(r.Context())
	if err != nil {
		s.logger.Error("failed to get latest block number", "error", err)
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get latest block")
		return
	}

	s.writeJSON(w, http.StatusOK, LatestBlockResponse{BlockNumber: num})
}

// handleHealth verifies the backend is responsive.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.syncSrv.HealthCheck(r.Context()); err != nil {
		s.logger.Error("health check failed", "error", err)
		s.writeJSON(w, http.StatusServiceUnavailable, HealthResponse{Healthy: false})
		return
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{Healthy: true})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// errorResponse matches the project-wide JSON error envelope.
type errorResponse struct {
	Success bool           `json:"success"`
	Error   *errorPayload  `json:"error,omitempty"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Success: false,
		Error: &errorPayload{
			Code:    code,
			Message: message,
		},
	})
}

// blockSigsToWire converts a slice of domain ValidatorSigs to wire format.
func blockSigsToWire(sigs []aicp.ValidatorSig) []ValidatorSigWire {
	if len(sigs) == 0 {
		return []ValidatorSigWire{}
	}
	out := make([]ValidatorSigWire, len(sigs))
	for i, s := range sigs {
		out[i] = ValidatorSigWire{
			ValidatorDID: s.ValidatorDID,
			Signature:    s.Signature,
		}
	}
	return out
}
