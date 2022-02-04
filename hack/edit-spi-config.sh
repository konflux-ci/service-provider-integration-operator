#!/bin/sh

set -e

CONFIG_SECRET=$(kubectl -n spi-system get secret -l app.kubernetes.io/part-of=service-provider-integration-operator -o name)

TEMP_SECRET=$(mktemp)
TEMP_CONFIG=$(mktemp)

kubectl get -n spi-system "${CONFIG_SECRET}" -o json > "${TEMP_SECRET}"

jq -r '.data["config.yaml"]' < "${TEMP_SECRET}" | base64 -d > "${TEMP_CONFIG}"

vim "${TEMP_CONFIG}"

NEW_CONFIG=$(cat "${TEMP_CONFIG}")

jq -r ".data[\"config.yaml\"] |= \"$(base64 -w 0 < "${TEMP_CONFIG}")\"" < "${TEMP_SECRET}" | kubectl apply -f -

rm "${TEMP_SECRET}"
rm "${TEMP_CONFIG}"
