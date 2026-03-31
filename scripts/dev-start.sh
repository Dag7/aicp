#!/bin/bash
#
# Start the full AICP development environment
# Run this inside the devcontainer after cluster is initialized
#
# Usage: ./scripts/dev-start.sh
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "============================================"
echo "  AICP Development Environment"
echo "============================================"

# Step 1: Check prerequisites
echo ""
echo "[1/4] Checking prerequisites..."

command -v kubectl >/dev/null 2>&1 || { echo "Error: kubectl not found"; exit 1; }
command -v k3d >/dev/null 2>&1 || { echo "Error: k3d not found"; exit 1; }
command -v tilt >/dev/null 2>&1 || { echo "Error: tilt not found"; exit 1; }
command -v helm >/dev/null 2>&1 || { echo "Error: helm not found"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker not found"; exit 1; }

docker info >/dev/null 2>&1 || { echo "Error: Docker is not running"; exit 1; }
echo "  All prerequisites met."

# Step 2: Ensure k3d cluster is running
echo ""
echo "[2/4] Ensuring k3d cluster..."

CLUSTER_NAME="aicp-cluster"
if k3d cluster ls 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    # Check if cluster is running
    if k3d cluster ls 2>/dev/null | grep "$CLUSTER_NAME" | grep -q "1/1"; then
        echo "  Cluster already running."
    else
        echo "  Starting stopped cluster..."
        k3d cluster start "$CLUSTER_NAME"
    fi
else
    echo "  Creating cluster..."
    cd "$PROJECT_ROOT/.devcontainer/k3s"
    bash init-cluster.sh
    cd "$PROJECT_ROOT"
fi

# Wait for cluster to be ready
echo "  Waiting for nodes..."
kubectl wait --for=condition=Ready nodes --all --timeout=60s

echo "  Cluster ready."

# Step 3: Ensure namespace exists
echo ""
echo "[3/4] Setting up namespace..."
kubectl apply -f "$PROJECT_ROOT/k8s/deployment/infra/local/namespace.yaml"
kubectl config set-context --current --namespace=aicp

# Step 4: Start Tilt
echo ""
echo "[4/4] Starting Tilt..."
echo ""
echo "  Dashboard: http://localhost:10350"
echo "  Server:    http://localhost:8080"
echo "  Metrics:   http://localhost:9090"
echo "  pgAdmin:   http://localhost:5050"
echo ""

cd "$PROJECT_ROOT"
tilt up
