# Contributing to AICP

Thank you for your interest in contributing to the Agentic Identity and Commerce Protocol.

## Development Environment

The recommended way to develop AICP is with the devcontainer, which provides all tools pre-installed:

1. Open the repo in VS Code
2. Click "Reopen in Container" when prompted
3. Run `task dev:start` to start the full stack

See [docs/development.md](docs/development.md) for the complete dev environment guide, including k3d cluster management, Tilt, and troubleshooting.

### Without Devcontainer

If you prefer to work on the host:

- Go 1.26+
- PostgreSQL 16+
- Redis 7+
- [Task](https://taskfile.dev) (`brew install go-task`)
- [golangci-lint](https://golangci-lint.run/) (`brew install golangci-lint`)
- [sqlc](https://sqlc.dev/) (`brew install sqlc`)
- Docker and Docker Compose
- Node.js 22+ (for TypeScript SDK)
- Python 3.12+ (for Python SDK)

```bash
git clone https://github.com/aicp-protocol/aicp.git
cd aicp
task docker:up    # Start PostgreSQL and Redis
task build        # Build the server
task test         # Run tests
task check        # Format, vet, lint, test
```

## Development Workflow

1. Create a branch from `main`
2. Make your changes
3. Run `task check` to format, vet, lint, and test
4. Commit with a descriptive message
5. Open a pull request against `main`

## Code Organization

```
server/internal/     Private Go packages -- the protocol implementation
server/pkg/aicp/     Public Go types (importable by other Go projects)
server/sqlc/         SQL queries and sqlc configuration
sdk/typescript/      TypeScript client SDK
sdk/python/          Python client SDK
operator/            Kubernetes operator
spec/                OpenAPI + JSON Schema (protocol source of truth)
deploy/              Docker, Helm, Grafana
```

See [docs/architecture.md](docs/architecture.md) for detailed architecture, design patterns, and key interfaces.

## Code Style

### Go

- Follow standard Go conventions and [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` and `goimports` (run `task fmt`)
- Use `log/slog` for structured logging
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Use `context.Context` as the first parameter for I/O methods
- Write table-driven tests
- Use `testcontainers-go` for integration tests against real PostgreSQL/Redis

### TypeScript SDK

- TypeScript strict mode enabled
- Use `vitest` for testing
- Export types from `types.ts`

### Python SDK

- Python 3.12+ type hints
- Use `pydantic` for data models
- Use `httpx` for async HTTP
- Run `ruff` for linting, `mypy` for type checking

## Common Tasks

### Adding a Payment Adapter

1. Create `server/internal/commerce/adapter/youradapter.go`
2. Implement the `PaymentAdapter` interface from `adapter.go`
3. Register it in `registry.go`
4. Add configuration in `server/internal/config/config.go`
5. Write tests using `testcontainers-go` or the adapter's sandbox mode

### Database Changes

```bash
task migrate:create -- descriptive_name  # Create migration files
# Edit the .up.sql and .down.sql files
task sqlc                                 # Regenerate Go code
task migrate:up                           # Apply migrations
```

### Protocol Changes

1. Update `spec/openapi.yaml` and relevant `spec/schemas/*.schema.json`
2. Update Go types in `server/pkg/aicp/types.go`
3. Update TypeScript types in `sdk/typescript/src/types.ts`
4. Update Python models in `sdk/python/src/aicp/types.py`

### Running E2E Tests

Against the live dev environment (requires `task dev:start`):

```bash
task dev:test
```

## Pull Request Guidelines

- Keep PRs focused on a single concern
- Include tests for new functionality
- Update documentation if behavior changes
- Ensure `task check` passes before submitting
- Reference relevant issues in the PR description

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
