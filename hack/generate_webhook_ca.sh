#!/bin/sh

#  This script generates the certificates needed for the webhook to work.

set -e


THIS_DIR="$(dirname "$(realpath "$0")")"
TEMP_DIR="${THIS_DIR}/../.tmp/deployment_remotesecret_k8s"

GENCERTS_DIR="${TEMP_DIR}/webhook/k8s/certs"

mkdir -p "${GENCERTS_DIR}"

openssl genrsa -out ${GENCERTS_DIR}/ca.key 2048
openssl req -x509 -new -nodes -key ${GENCERTS_DIR}/ca.key -subj "/CN=webhook-service.remotesecret.svc" -days 10000 -out ${GENCERTS_DIR}/ca.crt
openssl genrsa -out ${GENCERTS_DIR}/tls.key 2048
openssl req -new -key ${GENCERTS_DIR}/tls.key -out ${GENCERTS_DIR}/tls.csr -config ${THIS_DIR}/csr.conf
openssl x509 -req -in ${GENCERTS_DIR}/tls.csr -CA ${GENCERTS_DIR}/ca.crt -CAkey ${GENCERTS_DIR}/ca.key -CAcreateserial -out ${GENCERTS_DIR}/tls.crt -days 10000 -extensions v3_ext -extfile ${THIS_DIR}/csr.conf -sha256

CA_BUNDLE=$(cat ${GENCERTS_DIR}/ca.crt | base64 | tr -d '\n')
echo $CA_BUNDLE
