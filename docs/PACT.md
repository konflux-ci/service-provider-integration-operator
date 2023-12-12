# SPI contract testing using Pact 

SPI is a participant in contract testing within RHTAP using [Pact](https://pact.io/) framework. It is a provider of an API that is used by HAC-dev. This documentation is about the specifics of provider testing on the SPI side. If you want to know more about contract testing using Pact, follow the [official documentation](https://docs.pact.io/). For more information about the RHTAP contract testing, follow the documentation in the [HAC-dev repo](https://github.com/openshift/hac-dev/blob/main/pactTests.md).

## Table of content
  - [When does a test run](#when-does-test-run)
  - [Implementation details](#implementation-details)
    - [Configuring Pact Provider](#configuring-pact-provider)
    - [Setting up a state](#setting-up-a-state)
  - [Adding a new verification](#adding-a-new-verification)
  - [Failing test](#failing-test)
    - [Pending pacts](#pending-pacts)
    - [Fixing the tests](#fixing-the-tests)

## When does a test run
Pact tests are triggered during different life phases of a product. See the table below for the details.

| Event       | What is checked | Pushed to Pact broker | Implemented |
|-------------|-----------------|-----------------------|-------------|
| Locally <br />`make pact` | Runs verification against <br />consumer "main" branch | No | Yes |
| Locally together with unit tests<br />`make test` | Runs verification against <br />consumer "main" branch | No | No |
| PR update   | Runs verification against consumer  <br />"main" branch and all environments | No* | Yes** [link](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/.github/workflows/pr.yml#L92) |
| PR merge | Runs verification against consumer "main" branch | Yes<br />commit SHA is a version <br />tagged by branch "main" | No |

\* The idea was to push those tags, but for now, nothing is pushed as we don't have access to the secrets from this GH action.

\*\* Currently, we don't have any environment specified. There should be a "staging" and "production" environment in the future. 

For more information, follow the [HAC-dev documentation](https://github.com/openshift/hac-dev/blob/main/pactTests.md) also with the [Gating](https://github.com/openshift/hac-dev/blob/main/pactTests.md#gating) chapter.


## Implementation details
Pact tests live in a `pact` package in the SPI repo. The main test file is a `pact_tests.go`. The Pact setup is done there, including the way to obtain the contracts. 

The place where the tests are executed is the [line](https://github.com/redhat-appstudio/service-provider-integration-operator/tree/main/pact/contracts/pact_test.go#L46) 
```
// Run pact tests
	err := provider.NewVerifier().VerifyProvider(t, verifyRequest)
```
Pact itself takes care of downloading the contracts, executing the tests, generating results, and (if configured) pushing them to the Pact broker.

### Configuring Pact Provider
Pact V2 provider allows users to configure a wide amount of testing specifications, such as where to get contracts, against which version the test should be run, if/where to push results, what are the State Handlers, etc. The `VerifyRequest` is the most important [type](https://pkg.go.dev/github.com/pact-foundation/pact-go/v2@v2.0.2/provider#VerifyRequest) that the Provider offers as it covers the majority of configuration.

By default, contracts are downloaded from the Pact broker. To do that, the `BrokerURL` and `ConsumerVersionSelectors` have to be set within the `Provider.VerifyRequest`. You can see an example [here](https://github.com/redhat-appstudio/service-provider-integration-operator/tree/main/pact/contracts/pact_test.go#L69-L70). Please note - with the current Pact broker setup, it is not needed to provider credentials for obtaining the contracts.

If you want to specify the contract file locally (e.g. when developing a new test), you can export the environment variable `LOCAL_PACT_FILES_FOLDER` and set it to the folder with your contracts. 

It is also important to specify against which consumer version the tests should run. In our case, we usually run against the `main` branch of a consumer, but it is possible to run against another branch/tag/environment. This may be useful for testing new contracts from the consumer branch before the consumer merges them.

For more information about the possible setup regarding obtaining contracts, follow the [official documentation](https://docs.pact.io/implementation_guides/go/readme#provider-verification). 

### Setting up a state
The definition of StateHandlers is one of the most important parts of the test setup. It defines the state of the provider before the request from the contract is executed. The string defining the state has to be the same as the `providerState` field of the contract. Methods implementing a state are extracted to the `pact_state_handlers_methods.go` file. More information about states and how they work can be found in the [official documentation](https://docs.pact.io/getting_started/provider_states).

## Adding a new verification
When a new contract is created, it would probably include a new state that is not implemented yet on the provider side. Although the Pact tests would probably fail because of the undefined state, an error message may be misleading. Instead of telling you that the state is not defined, the pact just skips the state and executes the request which is probably going to fail. You can see this message in the log:
```
[WARN] state handler not found for state: <new state>
[DEBUG] skipping state handler for request <request>

```

To implement the state, add the string from the contract to the `setupStateHandler` method in the `pact_states.go` file.
```
func setupStateHandler() models.StateHandlers {
	return models.StateHandlers{
		"Application exists":         createApp,}
}
```
Add implementation of this state (the `createApp` method in the example) to the `pact_state_handlers_methods.go`. That's it! 

Please note, that after each test, the cleanup is done by executing `cleanUpNamespaces` method from the `pact_state_handlers_methods.go` file. The method executes `removeAllInstances` for specified types and removes all instances of that type. If tests create a resource of type that is not in the list, please update the cleanup method to avoid future test failures. 

## Failing test
Pact tests should be running locally before the PR is made. So if any breaking change is done, a developer should be notified immediately. Pact usually provides enough information to debug and fix the issue. If you're not sure why the tests are failing, feel free to ping kfoniok for more help.

### Pending pacts
The Pending Pacts feature is turned on for this repository. When the new contract arrives, the tests fail and log the errors to the output, so it can be inspected properly by developers. But - the whole test suite is passing, as Pact knows it is a new contract without adequate implementation on the provider side. With Pending Pacts on, Pact tests fail the whole suite only for those contracts that were passing in the past.

### Fixing the tests
The preferred way to deal with the failing contract test is to fix the code change to make it compatible with the contract again. If it is not possible, then someone from the consumer (HAC-dev) team should be contacted to make them aware that the breaking change is coming. If you decide to push the code to the PR with the failing contract tests, it should fail the Pact PR check job there too.
Verification results are not pushed to the Pact broker until the PR is merged. (TBD) PR should be merged only when Pact tests are passing.