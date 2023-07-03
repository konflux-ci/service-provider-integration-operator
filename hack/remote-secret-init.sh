#!/bin/bash

SPI_POLICY_NAME=${SPI_DATA_PATH_PREFIX//\//-}
SPI_DATA_PATH_PREFIX=${SPI_DATA_PATH_PREFIX:-spi}
VAULT_KUBE_CONFIG=${VAULT_KUBE_CONFIG:-${KUBECONFIG:-$HOME/.kube/config}}
VAULT_NAMESPACE=${VAULT_NAMESPACE:-spi-vault}
VAULT_PODNAME=${VAULT_PODNAME:-vault-0}
REMOTE_SECRET_APP_ROLE_FILE="$(realpath .tmp)/approle_remote_secret.yaml"
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
	echo "---" >>${REMOTE_SECRET_APP_ROLE_FILE}
	kubectl --kubeconfig=${VAULT_KUBE_CONFIG} create secret generic vault-approle-${1} \
		--from-literal=role_id=${ROLE_ID} --from-literal=secret_id=${SECRET_ID} \
		--dry-run=client -o yaml >>${REMOTE_SECRET_APP_ROLE_FILE}
}

function restart() {
	echo "restarting vault pod '${VAULT_PODNAME}' ..."
	kubectl --kubeconfig=${VAULT_KUBE_CONFIG} delete pod ${VAULT_PODNAME} -n ${VAULT_NAMESPACE} >/dev/null
}

function auth() {
	if ! vaultExec "vault auth list | grep -q approle"; then
		echo "setup approle authentication ..."
		vaultExec "vault auth enable approle"
	fi

	if [ ! -d ".tmp" ]; then mkdir -p .tmp; fi

	if [ -f ${REMOTE_SECRET_APP_ROLE_FILE} ]; then rm ${REMOTE_SECRET_APP_ROLE_FILE}; fi
	touch ${REMOTE_SECRET_APP_ROLE_FILE}
	approleSet remote-secret-operator

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
	restart
}

if ! timeout 300s bash -c "while ! kubectl get applications.argoproj.io -n openshift-gitops -o name | grep -q remote-secret-controller-in-cluster-local; do printf '.'; sleep 5; done"; then
	printf "Application remote-secret-controller-in-cluster-local not found (timeout)\n"
	kubectl get apps -n openshift-gitops -o name
	exit 1
else
	if [ "$(oc get applications.argoproj.io remote-secret-controller-in-cluster-local -n openshift-gitops -o jsonpath='{.status.health.status} {.status.sync.status}')" != "Healthy Synced" ]; then
		echo "Initializing remote secret controller"
		if [ ! -f "$REMOTE_SECRET_APP_ROLE_FILE" ]; then
			approleAuthRemoteSecret
		fi
		echo "$REMOTE_SECRET_APP_ROLE_FILE exists."
		kubectl apply -f $REMOTE_SECRET_APP_ROLE_FILE -n remotesecret
		echo "Remote secret controller initialization was completed"
	else
		echo "Remote secret controller initialization was skipped"
	fi
fi
