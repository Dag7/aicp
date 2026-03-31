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

### Run with Docker Compose

```bash
git clone https://github.com/aicp-protocol/aicp.git
cd aicp
task docker:up
```

Server API at `http://localhost:8080`, metrics at `http://localhost:9090`.

### Build from Source

```bash
task build
export AICP_DATABASE_URL="postgres://aicp:aicp@localhost:5432/aicp?sslmode=disable"
task migrate:up
./bin/aicp-server
```

### Verify

```bash
curl http://localhost:8080/health
# {"status":"ok","version":"dev","protocol":"1.0"}
```

## SDKs

### TypeScript

```typescript
import { AICPAgent } from '@aicp/sdk';

const agent = await AICPAgent.create({
  idpUrl: 'https://idp.aicp.example.com',
  principalToken: process.env.AICP_PRINCIPAL_TOKEN,
});

const results = await agent.discovery.search({ tags: ['cloud-compute'] });
const session = await agent.onboard(results.services[0], { tier: 'starter' });
```

### Python

```python
from aicp import AICPAgent

agent = await AICPAgent.create(
    idp_url="https://idp.aicp.example.com",
    principal_token=os.environ["AICP_PRINCIPAL_TOKEN"],
)

results = await agent.discovery.search(tags=["cloud-compute"])
session = await agent.onboard(results.services[0], tier="starter")
```

### Service Provider (~200 lines)

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

See [docs/architecture.md](docs/architecture.md) for detailed design, interfaces, and patterns.

## Repository Structure

```
server/          Go protocol server (chi, pgx, go-redis)
  cmd/aicp-server/   Server entry point
  cmd/aicp/          CLI tool (cobra)
  internal/          Protocol implementation (identity, discovery, onboarding, commerce)
  pkg/aicp/          Public Go types and error codes
operator/        K8s operator for consortium ledger management
sdk/typescript/  TypeScript client SDK (@aicp/sdk)
sdk/python/      Python client SDK (aicp)
spec/            OpenAPI 3.1 + JSON Schema (protocol source of truth)
deploy/          Docker, Helm charts, Grafana dashboards
docs/            Protocol spec, architecture, guides
```

## Complementary to Existing Protocols

AICP provides the identity and discovery substrate that existing protocols need:

- **A2A** (Google): AICP-credentialed agents authenticate to A2A servers
- **MCP** (Anthropic): Agents access MCP tool servers with verifiable authorization
- **ACP** (Stripe/OpenAI): Agents initiate ACP checkout with scoped payment tokens

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, patterns, interfaces, API reference |
| [Development](docs/development.md) | Dev environment setup (devcontainer, k3d, Tilt) |
| [Deployment](docs/deployment.md) | Docker, Helm, K8s operator, monitoring |
| [Contributing](CONTRIBUTING.md) | Code style, workflow, how to add adapters |
| [Protocol Spec](docs/spec/) | Layer-by-layer protocol specification |
| [API Spec](spec/openapi.yaml) | OpenAPI 3.1 endpoint definitions |

## Development

```bash
task check        # Format, vet, lint, test
task all          # Build and test everything
task dev:start    # Start full dev environment (k3d + Tilt)
task dev:test     # Run e2e tests against live cluster
```

See [docs/development.md](docs/development.md) for the full dev environment guide.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
