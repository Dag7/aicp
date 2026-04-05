// Package main is the entrypoint for the AICP protocol server.
// It wires together all protocol layers, starts the HTTP server
// and metrics endpoint, and handles graceful shutdown.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/aicp-protocol/aicp/server/internal/api"
	websocketpkg "github.com/aicp-protocol/aicp/server/internal/api/websocket"
	"github.com/aicp-protocol/aicp/server/internal/commerce"
	"github.com/aicp-protocol/aicp/server/internal/commerce/adapter"
	"github.com/aicp-protocol/aicp/server/internal/config"
	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/discovery"
	"github.com/aicp-protocol/aicp/server/internal/events"
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/internal/ledger/transport"
	"github.com/aicp-protocol/aicp/server/internal/onboarding"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server terminated with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize structured logger.
	logger := newLogger(cfg.Server.LogLevel, cfg.Server.LogFormat)
	slog.SetDefault(logger)

	slog.Info("starting AICP server",
		"version", aicp.Version,
		"commit", aicp.Commit,
		"protocol", aicp.ProtocolVersion,
		"addr", cfg.Server.Addr(),
		"metrics_addr", cfg.Server.MetricsAddr(),
	)

	// Create root context that cancels on OS signals.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── PostgreSQL ──────────────────────────────────────────────────────
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("parsing database URL: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("creating PostgreSQL pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging PostgreSQL: %w", err)
	}
	slog.Info("PostgreSQL connected", "url", maskDSN(cfg.Database.URL))

	// ── Redis ───────────────────────────────────────────────────────────
	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("parsing Redis URL: %w", err)
	}
	redisOpts.MaxRetries = cfg.Redis.MaxRetries
	redisOpts.ReadTimeout = cfg.Redis.ReadTimeout
	redisOpts.WriteTimeout = cfg.Redis.WriteTimeout
	redisOpts.PoolSize = cfg.Redis.PoolSize

	rdb := redis.NewClient(redisOpts)
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Error("closing Redis", "error", err)
		}
	}()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("pinging Redis: %w", err)
	}
	slog.Info("Redis connected", "addr", redisOpts.Addr)

	// ── Ledger Backend ──────────────────────────────────────────────────
	backend := ledger.NewPostgresBackend(pool)
	if err := backend.Initialize(ctx); err != nil {
		return fmt.Errorf("initializing ledger backend: %w", err)
	}
	slog.Info("ledger backend initialized")

	// ── Server Key Pair (for payment token signing) ─────────────────────
	serverKP, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		return fmt.Errorf("generating server key pair: %w", err)
	}
	serverDID := crypto.PrincipalDID(serverKP.PublicKey)
	issuerPubKeyMultibase := crypto.PublicKeyMultibase(serverKP.PublicKey)
	slog.Info("server identity generated", "did", serverDID)

	// ── Identity Layer ──────────────────────────────────────────────────
	didMgr := identity.NewDIDManager(backend)
	credMgr := identity.NewCredentialManager(backend, didMgr)
	delegationVerifier := identity.NewDelegationVerifier(backend, didMgr, credMgr)
	principalSvc := identity.NewPrincipalService(didMgr, credMgr, backend)

	// ── Discovery Layer ─────────────────────────────────────────────────
	ansMgr := discovery.NewANSManager(backend)
	searchSvc := discovery.NewSearchService(backend)
	wellKnownSvc := discovery.NewWellKnownService(ansMgr)

	// ── Event System ────────────────────────────────────────────────────
	emitter := events.NewEmitter(logger)

	// ── Commerce Layer ──────────────────────────────────────────────────
	tokenIssuer, err := commerce.NewTokenIssuer(serverKP, cfg.Commerce.PaymentTokenTTL)
	if err != nil {
		return fmt.Errorf("creating token issuer: %w", err)
	}

	tokenValidator := commerce.NewTokenValidator(backend)
	spendingTracker := commerce.NewSpendingTracker(backend)
	receiptGen := commerce.NewReceiptGenerator()
	adapterRegistry := adapter.DefaultRegistry()
	paymentExecutor := commerce.NewPaymentExecutor(
		adapterRegistry, tokenValidator, spendingTracker, receiptGen, backend, logger,
	)

	// ── Approval Service ───────────────────────────────────────────────
	approvalSvc := commerce.NewApprovalService(rdb, delegationVerifier, emitter, logger)
	paymentExecutor = paymentExecutor.
		WithApprovalService(approvalSvc).
		WithDelegationVerifier(delegationVerifier)

	// ── Onboarding Layer ────────────────────────────────────────────────
	sessionMgr := onboarding.NewSessionManager(rdb, 30*time.Minute)
	challengeSvc := onboarding.NewChallengeService(rdb, serverDID)
	trustComputer := onboarding.NewTrustComputer(backend)
	rateLimiter := onboarding.NewRateLimiter(rdb, 100, 1000, cfg.Server.RateLimitWindow)

	// ── Consensus & Block Production ───────────────────────────────────
	validatorMgr := ledger.NewValidatorManager(backend)

	grpcAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Ledger.GRPCPort)
	validator, err := validatorMgr.RegisterLocal(ctx, serverKP, grpcAddr)
	if err != nil {
		return fmt.Errorf("registering local validator: %w", err)
	}
	validatorDID := validator.DID

	consensus := ledger.NewConsensusEngine(backend, validatorDID, serverKP.PrivateKey, logger)
	if err := consensus.RefreshValidators(ctx); err != nil {
		return fmt.Errorf("refreshing validators: %w", err)
	}

	blockProducer := ledger.NewBlockProducer(backend, consensus, cfg.Ledger.BlockInterval, logger)

	// Create genesis block if no blocks exist yet.
	if _, err := backend.GetLatestBlock(ctx); errors.Is(err, aicp.ErrNotFound) {
		genesisHash := ledger.BuildBlockHash(0,
			"0000000000000000000000000000000000000000000000000000000000000000",
			crypto.SHA256Hex([]byte("genesis")),
			validatorDID,
		)
		genesisSignature, err := crypto.SignJSON(serverKP.PrivateKey, []byte(genesisHash))
		if err != nil {
			return fmt.Errorf("signing genesis block: %w", err)
		}
		genesis := ledger.BuildGenesisBlock(validatorDID, genesisSignature)
		if err := backend.StoreBlock(ctx, genesis); err != nil {
			return fmt.Errorf("storing genesis block: %w", err)
		}
		slog.Info("genesis block created", "hash", genesis.Hash)
	}

	blockProducer.Start(ctx)
	defer blockProducer.Stop()

	// ── Validator Transport & Block Sync ─────────────────────────────
	blockSyncer := ledger.NewBlockSyncer(backend, consensus, logger)
	_ = blockSyncer

	syncServer := ledger.NewBlockSyncServer(backend, logger)
	validatorTransport := transport.NewServer(consensus, syncServer, logger)

	// Register remote validator signers for configured peers.
	for _, peer := range cfg.Ledger.PeerAddresses {
		if peer == "" {
			continue
		}
		signer := transport.NewHTTPValidatorSigner(peer + "/internal/validator")
		consensus.RegisterSigner(peer, signer)
		slog.Info("registered remote validator signer", "peer", peer)
	}

	slog.Info("consensus engine started", "validator_did", validatorDID, "block_interval", cfg.Ledger.BlockInterval)

	// ── WebSocket ───────────────────────────────────────────────────────
	wsHub := websocketpkg.NewHub(logger)
	ticketAuth := websocketpkg.NewTicketAuth(rdb)

	// ── Event → WebSocket Bridge ─────────────────────────────────────
	bridge := events.NewWebSocketBridge(emitter, wsHub, logger)
	bridge.Start()

	// ── HTTP Router (chi) ───────────────────────────────────────────────
	router := api.NewRouter(api.RouterConfig{
		Logger:  logger,
		Backend: backend,
		Redis:   rdb,

		DIDManager:         didMgr,
		CredentialManager:  credMgr,
		DelegationVerifier: delegationVerifier,
		PrincipalService:   principalSvc,

		ANSManager:    ansMgr,
		SearchService: searchSvc,
		WellKnown:     wellKnownSvc,

		TokenIssuer:           tokenIssuer,
		PaymentExecutor:       paymentExecutor,
		ApprovalService:       approvalSvc,
		IssuerPubKeyMultibase: issuerPubKeyMultibase,

		SessionManager:   sessionMgr,
		ChallengeService: challengeSvc,
		TrustComputer:    trustComputer,
		RateLimiter:      rateLimiter,

		Hub:        wsHub,
		TicketAuth: ticketAuth,

		ValidatorRouter: validatorTransport.Router(),

		ServiceDID:      serverDID,
		AllowedOrigins:  cfg.Server.AllowedOrigins,
		RateLimitMax:    cfg.Server.RateLimitMax,
		RateLimitWindow: cfg.Server.RateLimitWindow,
	})

	slog.Info("router initialized with all protocol handlers")

	// ── Main HTTP Server ────────────────────────────────────────────────
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second, // Slowloris protection.
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    1 << 16, // 64KB header limit.
	}

	// ── Metrics Server ──────────────────────────────────────────────────
	metricsSrv := &http.Server{
		Addr:    cfg.Server.MetricsAddr(),
		Handler: api.NewMetricsRouter(),
	}

	// Channel to collect server errors.
	errCh := make(chan error, 2)

	go func() {
		slog.Info("metrics server listening", "addr", cfg.Server.MetricsAddr())
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	go func() {
		slog.Info("HTTP server listening", "addr", cfg.Server.Addr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining connections...")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	shutdownErrCh := make(chan error, 2)
	go func() { shutdownErrCh <- srv.Shutdown(shutdownCtx) }()
	go func() { shutdownErrCh <- metricsSrv.Shutdown(shutdownCtx) }()

	for range 2 {
		if err := <-shutdownErrCh; err != nil {
			slog.Error("error during shutdown", "error", err)
		}
	}

	slog.Info("server stopped gracefully")
	return nil
}

// maskDSN masks the password in a PostgreSQL DSN for safe logging.
func maskDSN(dsn string) string {
	// Simple masking: find ://user:pass@ and replace pass with ***
	for i := range len(dsn) - 3 {
		if dsn[i:i+3] == "://" {
			atIdx := -1
			colonIdx := -1
			for j := i + 3; j < len(dsn); j++ {
				if dsn[j] == ':' && colonIdx == -1 {
					colonIdx = j
				}
				if dsn[j] == '@' {
					atIdx = j
					break
				}
			}
			if colonIdx > 0 && atIdx > colonIdx {
				return dsn[:colonIdx+1] + "***" + dsn[atIdx:]
			}
			break
		}
	}
	return dsn
}

// newLogger creates a structured slog.Logger based on config.
func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: lvl == slog.LevelDebug,
	}

	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
