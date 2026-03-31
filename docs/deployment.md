# Deployment Guide

This guide covers deploying AICP to production environments.

## Docker Compose (Single Node)

The fastest way to run AICP with all dependencies:

```bash
task docker:build   # Build the server image
task docker:up      # Start server + PostgreSQL + Redis
task docker:logs    # Tail logs
task docker:down    # Stop everything
```

This uses [`deploy/docker/docker-compose.yml`](../deploy/docker/docker-compose.yml) which starts:
- AICP server on port 8080 (API) and 9090 (metrics)
- PostgreSQL 16 on port 5432
- Redis 7 on port 6379

### Configuration

The server reads configuration from environment variables. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `AICP_SERVER_HOST` | `0.0.0.0` | Bind address |
| `AICP_SERVER_PORT` | `8080` | HTTP API port |
| `AICP_SERVER_METRICS_PORT` | `9090` | Prometheus metrics port |
| `AICP_DATABASE_URL` | required | PostgreSQL connection string |
| `AICP_REDIS_URL` | required | Redis connection string |
| `AICP_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `AICP_LOG_FORMAT` | `json` | Log format (json, text) |

See [`server/internal/config/config.go`](../server/internal/config/config.go) for all options.

## Kubernetes with Helm

### Prerequisites

- Kubernetes 1.28+
- Helm 3.12+
- PostgreSQL 16+ (external or in-cluster)
- Redis 7+ (external or in-cluster)

### Install

```bash
# Lint the chart first
task helm:lint

# Install with default values
helm install aicp deploy/helm/aicp \
  --namespace aicp \
  --create-namespace \
  --set postgresql.externalUrl="postgres://..." \
  --set redis.externalUrl="redis://..."

# Or with a values file
helm install aicp deploy/helm/aicp \
  --namespace aicp \
  --create-namespace \
  -f my-values.yaml
```

### Production Values

See [`deploy/helm/aicp/values-production.yaml`](../deploy/helm/aicp/values-production.yaml) for recommended production settings. Key differences from defaults:

- Multiple replicas with PodDisruptionBudget
- HorizontalPodAutoscaler enabled
- Resource limits tuned for production load
- TLS termination via ingress
- NetworkPolicy restricting traffic
- ServiceMonitor for Prometheus scraping

### Chart Structure

```
deploy/helm/aicp/
  ├── Chart.yaml
  ├── values.yaml              # Default values
  ├── values-local.yaml        # Dev overrides (used by Tilt)
  ├── values-production.yaml   # Production overrides
  └── templates/
      ├── _helpers.tpl          # Template helpers
      ├── serviceaccount.yaml
      ├── configmap.yaml
      ├── secret.yaml
      ├── ingress.yaml
      ├── networkpolicy.yaml
      ├── servicemonitor.yaml
      ├── prometheusrule.yaml
      ├── server/
      │   ├── deployment.yaml
      │   ├── service.yaml
      │   ├── hpa.yaml
      │   └── pdb.yaml
      └── validator/
          ├── statefulset.yaml
          ├── service.yaml
          └── pvc.yaml
```

## K8s Operator

The AICP operator manages consortium ledger lifecycle via custom resources.

### Install CRDs

```bash
kubectl apply -k operator/config/crd
```

### Deploy Operator

```bash
# Using kustomize (includes RBAC, service account, deployment)
kubectl apply -k operator/config/default

# Or build and deploy the image
task operator:build
docker build -f operator/Dockerfile -t aicp-operator:latest operator/
kubectl apply -k operator/config/default
```

### Custom Resources

**AICPCluster** - Defines a consortium ledger cluster:

```yaml
apiVersion: aicp.aicp.dev/v1alpha1
kind: AICPCluster
metadata:
  name: aicp-mainnet
spec:
  replicas: 3
  version: "1.0"
```

**AICPValidator** - Manages individual validator nodes:

```yaml
apiVersion: aicp.aicp.dev/v1alpha1
kind: AICPValidator
metadata:
  name: validator-01
spec:
  clusterRef: aicp-mainnet
  resources:
    cpu: "2"
    memory: "4Gi"
```

## Monitoring

### Prometheus

The server exposes metrics at `/metrics` on the metrics port (default 9090). The Helm chart includes:

- `ServiceMonitor` for Prometheus Operator auto-discovery
- `PrometheusRule` with alerting rules

### Grafana Dashboards

Pre-built dashboards are in [`deploy/grafana/dashboards/`](../deploy/grafana/dashboards/):

| Dashboard | Description |
|-----------|-------------|
| `aicp-overview.json` | Request rates, latency, error rates |
| `identity-onboarding.json` | DID operations, onboarding flow metrics |
| `commerce.json` | Payment token issuance, execution, receipts |
| `ans-performance.json` | ANS resolution latency, cache hit rates |

### Health Checks

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Liveness probe (always returns 200 if process is running) |
| `GET /ready` | Readiness probe (checks PostgreSQL and Redis connectivity) |
| `GET /metrics` | Prometheus metrics |

## Docker Images

| Image | Description | Base |
|-------|-------------|------|
| `ghcr.io/aicp-protocol/aicp-server` | Protocol server + CLI | `scratch` (~15MB) |
| `ghcr.io/aicp-protocol/aicp-operator` | K8s operator | `gcr.io/distroless/static` |

Build locally:

```bash
task docker:build                                    # Server
docker build -f operator/Dockerfile -t aicp-operator operator/  # Operator
```
