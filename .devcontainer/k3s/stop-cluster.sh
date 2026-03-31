#!/bin/bash

stop_k3s_cluster() {
    echo "Stopping k3d cluster: aicp-cluster"
    k3d cluster stop aicp-cluster
}

stop_k3s_cluster
