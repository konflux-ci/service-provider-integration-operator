#!/bin/bash

SPI_POLICY_NAME=${SPI_DATA_PATH_PREFIX//\//-}
VAULT_KUBE_CONFIG=${VAULT_KUBE_CONFIG:-${KUBECONFIG:-$HOME/.kube/config}}
VAULT_NAMESPACE=${VAULT_NAMESPACE:-spi-vault}
VAULT_PODNAME=${VAULT_PODNAME:-vault-0}
SPI_APP_ROLE_FILE="$(realpath .tmp)/approle_secret.yaml"
ROOT_TOKEN_NAME=vault-root-token

function login() {
	ROOT_TOKEN=$(kubectl --kubeconfig=${VAULT_KUBE_CONFIG} get secret ${ROOT_TOKEN_NAME} -n ${VAULT_NAMESPACE} -o jsonpath="{.data.root_token}" | base64 --decode)
	vaultExec "vault login ${ROOT_TOKEN} > /dev/null"
}

function audit() {
	if ! vaultExec "vault audit list | grep -q file"; then
		echo "enabling audit log ..."
		vaultExec "vault audit enable file file_path=stdout"
	fi
}

function vaultExec() {
	COMMAND=${1}
	kubectl --kubeconfig=${VAULT_KUBE_CONFIG} exec ${VAULT_PODNAME} -n ${VAULT_NAMESPACE} -- sh -c "${COMMAND}" 2>/dev/null
}

function approleSet() {
	vaultExec "vault write auth/approle/role/${1} token_policies=${SPI_POLICY_NAME}"
	ROLE_ID=$(vaultExec "vault read auth/approle/role/${1}/role-id --format=json" | jq -r '.data.role_id')
	SECRET_ID=$(vaultExec "vault write -force auth/approle/role/${1}/secret-id --format=json" | jq -r '.data.secret_id')
	echo "---" >>${SPI_APP_ROLE_FILE}
	kubectl --kubeconfig=${VAULT_KUBE_CONFIG} create secret generic vault-approle-${1} \
		--from-literal=role_id=${ROLE_ID} --from-literal=secret_id=${SECRET_ID} \
		--dry-run=client -o yaml >>${SPI_APP_ROLE_FILE}
}

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
		kubectl apply -f $SPI_APP_ROLE_FILE -n spi-system
		echo "SPI initialization was completed"
	else
		echo "SPI initialization was skipped"
	fi
fi
