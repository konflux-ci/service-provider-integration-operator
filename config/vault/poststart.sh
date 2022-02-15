#!/bin/bash

VAULT_KEYS_DIR=/vault/userconfig/keys
VAULT_ROOT_TOKEN="${VAULT_KEYS_DIR}/root_token"

function isInitialized() {
  INITIALIZED=$( vault status -format=yaml | grep "initialized" | awk '{split($0,a,": "); print a[2]}' )
  echo "${INITIALIZED}"
}

function login() {
  vault login "$( cat "${VAULT_ROOT_TOKEN}" )"
}

function unseal() {
  if [ "$( isSealed )" == "true" ]; then
    echo "unsealing ..."
    KEYS=$( ls ${VAULT_KEYS_DIR}/key* )
    for KEY in ${KEYS}; do
      if [ "$( isSealed )" == "true" ]; then
        vault operator unseal "$( cat "${KEY}" )"
      else
        echo "unsealed"
        return
      fi
    done
  fi
}

function isSealed() {
  SEALED=$( vault status -format=yaml | grep "sealed" | awk '{split($0,a,": "); print a[2]}' )
  echo "${SEALED}"
}

function audit() {
  if ! vault audit list | grep -q file ; then
    vault audit enable file file_path=/vault/logs/audit.log
  fi
}

function k8sAuth() {
  if ! vault auth list | grep -q kubernetes ; then
    vault auth enable kubernetes
    vault write auth/kubernetes/config \
      token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
      kubernetes_host=https://${KUBERNETES_PORT_443_TCP_ADDR}:443 \
      kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  fi
}

function spiSecretEngine() {
  if ! vault secrets list | grep -q spi ; then
    vault secrets enable -path=spi kv-v2
  fi
}

if [ "$( isInitialized )" == "false" ]; then
  echo "vault not initialized. This is manual action."
  return
fi

# shellcheck disable=SC2012
if [ "$( ls "${VAULT_KEYS_DIR}" | wc -l )" == "0" ]; then
  echo "no keys found."
  return
fi

unseal
login
audit
k8sAuth
spiSecretEngine
