## Service Provider Integration Glossary

[User](#user)
[SPIController Manager](#spi-controller-manager) 
[SPI OAuth Service](#spi-oauth-service)
[SPIAccessTokenBinding](#spiaccesstokenbinding)
[SPIAccessToken](#spiaccesstoken)
[SPIFileContentRequest](#spifilecontentrequest)

### User
A user is understood to have identity in the Kubernetes cluster and has some RBAC rules associated with it that gives it permissions to create at least `SPIAccessTokenBinding` and `SPIAccessToken` CRs. 
The user is also assumed to have identity in the service provider, such that there can be 1 or more service provider tokens associated with a single kubernetes identity.

### SPI Controller Manager
The controller manager is in charge of reconciling the `SPIAccessTokenBinding` and `SPIAccessToken` CRs.

### SPI OAuth Service
The HTTP API of the OAuth Service is in charge of the parts of the workflow that require interaction with the user. 
E.g. initiating the OAuth flow to obtain the user token data or receive tokens supplied by the caller manually.

### SPIAccessTokenBinding
A CR that is basically a request for storing access token for certain repository with certain permissions as a secret within some namespace. 
SPI is not interested further in how such a secret is going to be consumed.

### SPIAccessToken
This CR represents the access token itself as a kubernetes object so that RBAC rules can be applied on it. 
Importantly this CR does not contain the token data, which is stored in Vault. The CR only contains metadata required to identify the token.

### SPIFileContentRequest
This CR represents a request for content of the specific file in the given SCM repository.
