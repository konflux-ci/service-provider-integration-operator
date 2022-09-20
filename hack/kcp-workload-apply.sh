#!/bin/bash

KCP_CLUSTER_NAME=${KCP_CLUSTER_NAME:-spi-workload}

kubectl apply -f .tmp/${KCP_CLUSTER_NAME}.yaml
