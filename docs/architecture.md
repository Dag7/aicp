# Architecture

This document describes the AICP server architecture, design patterns, and key interfaces.

## System Architecture

```
                          ┌──────────────────────┐
                          │     Agent / SDK       │
                          │  (TypeScript, Python) │
                          └──────────┬───────────┘
                                     │ HTTPS / WebSocket
                          ┌──────────▼───────────┐
                          │    AICP Server (Go)   │
                          ├──────────────────────┤
                          │   chi Router          │
                          │   ├── DID-Auth MW     │
                          │   ├── Rate Limit MW   │
                          │   ├── Metrics MW      │
                          │   └── Handlers        │
                          ├──────────────────────┤
                          │   Protocol Layers     │
                          │   ├── Identity        │
                          │   ├── Discovery       │
                          │   ├── Onboarding      │
                          │   └── Commerce        │
                          ├──────────────────────┤
                          │   Event System        │
                          │   (emitter + pub/sub) │
                          ├──────────────────────┤
                          │   Consortium Ledger   │
                          │   (blocks + consensus)│
                          ├─────────┬────────────┤
                          │ PostgreSQL │  Redis   │
                          └─────────┴────────────┘
```

## Package Layout

```
server/
  cmd/
    aicp-server/     Entry point: config, DI, graceful shutdown
    aicp/            CLI tool (cobra): did, discover, onboard, pay, mcp
  internal/
    config/          Environment/YAML configuration with validation
    crypto/          Ed25519 keys, signing, JWS (go-jose/v4), Merkle trees, standalone SignJWS/VerifyJWS
    identity/        Layer 1: DID lifecycle, VCs, delegation, revocation, multi-DID resolution, capabilities, OIDC bridge
    discovery/       Layer 2: ANS records, tiered resolver, federated search, peer registry
    onboarding/      Layer 3: 4-phase state machine, sessions, challenges
    commerce/        Layer 4: Payment tokens, executor, spending limits
      adapter/       PaymentAdapter implementations (Stripe, PayPal, USDC)
    ledger/          Consortium ledger: PostgreSQL backend, blocks, consensus
      repository/    Data access layer (sqlc-generated)
      migrations/    PostgreSQL schema migrations
    api/             HTTP layer
      handlers/      Request handlers for all endpoints
      middleware/     DID-auth, rate limiting, metrics, CORS, recovery
      websocket/     Connection hub, ticket-based auth, event delivery
    mcp/             MCP server: JSON-RPC 2.0 over stdio, AICP tools for AI agents
    events/          Typed event emitter, Redis pub/sub bridge
  pkg/aicp/          Public types, error codes, version constants
  sqlc/              SQL queries and generated type-safe code
```

## Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Strategy** | `PaymentAdapter` (Stripe/PayPal/USDC), `LedgerBackend`, `DIDMethodResolver` (aicp/key/web) | Swap implementations without changing consumers |
| **State Machine** | Onboarding 4-phase flow | Prevent invalid state transitions, enforce guards |
| **Repository** | DID, Credential, Receipt, Block, Trust repos | Decouple storage from business logic |
| **Observer** | Event emitter -> Redis pub/sub -> WebSocket | Loose coupling for cross-cutting concerns |
| **Builder** | `ANSRecordBuilder`, `DIDDocumentBuilder` | Fluent API for complex objects |
| **Adapter** | DID method resolvers, payment providers, MCP tool adapter | Uniform interface across implementations |
| **Middleware Chain** | chi middleware stack | Composable request pipeline |
| **Factory** | Payment adapter registry | Hide multi-step creation complexity |
| **Bridge** | `OIDCBridge` (AICP DID ↔ OIDC tokens), MCP server (AICP ↔ MCP tools) | Protocol translation between systems |
| **Registry** | `MultiMethodResolver`, `PeerRegistry` | Thread-safe dynamic service registration |

## Key Interfaces

### LedgerBackend

The single abstraction for all persistent storage. PostgreSQL implements it via `PostgresBackend`.

```go
type LedgerBackend interface {
    // DID operations
    AnchorDID(ctx context.Context, did string, doc DIDDocument, proof Proof) (*AnchorReceipt, error)
    ResolveDID(ctx context.Context, did string) (*DIDDocument, error)
    UpdateDIDDocument(ctx context.Context, did string, doc DIDDocument, proof Proof) (*AnchorReceipt, error)
    DeactivateDID(ctx context.Context, did string, proof Proof) error

    // Credential operations
    RevokeCredential(ctx context.Context, credentialID string, proof Proof) error
    IsCredentialRevoked(ctx context.Context, credentialID string) (bool, error)

    // Receipt operations
    StoreReceipt(ctx context.Context, receipt SignedPaymentReceipt) error
    GetReceipt(ctx context.Context, receiptID string) (*SignedPaymentReceipt, error)

    // Block/consensus operations
    ProposeBlock(ctx context.Context, block Block) error
    GetLatestBlock(ctx context.Context) (*Block, error)

    // Trust
    GetTrustScore(ctx context.Context, did string) (*TrustScore, error)
    UpdateTrustScore(ctx context.Context, did string, score TrustScore) error
}
```

### PaymentAdapter

Strategy interface for payment providers. Each provider (Stripe, PayPal, USDC) implements this.

```go
type PaymentAdapter interface {
    Name() string
    SupportedCurrencies() []string
    CreatePaymentIntent(ctx context.Context, params CreateIntentParams) (*PaymentIntent, error)
    CapturePayment(ctx context.Context, intentID string) (*PaymentCapture, error)
    RefundPayment(ctx context.Context, intentID string, params *RefundParams) (*PaymentRefund, error)
    GetPaymentStatus(ctx context.Context, intentID string) (*PaymentStatus, error)
}
```

### OnboardingFlow

The 4-phase state machine for agent registration:

```
initiated -> identity_presented -> capabilities_negotiated -> payment_authorized -> provisioned
     \              \                     \                         \
      \--> failed    \--> failed           \--> failed               \--> failed
```

## REST API

### Public Endpoints (no authentication)

| Endpoint | Method | Layer | Description |
|----------|--------|-------|-------------|
| `/health` | GET | Infra | Liveness probe |
| `/ready` | GET | Infra | Readiness probe |
| `/metrics` | GET | Infra | Prometheus metrics (`aicp_*` prefix) |
| `/.well-known/aicp.json` | GET | Discovery | ANS service manifest |
| `/aicp/v1/did` | POST | Identity | Create principal DID |
| `/aicp/v1/did/{did}` | GET | Identity | Resolve DID document |
| `/aicp/v1/discover/search` | GET | Discovery | Full-text ANS search |
| `/aicp/v1/discover/{did}` | GET | Discovery | Get ANS record by DID |
| `/aicp/v1/trust/{did}` | GET | Trust | Get trust score |
| `/aicp/v1/onboard` | POST | Onboarding | Initiate registration |
| `/aicp/v1/onboard/{id}` | GET | Onboarding | Get session status |
| `/aicp/v1/onboard/{id}/negotiate` | POST | Onboarding | Negotiate capabilities |
| `/aicp/v1/onboard/{id}/authorize` | POST | Onboarding | Authorize payment |
| `/aicp/v1/onboard/{id}/provision` | POST | Onboarding | Provision agent |
| `/aicp/v1/verify` | POST | Identity | Verify delegation chain |

### Authenticated Endpoints (DID-Auth JWS Bearer)

| Endpoint | Method | Layer | Description |
|----------|--------|-------|-------------|
| `/aicp/v1/did/agent` | POST | Identity | Create agent DID |
| `/aicp/v1/did/{did}` | PUT | Identity | Update DID document |
| `/aicp/v1/did/{did}/deactivate` | POST | Identity | Deactivate DID |
| `/aicp/v1/discover/register` | POST | Discovery | Register ANS record |
| `/aicp/v1/discover/{did}` | DELETE | Discovery | Delete ANS record |
| `/aicp/v1/payment/token` | POST | Commerce | Request payment token |
| `/aicp/v1/payment/execute` | POST | Commerce | Execute payment |
| `/aicp/v1/payment/status/{id}` | GET | Commerce | Check payment status |
| `/aicp/v1/payment/approval/{id}` | GET | Commerce | Get approval status |
| `/aicp/v1/payment/approval/{id}/approve` | POST | Commerce | Approve payment |
| `/aicp/v1/payment/approval/{id}/deny` | POST | Commerce | Deny payment |
| `/aicp/v1/payment/approvals/pending` | GET | Commerce | List pending approvals |
| `/aicp/v1/credentials/issue` | POST | Identity | Issue credential |
| `/aicp/v1/credentials/{id}/revoke` | POST | Identity | Revoke credential |
| `/aicp/v1/ws/ticket` | POST | WebSocket | Get connection ticket |
| `/aicp/v1/ws` | GET | WebSocket | WebSocket upgrade (ticket auth) |

Full specification: [`spec/openapi.yaml`](../spec/openapi.yaml)

## Identity Model

### DID Format

```
did:aicp:<principal-id>                  # Principal (human/org)
did:aicp:<principal-id>:<agent-id>       # Agent (autonomous software)
```

IDs are base58btc-encoded truncated SHA-256 (first 16 bytes) of the Ed25519 public key.

### Delegation Chain

```
Principal (human/org)
  └── Agent 1 (purchasing-bot)
  └── Agent 2 (monitoring-bot)
  └── Agent 3 (support-bot)
```

Each agent has its own independent Ed25519 key pair. The principal DID controls (can deactivate) all agent DIDs.

### Payment Tokens

JWS (EdDSA) signed by the payment orchestrator:
- Content type: `aicp-payment-token+jws`
- Scoped to: amount, service, time window (5 min), purpose
- Replay prevention: `jti` uniqueness (PostgreSQL), `nonce` cache (Redis)

## Consortium Ledger

Append-only blocks stored in PostgreSQL:

```
Block N
  ├── header (number, timestamp, prev_hash, merkle_root)
  ├── entries[] (DID anchors, credential revocations, payment receipts)
  ├── validator_signatures[] (2f+1 required)
  └── merkle_root (SHA-256 of all entry hashes)
```

Consensus: round-robin leader election with 2f+1 validator signatures for finalization.

## Trust Score

Composite score (0-1000) computed from:

| Factor | Weight |
|--------|--------|
| Identity verification completeness | 25% |
| Account age | 15% |
| Successful transactions | 20% |
| Service endorsements | 15% |
| Credential completeness | 10% |
| Dispute history (penalty) | 15% |

Cached in Redis, recomputed on relevant events.
