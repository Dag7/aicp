# AICP - Agentic Identity and Commerce Protocol

A unified, open-source protocol for autonomous AI agent identity, service discovery, and commerce.

AICP provides AI agents with the foundational infrastructure to participate as first-class economic actors: DID-based cryptographic identity, intelligent service discovery via the Agent Name System (ANS), standardized onboarding, and provider-agnostic payment abstraction.

## Protocol Layers

| Layer | Name | Purpose |
|-------|------|---------|
| 1 | **Identity** | W3C DID-anchored agent credentials with verifiable credentials |
| 2 | **Discovery** | Agent Name System (ANS) for capability-based service discovery |
| 3 | **Onboarding** | Standardized 4-phase machine-executable registration |
| 4 | **Commerce** | Provider-agnostic payment tokens (Stripe, PayPal, USDC) |

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 16+
- Redis 7+
- [Task](https://taskfile.dev) (build runner)
- Docker (optional, for containerized deployment)

### Run with Docker Compose

```bash
git clone https://github.com/aicp-protocol/aicp.git
cd aicp
task docker:up
```

This starts the AICP server, PostgreSQL, and Redis. The API is available at `http://localhost:8080`.

### Build from Source

```bash
# Build the server
task build

# Run migrations
export AICP_DATABASE_URL="postgres://aicp:aicp@localhost:5432/aicp?sslmode=disable"
export AICP_REDIS_URL="redis://localhost:6379"
task migrate:up

# Start the server
./bin/aicp-server
```

### Verify

```bash
curl http://localhost:8080/health
curl http://localhost:8080/.well-known/aicp.json
```

## SDKs

### TypeScript (Agent Developer)

```typescript
import { AICPAgent } from '@aicp/sdk';

const agent = await AICPAgent.create({
  idpUrl: 'https://idp.aicp.example.com',
  principalToken: process.env.AICP_PRINCIPAL_TOKEN,
});

// Discover services
const results = await agent.discovery.search({
  tags: ['cloud-compute', 'container-hosting'],
});

// Onboard (identity -> negotiate -> authorize -> provision)
const session = await agent.onboard(results.services[0], { tier: 'starter' });
```

### Python (Agent Developer)

```python
from aicp import AICPAgent

agent = await AICPAgent.create(
    idp_url="https://idp.aicp.example.com",
    principal_token=os.environ["AICP_PRINCIPAL_TOKEN"],
)

results = await agent.discovery.search(tags=["cloud-compute"])
session = await agent.onboard(results.services[0], tier="starter")
```

### Service Provider Integration (~200 lines)

Implement the `AICPServiceAdapter` interface to accept AI agent customers:

```typescript
import { createAICPMiddleware, AICPServiceAdapter } from '@aicp/sdk';

class MyServiceAdapter implements AICPServiceAdapter {
  async getServiceManifest() { /* ... */ }
  async verifyAgent(did, credentials) { /* ... */ }
  async negotiate(request) { /* ... */ }
  async authorizePayment(token) { /* ... */ }
  async provision(agent, tier) { /* ... */ }
}

app.use('/aicp', createAICPMiddleware(new MyServiceAdapter()));
```

## Architecture

AICP runs entirely over HTTP/REST and WebSocket. The consortium ledger uses PostgreSQL with append-only blocks, Merkle roots, and multi-node validator consensus.

```
┌─────────────────────────────────────────────┐
│              AICP Protocol Server            │
├──────────┬──────────┬──────────┬────────────┤
│ Identity │ Discovery│Onboarding│  Commerce  │
│  (DIDs)  │  (ANS)   │ (4-phase)│ (Payments) │
├──────────┴──────────┴──────────┴────────────┤
│           Consortium Ledger (PG)            │
│    Blocks + Merkle Trees + Consensus        │
├─────────────────────────────────────────────┤
│     PostgreSQL          Redis               │
└─────────────────────────────────────────────┘
```

## Complementary to Existing Protocols

AICP does not compete with existing protocols. It provides the identity and discovery substrate they need:

- **A2A** (Google): AICP-credentialed agents authenticate to A2A servers using their DID
- **MCP** (Anthropic): Agents access MCP tool servers with verifiable authorization
- **ACP** (Stripe/OpenAI): Agents initiate ACP checkout with cryptographically scoped payment tokens

## Development

```bash
task check        # Format, vet, lint, test
task all          # Build and test everything
task docker:up    # Start local development stack
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
