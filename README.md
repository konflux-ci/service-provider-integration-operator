# SPI
[![Code Coverage Report](https://github.com/redhat-appstudio/service-provider-integration-operator/actions/workflows/codecov.yaml/badge.svg)](https://github.com/redhat-appstudio/service-provider-integration-operator/actions/workflows/codecov.yaml)
[![codecov](https://codecov.io/gh/redhat-appstudio/service-provider-integration-operator/branch/main/graph/badge.svg?token=EH16HO2RHP)](https://codecov.io/gh/redhat-appstudio/service-provider-integration-operator)

A Kubernetes controller/operator that manages service provider integration tasks.

## Table of contents

- [About](#About)
- [Architecture](#architecture)
- [Glossary](#glossary)
    * [User](#user)
    * [SPI Controller Manager](#spi-controller-manager)
    * [SPI OAuth Service](#spi-oauth-service)
    * [SPIAccessTokenBinding](#spiaccesstokenbinding)
    * [SPIAccessToken](#spiaccesstoken)
    * [SPIFileContentRequest](#spifilecontentrequest)
- [Administration Guide](docs/ADMIN.md) 
  - [Installation](docs/ADMIN.md#installation)
  - [Configuration](docs/ADMIN.md#configuration)
      + [Common configuration parameters](docs/ADMIN.md#common-configuration-parameters)
      * [Operator configuration parameters](docs/ADMIN.md#operator-configuration-parameters)
      * [OAuth service configuration parameters](docs/ADMIN.md#oauth-service-configuration-parameters)
  - [Vault](docs/ADMIN.md#vault)
  - [Running](docs/ADMIN.md#running)
- [User Guide](docs/USER.md)
- [Contributing](docs/DEVELOP.md)
  - [Building & Testing](docs/DEVELOP.md#building---testing)
      * [Out of cluster](docs/DEVELOP.md#out-of-cluster)
      * [In cluster](docs/DEVELOP.md#in-cluster)
  - [Debugging](docs/DEVELOP.md#debugging)
  - [Manual testing with custom images](docs/DEVELOP.md#manual-testing-with-custom-images)
      * [Requirements on the Service Providers](docs/DEVELOP.md#requirements-on-the-service-providers)
- [License](LICENSE)

## About
The Service Provider Integration aims to provide a service-provider-neutral way (as much as reasonable) of obtaining authentication tokens so that tools accessing the service provider do not have to deal with the intricacies of obtaining the access tokens from the different service providers.
The caller creates an `SPIAccessTokenBinding` object in a Kubernetes namespace where they specify the repository to which they require access and the required permissions
in the repo. The caller also specifies the target `Secret` to which they want the access token to the repository to be persisted.
The caller then watches the `SPIAccessTokenBinding` and reacts to its status changing by either initiating the provided OAuth flow or using the created Secret object.
Additionally, SPI provides an HTTP endpoint for manually uploading the access token for a certain `SPIAccessToken` object. Therefore,
the user doesn’t have to go through OAuth flow for tokens that they manually provide the access token for.

### Architecture
There are 2 main components. SPI HTTP API which is required for parts of the workflow that require direct user interaction and SPI CRDs (and a controller manager for them).
The custom resources are meant to be used by the eventual Consumers of the secrets that require access tokens to communicate with service providers.
Therefore, the main audience of SPI is the Consumers of the secrets. In the case of App Studio, this is most probably going to be HAS and/or HAC.
Because SPI requires user interaction for a part of its functionality, HAC will need to interface with the SPI REST API so that authenticated requests can be made to
the cluster and the service providers.
SPI will not provide any user-facing UI on its own.
Instead, for stuff that will need user interaction, it will provide links to its REST API that will either consume supplied data or redirect to a service provider (in case of OAuth flow).

## Glossary

### User
A user is understood to have identity in the Kubernetes cluster and has some RBAC rules associated with it that gives it permissions to create at least `SPIAccessTokenBinding` and `SPIAccessToken` CRs.
The user is also assumed to have identity in the service provider, such that there can be 1 or more service provider tokens associated with a single kubernetes identity.

### SPI Controller Manager
The controller manager is in charge of reconciling the `SPIAccessTokenBinding`,`SPIAccessToken`,`SPIAccessCheck`, `SPIFileContentRequest` CRs.

### SPI OAuth Service
The HTTP API of the OAuth Service is in charge of the parts of the workflow that require interaction with the user.
E.g. initiating the OAuth flow to obtain the user token data or receive tokens supplied by the caller manually.

### SPIAccessTokenBinding
A CR that is basically a request for storing access token for certain repository with certain permissions as a secret within some namespace.
SPI is not interested further in how such a secret is going to be consumed.

### SPIAccessToken
This CR represents the access token itself as a kubernetes object so that RBAC rules can be applied on it.
Importantly this CR does not contain the token data, which is stored in Vault. The CR only contains metadata required to identify the token.

### SPIAccessCheck
This CR represents a request to check repository accessibility.

### SPIFileContentRequest
This CR represents a request for content of the specific file in the given SCM repository.