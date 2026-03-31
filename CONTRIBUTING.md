# Contributing to AICP

Thank you for your interest in contributing to the Agentic Identity and Commerce Protocol.

## Prerequisites

- Go 1.22+
- PostgreSQL 16+
- Redis 7+
- [Task](https://taskfile.dev) (`brew install go-task`)
- [golangci-lint](https://golangci-lint.run/) (`brew install golangci-lint`)
- [sqlc](https://sqlc.dev/) (`brew install sqlc`)
- Docker and Docker Compose
- Node.js 24+ (for TypeScript SDK)
- Python 3.12+ (for Python SDK)

## Getting Started

```bash
# Clone the repository
git clone https://github.com/aicp-protocol/aicp.git
cd aicp

# Start PostgreSQL and Redis
task docker:up

# Build the server
task build

# Run tests
task test

# Run all checks
task check
```

## Code Organization

```
server/internal/     # Private Go packages -- the protocol implementation
server/pkg/aicp/     # Public Go types (importable by other Go projects)
server/sqlc/         # SQL queries and sqlc configuration
sdk/typescript/      # TypeScript client SDK
sdk/python/          # Python client SDK
operator/            # Kubernetes operator
spec/                # OpenAPI + JSON Schema (protocol source of truth)
deploy/              # Docker, Helm, Grafana
```

## Development Workflow

1. Create a branch from `main`
2. Make your changes
3. Run `task check` to format, vet, lint, and test
4. Commit with a descriptive message
5. Open a pull request against `main`

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

## Adding a Payment Adapter

1. Create `server/internal/commerce/adapter/youradapter.go`
2. Implement the `PaymentAdapter` interface from `adapter.go`
3. Register it in `registry.go`
4. Add configuration in `server/internal/config/config.go`
5. Write tests using `testcontainers-go` or the adapter's sandbox/test mode

## Database Changes

1. Create a new migration: `task migrate:create -- descriptive_name`
2. Write the `.up.sql` and `.down.sql` files
3. Update sqlc queries if needed: `server/sqlc/queries/`
4. Regenerate: `task sqlc`
5. Run migrations: `task migrate:up`

## Protocol Changes

1. Update `spec/openapi.yaml` and relevant `spec/schemas/*.schema.json`
2. Update Go types in `server/pkg/aicp/types.go`
3. Update TypeScript types in `sdk/typescript/src/types.ts`
4. Update Python models in `sdk/python/src/aicp/types.py`

## Pull Request Guidelines

- Keep PRs focused on a single concern
- Include tests for new functionality
- Update documentation if behavior changes
- Ensure `task check` passes before submitting
- Reference relevant issues in the PR description

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
