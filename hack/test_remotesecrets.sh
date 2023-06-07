#!/bin/bash
set -e

# The script assumes that the RemoteSecret namespace exists.
RS_NAMESPACE=${RS_NAMESPACE:-"default"}
TARGET_NS_1=${TARGET_NS_1:-"spi-test-target1"}
TARGET_NS_2=${TARGET_NS_2:-"spi-test-target2"}
RS_NAME=${RS_NAME:-"test-remote-secret"}

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

# This cleanup is useful when the previous run ended in error and the cleanup function at the end was not called.
# Note that if you run this script each time with different values of RS_NAMESPACE, TARGET_NS_1...
# then this cleanup may not work properly.
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
  name: ${RS_NAME}-secret
  labels:
    appstudio.redhat.com/upload-secret: remotesecret
  annotations:
    appstudio.redhat.com/remotesecret-name: ${RS_NAME}
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
TARGETS=$(echo "${TARGETS}" | jq ".[]")
TARGET1_SECRET=$(echo "${TARGETS}" | jq "select(.namespace==\"${TARGET_NS_1}\") | .secretName" | tr -d '"')
TARGET2_SECRET=$(echo "${TARGETS}" | jq "select(.namespace==\"${TARGET_NS_2}\") | .secretName" | tr -d '"')
kubectl get secret/"${TARGET1_SECRET}" -n "${TARGET_NS_1}" --no-headers -o custom-columns=":metadata.name"
kubectl get secret/"${TARGET2_SECRET}" -n "${TARGET_NS_2}" --no-headers -o custom-columns=":metadata.name"

echo
echo "#####################################################"
echo "Test was successful, cleaning up resources..."
echo "#####################################################"
cleanup
