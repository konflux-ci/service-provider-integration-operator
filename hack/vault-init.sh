#!/bin/bash

NAMESPACE=spi-system
SECRET_NAME=spi-vault-keys
POD_NAME=spi-vault-0
KEYS_FILE=${KEYS_FILE:-$( mktemp )}

function vaultExec() {
  COMMAND=${1}
  kubectl exec ${POD_NAME} -n ${NAMESPACE} -- ${COMMAND} 2> /dev/null
}

function init() {
  if [ "$( isInitialized )" == "false" ]; then
    vaultExec "vault operator init" > "${KEYS_FILE}"
    echo "Keys written at ${KEYS_FILE}"
  else
    echo "Already initialized"
  fi
}

function isInitialized() {
  STATUS=$( vaultExec "vault status -format=yaml" )
  INITIALIZED=$( echo "${STATUS}" | sed -n "s/^.*initialized: \(\S*\).*/\1/p" )
  echo "${INITIALIZED}"
}

function secret() {
  if [ ! -s "${KEYS_FILE}" ]; then
    echo "No keys, cant create secret."
    return
  fi

  if kubectl get secret ${SECRET_NAME} -n ${NAMESPACE}; then
    echo "Secret 5{SECRET_NAME} already exists. Nothing to do."
    return
  fi

  COMMAND="kubectl create secret generic ${SECRET_NAME} -n ${NAMESPACE}"
  KEYI=1
  # shellcheck disable=SC2013
  for KEY in $( grep "Unseal Key" "${KEYS_FILE}" | awk '{split($0,a,": "); print a[2]}'); do
    COMMAND="${COMMAND} --from-literal=key${KEYI}=${KEY}"
    (( KEYI++ ))
  done

  ROOT_TOKEN=$( grep "Root Token" "${KEYS_FILE}" | awk '{split($0,a,": "); print a[2]}' )
  COMMAND="${COMMAND} --from-literal=root_token=${ROOT_TOKEN}"

  ${COMMAND}
}

function restart() {
  kubectl delete pod ${POD_NAME} -n ${NAMESPACE}
}

init
secret
restart
