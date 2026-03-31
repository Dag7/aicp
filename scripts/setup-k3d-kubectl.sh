#!/bin/bash
#
# Setup kubectl context to connect to k3d cluster from the host machine
# This script configures kubectl to connect to the k3d cluster running in Docker
#
# Usage: ./scripts/setup-k3d-kubectl.sh
#

set -e

CLUSTER_NAME="aicp-cluster"
K3D_SERVER_CONTAINER="k3d-${CLUSTER_NAME}-server-0"
K3D_LB_CONTAINER="k3d-${CLUSTER_NAME}-serverlb"
CONTEXT_NAME="k3d-${CLUSTER_NAME}"

echo "Setting up kubectl context for k3d cluster: ${CLUSTER_NAME}"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
  echo "Error: Docker is not running. Please start Docker first."
  exit 1
fi

# Check if the k3d server container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${K3D_SERVER_CONTAINER}$"; then
  echo "Error: k3d container '${K3D_SERVER_CONTAINER}' is not running."
  echo "Make sure you're running devcontainer or k3d cluster is started."
  exit 1
fi

# Find the port mapping for the k3d API server (6443 inside container)
# The port is exposed via the serverlb container
K3D_PORT=$(docker port "${K3D_LB_CONTAINER}" 6443 2>/dev/null | head -1 | cut -d: -f2)

if [ -z "${K3D_PORT}" ]; then
  echo "Trying to find port from docker ps..."
  K3D_PORT=$(docker ps --filter "name=${K3D_LB_CONTAINER}" --format '{{.Ports}}' | grep -oE '0\.0\.0\.0:[0-9]+->6443' | cut -d: -f2 | cut -d- -f1)
fi

if [ -z "${K3D_PORT}" ]; then
  echo "Error: Could not determine k3d API server port."
  exit 1
fi

echo "Found k3d API server on port: ${K3D_PORT}"

# Extract kubeconfig from k3d container
echo "Extracting kubeconfig from k3d container..."
TEMP_KUBECONFIG=$(mktemp)
docker exec "${K3D_SERVER_CONTAINER}" cat /etc/rancher/k3s/k3s.yaml > "${TEMP_KUBECONFIG}"

# Extract the client certificate and key from the k3d kubeconfig
CLIENT_CERT_DATA=$(grep 'client-certificate-data:' "${TEMP_KUBECONFIG}" | awk '{print $2}')
CLIENT_KEY_DATA=$(grep 'client-key-data:' "${TEMP_KUBECONFIG}" | awk '{print $2}')

# Clean up temp file
rm -f "${TEMP_KUBECONFIG}"

if [ -z "${CLIENT_CERT_DATA}" ] || [ -z "${CLIENT_KEY_DATA}" ]; then
  echo "Error: Could not extract credentials from k3d kubeconfig."
  exit 1
fi

# Create directory for k3d credentials
KUBE_DIR="${HOME}/.kube"
K3D_CREDS_DIR="${KUBE_DIR}/k3d-creds"
mkdir -p "${K3D_CREDS_DIR}"

# Write decoded certificates to files
echo "Writing credentials to ${K3D_CREDS_DIR}..."
echo "${CLIENT_CERT_DATA}" | base64 -d > "${K3D_CREDS_DIR}/client.crt"
echo "${CLIENT_KEY_DATA}" | base64 -d > "${K3D_CREDS_DIR}/client.key"
chmod 600 "${K3D_CREDS_DIR}/client.key"

# Configure kubectl cluster
echo "Configuring kubectl cluster..."
kubectl config set-cluster "${CONTEXT_NAME}" \
  --server="https://localhost:${K3D_PORT}" \
  --insecure-skip-tls-verify=true

# Configure kubectl credentials
echo "Configuring kubectl credentials..."
kubectl config set-credentials "${CONTEXT_NAME}" \
  --client-certificate="${K3D_CREDS_DIR}/client.crt" \
  --client-key="${K3D_CREDS_DIR}/client.key"

# Configure kubectl context
echo "Configuring kubectl context..."
kubectl config set-context "${CONTEXT_NAME}" \
  --cluster="${CONTEXT_NAME}" \
  --user="${CONTEXT_NAME}"

# Set as current context
echo "Setting current context to ${CONTEXT_NAME}..."
kubectl config use-context "${CONTEXT_NAME}"

# Verify connection
echo ""
echo "Verifying connection..."
if kubectl cluster-info > /dev/null 2>&1; then
  echo "Successfully connected to k3d cluster!"
  echo ""
  kubectl cluster-info
else
  echo "Warning: Could not verify connection. You may need to check the configuration."
fi

echo ""
echo "Done! kubectl is now configured to use k3d cluster."
echo "Current context: $(kubectl config current-context)"
