#!/bin/bash

delete_k3s_cluster() {
    echo "Deleting k3d cluster: aicp-cluster"
    k3d cluster delete aicp-cluster
}

delete_k3s_cluster
