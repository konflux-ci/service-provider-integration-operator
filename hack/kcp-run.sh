#!/bin/sh

#set -x

if ! which kcp; then
  echo "kcp binary required on path"
  echo "you can install it with running:"
  echo "    $ git clone https://github.com/kcp-dev/kcp && cd kcp && make install"
  exit 1
fi

if ! which kubectl-kcp; then
  echo "kubectl-kcp binary required on path"
  echo "you can install it with running:"
  echo "    $ git clone https://github.com/kcp-dev/kcp && cd kcp && make install"
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

echo "setting KUBECONFIG to ${KCP_DATA_DIR}/.kcp/admin.kubeconfig"
export KUBECONFIG=${KCP_DATA_DIR}/.kcp/admin.kubeconfig

echo "creating spi kcp workspace"
kubectl-kcp workspace create spi --enter


CLUSTER=spi-workload-cluster
KCP_SYNCER_VERSION=release-0.6
# TODO: for openshift we need to add routes to --resources
kubectl kcp workload sync ${CLUSTER} --syncer-image ghcr.io/kcp-dev/kcp/syncer:${KCP_SYNCER_VERSION} --resources=services,ingresses.networking.k8s.io > cluster_${CLUSTER}_syncer.yaml
echo "configuration for workload cluster was generated at cluster_${CLUSTER}_syncer.yaml"
echo "now you need to apply it on your cluster directly with:"
echo "kubectl apply -f ${KCP_DATA_DIR}/cluster_${CLUSTER}_syncer.yaml"
echo ""
echo "don't forget to check your kubectl context!!! 'kubectl config get-contexts' (switch with 'kubectl config use-context <ctx>')"

popd

echo "kcp has started. It's log is at .kcp/kcp.log"
