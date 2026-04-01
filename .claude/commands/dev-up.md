Start the AICP devcontainer and full development environment.

Steps:
1. Start the devcontainer using `npx @devcontainers/cli up --workspace-folder .`
2. Wait for container to be ready (check for the JSON success output)
3. Extract the container ID from the output
4. Verify k3d cluster is running: `docker exec -e KUBECONFIG=/home/vscode/.kube/config <container> kubectl cluster-info`
5. If cluster not running, init it: `docker exec -e KUBECONFIG=/home/vscode/.kube/config <container> bash .devcontainer/k3s/init-cluster.sh`
6. Start Tilt in CI mode: `docker exec -e KUBECONFIG=/home/vscode/.kube/config -w /workspaces/aicp <container> tilt ci --timeout 5m`
7. Report status of all pods

All docker exec commands must include `-e KUBECONFIG=/home/vscode/.kube/config`.

After Tilt succeeds, run the e2e tests: `docker exec -e KUBECONFIG=/home/vscode/.kube/config -w /workspaces/aicp <container> bash scripts/e2e-test.sh`
