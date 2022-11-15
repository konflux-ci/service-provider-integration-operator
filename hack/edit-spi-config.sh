#!/bin/sh

# This script is a helper to update the configuration of SPI in the cluster. This is stored as a secret, because it
# contains the client secrets for OAuth applications in individual service providers.

# The scripts downloads the configuration secret and opens vim with the decoded SPI configuration file. You can make
# any changes you want, save and exit vim. The script then picks up the saved file and updates the SPI configuration
# in the cluster.

# This assumes SPI deployment in the "spi-system" namespace.

set -e

CONFIG_SECRET=$(kubectl -n spi-system get secret -o name | grep shared-configuration-file)

TEMP_SECRET=$(mktemp)
TEMP_CONFIG=$(mktemp)

kubectl get -n spi-system "${CONFIG_SECRET}" -o json > "${TEMP_SECRET}"

jq -r '.data["config.yaml"]' < "${TEMP_SECRET}" | base64 -d > "${TEMP_CONFIG}"

vim "${TEMP_CONFIG}"

jq -r ".data[\"config.yaml\"] |= \"$(base64 -w 0 < "${TEMP_CONFIG}")\"" < "${TEMP_SECRET}" | kubectl apply -f -

rm "${TEMP_SECRET}"
rm "${TEMP_CONFIG}"
