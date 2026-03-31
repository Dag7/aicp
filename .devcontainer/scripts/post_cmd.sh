#!/bin/bash

# Set base path depends on running workspace environment
base_path="${GITHUB_WORKSPACE:-/workspaces/aicp}"

# Copy host configuration files to container
source $base_path/.devcontainer/scripts/copy_configs.sh
source $base_path/.devcontainer/scripts/set-docker-permissions.sh

# Disable global git hooks in dev container
git config --global core.hooksPath /dev/null

# Fix Claude config volume permissions (volumes are mounted as root)
if [ -d "/home/vscode/.claude" ]; then
    echo "Fixing Claude config volume permissions..."
    sudo chown -R vscode:vscode /home/vscode/.claude
fi

set_docker_permissions
copy_host_configs

# Initialize k3d cluster
echo "Initializing k3d cluster..."
cd $base_path/.devcontainer/k3s && bash init-cluster.sh
