#!/bin/bash

KCP_CLUSTER_NAME=${KCP_CLUSTER_NAME:-spi-workload}

mkdir -p .tmp

kubectl kcp workload sync ${KCP_CLUSTER_NAME} \
  --syncer-image ghcr.io/kcp-dev/kcp/syncer:release-0.8 \
  --output-file=.tmp/${KCP_CLUSTER_NAME}.yaml \
  --resources=services,routes.route.openshift.io
