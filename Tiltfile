# AICP Development Environment
# k3d + Tilt orchestration for the Agentic Identity and Commerce Protocol

# ── Config ──────────────────────────────────────────────────
docker_prune_settings(max_age_mins=60, num_builds=1)
update_settings(max_parallel_updates=3, k8s_upsert_timeout_secs=60)
allow_k8s_contexts('k3d-aicp-cluster')

# ── Load utilities ──────────────────────────────────────────
load('./utils/tilt/go.utils.tilt', 'go_service')
load('./utils/tilt/k8s/namespace.utils.tilt', 'create_namespace')
load('./utils/tilt/k8s/pvc.utils.tilt', 'clear_pvc')
load('./utils/tilt/k8s/helm.utils.tilt', 'ensure_helm_deps')

# Load AICP server Helm chart setup
load('./deploy/helm/aicp/Tiltfile', 'setup_aicp_server')

# ── Local Infrastructure ────────────────────────────────────
# PostgreSQL, Redis, pgAdmin deployed as plain K8s manifests
load('./k8s/deployment/infra/local/Tiltfile', 'setup_local_infra')
setup_local_infra()

# ── Database Migrations ─────────────────────────────────────
# Run migrations after PostgreSQL is ready. Uses port-forwarded local address.
local_resource('db-migrate',
    cmd='cd server && go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path internal/ledger/migrations -database "postgres://aicp:aicp_dev@localhost:5432/aicp?sslmode=disable" up',
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
# Build operator image from the operator-sdk generated Dockerfile
docker_build(
    'aicp-operator',
    context='./operator',
    dockerfile='./operator/Dockerfile',
)

# Apply CRDs directly from bases (no kustomize namePrefix needed for CRDs)
k8s_yaml(kustomize('./operator/config/crd'))
k8s_resource(
    new_name='operator-crds',
    objects=[
        'aicpclusters.aicp.aicp.dev:customresourcedefinition',
        'aicpvalidators.aicp.aicp.dev:customresourcedefinition',
    ],
    labels=['00_operator'],
)

# Operator namespace
create_namespace('aicp-operator-system')

# Apply RBAC via kustomize (includes service account, roles, bindings)
k8s_yaml(kustomize('./operator/config/rbac'))
k8s_resource(
    new_name='operator-rbac',
    objects=[
        'aicp-operator-system:namespace',
        'controller-manager:serviceaccount',
        'manager-role:clusterrole',
        'manager-rolebinding:clusterrolebinding',
        'leader-election-role:role',
        'leader-election-role-binding:rolebinding',
        'metrics-auth-role:clusterrole',
        'metrics-auth-role-binding:clusterrolebinding',
        'metrics-reader:clusterrole',
        'aicpcluster-admin-role:clusterrole',
        'aicpcluster-editor-role:clusterrole',
        'aicpcluster-viewer-role:clusterrole',
        'aicpvalidator-admin-role:clusterrole',
        'aicpvalidator-editor-role:clusterrole',
        'aicpvalidator-viewer-role:clusterrole',
    ],
    labels=['00_operator'],
)

# Deploy operator manager as a simple Deployment for local dev.
# Uses --leader-elect=false since we run a single replica locally.
k8s_yaml(blob("""
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aicp-operator
  namespace: aicp-operator-system
  labels:
    control-plane: controller-manager
    app.kubernetes.io/name: aicp-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
      app.kubernetes.io/name: aicp-operator
  template:
    metadata:
      labels:
        control-plane: controller-manager
        app.kubernetes.io/name: aicp-operator
    spec:
      serviceAccountName: controller-manager
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: manager
        image: aicp-operator
        command:
        - /manager
        args:
        - --leader-elect=false
        - --health-probe-bind-address=:8081
        ports:
        - containerPort: 8081
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        resources:
          limits:
            cpu: 500m
            memory: 256Mi
          requests:
            cpu: 100m
            memory: 128Mi
"""))

k8s_resource('aicp-operator',
    resource_deps=['operator-crds', 'operator-rbac'],
    labels=['00_operator'],
)

# ── Sample CRs ──────────────────────────────────────────────
# Apply sample AICPCluster and AICPValidator CRs for development.
# Manual trigger only -- use the Tilt UI button to apply.
local_resource('apply-sample-cr',
    cmd='kubectl apply -f operator/config/samples/ -n aicp',
    resource_deps=['aicp-operator'],
    auto_init=False,
    labels=['00_operator'],
)
