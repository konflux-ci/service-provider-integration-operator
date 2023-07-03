#!/bin/bash

# !!! Note that this script should not be used for production purposes !!!

# in the infra-deployment repo, utils.sh script will be saved in a tmp folder
# https://github.com/rsoaresd/infra-deployments/blob/SVPI-518/hack/preview.sh#L206-L207
source $(realpath .tmp)/utils.sh

SPI_POLICY_NAME=${SPI_DATA_PATH_PREFIX//\//-}
VAULT_KUBE_CONFIG=${VAULT_KUBE_CONFIG:-${KUBECONFIG:-$HOME/.kube/config}}
VAULT_NAMESPACE=${VAULT_NAMESPACE:-spi-vault}
VAULT_PODNAME=${VAULT_PODNAME:-vault-0}
SPI_APP_ROLE_FILE="$(realpath .tmp)/approle_secret.yaml"
ROOT_TOKEN_NAME=vault-root-token

function k8sAuth() {
	if ! vaultExec "vault auth list | grep -q kubernetes"; then
		echo "setup kubernetes authentication ..."
		vaultExec "vault auth enable kubernetes"
	fi
	vaultExec "vault write auth/kubernetes/role/spi-controller-manager \
        bound_service_account_names=spi-controller-manager \
        bound_service_account_namespaces=spi-system \
        policies=${SPI_POLICY_NAME}"
	vaultExec "vault write auth/kubernetes/role/spi-oauth \
          bound_service_account_names=spi-oauth-sa \
          bound_service_account_namespaces=spi-system \
          policies=${SPI_POLICY_NAME}"
	# shellcheck disable=SC2016
	vaultExec 'vault write auth/kubernetes/config \
        kubernetes_host=https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT'
}

function approleAuth() {
	if ! vaultExec "vault auth list | grep -q approle"; then
		echo "setup approle authentication ..."
		vaultExec "vault auth enable approle"
	fi

	if [ ! -d ".tmp" ]; then mkdir -p .tmp; fi

	if [ -f ${SPI_APP_ROLE_FILE} ]; then rm ${SPI_APP_ROLE_FILE}; fi
	touch ${SPI_APP_ROLE_FILE}
	approleSet spi-operator
	approleSet spi-oauth

	cat <<EOF

secret yaml with Vault credentials prepared
make sure your kubectl context targets cluster with SPI deployment and create the secret using (check spi namespace):

  $ kubectl apply -f ${SPI_APP_ROLE_FILE} -n spi-system

EOF
}

function auth() {
	k8sAuth
	approleAuth
}

function approleAuthSPI() {
	login
	audit
	auth
}

if ! timeout 100s bash -c "while ! kubectl get applications.argoproj.io -n openshift-gitops -o name | grep -q spi-in-cluster-local; do printf '.'; sleep 5; done"; then
	printf "Application spi-in-cluster-local not found (timeout)\n"
	kubectl get apps -n openshift-gitops -o name
	exit 1
else
	if [ "$(oc get applications.argoproj.io spi-in-cluster-local -n openshift-gitops -o jsonpath='{.status.health.status} {.status.sync.status}')" != "Healthy Synced" ]; then
		echo "Initializing SPI"
		if [ ! -f "$SPI_APP_ROLE_FILE" ]; then
			approleAuthSPI
		fi
		echo "$SPI_APP_ROLE_FILE exists."
		restart
		kubectl apply -f $SPI_APP_ROLE_FILE -n spi-system
		echo "SPI initialization was completed"
	else
		echo "SPI initialization was skipped"
	fi
fi
