# AICP Development Environment
# k3d + Tilt orchestration for the Agentic Identity and Commerce Protocol

# ── Config ──────────────────────────────────────────────────
docker_prune_settings(max_age_mins=60, num_builds=1)
update_settings(max_parallel_updates=3, k8s_upsert_timeout_secs=60)
allow_k8s_contexts('k3d-aicp-cluster')

# ── Load utilities ──────────────────────────────────────────
load('./utils/tilt/go.utils.tilt', 'go_service')
load('./utils/tilt/k8s/helm.utils.tilt', 'ensure_helm_deps')

# Load AICP server Helm chart setup
load('./deploy/helm/aicp/Tiltfile', 'setup_aicp_server')

# ── Local Infrastructure ────────────────────────────────────
# PostgreSQL, Redis, pgAdmin deployed as plain K8s manifests
load('./k8s/deployment/infra/local/Tiltfile', 'setup_local_infra')
setup_local_infra()

# ── Database Migrations ─────────────────────────────────────
# Run migrations after PostgreSQL is ready. The 'postgres' build tag includes
# the PostgreSQL driver in the migrate CLI binary.
local_resource('db-migrate',
    cmd='cd server && go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path internal/ledger/migrations -database "postgres://aicp:aicp_dev@localhost:5432/aicp?sslmode=disable" up',
    resource_deps=['postgresql'],
    labels=['00_setup'],
)

# ── AICP Server ─────────────────────────────────────────────
# Build the Go server image with live update for fast iteration
go_service(
    'aicp-server',
    context='.',
    dockerfile='deploy/docker/Dockerfile.server',
)

# Deploy server via Helm chart (values-local.yaml overrides for dev)
k8s_yaml(setup_aicp_server())

k8s_resource('aicp-server',
    port_forwards=['8080:8080', '9090:9090'],
    resource_deps=['db-migrate', 'postgresql', 'redis'],
    labels=['00_server'],
)

# ── Operator ────────────────────────────────────────────────
# Build operator image -- matches 'controller:latest' referenced by kustomize
docker_build(
    'controller',
    context='./operator',
    dockerfile='./operator/Dockerfile',
)

# Apply all operator resources via kustomize (CRDs + RBAC + Manager deployment).
# config/default properly namespaces everything under aicp-operator-system.
k8s_yaml(kustomize('./operator/config/default'))

# Group CRDs
k8s_resource(
    new_name='operator-crds',
    objects=[
        'aicpclusters.aicp.aicp.dev:customresourcedefinition',
        'aicpvalidators.aicp.aicp.dev:customresourcedefinition',
    ],
    labels=['00_operator'],
)

# Group RBAC + supporting objects
k8s_resource(
    new_name='operator-rbac',
    objects=[
        'aicp-operator-system:namespace',
        'aicp-operator-controller-manager:serviceaccount',
        'aicp-operator-leader-election-role:role',
        'aicp-operator-manager-role:clusterrole',
        'aicp-operator-metrics-auth-role:clusterrole',
        'aicp-operator-metrics-reader:clusterrole',
        'aicp-operator-aicpcluster-admin-role:clusterrole',
        'aicp-operator-aicpcluster-editor-role:clusterrole',
        'aicp-operator-aicpcluster-viewer-role:clusterrole',
        'aicp-operator-aicpvalidator-admin-role:clusterrole',
        'aicp-operator-aicpvalidator-editor-role:clusterrole',
        'aicp-operator-aicpvalidator-viewer-role:clusterrole',
        'aicp-operator-leader-election-rolebinding:rolebinding',
        'aicp-operator-manager-rolebinding:clusterrolebinding',
        'aicp-operator-metrics-auth-rolebinding:clusterrolebinding',
    ],
    labels=['00_operator'],
)

# Operator deployment (from kustomize, image auto-matched by docker_build).
# The metrics service is auto-associated with this deployment by Tilt.
k8s_resource('aicp-operator-controller-manager',
    resource_deps=['operator-crds', 'operator-rbac'],
    labels=['00_operator'],
)

# ── Sample CRs ──────────────────────────────────────────────
# Apply sample AICPCluster and AICPValidator CRs for development.
# Manual trigger only -- use the Tilt UI button to apply.
local_resource('apply-sample-cr',
    cmd='kubectl apply -f operator/config/samples/ -n aicp',
    resource_deps=['aicp-operator-controller-manager'],
    auto_init=False,
    labels=['00_operator'],
)
