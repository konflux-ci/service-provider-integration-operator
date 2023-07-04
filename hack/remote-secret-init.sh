#!/bin/bash

# !!! Note that this script should not be used for production purposes !!!

# in the infra-deployment repo, utils.sh script will be saved in a tmp folder
# https://github.com/rsoaresd/infra-deployments/blob/SVPI-518/hack/preview.sh#L206-L207
source $(realpath .tmp)/utils.sh

SPI_POLICY_NAME=${SPI_DATA_PATH_PREFIX//\//-}
SPI_DATA_PATH_PREFIX=${SPI_DATA_PATH_PREFIX:-spi}
VAULT_KUBE_CONFIG=${VAULT_KUBE_CONFIG:-${KUBECONFIG:-$HOME/.kube/config}}
VAULT_NAMESPACE=${VAULT_NAMESPACE:-spi-vault}
VAULT_PODNAME=${VAULT_PODNAME:-vault-0}
REMOTE_SECRET_APP_ROLE_FILE="$(realpath .tmp)/approle_remote_secret.yaml"
ROOT_TOKEN_NAME=vault-root-token

function auth() {
	if ! vaultExec "vault auth list | grep -q approle"; then
		echo "setup approle authentication ..."
		vaultExec "vault auth enable approle"
	fi

	if [ ! -d ".tmp" ]; then mkdir -p .tmp; fi

	if [ -f ${REMOTE_SECRET_APP_ROLE_FILE} ]; then rm ${REMOTE_SECRET_APP_ROLE_FILE}; fi

	touch ${REMOTE_SECRET_APP_ROLE_FILE}
	approleSet remote-secret-operator ${REMOTE_SECRET_APP_ROLE_FILE}

	cat <<EOF

secret yaml with Vault credentials prepared
make sure your kubectl context targets cluster with SPI/RemoteSecret deployment and create the secret using:

  $ kubectl apply -f ${REMOTE_SECRET_APP_ROLE_FILE} -n remotesecret

EOF
}

function approleAuthRemoteSecret() {
	login
	audit
	auth
}

if ! timeout 300s bash -c "while ! kubectl get applications.argoproj.io -n openshift-gitops -o name | grep -q remote-secret-controller-in-cluster-local; do printf '.'; sleep 5; done"; then
	printf "Application remote-secret-controller-in-cluster-local not found (timeout)\n"
	kubectl get apps -n openshift-gitops -o name
	exit 1
else
	if [ "$(oc get applications.argoproj.io remote-secret-controller-in-cluster-local -n openshift-gitops -o jsonpath='{.status.health.status} {.status.sync.status}')" != "Healthy Synced" ]; then
		echo "Initializing remote secret controller"
		approleAuthRemoteSecret
		restart
		kubectl apply -f $REMOTE_SECRET_APP_ROLE_FILE -n remotesecret
		echo "Remote secret controller initialization was completed"
	else
		echo "Remote secret controller initialization was skipped"
	fi
fi
