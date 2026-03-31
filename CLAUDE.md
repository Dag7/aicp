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
    identity/    Layer 1: DID lifecycle, credentials, delegation, revocation
    discovery/   Layer 2: ANS records, tiered resolver, search, well-known
    onboarding/  Layer 3: 4-phase state machine, sessions, challenges, trust
    commerce/    Layer 4: Payment tokens (JWS), executor, spending, adapters
    ledger/      Consortium ledger: PostgreSQL backend, blocks, consensus, sync
    api/         HTTP router (chi), handlers, middleware, WebSocket hub
    events/      Typed event emitter, Redis pub/sub bridge
  pkg/aicp/      Public types, errors, version constants
  sqlc/          SQL queries + generated type-safe Go code

operator/        K8s Operator (Operator SDK) for consortium ledger management
sdk/typescript/  TypeScript client SDK (@aicp/sdk on npm)
sdk/python/      Python client SDK (aicp on PyPI)
spec/            OpenAPI 3.1 + JSON Schema (protocol source of truth)
deploy/          Docker, Helm charts, Grafana dashboards
docs/            Whitepaper, protocol spec, Docusaurus site
```

## Tech Stack

- **Server**: Go 1.22+ with chi router
- **Database**: PostgreSQL 16+ via pgx driver + sqlc for type-safe queries
- **Cache**: Redis 7+ via go-redis (rate limiting, sessions, pub/sub, nonce cache)
- **Crypto**: Go stdlib crypto/ed25519, crypto/sha256 + go-jose/v4 for JWS
- **Testing**: Go testing + testify + testcontainers-go (real PG + Redis in tests)
- **Linting**: golangci-lint
- **Build**: Taskfile (task runner)
- **CI/CD**: GitHub Actions

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
- Block sync via gRPC between validators

### CLI (`server/cmd/aicp/`)
- Built with cobra for subcommand structure
- Commands: `did`, `credential`, `discover`, `onboard`, `payment`, `server`, `key`
- Output formats: `--output json|table|yaml`
- Config: `~/.aicp/config.yaml` for server URL, default DID, key paths
- Key storage: `~/.aicp/keys/` for Ed25519 key pairs
- Designed for agent automation (JSON output, exit codes) and human use (table output, colors)

### Implementation Notes
- Response helpers live in `handlers` package (not `api`) to avoid import cycles
- All handler files import from local package: `JSON()`, `JSONError()`, `DecodeJSON()`
- Router inline handlers (ws/ticket, ws) reference `handlers.JSON`, `handlers.JSONError`
- LedgerBackend interface is the single abstraction for all storage (DID, cred, receipt, block, trust, ANS)
- PostgresBackend implements LedgerBackend with compile-time check: `var _ LedgerBackend = (*PostgresBackend)(nil)`
- WebSocket uses ticket-based auth: POST /ws/ticket → GET /ws?ticket=<ticket>
