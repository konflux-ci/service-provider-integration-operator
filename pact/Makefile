ifndef PACT_BROKER_URL
	PACT_BROKER_URL = "https://pact-broker-hac-pact-broker.apps.hac-devsandbox.5unc.p1.openshiftapps.com"
endif

ENVTEST_K8S_VERSION = latest

LOCALBIN ?= $(shell pwd)/../bin
ENVTEST ?= $(LOCALBIN)/setup-envtest

# Main target that sets up the environment, install SPI and executes Pact contract tests.
# This target should be used within a package. For running tests from root, use root target `pact`.
test: 
	make -C ../ -f Makefile manifests generate envtest install
	make test-pact

# This target executes Pact contract tests. It expects the setup to be done before.
# If you're not executing test env setup before, consider using `test` or `test-verbose` targets instead.
# It is used e.g. during root `test` command where the setup is done and then unit and contract tests are executed.
test-pact:
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" PACT_BROKER_URL=${PACT_BROKER_URL} go test ./... -count=1

# This target sets up the environment, install SPI and runs Pact contract tests in a verbose mode.
test-verbose: 
	make -C ../ -f Makefile manifests generate envtest install
	GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=10s KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" PACT_BROKER_URL=${PACT_BROKER_URL} go test ./... -count=1 -v
