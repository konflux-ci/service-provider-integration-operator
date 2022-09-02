#!/bin/sh

#set -x

THIS_DIR="$(dirname "$(realpath "$0")")"
KCP_DATA_DIR="${THIS_DIR}/../.kcp"

# kill kcp
if [ -d ${KCP_DATA_DIR} ]; then
  pushd ${KCP_DATA_DIR}
  if [ -f kcp.pid ]; then
    echo "killing kcp process ..."
    kill $( cat kcp.pid )
    echo "cleaning .kcp directory ..."
    rm kcp.pid
    rm kcp.log
    rm -rf .kcp
  fi
  popd

  rm -rf ${KCP_DATA_DIR}
else
  echo ".kcp dir does not exist, nothing to clean"
fi
