#!/bin/sh

# This script copies the config directory containing the kustomize templates into a subdirectory under .tmp.
# It then replaces the placeholders in those templates using envsubst, sets the images according to the SPIO_IMG
# and SPIS_IMG env vars (as defined by the Makefile) and deploys.

set -e

# the path to the kustomize executable
KUSTOMIZE=$1

# the name of the deployment - only used as a part of the directory the templates are copied into
DEPL_NAME=$2

# The name of the kustomize overlay directory to apply
OVERLAY=$3

THIS_DIR="$(dirname "$(realpath "$0")")"
TEMP_DIR="${THIS_DIR}/../.tmp/deployment_${DEPL_NAME}"

OVERLAY_DIR="${TEMP_DIR}/${OVERLAY}"

# we need this to keep kustomize patches intact
export patch="\$patch"

mkdir -p "${TEMP_DIR}"
cp -r "${THIS_DIR}/../config/"* "${TEMP_DIR}"
find "${TEMP_DIR}" -name '*.yaml' | while read -r f; do
  tmp=$(mktemp)
  envsubst > "$tmp" < "$f"
  mv "$tmp" "$f"
done

CURDIR=$(pwd)

cd "${OVERLAY_DIR}" || exit
if [ ! -z ${SPIO_IMG} ]; then
  ${KUSTOMIZE} edit set image quay.io/redhat-appstudio/service-provider-integration-operator="${SPIO_IMG}"
fi

if [ ! -z ${SPIS_IMG} ]; then
  ${KUSTOMIZE} edit set image quay.io/redhat-appstudio/service-provider-integration-oauth="${SPIS_IMG}"
fi

${KUSTOMIZE} build . | kubectl apply -f -

cd "${CURDIR}" || exit
