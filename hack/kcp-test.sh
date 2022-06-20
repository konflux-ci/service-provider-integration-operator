#!/bin/sh

#set -x

if ! which kcp; then
  echo "kcp binary required on path"
  exit 1
fi

if ! which kubectl-kcp; then
  echo "kubectl-kcp binary required on path"
  exit 1
fi

THIS_DIR="$(dirname "$(realpath "$0")")"
KCP_DATA_DIR="${THIS_DIR}/../.kcp"

if [ -d ${KCP_DATA_DIR} ]; then
  echo ".kcp dir exists, kcp might be already running. To continue please kill and cleanup first with ./hack/kcp-kill.sh"
  exit 1
fi

mkdir -p ${KCP_DATA_DIR}
cp ${THIS_DIR}/kcp-tokens ${KCP_DATA_DIR}
pushd ${KCP_DATA_DIR}

if [ ! -f kcp.pid ]; then
  echo "Starting KCP server ..."
  exec kcp start \
  --token-auth-file kcp-tokens \
  --discovery-poll-interval 3s \
  &> kcp.log &

  KCP_PID=$!
  echo "${KCP_PID}" > kcp.pid

  until grep 'Ready to start controllers' kcp.log; do
    echo "waiting for kcp to start ..."
    sleep 1
  done

  echo "KCP server started: $KCP_PID"
fi

KUBECONFIG=${KCP_DATA_DIR}/.kcp/admin.kubeconfig
kubectl --kubeconfig=${KUBECONFIG} api-resources

#
#export KUBECONFIG=${KCP_DATA_DIR}/.kcp/admin.kubeconfig
#kubectl-kcp workspace create spi --enter

popd
