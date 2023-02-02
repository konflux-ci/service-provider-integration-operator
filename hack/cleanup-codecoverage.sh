#!/bin/bash


THIS_DIR="$( dirname "$(realpath "$0")")"
REPO_DIR="$( realpath ${THIS_DIR}/../ )"

COVER_FILE=${REPO_DIR}/cover.out

if [ ! -f ${COVER_FILE} ]; then
  echo "${COVER_FILE} not found, nothing to do"
  exit 1
fi

echo "creating backup of ${COVER_FILE} to ${COVER_FILE}_bck"
cp ${COVER_FILE} ${COVER_FILE}_bck

declare -a EXCLUDE=("integration_tests" "test_util.go" "testtokenstorage.go")

for E in "${EXCLUDE[@]}"; do
  sed -i "/${E}/d" ${COVER_FILE}
done
