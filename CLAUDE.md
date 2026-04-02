# AICP - Agentic Identity and Commerce Protocol

## Project Overview

AICP is a unified, open-source protocol (Apache 2.0) that provides autonomous AI agents with:
- **Identity** (Layer 1): W3C DID-based cryptographic identity anchored to human/org principals
- **Discovery** (Layer 2): Agent Name System (ANS) for service discovery
- **Onboarding** (Layer 3): Standardized 4-phase agent registration
- **Commerce** (Layer 4): Provider-agnostic payment abstraction (Stripe, PayPal, USDC)

## Architecture

```
server/          Go protocol server (chi router, PostgreSQL, Redis)
  cmd/
    aicp-server/ Server entrypoint (main.go)
    aicp/        CLI tool (cobra-based, for agents and developers)
  internal/
    config/      Env/YAML config with validation
    crypto/      Ed25519, signing, JWS (go-jose/v4), Merkle trees, base58
    identity/    Layer 1: DID lifecycle, credentials, delegation, revocation, multi-DID resolution, capabilities, OIDC bridge
    discovery/   Layer 2: ANS records, tiered resolver, federated search, well-known, peer registry
    onboarding/  Layer 3: 4-phase state machine, sessions, challenges, trust
    commerce/    Layer 4: Payment tokens (JWS), executor, spending, adapters, approvals
    ledger/      Consortium ledger: PostgreSQL backend, blocks, consensus, sync
    mcp/         MCP server: JSON-RPC 2.0 over stdio, exposes AICP as MCP tools
      transport/ HTTP-based validator-to-validator communication
    api/         HTTP router (chi), handlers, middleware, WebSocket hub
      handlers/  DID, discovery, onboarding, payment, approval, credential, trust, verify
      middleware/ DID-Auth (JWS), rate limiting, CORS, metrics, logging, body limit, recovery
      websocket/ Hub, client, ticket auth, event delivery
    events/      Typed event emitter, Redis pub/sub bridge, WebSocket bridge
  pkg/aicp/      Public types, errors, version constants
  sqlc/          SQL queries + generated type-safe Go code

operator/        K8s Operator (Operator SDK) for consortium ledger management
sdk/typescript/  TypeScript client SDK (@aicp/sdk on npm)
sdk/python/      Python client SDK (aicp on PyPI)
spec/            OpenAPI 3.1 + JSON Schema (protocol source of truth)
deploy/          Docker, Helm charts, Grafana dashboards
docs/            Whitepaper, protocol spec, Docusaurus site
.devcontainer/   Dev environment (Dockerfile, k3d config, scripts)
k8s/             Local K8s infra manifests (PostgreSQL, Redis, pgAdmin)
utils/tilt/      Tilt utility functions (Go builds, Helm, namespaces)
```

## Tech Stack

- **Server**: Go 1.26+ with chi router
- **Database**: PostgreSQL 16+ via pgx driver + sqlc for type-safe queries
- **Cache**: Redis 7+ via go-redis (rate limiting, sessions, pub/sub, nonce cache)
- **Crypto**: Go stdlib crypto/ed25519, crypto/sha256 + go-jose/v4 for JWS
- **Operator**: K8s Operator SDK v1.42.2 (Go 1.24, controller-runtime v0.21.0)
- **Testing**: Go testing + testify + testcontainers-go (real PG + Redis in tests)
- **Linting**: golangci-lint
- **Build**: Taskfile (task runner)
- **CI/CD**: GitHub Actions
- **Local Dev**: Devcontainer + k3d + Tilt (live reload, local K8s)

## Build Commands

```bash
# Go server + CLI
task build          # Build server binary + CLI
task test           # Run all Go tests
task lint           # Run golangci-lint
task fmt            # Format Go code
task migrate:up     # Run database migrations
task migrate:down   # Rollback last migration
task sqlc           # Regenerate sqlc code

# CLI (built from server/cmd/aicp/)
task cli:build      # Build CLI binary
# Usage: aicp did create, aicp did resolve, aicp discover, aicp onboard, aicp pay, etc.

# Docker
task docker:build   # Build Docker image
task docker:up      # Start full stack (server + PG + Redis)
task docker:down    # Stop stack

# SDKs
task sdk:ts:build   # Build TypeScript SDK
task sdk:ts:test    # Test TypeScript SDK
task sdk:py:build   # Build Python SDK
task sdk:py:test    # Test Python SDK

# Helm
task helm:lint      # Lint Helm chart
task all            # Build + test + lint everything
```

## REST API Endpoints

### Public Endpoints (no authentication)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/health` | HealthHandler.Health | Liveness probe |
| GET | `/ready` | HealthHandler.Ready | Readiness probe |
| GET | `/metrics` | promhttp | Prometheus metrics |
| GET | `/.well-known/aicp.json` | WellKnownHandler | Service discovery metadata |
| POST | `/aicp/v1/did` | DIDHandler.Create | Create principal DID |
| GET | `/aicp/v1/did/{did}` | DIDHandler.Resolve | Resolve DID document |
| GET | `/aicp/v1/discover/search` | DiscoveryHandler.Search | Search by capability |
| GET | `/aicp/v1/discover/{did}` | DiscoveryHandler.Get | Get ANS record |
| GET | `/aicp/v1/trust/{did}` | TrustHandler.GetScore | Get trust score |
| POST | `/aicp/v1/onboard` | OnboardHandler.Initiate | Start onboarding |
| POST | `/aicp/v1/onboard/{id}/negotiate` | OnboardHandler.Negotiate | Negotiate capabilities |
| POST | `/aicp/v1/onboard/{id}/authorize` | OnboardHandler.Authorize | Authorize payment |
| POST | `/aicp/v1/onboard/{id}/provision` | OnboardHandler.Provision | Complete onboarding |
| POST | `/aicp/v1/verify` | VerifyHandler.Handle | Verify agent identity |
| GET | `/aicp/v1/ws` | WebSocket upgrade | Real-time events (ticket auth) |

### Authenticated Endpoints (require DID-Auth JWS Bearer token)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/aicp/v1/did/agent` | DIDHandler.CreateAgent | Create agent DID under principal |
| PUT | `/aicp/v1/did/{did}` | DIDHandler.Update | Update DID document |
| POST | `/aicp/v1/did/{did}/deactivate` | DIDHandler.Deactivate | Deactivate DID |
| POST | `/aicp/v1/discover/register` | DiscoveryHandler.Register | Register ANS record |
| DELETE | `/aicp/v1/discover/{did}` | DiscoveryHandler.Delete | Delete ANS record |
| POST | `/aicp/v1/payment/token` | PaymentHandler.RequestToken | Request payment token |
| POST | `/aicp/v1/payment/execute` | PaymentHandler.Execute | Execute payment |
| GET | `/aicp/v1/payment/status/{id}` | PaymentHandler.Status | Check payment status |
| GET | `/aicp/v1/payment/approval/{id}` | ApprovalHandler.GetApproval | Get approval status |
| POST | `/aicp/v1/payment/approval/{id}/approve` | ApprovalHandler.Approve | Approve payment |
| POST | `/aicp/v1/payment/approval/{id}/deny` | ApprovalHandler.Deny | Deny payment |
| GET | `/aicp/v1/payment/approvals/pending` | ApprovalHandler.ListPending | List pending approvals |
| POST | `/aicp/v1/credentials/issue` | CredentialHandler.Issue | Issue credential |
| POST | `/aicp/v1/credentials/{id}/revoke` | CredentialHandler.Revoke | Revoke credential |
| POST | `/aicp/v1/ws/ticket` | inline | Get WebSocket auth ticket |

### Internal Endpoints (validator-to-validator, not exposed)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/internal/validator/sign` | Request block signature from peer |
| GET | `/internal/validator/blocks` | Fetch block range from peer |
| GET | `/internal/validator/latest` | Get latest block number |
| GET | `/internal/validator/health` | Validator health check |

## Security Architecture

### Authentication
- **DID-Auth middleware with cryptographic JWS verification**: Parses `Authorization: Bearer <JWS>` headers, resolves the signer's DID document from the ledger, and **verifies the JWS signature** against the resolved public key. Only after signature verification succeeds is the DID stored in context.
- `DIDAuth(resolver DIDKeyResolver)` accepts a resolver interface for public key lookup — fail-closed when resolver is nil
- `DIDKeyResolver` interface: `ResolvePublicKey(ctx, did, keyID) (ed25519.PublicKey, error)`
- `DIDAuthResolver` in identity package implements `DIDKeyResolver` by resolving DID documents via `DIDManager`
- JWS tokens signed with EdDSA (Ed25519) via go-jose/v4, algorithm restricted to EdDSA only
- `kid` header contains `did:aicp:<id>#key-1`, DID extracted and stored in context after verification
- Protected routes use `middleware.RequireAuth()` group middleware

### Authorization
- **Ownership enforcement**: Mutating operations verify caller DID matches the target resource
  - DID update/deactivate: caller must be the DID or its controller, changes persisted to ledger
  - Agent creation: caller must be the principal, agent DID anchored to ledger
  - ANS register/delete: caller must own the service DID
  - Credential issue: caller must be the issuer
  - Credential revoke: caller DID must match credential's `issuer` field (enforced in `RevokeCredentialByID`)
  - Payment token request: caller DID must be the `agent_did` or its controlling principal (`IsControllerOf`)
  - Approval approve/deny: caller must be the principal of the agent
  - Onboarding negotiate: DID-Auth challenge-response is mandatory (not optional)

### Input Validation
- **DID format validation**: `middleware.IsValidDID()` enforces `did:aicp:<base58btc>` format
- **Body size limiting**: `middleware.BodyLimit()` caps request bodies at 1MB
- **Pagination limits**: Max 100 results per page (`aicp.MaxPageLimit`)
- **Query length limits**: Capability search limited to 256 chars

### Rate Limiting
- Redis-backed per-IP rate limiting via `middleware.RateLimit()`
- Configurable max requests and window via env vars

### Payment Security
- **Atomic double-spend prevention**: Token JTI uniqueness enforced via PostgreSQL unique constraint violations (error code 23505) — INSERT fails atomically on duplicate, preventing TOCTOU race conditions
- **Atomic spending limits**: `CheckAndRecordSpending` combines limit check and record in a single SQL CTE with `FOR UPDATE` row locking — prevents concurrent overspend
- **Amount bounds validation**: Payments capped at 999,999,999.99, currency must be valid ISO 4217 (3-letter uppercase) or recognized crypto ticker
- **Human approval workflow**: Transactions above configurable thresholds require principal approval via `ApprovalService` with 15-minute TTL
- **Error sanitization**: `ProtocolErrorResponse` returns generic "an internal error occurred" for non-ProtocolError types — never leaks database, stack, or internal path information

### Cryptographic Security
- **Constant-time challenge verification**: `crypto/subtle.ConstantTimeCompare` prevents timing attacks on DID-Auth challenge-response (onboarding/challenge.go)
- **Merkle tree domain separation**: RFC 6962 compliant — leaf nodes prefixed with 0x00, internal nodes with 0x01, preventing second preimage attacks
- **Ed25519 only**: All JWS signatures use EdDSA algorithm exclusively, enforced by go-jose/v4 algorithm allowlist

### Infrastructure Security
- **No JSON injection**: All error responses use `json.Marshal`, never string concatenation
- **Panic recovery**: `middleware.Recovery()` catches panics, logs stack traces
- **WebSocket authentication required**: WebSocket upgrade rejects connections when `TicketAuth` is nil (returns 503) — no unauthenticated fallback
- **WebSocket DID isolation**: `Hub.Broadcast()` filters events by `Subject` DID — clients only receive events targeting their own DID. System-wide events (empty Subject) are broadcast to all subscribers
- **WebSocket origin check**: Uses configured `AllowedOrigins` (not wildcard `*` in production)
- **Unpredictable WebSocket client IDs**: Generated with `crypto/rand` (16 bytes hex), not timestamps
- **SSRF prevention**: Domain validation (`validateDomain`) rejects path traversal and special characters in federation peer discovery
- **Response size limiting**: Federation responses capped at 1 MiB via `io.LimitReader`
- **DID mutations persisted**: All DID create-agent, update, and deactivate operations write through to the ledger backend — no phantom success responses

## Agent Identity Model

Any framework/software (OpenClaw, Claude, Codex, custom agents) can register agent identities:

```
Human/Org → POST /aicp/v1/did → Principal DID (did:aicp:<id>)
         → POST /aicp/v1/did/agent → Agent DID (did:aicp:<principal-id>:<agent-id>)
                                    → Delegation credential (scopes capabilities)
                                    → Spending authorization (limits payments)
```

Companies register the same way — they get a Principal DID and optionally publish `/.well-known/aicp.json` on their domain for DNS→DID bridging.

### Agent Lifecycle
1. **Register Principal**: Human/org creates identity (Ed25519 key pair generated server-side)
2. **Create Agent**: Principal creates agent DID with requested capabilities
3. **Discover Services**: Agent searches ANS for services matching needed capabilities
4. **Onboard**: 4-phase flow (initiate → negotiate → authorize → provision)
5. **Transact**: Payment tokens (JWS), spending limits, human approval for large amounts
6. **Verify**: Any party can verify an agent's identity, credentials, and delegation chain

## Middleware Stack

Applied in order (outermost first):
1. `Recovery` — Panic recovery with structured logging
2. `BodyLimit` — 1MB max request body
3. `Logging` — Structured HTTP request logging (slog)
4. `Metrics` — Prometheus request counters and histograms
5. `CORS` — Configurable cross-origin headers
6. `DIDAuth(resolver)` — JWS Bearer token parsing, DID resolution, **signature verification**, context injection
7. `RequestID` — chi request ID generation
8. `RealIP` — X-Forwarded-For parsing
9. `RateLimit` — Redis-backed per-IP rate limiting (conditional)

Route groups add `RequireAuth()` for endpoints that require authentication.

## Development Environment (k3d + Tilt)

The project uses a devcontainer with k3d (k3s-in-Docker) and Tilt for local Kubernetes development.

### Prerequisites
- Docker Desktop
- VS Code with Remote Containers extension

### Getting Started

1. **Open in devcontainer**: VS Code will auto-detect `.devcontainer/` and prompt to reopen
2. **Create k3d cluster** (runs automatically via `post_cmd.sh`, or manually):
   ```bash
   k3d cluster create --config .devcontainer/k3s/aicp-cluster.config.yaml \
     --registry-config .devcontainer/k3s/aicp-cluster.registries.yaml
   ```
3. **Start Tilt**:
   ```bash
   tilt up
   ```
4. **Access Tilt dashboard**: http://localhost:10350

### What Tilt Deploys

| Resource | Label | Port Forward | Description |
|----------|-------|--------------|-------------|
| `postgresql` | 01_infra | 5432 | PostgreSQL 17 with uuid-ossp + pgcrypto |
| `redis` | 01_infra | 6379 | Redis 7 Alpine with AOF persistence |
| `pgadmin` | 01_infra_dashboard | 5050 | pgAdmin4 (admin@aicp.dev / admin) |
| `prometheus` | 01_infra | 9091 | Prometheus v3.2.1 scraping AICP metrics |
| `grafana` | 01_infra_dashboard | 3000 | Grafana 11.6 with 4 AICP dashboards |
| `ingress-routes` | 01_infra | - | Traefik ingress (*.localhost:8081) |
| `db-migrate` | 00_setup | - | golang-migrate runs schema migrations |
| `aicp-server` | 00_server | 8080, 9090 | AICP protocol server (Helm chart) |
| `operator-crds` | 00_operator | - | AICPCluster + AICPValidator CRDs |
| `operator-rbac` | 00_operator | - | Operator RBAC (roles, bindings, SA) |
| `aicp-operator` | 00_operator | - | Operator controller-manager |
| `apply-sample-cr` | 00_operator | - | Sample CRs (manual trigger in Tilt UI) |

### k3d Cluster Management

```bash
# Stop cluster (preserves state)
.devcontainer/k3s/stop-cluster.sh

# Delete cluster (destroys everything)
.devcontainer/k3s/clean-cluster.sh

# Connect kubectl from host machine (outside devcontainer)
./scripts/setup-k3d-kubectl.sh
```

### Key Dev URLs (Traefik Ingress — port 8081)

| Service | Ingress URL | Port Forward |
|---------|-------------|--------------|
| **AICP Server** | http://aicp.localhost:8081 | http://localhost:8080 |
| **Grafana** | http://grafana.localhost:8081 | http://localhost:3000 |
| **pgAdmin** | http://pgadmin.localhost:8081 | http://localhost:5050 |
| **Prometheus** | http://prometheus.localhost:8081 | http://localhost:9091 |
| **Metrics** | http://metrics.localhost:8081/metrics | http://localhost:9090/metrics |
| **Tilt Dashboard** | http://localhost:10350 | — |

Grafana dashboards (auto-provisioned in AICP folder):
- **AICP Overview**: Request rate, latency percentiles, error rate, uptime, memory
- **Identity & Onboarding**: DID ops, credential lifecycle, onboarding phases, trust scores
- **Commerce**: Payment tokens, execution, revenue by adapter, spending limits
- **ANS Performance**: Resolution rate, search latency, cache hit ratio, top services

### Dev Environment Architecture

```
.devcontainer/
  Dockerfile         Dev image: kubectl, helm, k3d, tilt, operator-sdk, golangci-lint
  devcontainer.json  VS Code config (4 CPUs, 8GB RAM, host network, Docker socket)
  scripts/           init_host.sh, post_cmd.sh, copy_configs.sh, zsh-prompt.sh
  k3s/               Cluster config, registries, stop/clean scripts

k8s/deployment/infra/local/
  namespace.yaml              aicp namespace
  postgresql.yaml             PG 17 Deployment + PVC + init SQL + Service
  redis.yaml                  Redis 7 Deployment + PVC + Service
  pgadmin.yaml                pgAdmin4 with pre-configured AICP connection
  prometheus.yaml             Prometheus v3.2.1 scraping aicp-server metrics
  grafana.yaml                Grafana 11.6 with datasource + dashboard provisioning
  grafana-dashboards.yaml     ConfigMap with 4 dashboard JSONs (from deploy/grafana/dashboards/)
  ingress.yaml                Traefik Ingress routes (*.localhost:8081)
  Tiltfile                    Local infra orchestration (loaded by root Tiltfile)

utils/tilt/
  go.utils.tilt      Go Docker build helper
  k8s/
    namespace.utils.tilt  Namespace creation
    pvc.utils.tilt        PVC cleanup for dev
    helm.utils.tilt       Helm dependency management

Tiltfile             Root orchestrator: infra → migrations → server → operator
.tiltignore          Excludes .git, node_modules, docs, tests from Tilt watches
```

## Project Conventions

### Go Code
- Follow standard Go project layout: `cmd/`, `internal/`, `pkg/`
- All interfaces in their own files, named with the `I` prefix omitted (Go convention)
- Use `context.Context` as first parameter for all methods that do I/O
- Use `log/slog` for structured logging -- never `fmt.Println` or `log.Println`
- Error handling: wrap errors with `fmt.Errorf("doing X: %w", err)` for context
- Use `testcontainers-go` for integration tests -- test against real PostgreSQL and Redis
- Run `golangci-lint run` before committing

### Design Principles
- **SOLID**: Single responsibility per package, depend on interfaces not implementations
- **Strategy pattern**: Payment adapters, ledger backends
- **State machine**: Onboarding flow with explicit phase transitions
- **Repository pattern**: All database access through repository interfaces
- **Observer pattern**: Event emitter for cross-cutting concerns (audit, metrics, WebSocket)
- **Builder pattern**: Complex object construction (ANS records, DID documents)

### Protocol Types
- Source of truth: `spec/openapi.yaml` and `spec/schemas/*.schema.json`
- Go structs: `server/pkg/aicp/types.go`
- TypeScript types: `sdk/typescript/src/types.ts`
- Python models: `sdk/python/src/aicp/types.py`

### Key DID Format
```
did:aicp:<principal-id>                  # Principal DID
did:aicp:<principal-id>:<agent-id>       # Agent DID
```
IDs are base58btc-encoded truncated SHA-256 (first 16 bytes) of the public key.

### Database
- PostgreSQL is the only production database -- no SQLite fallbacks
- Migrations in `server/internal/ledger/migrations/` using golang-migrate
- SQL queries in `server/sqlc/queries/` -- run `task sqlc` to regenerate Go code
- JSONB columns for DID Documents and credential data

### Payment Adapters
- Interface: `server/internal/commerce/adapter/adapter.go`
- Implementations: Stripe, PayPal, USDC (Circle API)
- Register via factory in `adapter/registry.go`

### Consortium Ledger
- Append-only blocks with Merkle roots on PostgreSQL
- Multi-node consensus: round-robin leader, 2f+1 validator signatures
- Block sync via HTTP transport between validators (`server/internal/ledger/transport/`)
- Validator transport mounted at `/internal/validator` (sign, blocks, latest, health)

### CLI (`server/cmd/aicp/`)
- Built with cobra for subcommand structure
- Commands: `did`, `credential`, `discover`, `onboard`, `payment`, `server`, `key`, `mcp`
- Output formats: `--output json|table|yaml`
- Config: `~/.aicp/config.yaml` for server URL, default DID, key paths
- Key storage: `~/.aicp/keys/` for Ed25519 key pairs
- Designed for agent automation (JSON output, exit codes) and human use (table output, colors)
- `aicp mcp` starts an MCP server over stdio exposing all AICP operations as tools

### MCP Server (`server/internal/mcp/`)
- JSON-RPC 2.0 over stdio (standard MCP transport)
- Exposes 16+ AICP tools: identity, discovery, onboarding, commerce, verification
- Thin adapter layer: forwards tool calls to the AICP REST API
- Tool names: `aicp_did_create`, `aicp_did_resolve`, `aicp_discover_search`, `aicp_onboard_initiate`, `aicp_payment_request_token`, `aicp_verify`, `aicp_trust_score`, etc.
- Supports resources and resource reading
- Usage: `aicp mcp --server http://localhost:8080`

### Multi-DID Method Resolution (`server/internal/identity/resolver.go`)
- Strategy pattern: `DIDMethodResolver` interface with `Method()` and `Resolve()`
- `MultiMethodResolver` registry dispatches by DID method prefix
- Built-in resolvers: `did:aicp` (ledger-backed), `did:key` (stateless), `did:web` (HTTP-based)
- `did:key` derives DID Document from multibase-encoded public key
- `did:web` fetches DID Document from `https://domain/.well-known/did.json`

### Federated ANS (`server/internal/discovery/resolver.go`)
- 3-tier resolution: local ledger → domain (.well-known) → federated peers
- `PeerRegistry` manages known federation peers (thread-safe)
- `FederatedSearch` queries all peers in parallel, deduplicates results
- Federation requests include `X-AICP-Federation: true` header

### Capability-Based Access Control (`server/internal/identity/capabilities.go`)
- Fine-grained `CapabilityType` enums: `payment:read`, `did:create`, `credential:issue`, etc.
- `CapabilityChecker` verifies capabilities via delegation chain inspection
- Supports exact match, wildcard `*`, hierarchical `payment:*`, and category matching
- `CapabilityGrant` for resource-scoped permissions with conditions

### OIDC Bridge (`server/internal/identity/oidc.go`)
- Maps AICP DIDs to OpenID Connect ID tokens (JWS/EdDSA)
- Custom claims: `aicp_did`, `aicp_principal`, `aicp_capabilities`, `aicp_trust_score`
- Standard OIDC discovery (`/.well-known/openid-configuration`) and JWKS endpoints
- Token verification with expiration and issuer validation

### Testing
- **Unit tests**: `go test ./...` — all packages have test coverage
- **Security tests**: Dedicated tests for double-spend prevention, timing attack resistance, error sanitization, Merkle collision resistance, capability matching
- **E2E tests**: `go test -tags=e2e ./internal/api/` — requires live server at localhost:8080, exercises full protocol flow (DID → Agent → Onboard → Verify → Pay → Trust)
- **Concurrency tests**: Race condition tests for token replay and spending limit enforcement
- **Miniredis**: Onboarding tests use `alicebob/miniredis/v2` for in-process Redis
- **Build tag**: E2E tests gated behind `//go:build e2e` to avoid running in CI without a live server

### Implementation Notes
- Response helpers live in `handlers` package (not `api`) to avoid import cycles
- All handler files import from local package: `JSON()`, `JSONError()`, `DecodeJSON()`
- Router inline handlers (ws/ticket, ws) reference `handlers.JSON`, `handlers.JSONError`
- LedgerBackend interface is the single abstraction for all storage (DID, cred, receipt, block, trust, ANS)
- PostgresBackend implements LedgerBackend with compile-time check: `var _ LedgerBackend = (*PostgresBackend)(nil)`
- WebSocket uses ticket-based auth: POST /ws/ticket → GET /ws?ticket=<ticket>
- Events bridge: `events.WebSocketBridge` subscribes to emitter and forwards to WebSocket hub
- Human approval: `commerce.ApprovalService` with Redis-backed storage, 15min TTL, event emission
- Capability enforcement: `identity.CapabilityEnforcer` validates agent delegation credentials

### Observability
- Prometheus metrics under `aicp_` prefix on metrics port (default 9090)
- 4 Grafana dashboards in `deploy/grafana/dashboards/`: overview, identity-onboarding, commerce, ans-performance
- Local K8s: dashboards embedded in `k8s/deployment/infra/local/grafana-dashboards.yaml` ConfigMap
- Structured JSON logging via `log/slog` with correlation IDs

### Documentation
- Protocol paper: `docs/paper/AICP_Protocol_Paper_v2.0.html` (17-section publication with SVG diagrams)
- Architecture: `docs/architecture.md`
- Development: `docs/development.md`
- Deployment: `docs/deployment.md`
- Protocol spec: `docs/spec/` (layer-by-layer specifications)
