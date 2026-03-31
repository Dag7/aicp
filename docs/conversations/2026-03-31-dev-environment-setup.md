# Dev Environment Setup - March 31, 2026

## Summary

Full production-grade development environment setup for AICP using k3d + Tilt inside a devcontainer. Includes E2E testing of the full stack.

## What Was Done

### 1. k3d Cluster Lifecycle Scripts
- `.devcontainer/k3s/init-cluster.sh` -- Creates k3d cluster with registry, applies namespace
- `.devcontainer/k3s/stop-cluster.sh` -- Stops cluster preserving state
- `.devcontainer/k3s/clean-cluster.sh` -- Deletes cluster entirely
- `.devcontainer/scripts/post_cmd.sh` -- Auto-inits cluster on devcontainer start

### 2. Helm Chart Tiltfile (doorz pattern)
- `deploy/helm/aicp/Tiltfile` -- Per-chart Tiltfile exporting `setup_aicp_server()`
- `deploy/helm/aicp/values-local.yaml` -- Dev overrides (pullPolicy=Never, single replica)
- `deploy/helm/aicp/templates/serviceaccount.yaml` -- Missing template (caused pod startup failure)
- `utils/tilt/k8s/helm.utils.tilt` -- Full `ensure_helm_deps()` matching doorz

### 3. PVC Utils (doorz pattern)
- `utils/tilt/k8s/pvc.utils.tilt` -- Rewrote from `kubectl delete` (hangs on finalizers) to `kubectl patch` (doorz pattern)
- `k8s/deployment/infra/local/Tiltfile` -- Updated to new signature

### 4. Operator Kustomize Fix
- Switched from separate `config/rbac` + `config/crd` to `config/default` (proper namespace overlay)
- Fixed all RBAC object names to match kustomize `namePrefix: aicp-operator-`
- Metrics service auto-associated with deployment by Tilt

### 5. Migration Fix
- Added `-tags postgres` build tag to `golang-migrate` CLI
- Fixed in both Tiltfile and Taskfile

### 6. Dev Automation
- `scripts/dev-start.sh` -- Checks prerequisites, ensures cluster, starts tilt
- `scripts/e2e-test.sh` -- 6-section test suite with kubectl port-forward for health checks
- Taskfile tasks: `dev:start`, `dev:test`, `dev:cluster:init/stop/clean`

### 7. Documentation
- `docs/architecture.md` -- System design, patterns, interfaces, API reference
- `docs/development.md` -- Full dev environment guide
- `docs/deployment.md` -- Docker, Helm, K8s operator, monitoring
- `README.md` -- Updated with documentation table, correct Go version, dev commands
- `CONTRIBUTING.md` -- Updated with devcontainer-first workflow, correct prerequisites

### 8. Docker/CI Fixes
- Go 1.26 standardized across all Dockerfiles, CI workflows, go.mod, devcontainer
- `.dockerignore` files for root and operator
- Operator Dockerfile entrypoint fixed
- Missing Helm values added

## E2E Test Results (17/17 pass)

```
[1/6] Cluster Health:    3 pass
[2/6] Infrastructure:    3 pass (PostgreSQL, Redis, pgAdmin)
[3/6] Database:          3 pass (PG connections, 15 tables, Redis PONG)
[4/6] AICP Server:       3 pass (deployment ready, /health, /metrics)
[5/6] Operator:          3 pass (CRDs, deployment ready)
[6/6] Helm Chart:        2 pass (lint, template)
```
