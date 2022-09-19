SHELL := bash
.SHELLFLAGS = -ec
.ONESHELL:
.DEFAULT_GOAL := help

ifndef VERBOSE
  MAKEFLAGS += --silent
endif

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.3.0
TAG_NAME ?= next

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# SPIO_IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# appstudio.redhat.org/service-provider-integration-operator-bundle:$VERSION and appstudio.redhat.org/service-provider-integration-operator-catalog:$VERSION.
SPIO_IMAGE_TAG_BASE ?= quay.io/redhat-appstudio/service-provider-integration-operator

# SPIO_BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build SPIO_BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
SPIO_BUNDLE_IMG ?= $(SPIO_IMAGE_TAG_BASE)-bundle:$(TAG_NAME)

# Image URL to use all building/pushing image targets
SPIO_IMG ?= $(SPIO_IMAGE_TAG_BASE):$(TAG_NAME)

# SPIS_IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for the OAuth service image.
SPIS_IMAGE_TAG_BASE ?= quay.io/redhat-appstudio/service-provider-integration-oauth

# Image URL to use for deploying of the OAuth service
SPIS_IMG ?= $(SPIS_IMAGE_TAG_BASE):$(TAG_NAME)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq (,$(shell which kubectl))
ifeq (,$(shell which oc))
$(error oc or kubectl is required to proceed)
else
K8S_CLI := oc
endif
else
K8S_CLI := kubectl
endif

# create the temporary directory under the same parent dir as the Makefile
TEMP_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/.tmp

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(MAKE) manifests-kcp

manifests-kcp:	## Generate KCP APIResourceSchema from CRDs
	hack/generate-kcp-api.sh

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

### fmt: Runs go fmt against code
fmt:
  ifneq ($(shell command -v goimports 2> /dev/null),)
	  find . -not -path '*/\.*' -name '*.go' -exec goimports -w {} \;
  else
	  @echo "WARN: goimports is not installed -- formatting using go fmt instead."
	  @echo "      Please install goimports to ensure file imports are consistent."
	  go fmt -x ./...
  endif

### fmt_license: Ensures the license header is set on all files
fmt_license:
  ifneq ($(shell command -v addlicense 2> /dev/null),)
	  @echo 'addlicense -v -f license_header.txt **/*.go'
	  addlicense -v -f license_header.txt $$(find . -not -path '*/\.*' -name '*.go')
  else
	  $(error addlicense must be installed for this rule: go install github.com/google/addlicense)
  endif

### check_fmt: Checks the formatting on files in repo
check_fmt:
  ifeq ($(shell command -v goimports 2> /dev/null),)
	  $(error "goimports must be installed for this rule" && exit 1)
  endif
  ifeq ($(shell command -v addlicense 2> /dev/null),)
	  $(error "error addlicense must be installed for this rule: go install github.com/google/addlicense")
  endif
	  if [[ $$(find . -not -path '*/\.*' -name '*.go' -exec goimports -l {} \;) != "" ]]; then \
	    echo "Files not formatted; run 'make fmt'"; exit 1 ;\
	  fi ;\
	  if ! addlicense -check -f license_header.txt $$(find . -not -path '*/\.*' -name '*.go'); then \
	    echo "Licenses are not formatted; run 'make fmt_license'"; exit 1 ;\
	  fi

lint: ## Run the linter on the codebase
  ifeq ($(shell command -v golangci-lint 2> /dev/null),)
	  $(error "golangci-lint must be installed for this rule" && exit 1)
  endif
	golangci-lint run

check: check_fmt lint test ## Check that the code conforms to all requirements for commit. Formatting, licenses, vet, tests and linters

vet: ## Run go vet against code.
	go vet ./...

test: manifests generate fmt vet envtest ## Run unit tests
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) --arch=amd64 use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out -covermode=atomic -coverpkg=./...

itest: manifests generate fmt vet envtest ## Run only integration tests.
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./integration_tests/...


itest_debug: manifests generate fmt vet envtest ## Start the integration tests in the debugger (suited for "remote debugging")
	$(shell rm ./debug.out)
	$(shell touch ./debug.out)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" dlv test --listen=:2345 --headless=true --api-version=2 --accept-multiclient --redirect=stdout:./debug.out --redirect=stderr:./debug.out ./integration_tests -- -test.v -test.run=TestSuite

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run_as_current_user: manifests generate fmt vet install ## Run a controller from your host as the current user in ~/.kubeconfig
	go run ./main.go

run: ensure-tmp manifests generate fmt vet prepare ## Run a controller from your host using the same RBAC as if deployed in the cluster
	$(eval KUBECONFIG:=$(shell hack/generate-restricted-kubeconfig.sh $(TEMP_DIR) spi-controller-manager spi-system))
	KUBECONFIG=$(KUBECONFIG) go run ./main.go || true
	rm $(KUBECONFIG)

docker-build: test ## Build docker image with the manager.
	docker build -t ${SPIO_IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${SPIO_IMG}

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

prepare: install ## In addition to CRDs also install the RBAC rules
	$(KUSTOMIZE) build config/prepare | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: ensure-tmp manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	VAULT_HOST=`hack/vault-host.sh` SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "default" "default"

deploy_k8s: ensure-tmp manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "k8s" "k8s"
	hack/vault-init.sh
	kubectl apply -f .tmp/approle_secret.yaml -n spi-system

deploy_minikube: ensure-tmp manifests kustomize deploy_vault_minikube ## Deploy controller to the Minikube cluster specified in ~/.kube/config.
	OAUTH_HOST=spi.`minikube ip`.nip.io VAULT_HOST=`hack/vault-host.sh` SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "minikube" "minikube"
	kubectl apply -f .tmp/approle_secret.yaml -n spi-system

deploy_openshift: ensure-tmp manifests kustomize deploy_vault_openshift ## Deploy controller to the K8s cluster specified in ~/.kube/config using the example OpenShift kustomization
	VAULT_HOST=`./hack/vault-host.sh` SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "openshift-example" "openshift-example"
	kubectl apply -f .tmp/approle_secret.yaml -n spi-system

deploy_kcp: ensure-tmp manifests kustomize
	if [ -z ${VAULT_HOST} ]; then echo "VAULT_HOST must be set"; exit 1; fi
	$(eval KCP_WORKSPACE?=$(shell kubectl kcp workspace . --short))
	KCP_WORKSPACE=$(KCP_WORKSPACE) SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "kcp" "kcp"
	kubectl apply -f .tmp/approle_secret.yaml -n spi-system
	kubectl apply -f .tmp/deployment_kcp/kcp/apibinding_spi.yaml

deploy_kcp_openshift: ensure-tmp manifests kustomize
	if [ -z ${VAULT_HOST} ]; then echo "VAULT_HOST must be set"; exit 1; fi
	$(eval KCP_WORKSPACE?=$(shell kubectl kcp workspace . --short))
	KCP_WORKSPACE=$(KCP_WORKSPACE) SPIO_IMG=$(SPIO_IMG) SPIS_IMG=$(SPIS_IMG) hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "kcp" "kcp_openshift"
	kubectl apply -f .tmp/approle_secret.yaml -n spi-system
	kubectl apply -f .tmp/deployment_kcp/kcp/apibinding_spi.yaml

undeploy_k8s: undeploy_vault_k8s ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	if [ ! -d ${TEMP_DIR}/deployment_k8s ]; then echo "No deployment files found in .tmp/deployment_k8s"; exit 1; fi
	$(KUSTOMIZE) build ${TEMP_DIR}/deployment_k8s/k8s | kubectl delete -f -

undeploy_minikube: undeploy_vault_k8s ## Undeploy controller from the Minikube cluster specified in ~/.kube/config.
	if [ ! -d ${TEMP_DIR}/deployment_minikube ]; then echo "No deployment files found in .tmp/deployment_minikube"; exit 1; fi
	$(KUSTOMIZE) build ${TEMP_DIR}/deployment_minikube/k8s | kubectl delete -f -

undeploy_kcp:
	if [ ! -d ${TEMP_DIR}/deployment_kcp ]; then echo "No deployment files found in .tmp/deployment_kcp"; exit 1; fi
	$(KUSTOMIZE) build ${TEMP_DIR}/deployment_kcp/kcp | kubectl delete -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build ${TEMP_DIR}/deployment_default/default | kubectl delete -f -

deploy_vault_openshift:
	$(KUSTOMIZE) build config/vault/openshift | kubectl apply -f -
	POD_NAME=vault-0 NAMESPACE=spi-vault hack/vault-init.sh

undeploy_vault_openshift:
	$(KUSTOMIZE) build config/vault/openshift | kubectl delete -f -

deploy_vault_minikube:
	VAULT_HOST=vault.`minikube ip`.nip.io hack/replace_placeholders_and_deploy.sh "${KUSTOMIZE}" "vault_k8s" "vault/k8s"
	VAULT_NAMESPACE=spi-vault POD_NAME=vault-0 hack/vault-init.sh

undeploy_vault_k8s:
	$(KUSTOMIZE) build ${TEMP_DIR}/deployment_vault_k8s/vault/k8s | kubectl delete -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v4.4.1
CONTROLLER_TOOLS_VERSION ?= v0.9.2

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(SPIO_IMG)  && cd ../..
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(SPIO_BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push SPIO_IMG=$(SPIO_BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build SPIO_BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.3.0).
# These images MUST exist in a registry and be pull-able.
SPIO_BUNDLE_IMGS ?= $(SPIO_BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.3.0).
CATALOG_IMG ?= $(SPIO_IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $SPIO_BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(SPIO_BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push SPIO_IMG=$(CATALOG_IMG)

ensure-tmp:
	mkdir -p $(TEMP_DIR)
