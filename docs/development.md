# Development Environment

This guide covers setting up and using the AICP local development environment.

## Overview

AICP uses a devcontainer with k3d (k3s-in-Docker) and Tilt for live Kubernetes development. This gives you a full-stack environment matching production topology: server, PostgreSQL, Redis, operator, and CRDs all running in a local cluster.

```
Host (macOS/Linux)
  └── Devcontainer (Docker)
        ├── k3d cluster (aicp-cluster)
        │     ├── aicp namespace
        │     │     ├── postgresql
        │     │     ├── redis
        │     │     ├── pgadmin
        │     │     └── aicp-server
        │     └── aicp-operator-system namespace
        │           └── aicp-operator-controller-manager
        └── k3d-aicp-registry (localhost:5005)
```

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Docker Desktop | latest | Container runtime + devcontainer host |
| VS Code | latest | IDE with devcontainer extension |

Everything else (Go, kubectl, k3d, Tilt, Helm, Node.js) is installed inside the devcontainer automatically.

## Getting Started

### Option 1: VS Code (Recommended)

1. Open the repo in VS Code
2. When prompted, click **"Reopen in Container"** (or Cmd+Shift+P > "Dev Containers: Reopen in Container")
3. Wait for the container to build and initialize (~2-3 min first time)
4. The k3d cluster is created automatically on first start

### Option 2: CLI

```bash
# Start devcontainer
npx @devcontainers/cli up --workspace-folder .

# Exec into it
docker exec -it vsc-aicp-dev bash
```

### Start the Dev Stack

```bash
# Inside the devcontainer:
task dev:start
# Or manually:
tilt up
```

Tilt deploys everything and opens a dashboard at http://localhost:10350.

## Services and Ports

| Service | Port | URL |
|---------|------|-----|
| AICP Server | 8080 | http://localhost:8080 |
| Metrics (Prometheus) | 9090 | http://localhost:9090/metrics |
| PostgreSQL | 5432 | `postgres://aicp:aicp_dev@localhost:5432/aicp` |
| Redis | 6379 | `redis://localhost:6379` |
| pgAdmin | 5050 | http://localhost:5050 |
| Tilt Dashboard | 10350 | http://localhost:10350 |
| k3d Registry | 5005 | `localhost:5005` |

## Cluster Management

```bash
# Initialize or recreate the k3d cluster
task dev:cluster:init

# Stop cluster (preserves state)
task dev:cluster:stop

# Delete cluster (destroys everything)
task dev:cluster:clean

# Force recreate cluster
cd .devcontainer/k3s && bash init-cluster.sh -f
```

## Database Migrations

Migrations run automatically when Tilt starts (via the `db-migrate` resource). To run manually:

```bash
export AICP_DATABASE_URL="postgres://aicp:aicp_dev@localhost:5432/aicp?sslmode=disable"

# Apply all pending migrations
task migrate:up

# Rollback last migration
task migrate:down

# Create a new migration
task migrate:create -- add_something
```

## Running Tests

### Unit Tests (Go)

```bash
task test           # All tests with race detection + coverage
task test:short     # Unit tests only (no integration)
```

### E2E Tests (Live Cluster)

Run against the running dev environment:

```bash
task dev:test
```

This tests: cluster health, infrastructure pods, database connectivity, server health/metrics endpoints, operator CRDs, and Helm chart validation.

### SDK Tests

```bash
task sdk:ts:test    # TypeScript SDK (vitest)
task sdk:py:test    # Python SDK (pytest)
```

## Tiltfile Structure

The Tilt configuration follows a modular pattern:

```
Tiltfile                              # Root orchestrator
  ├── utils/tilt/go.utils.tilt        # Go service builder (docker_build wrapper)
  ├── utils/tilt/k8s/helm.utils.tilt  # Helm dependency management
  ├── utils/tilt/k8s/pvc.utils.tilt   # PVC finalizer cleanup
  ├── utils/tilt/k8s/namespace.utils.tilt
  ├── k8s/deployment/infra/local/Tiltfile   # PostgreSQL, Redis, pgAdmin
  └── deploy/helm/aicp/Tiltfile             # Server Helm chart setup
```

Key patterns:
- **Per-chart Tiltfile**: Each Helm chart exports a `setup_*()` function
- **Kustomize for operator**: Uses `operator/config/default` which properly namespaces all resources
- **PVC cleanup**: Manual `local_resource` (trigger from Tilt UI) that patches finalizers

## Devcontainer Details

The devcontainer ([`.devcontainer/devcontainer.json`](../.devcontainer/devcontainer.json)) provides:

- **Resources**: 4 CPUs, 8GB RAM
- **Network**: Host networking (for k3d port exposure)
- **Docker-in-Docker**: Mounts host Docker socket
- **Pre-installed tools**: Go 1.26, Node.js 22, kubectl, k3d, Tilt, Helm, gh CLI
- **VS Code extensions**: Go, Docker, Kubernetes, GitLens, SQLTools, Tilt, Claude Code

The `postCreateCommand` runs [`.devcontainer/scripts/post_cmd.sh`](../.devcontainer/scripts/post_cmd.sh) which:
1. Copies host git config and kubeconfig
2. Installs Go tools (golangci-lint, sqlc, controller-gen, etc.)
3. Installs Tilt and Helm
4. Initializes the k3d cluster

## Troubleshooting

### PVC stuck in Terminating

PVCs with finalizers block deletion. Use the Tilt UI to trigger `clear-postgresql-data` or `clear-redis-data`, or manually:

```bash
kubectl patch pvc postgresql-data -p '{"metadata":{"finalizers":null}}' --type=merge -n aicp
```

### kubectl can't connect

Ensure `KUBECONFIG` is set:

```bash
export KUBECONFIG=/home/vscode/.kube/config
kubectl cluster-info
```

### Server pod not starting

Check if the ServiceAccount exists:

```bash
kubectl get sa -n aicp
kubectl describe deploy aicp-server -n aicp
```

### Migration fails

Ensure PostgreSQL is running and port-forward is active:

```bash
kubectl get pods -n aicp -l app=postgresql
kubectl port-forward -n aicp svc/postgresql 5432:5432
```
