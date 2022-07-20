#!/bin/sh

set -e
#set -x

NAMESPACE=${NAMESPACE:-spi-vault}
POD_NAME=${POD_NAME:-vault-0}

API_RESOURCES=$( kubectl api-resources )
if echo ${API_RESOURCES} | grep routes > /dev/null; then
  VAULT_HOST=$( kubectl get route -n ${NAMESPACE} vault -o json | jq -r .spec.host )
  echo "https://${VAULT_HOST}"
elif echo ${API_RESOURCES} | grep ingresses > /dev/null; then
  echo "has ingress"
fi
