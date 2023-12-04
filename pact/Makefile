ENVTEST_K8S_VERSION = latest

LOCALBIN ?= $(shell pwd)/../bin
ENVTEST ?= $(LOCALBIN)/setup-envtest

test: 
	make -C ../ -f Makefile manifests generate
	make test-pact

test-pact:
	unset SKIP_PACT_TESTS
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -count=1

test-verbose: 
	make -C ../ -f Makefile manifests generate install
	unset SKIP_PACT_TESTS
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -count=1 -v
