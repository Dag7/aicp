package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	ws "github.com/coder/websocket"

	"github.com/aicp-protocol/aicp/server/internal/api/handlers"
	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	websocketpkg "github.com/aicp-protocol/aicp/server/internal/api/websocket"
	"github.com/aicp-protocol/aicp/server/internal/commerce"
	"github.com/aicp-protocol/aicp/server/internal/discovery"
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/internal/onboarding"
)

// RouterConfig holds all dependencies needed to build the HTTP router.
type RouterConfig struct {
	Logger  *slog.Logger
	Backend ledger.LedgerBackend
	Redis   *redis.Client

	// Identity layer
	DIDManager         *identity.DIDManager
	CredentialManager  *identity.CredentialManager
	DelegationVerifier *identity.DelegationVerifier
	PrincipalService   *identity.PrincipalService

	// Discovery layer
	ANSManager    *discovery.ANSManager
	SearchService *discovery.SearchService
	WellKnown     *discovery.WellKnownService

	// Commerce layer
	TokenIssuer           *commerce.TokenIssuer
	PaymentExecutor       *commerce.PaymentExecutor
	ApprovalService       *commerce.ApprovalService
	IssuerPubKeyMultibase string

	// Onboarding layer
	SessionManager   *onboarding.SessionManager
	ChallengeService *onboarding.ChallengeService
	TrustComputer    *onboarding.TrustComputer
	RateLimiter      *onboarding.RateLimiter

	// WebSocket
	Hub        *websocketpkg.Hub
	TicketAuth *websocketpkg.TicketAuth

	// Validator transport (inter-node communication)
	ValidatorRouter http.Handler

	// Config
	ServiceDID      string
	AllowedOrigins  []string
	RateLimitMax    int
	RateLimitWindow time.Duration
}

// NewRouter creates the chi router with all AICP endpoints and middleware.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// ─── Global Middleware ───────────────────────────────────────────────
	r.Use(middleware.Recovery(cfg.Logger))
	r.Use(middleware.BodyLimit(middleware.MaxBodySize))
	r.Use(middleware.Logging(cfg.Logger))
	r.Use(middleware.Metrics())
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.DIDAuth())
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)

	if cfg.Redis != nil && cfg.RateLimitMax > 0 {
		r.Use(middleware.RateLimit(cfg.Redis, cfg.RateLimitMax, cfg.RateLimitWindow))
	}

	// ─── Health (unauthenticated) ───────────────────────────────────────
	health := handlers.NewHealthHandler(cfg.Backend)
	r.Get("/health", health.Health)
	r.Get("/ready", health.Ready)
	r.Handle("/metrics", promhttp.Handler())

	// ─── Well-Known (unauthenticated) ───────────────────────────────────
	if cfg.WellKnown != nil {
		wk := handlers.NewWellKnownHandler(cfg.WellKnown, cfg.ServiceDID)
		r.Get("/.well-known/aicp.json", wk.Handle)
	}

	// ─── AICP v1 API ────────────────────────────────────────────────────
	r.Route("/aicp/v1", func(r chi.Router) {
		// DID management
		if cfg.DIDManager != nil && cfg.PrincipalService != nil {
			did := handlers.NewDIDHandler(
				cfg.DIDManager,
				cfg.PrincipalService,
				cfg.CredentialManager,
			)
			// Create principal: no auth required (it IS registration).
			r.Post("/did", did.Create)
			// Read: no auth required.
			r.Get("/did/{did}", did.Resolve)

			// Mutating DID operations require authentication.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				r.Post("/did/agent", did.CreateAgent)
				r.Put("/did/{did}", did.Update)
				r.Post("/did/{did}/deactivate", did.Deactivate)
			})
		}

		// Discovery
		if cfg.ANSManager != nil && cfg.SearchService != nil {
			disc := handlers.NewDiscoveryHandler(cfg.ANSManager, cfg.SearchService)
			// Read: no auth required.
			r.Get("/discover/search", disc.Search)
			r.Get("/discover/{did}", disc.Get)

			// Mutating discovery operations require authentication.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				r.Post("/discover/register", disc.Register)
				r.Delete("/discover/{did}", disc.Delete)
			})
		}

		// Onboarding (first step is unauthenticated; subsequent steps use session ID)
		if cfg.SessionManager != nil {
			onboard := handlers.NewOnboardHandler(
				cfg.SessionManager,
				cfg.ChallengeService,
				cfg.TrustComputer,
				cfg.RateLimiter,
				cfg.ServiceDID,
			)
			r.Post("/onboard", onboard.Initiate)
			r.Post("/onboard/{id}/negotiate", onboard.Negotiate)
			r.Post("/onboard/{id}/authorize", onboard.Authorize)
			r.Post("/onboard/{id}/provision", onboard.Provision)
		}

		// Verify (accepts signed proof in body, no auth header needed)
		if cfg.DIDManager != nil {
			verify := handlers.NewVerifyHandler(
				cfg.DIDManager,
				cfg.CredentialManager,
				cfg.DelegationVerifier,
				cfg.TrustComputer,
			)
			r.Post("/verify", verify.Handle)
		}

		// Payment (all operations require authentication)
		if cfg.TokenIssuer != nil {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				payment := handlers.NewPaymentHandler(
					cfg.TokenIssuer,
					cfg.PaymentExecutor,
					cfg.IssuerPubKeyMultibase,
				)
				r.Post("/payment/token", payment.RequestToken)
				r.Post("/payment/execute", payment.Execute)
				r.Get("/payment/status/{id}", payment.Status)
			})
		}

		// Payment Approvals (all operations require authentication)
		if cfg.ApprovalService != nil {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				approval := handlers.NewApprovalHandler(cfg.ApprovalService)
				r.Get("/payment/approval/{id}", approval.GetApproval)
				r.Post("/payment/approval/{id}/approve", approval.Approve)
				r.Post("/payment/approval/{id}/deny", approval.Deny)
				r.Get("/payment/approvals/pending", approval.ListPending)
			})
		}

		// Credentials (all operations require authentication)
		if cfg.CredentialManager != nil {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				cred := handlers.NewCredentialHandler(cfg.CredentialManager)
				r.Post("/credentials/issue", cred.Issue)
				r.Post("/credentials/{id}/revoke", cred.Revoke)
			})
		}

		// Trust (read-only, no auth required)
		if cfg.TrustComputer != nil {
			trust := handlers.NewTrustHandler(cfg.TrustComputer)
			r.Get("/trust/{did}", trust.GetScore)
		}

		// WebSocket ticket (requires authentication)
		if cfg.TicketAuth != nil {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth())
				r.Post("/ws/ticket", func(w http.ResponseWriter, r *http.Request) {
					did, _ := middleware.GetAuthDID(r.Context())
					ticket, err := cfg.TicketAuth.IssueTicket(r.Context(), did)
					if err != nil {
						handlers.JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue ticket")
						return
					}
					handlers.JSON(w, http.StatusOK, map[string]string{"ticket": ticket})
				})
			})
		}

		// WebSocket upgrade (ticket-authenticated, not header-authenticated)
		if cfg.Hub != nil {
			r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
				ticket := r.URL.Query().Get("ticket")
				if ticket == "" {
					handlers.JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "ticket query parameter required")
					return
				}

				did := ""
				if cfg.TicketAuth != nil {
					var err error
					did, err = cfg.TicketAuth.ValidateTicket(r.Context(), ticket)
					if err != nil {
						handlers.JSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired ticket")
						return
					}
				}

				conn, err := ws.Accept(w, r, &ws.AcceptOptions{
					OriginPatterns: cfg.AllowedOrigins,
				})
				if err != nil {
					cfg.Logger.Error("websocket accept failed", "error", err)
					return
				}

				clientID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
				client := websocketpkg.NewClient(clientID, did, conn, cfg.Hub, cfg.Logger)
				cfg.Hub.Register(client)
				client.Run(context.Background())
			})
		}
	})

	// ─── Internal Validator Transport ─────────────────────────────────
	if cfg.ValidatorRouter != nil {
		r.Mount("/internal/validator", cfg.ValidatorRouter)
	}

	return r
}

// NewMetricsRouter creates a separate router for the metrics/admin endpoint.
func NewMetricsRouter() http.Handler {
	r := chi.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	return r
}
