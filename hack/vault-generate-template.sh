#!/bin/sh

helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update hashicorp

VERSION=0.22.0

# Openshift template
echo "# This file is generated by './hack/vault-generate-template.sh'. Please do not edit this directly." > config/vault/openshift/deployment.yaml
helm template vault hashicorp/vault --version ${VERSION} --set "global.openshift=true" --values hack/vault-helm-values.yaml --namespace spi-vault >> config/vault/openshift/deployment.yaml

# Kubernetes template
echo "# This file is generated by './hack/vault-generate-template.sh'. Please do not edit this directly." > config/vault/k8s/deployment.yaml
helm template vault hashicorp/vault --version ${VERSION} --values hack/vault-helm-values.yaml --namespace spi-vault >> config/vault/k8s/deployment.yaml
