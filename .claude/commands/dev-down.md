Stop the AICP devcontainer and clean up.

Steps:
1. Find the running devcontainer: `docker ps --filter name=vsc-aicp-dev --format '{{.ID}}'`
2. Stop k3d cluster inside: `docker exec -e KUBECONFIG=/home/vscode/.kube/config <container> k3d cluster stop aicp-cluster`
3. Stop the container: `docker stop <container>`
4. Report that the environment is stopped
