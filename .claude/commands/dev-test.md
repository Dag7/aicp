Run e2e tests against the running AICP devcontainer environment.

Steps:
1. Find the running devcontainer: `docker ps --filter name=vsc-aicp-dev --format '{{.ID}}'`
2. If no container found, tell the user to run /dev-up first
3. Run e2e tests: `docker exec -e KUBECONFIG=/home/vscode/.kube/config -w /workspaces/aicp <container> bash scripts/e2e-test.sh`
4. Report results

All docker exec commands must include `-e KUBECONFIG=/home/vscode/.kube/config`.
