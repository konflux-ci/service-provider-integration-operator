#!/bin/sh

set -e
#set -x

NAMESPACE=${NAMESPACE:-spi-vault}
POD_NAME=${POD_NAME:-vault-0}

API_RESOURCES=$( kubectl api-resources )
if echo ${API_RESOURCES} | grep routes > /dev/null; then
  VAULT_HOST=$( kubectl get route -n ${NAMESPACE} vault -o json | jq -r .spec.host )
elif echo ${API_RESOURCES} | grep ingresses > /dev/null; then
  VAULT_HOST=$( kubectl get ingress -n ${NAMESPACE} vault -o json | jq -r '.spec.rules[0].host' )
fi

if [ ! -z ${VAULT_HOST} ]; then
  echo "https://${VAULT_HOST}"
fi
