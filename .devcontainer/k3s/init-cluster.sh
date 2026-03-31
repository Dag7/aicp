#!/bin/bash

base_path="${GITHUB_WORKSPACE:-/workspaces/aicp}"

source $base_path/.devcontainer/scripts/set-docker-permissions.sh

set_docker_permissions

CLUSTER_NAME=aicp-cluster

# Check if cluster exists
IS_K3S_INIT=$(k3d cluster ls 2>/dev/null | grep $CLUSTER_NAME | wc -l)

if [[ $IS_K3S_INIT != '0' ]]; then
    echo "Using existing k8s cluster: $CLUSTER_NAME"
    kubectl config use-context k3d-${CLUSTER_NAME} --namespace=aicp
fi

# Parse command-line options
while getopts ":fh" opt; do
    case $opt in
    f)
        FORCE_CREATE=true
        ;;
    h)
        echo "Usage: init-cluster.sh [-f] [-h]"
        echo "  -f  Force recreate cluster (deletes existing)"
        echo "  -h  Show this help"
        exit 0
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        exit 1
        ;;
    esac
done

if [[ $IS_K3S_INIT != '0' && $FORCE_CREATE ]]; then
    echo "Force recreating cluster: $CLUSTER_NAME"
    k3d cluster delete $CLUSTER_NAME
fi

if [[ $IS_K3S_INIT == '0' || $FORCE_CREATE ]]; then
    echo "Creating k8s cluster: $CLUSTER_NAME"

    k3d cluster create \
        --config ${CLUSTER_NAME}.config.yaml \
        --registry-config ${CLUSTER_NAME}.registries.yaml \
        --wait \
        --verbose
fi

echo "Starting cluster: $CLUSTER_NAME"
k3d cluster start $CLUSTER_NAME
k3d kubeconfig merge -sd $CLUSTER_NAME

until kubectl get nodes; do
    sleep 1
done

# Apply namespace
echo "Applying aicp namespace..."
kubectl apply -f $base_path/k8s/deployment/infra/local/namespace.yaml

# Set default context
echo "Setting k8s context..."
kubectl config set-context --current --namespace=aicp
echo "k8s cluster ready: $CLUSTER_NAME"
