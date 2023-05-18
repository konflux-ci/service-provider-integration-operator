#!/bin/bash
set -e

RS_NAMESPACE=${1:-default}
TARGET_NS_1="spi-test-target1"
TARGET_NS_2="spi-test-target2"
RS_NAME="test-remote-secret"

function cleanup() {
    kubectl delete namespace "${TARGET_NS_1}" --ignore-not-found=true
    kubectl delete namespace "${TARGET_NS_2}" --ignore-not-found=true
    kubectl delete remotesecret/"${RS_NAME}" -n "${RS_NAMESPACE}" --ignore-not-found=true
}

function print() {
  echo
  echo "${1}"
  echo "--------------------------------------------------------"
}

print "Cleaning up resources from previous runs..."
cleanup


print "Enabling remote secrets..."
kubectl -n spi-system patch configmap spi-controller-manager-environment-config --type=merge -p '{"data": {"ENABLEREMOTESECRETS": "true"}}'
kubectl -n spi-system rollout restart deployment/spi-controller-manager


print "Creating two test target namespaces..."
kubectl create namespace "${TARGET_NS_1}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${TARGET_NS_2}" --dry-run=client -o yaml | kubectl apply -f -


print 'Creating remote secret with previously created namespaces as targets...'
cat <<EOF | kubectl create -n "${RS_NAMESPACE}" -f -
apiVersion: appstudio.redhat.com/v1beta1
kind: RemoteSecret
metadata:
  name: ${RS_NAME}
spec:
  secret:
    generateName: some-secret-
  targets:
  - namespace: ${TARGET_NS_1}
  - namespace: ${TARGET_NS_2}
EOF
kubectl wait --for=condition=DataObtained=false remotesecret/"${RS_NAME}" -n "${RS_NAMESPACE}"
echo 'RemoteSecret successfully created.'
kubectl get remotesecret "${RS_NAME}" -n "${RS_NAMESPACE}" -o yaml


print 'Creating upload secret...'
cat <<EOF | kubectl create -n "${RS_NAMESPACE}" -f -
apiVersion: v1
kind: Secret
metadata:
  name: test-remote-secret-secret
  labels:
    spi.appstudio.redhat.com/upload-secret: remotesecret
  annotations:
    spi.appstudio.redhat.com/remotesecret-name: ${RS_NAME}
type: Opaque
stringData:
  a: b
  c: d
EOF
kubectl wait --for=condition=Deployed -n "${RS_NAMESPACE}" remotesecret "${RS_NAME}"
echo 'Upload secret successfully created.'


print 'Checking targets in RemoteSecret status...'
TARGETS=$(kubectl get remotesecret "${RS_NAME}" -n "${RS_NAMESPACE}" --output="jsonpath={.status.targets}")
echo "${TARGETS}"
TARGETS_LEN=$(echo "${TARGETS}" | jq length)
if [ "$TARGETS_LEN" != 2 ]; then
    echo "ERROR: Expected 2 targets, got $TARGETS_LEN"
    exit 1
fi


print "Checking if secret was created in target namespaces..."
TARGETS=$(echo "${TARGETS}" | jq ".[].namespace")
TARGET1_SECRET=$(echo "${TARGETS}" | jq "select(.namespace==\"${TARGET_NS_1}\") | .secretName" | tr -d '"')
TARGET2_SECRET=$(echo "${TARGETS}" | jq "select(.namespace==\"${TARGET_NS_2}\") | .secretName" | tr -d '"')
kubectl get secret/"${TARGET1_SECRET}" -n "${TARGET_NS_1}" --no-headers -o custom-columns=":metadata.name"
kubectl get secret/"${TARGET2_SECRET}" -n "${TARGET_NS_2}" --no-headers -o custom-columns=":metadata.name"

echo
echo "#####################################################"
echo "Test was successful, cleaning up resources..."
echo "#####################################################"
cleanup
