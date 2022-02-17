#!/bin/bash

VAULT_KEYS_DIR=/vault/userconfig/keys

function isInitialized() {
  INITIALIZED=$( vault status -format=yaml | grep "initialized" | awk '{split($0,a,": "); print a[2]}' )
  echo "${INITIALIZED}"
}

function isSealed() {
  SEALED=$( vault status -format=yaml | grep "sealed" | awk '{split($0,a,": "); print a[2]}' )
  echo "${SEALED}"
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
