#!/bin/sh

# This script set remote-secret commit id in:
# - go.mod
# - config/remotesecret/overlays/*

set -e

if [ $# -eq 0 ]
  then
    echo "commit id in https://github.com/redhat-appstudio/remote-secret is not supplied."
fi

# commit id in https://github.com/redhat-appstudio/remote-secret
REMOTE_SECRET_COMMIT_ID=$1
go get  github.com/redhat-appstudio/remote-secret@"$REMOTE_SECRET_COMMIT_ID"
go mod tidy

# shellcheck disable=SC2016
sed -i -e 's|\(https://github.com/redhat-appstudio/remote-secret/config/overlays/minikube_vault?ref=\)\(.*\)|\1'"$REMOTE_SECRET_COMMIT_ID"'|' config/remotesecret/overlays/minikube_vault/kustomization.yaml
