#!/bin/bash

echo "generating root token ..."

vault operator generate-root -cancel > /dev/null
INIT=$( vault operator generate-root -init -format=yaml )
NONCE=$( echo "${INIT}" | grep "nonce:" | awk '{split($0,a,": "); print a[2]}' )
OTP=$( echo "${INIT}" | grep "otp:" | awk '{split($0,a,": "); print a[2]}' )

KEYS=$( ls /vault/userconfig/keys/key* )
for KEY_FILE in ${KEYS}; do
  KEY=$( cat "${KEY_FILE}" )
  if [ -z "${KEY}" ]; then
    echo "failed to generate token"
    exit 1
  fi
  GENERATE_OUTPUT=$( echo "${KEY}" | vault operator generate-root -nonce="${NONCE}" -format=yaml - )
  COMPLETE=$( echo "${GENERATE_OUTPUT}" | grep "complete:" | awk '{split($0,a,": "); print a[2]}' )
  if [ "${COMPLETE}" == "true" ]; then
    ENCODED_TOKEN=$( echo "${GENERATE_OUTPUT}" | grep "encoded_token" | awk '{split($0,a,": "); print a[2]}' )
    ROOT_TOKEN=$( vault operator generate-root \
      -decode="${ENCODED_TOKEN}" \
      -otp="${OTP}" -format=yaml | awk '{split($0,a,": "); print a[2]}' )
    vault login "${ROOT_TOKEN}"
    exit 0
  fi
done

exit 1
