# Dev Environment Setup - March 31, 2026

## Summary

Full production-grade development environment setup for AICP using k3d + Tilt inside a devcontainer.

## What Was Done

### 1. k3d Cluster Lifecycle Scripts
- **`.devcontainer/k3s/init-cluster.sh`** — Creates k3d cluster (`aicp-cluster`) with registry (port 5005), applies namespace, sets kubectl context. Supports `-f` force recreate.
- **`.devcontainer/k3s/stop-cluster.sh`** — Stops cluster preserving state
- **`.devcontainer/k3s/clean-cluster.sh`** — Deletes cluster entirely
- **`.devcontainer/scripts/post_cmd.sh`** — Auto-inits cluster on devcontainer start

### 2. Helm Chart Tiltfile (doorz pattern)
- **`deploy/helm/aicp/Tiltfile`** — Per-chart Tiltfile exporting `setup_aicp_server()` function
- **`deploy/helm/aicp/values-local.yaml`** — Local dev overrides (pullPolicy=Never, single replica, external PG/Redis, debug logging)
- **`utils/tilt/k8s/helm.utils.tilt`** — Full `ensure_helm_deps()` with `config.main_dir`, charts/ dir check, external repo support (matching doorz)
- **Root `Tiltfile`** — Loads chart setup, wires infra + server + operator + CRDs

### 3. Docker/CI Fixes
- **`.dockerignore`** (root) — Excludes .git, .devcontainer, docs, tests, sdk, spec from builds
- **`operator/.dockerignore`** — Enhanced with config/, test files, IDE files
- **Go 1.26 standardized** across all Dockerfiles, CI workflows, go.mod, devcontainer features
- **Operator Dockerfile entrypoint** fixed: `cmd/manager/main.go` → `cmd/main.go`
- **Missing Helm values** added: `serviceDID`, `allowedOrigins`, `rateLimitMax`, `rateLimitWindow`

### 4. Dev Environment Automation
- **`scripts/dev-start.sh`** — Checks prerequisites, ensures cluster, sets namespace, starts tilt
- **`scripts/e2e-test.sh`** — 6-section test suite (cluster health, infra pods, database, server, operator, helm)
- **Taskfile tasks**: `dev:start`, `dev:test`, `dev:cluster:init`, `dev:cluster:stop`, `dev:cluster:clean`

### 5. CLAUDE.md Updated
- Added "Development Environment (k3d + Tilt)" section with architecture diagram
- Documented cluster management, key URLs, resource table

## Issues Found & Fixed
| Issue | Fix |
|-------|-----|
| Operator Dockerfile wrong entrypoint | `cmd/manager/main.go` → `cmd/main.go` |
| Go version inconsistency (1.22-1.24) | Standardized all to 1.26 |
| Missing ConfigMap values | Added serviceDID, allowedOrigins, rateLimitMax, rateLimitWindow |
| No .dockerignore files | Created root + operator .dockerignore |
| Helm chart inline in Tiltfile | Refactored to per-chart Tiltfile pattern (doorz) |

## Pending
- E2E testing inside devcontainer (spin up → init-cluster → tilt up → run e2e-test.sh)
