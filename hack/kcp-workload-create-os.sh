#!/bin/bash

NAME=${NAME:-spi-workload}

mkdir -p .tmp

kubectl kcp workload sync ${NAME} \
  --syncer-image ghcr.io/kcp-dev/kcp/syncer:release-0.7 \
  --output-file=.tmp/${NAME}.yaml \
  --resources=services,routes.route.openshift.io
