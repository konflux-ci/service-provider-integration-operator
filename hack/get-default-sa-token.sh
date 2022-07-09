#!/bin/bash

KUBE_VERSION=$( kubectl version -o json | jq -r .serverVersion.minor )

# Kubernetes 24+ doesn't create secret with token for serviceaccount
if [ "${KUBE_VERSION}" -ge "24" ]; then
  kubectl create token default -n default
else
  SECRET_NAME=$(kubectl get serviceaccount default -n default -o jsonpath='{.secrets[0].name}')
  kubectl get secret "${SECRET_NAME}" -o jsonpath='{.data.token}' | base64 -d
fi
