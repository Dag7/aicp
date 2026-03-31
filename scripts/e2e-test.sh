#!/bin/bash
#
# End-to-end test for the AICP development environment
# Verifies the full stack is running correctly in k3d + Tilt
#
# Usage: ./scripts/e2e-test.sh
#
# Prerequisites:
#   - k3d cluster running (init-cluster.sh)
#   - tilt up (or dev-start.sh)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP: $1"; SKIP=$((SKIP + 1)); }

echo "============================================"
echo "  AICP E2E Test Suite"
echo "============================================"

# ── 1. Cluster Health ─────────────────────────────────────────
echo ""
echo "[1/6] Cluster Health"

if kubectl cluster-info >/dev/null 2>&1; then
    pass "kubectl connected to cluster"
else
    fail "kubectl cannot connect to cluster"
    echo "Aborting -- cluster not available."
    exit 1
fi

if kubectl get ns aicp >/dev/null 2>&1; then
    pass "aicp namespace exists"
else
    fail "aicp namespace not found"
fi

NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [ "$NODE_COUNT" -ge 1 ]; then
    pass "cluster has $NODE_COUNT node(s)"
else
    fail "no nodes found"
fi

# ── 2. Infrastructure Pods ────────────────────────────────────
echo ""
echo "[2/6] Infrastructure"

check_pod() {
    local name="$1"
    local ns="${2:-aicp}"
    if kubectl get pods -n "$ns" -l "app=$name" --field-selector=status.phase=Running --no-headers 2>/dev/null | grep -q Running; then
        pass "$name pod running"
    else
        # Check if pod exists at all
        if kubectl get pods -n "$ns" -l "app=$name" --no-headers 2>/dev/null | grep -q .; then
            local status=$(kubectl get pods -n "$ns" -l "app=$name" --no-headers 2>/dev/null | awk '{print $3}' | head -1)
            fail "$name pod exists but status: $status"
        else
            fail "$name pod not found"
        fi
    fi
}

check_pod "postgresql"
check_pod "redis"

# pgAdmin is optional
if kubectl get pods -n aicp -l "app=pgadmin" --no-headers 2>/dev/null | grep -q .; then
    check_pod "pgadmin"
else
    skip "pgadmin not deployed"
fi

# ── 3. Database Connectivity ──────────────────────────────────
echo ""
echo "[3/6] Database"

# Test PostgreSQL connectivity via port-forward or in-cluster
PG_POD=$(kubectl get pods -n aicp -l app=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$PG_POD" ]; then
    if kubectl exec -n aicp "$PG_POD" -- pg_isready -U aicp -d aicp >/dev/null 2>&1; then
        pass "PostgreSQL accepting connections"
    else
        fail "PostgreSQL not ready"
    fi

    # Check if migrations ran (tables exist)
    TABLE_COUNT=$(kubectl exec -n aicp "$PG_POD" -- psql -U aicp -d aicp -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'" 2>/dev/null | tr -d ' ')
    if [ -n "$TABLE_COUNT" ] && [ "$TABLE_COUNT" -gt 0 ]; then
        pass "database has $TABLE_COUNT tables (migrations applied)"
    else
        skip "no tables found (migrations may not have run yet)"
    fi
else
    fail "PostgreSQL pod not found"
fi

# Test Redis connectivity
REDIS_POD=$(kubectl get pods -n aicp -l app=redis -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$REDIS_POD" ]; then
    if kubectl exec -n aicp "$REDIS_POD" -- redis-cli ping 2>/dev/null | grep -q PONG; then
        pass "Redis responding to PING"
    else
        fail "Redis not responding"
    fi
else
    fail "Redis pod not found"
fi

# ── 4. AICP Server ────────────────────────────────────────────
echo ""
echo "[4/6] AICP Server"

# Check if server deployment exists and has ready replicas
SERVER_READY=$(kubectl get deploy -n aicp aicp-server -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
if [ "$SERVER_READY" = "1" ] || [ "$SERVER_READY" -ge 1 ] 2>/dev/null; then
    pass "aicp-server deployment ready ($SERVER_READY replicas)"
else
    if kubectl get deploy -n aicp aicp-server >/dev/null 2>&1; then
        fail "aicp-server deployment exists but not ready (readyReplicas=$SERVER_READY)"
    else
        fail "aicp-server deployment not found"
    fi
fi

# Test health endpoint via ephemeral port-forward
SERVER_POD=$(kubectl get pods -n aicp -l app.kubernetes.io/name=aicp -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$SERVER_POD" ]; then
    # Start port-forward in background on a random high port
    PF_PORT=18080
    kubectl port-forward -n aicp "$SERVER_POD" ${PF_PORT}:8080 &
    PF_PID=$!
    sleep 2

    HEALTH_STATUS=$(curl -sf --max-time 5 http://localhost:${PF_PORT}/health 2>/dev/null || echo "")
    if echo "$HEALTH_STATUS" | grep -qi "ok\|healthy\|status"; then
        pass "server /health endpoint responding"
    else
        fail "server /health not responding (got: $HEALTH_STATUS)"
    fi

    # Test metrics endpoint
    PF_METRICS_PORT=19090
    kubectl port-forward -n aicp "$SERVER_POD" ${PF_METRICS_PORT}:9090 &
    PF_METRICS_PID=$!
    sleep 1

    if curl -sf --max-time 5 http://localhost:${PF_METRICS_PORT}/metrics >/dev/null 2>&1; then
        pass "server /metrics endpoint responding"
    else
        skip "server /metrics not responding"
    fi

    # Cleanup port-forwards
    kill $PF_PID $PF_METRICS_PID 2>/dev/null || true
    wait $PF_PID $PF_METRICS_PID 2>/dev/null || true
else
    skip "server pod not found for health check"
fi

# ── 5. Operator ───────────────────────────────────────────────
echo ""
echo "[5/6] Operator"

# Check CRDs
if kubectl get crd aicpclusters.aicp.aicp.dev >/dev/null 2>&1; then
    pass "AICPCluster CRD installed"
else
    fail "AICPCluster CRD not found"
fi

if kubectl get crd aicpvalidators.aicp.aicp.dev >/dev/null 2>&1; then
    pass "AICPValidator CRD installed"
else
    fail "AICPValidator CRD not found"
fi

# Check operator deployment (kustomize names it aicp-operator-controller-manager)
OPERATOR_READY=$(kubectl get deploy -n aicp-operator-system aicp-operator-controller-manager -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
if [ "${OPERATOR_READY:-0}" -ge 1 ] 2>/dev/null; then
    pass "aicp-operator deployment ready"
else
    if kubectl get deploy -n aicp-operator-system aicp-operator-controller-manager >/dev/null 2>&1; then
        fail "aicp-operator exists but not ready (readyReplicas=${OPERATOR_READY:-0})"
    else
        skip "aicp-operator not deployed (optional)"
    fi
fi

# ── 6. Helm Chart Validation ─────────────────────────────────
echo ""
echo "[6/6] Helm Chart"

if helm lint "$PROJECT_ROOT/deploy/helm/aicp" >/dev/null 2>&1; then
    pass "Helm chart lint passed"
else
    fail "Helm chart lint failed"
fi

# Validate template rendering
if helm template aicp "$PROJECT_ROOT/deploy/helm/aicp" >/dev/null 2>&1; then
    pass "Helm template renders successfully"
else
    fail "Helm template rendering failed"
fi

# ── Summary ───────────────────────────────────────────────────
echo ""
echo "============================================"
echo "  Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "============================================"

if [ $FAIL -gt 0 ]; then
    echo ""
    echo "Some tests FAILED. Check the output above."
    exit 1
else
    echo ""
    echo "All tests passed!"
    exit 0
fi
