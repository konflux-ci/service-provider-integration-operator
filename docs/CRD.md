# Service Provider Integration CRDs

## Table of Contents

- [SPIAccessToken](#SPIAccessToken)
- [SPIAccessTokenBinding](#SPIAccessTokenBinding)
- [SPIAccessTokenDataUpdate](#SPIAccessTokenDataUpdate)
- [SPIAccessCheck](#SPIAccessCheck)
- [SPIFileContentRequest](#SPIFileContentRequest)

## SPIAccessToken
CRs of this CRD are used to represent an access token for some concrete “repository” or “repositories” in some service provider. The fact whether a certain token can give access to one or more repos is service-provider specific. 
The CRD only specifies data using which the controller manager can determine whether a certain token matches criteria specified in some `SPIAccessTokenBinding`. 
The sensitive data itself, i.e. the token (and possibly some other metadata like user id, etc) will be stored in Vault to provide an additional layer of security from unauthorized access. 
The CRs can be thought of as just mere proxies of the data itself in the Kubernetes cluster. The main reason for wanting to have them is to be able to enforce the RBAC rules.

### Required Fields

| Name                              | Type     | Description                                                                                     | Example                                                                                             | Immutable |
|-----------------------------------|----------|-------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|-----------|
| spec.permissions                  | object   | The list of permissions granted by this token.                                                  | {“required”: [{“type”: “rw”, “area”: “webhooks”}], “additionalScopes: [“repo:read”, “hooks:admin”]} | true      |
| spec.permissions.required         | []       | The list of permissions expressed in the SPI-specific way                                       | [{“type”: “rw”, “area”: “webhooks”}]                                                                | true      |
| spec.permissions.required[].type  | enum     | “r”, “w” or “rw” (read, write or read and write)                                                | “rw”                                                                                                | true      |
| spec.permissions.required[].area  | enum     | “repository”, “webhooks” or “user” - the area for which the permission applies                  | “repository”                                                                                        | true      |
| spec.permissions.additionalScopes | []string | The list of scopes (as understood by the service provider) that the token should have           | [“repo:read”, “hooks:admin”]                                                                        | true      |
| spec.serviceProviderUrl           | string   | The base URL of the service provider. This should also be a label on the CRs for faster search. | https://github.com                                                                                  | true      |



### Optional Fields

| Name                 | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Example                         | Immutable |
|----------------------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------|-----------|
| status.phase         | enum   | This can be “AwaitingTokenData” or “Ready”, “Invalid” or “Error”. AwaitingTokenData - initial state before the token data is supplied to the object either through OAuth flow or through manual upload. “Ready” - the token data exists, so the object is ready for matching with the bindings. Invalid - this can happen with manually uploaded token data - the controller was unable to read  metadata about the token from the service provider. Error - there was some other error when reconciling the token | "Ready"                         | false     |
| status.errorReason   | enum   | “UnknownServiceProvider” or “MetadataFailure”. UnknownServiceProvider - the controller was unable to deduce the service provider from spec.serviceProviderUrl. MetadataFailure - the controller failed to reconcile the metadata of the token.                                                                                                                                                                                                                                                                     | “MetadataFailure”               | false     |
| status.errorMessage  | string | The details of the error                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | “failed to update the metadata” | false     |
| status.oauthUrl      | string | When the phase is “AwaitingTokenData” this field contains the URL for initiating the OAuth flow.                                                                                                                                                                                                                                                                                                                                                                                                                   |                                 | false     |
| status.uploadUrl     | string | URL for manual upload token data                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |                                 | true      |
| status.tokenMetadata | object | The metadata that the controller learned about the token. Nil if the token data is not available yet. This is used internally by the controller and shouldn't be of interest to other parties.                                                                                                                                                                                                                                                                                                                     |                                 | false     |



## SPIAccessTokenBinding
When a 3rd party application requires access to a token, it creates an `SPIAccessTokenBinding` object that instructs SPI operator to look up the `SPIAccessToken` matching the criteria provided in the binding and persist the token in the configured secret.

### Required Fields

| Name                              | Type     | Description                                                                                                                                                                                                                | Example                                                                                             | Immutable |
|-----------------------------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|-----------|
| spec.repoUrl                      | string   | The URL to the repository for which the token should be obtained. The operator uses this information to deduce the service provider and try to match the existing access tokens with it in service-provider-specific ways. | https://github.com/acme/app                                                                         | false     |
| spec.permissions                  | object   | The list of permissions required by this binding.                                                                                                                                                                          | {“required”: [{“type”: “rw”, “area”: “webhooks”}], “additionalScopes: [“repo:read”, “hooks:admin”]} | true      |
| spec.permissions.required         | object[] | The list of permissions expressed in the SPI-specific way                                                                                                                                                                  | [{“type”: “rw”, “area”: “webhooks”}]                                                                | true      |
| spec.permissions.required[].type  | enum     | “r”, “w” or “rw” (read, write or read and write)                                                                                                                                                                           | “rw”                                                                                                | true      |
| spec.permissions.required[].area  | enum     | “repository”, “webhooks” or “user” - the area for which the permission applies                                                                                                                                             | “repository”                                                                                        | true      |
| spec.permissions.additionalScopes | string[] | The list of scopes (as understood by the service provider) that the token should have. Optional.                                                                                                                           | [“repo:read”, “hooks:admin”]                                                                        | true      |


### Optional Fields

| Name                                       | Type              | Description                                                                                                                                                                         | Example              | Immutable |
|--------------------------------------------|-------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------|-----------|
| spec.secret.name                           | string            | The name of the secret that should contain the token data once the data is available. If not specified, a random name is used.                                                      |                      | true      |
| spec.secret.labels                         | map[string]string | The labels to be put on the created secret                                                                                                                                          | acme.com/for=app1    | false     |
| spec.secret.annotations                    | map[string]string | The annotations to be put on the created secret                                                                                                                                     |                      | false     |
| spec.secret.type                           | enum              | The type of the secret created as specified by Kubernetes (e.g. “Opaque”, “kubernetes.io/basic-auth”, …)                                                                            |                      | false     |
| spec.secret.fields.token                   | string            | The name of the key in the secret for the access token.                                                                                                                             | “access_token”       | false     |
| spec.secret.fields.name                    | string            | The name of the key in the secret where the name of the token object should be stored.                                                                                              | “spiAccessTokenName” | true      |
| spec.secret.fields.serviceProviderUrl      | string            | The key for the URL of the service provider that the token was obtained from.                                                                                                       | REPO_HOST            | false     |
| spec.secret.fields.serviceProviderUserName | string            | The key for the user name in the service provider that the token was obtained from.                                                                                                 | GITHUB_USERNAME      | false     |
| spec.secret.fields.serviceProviderUserId   | string            | The key for the user id of the service provider that the token was obtained from.                                                                                                   | GITHUB_USERID        | false     |
| spec.secret.fields.userId                  | string            | The key for the userId of the App Studio user that created the token in the SPI.                                                                                                    | K8S_USER             | false     |
| spec.secret.fields.expiredAfter            | string            | The key for the token expiry date.                                                                                                                                                  | TOKEN_VALID_UNTIL    | false     |
| spec.secret.fields.scopes                  | string            | The key for the comma-separated list of scopes that the token is valid for in the service provider                                                                                  | GITHUB_SCOPES        | false     |
| status.phase                               | enum              | One of AwaitingTokenData, Injected, Error                                                                                                                                           |                      | false     |
| status.errorReason                         | enum              | Detailed error reason                                                                                                                                                               |                      | false     |
| status.errorMessage                        | string            | Error message if phase==Error                                                                                                                                                       |                      | false     |
| status.linkedAccessTokenName               | string            | The name of the linked SPIAccessToken object                                                                                                                                        |                      | false     |
| status.oauthUrl                            | string            | When the phase is “AwaitingTokenData” this field contains the URL for initiating the OAuth flow.                                                                                    |                      | false     |
| status.uploadUrl                           | string            | URL for manual upload token data                                                                                                                                                    |                      | true      |
| status.syncedObjectRef.name                | string            | The name of the secret that contains the data of the bound token. Empty if the token is not bound (the phase is AwaitingTokenData). If not empty, this should be identical to spec. |                      | false     |


## SPIAccessTokenDataUpdate
This is a more or less internal CRD that is used to inform the controller that an update has been made to the token data in the token storage in Vault (which lives outside of the cluster). These objects are created when completing the OAuth flow or during manual upload of a token. As such, it is required that the end-user has permissions to create these objects.

These objects are automatically deleted by the controller as soon as they are processed.

### Required Fields

| Name           | Type   | Description                                                                                       | Example | Immutable |
|----------------|--------|---------------------------------------------------------------------------------------------------|---------|-----------|
| spec.tokenName | string | The name of the SPIAccessToken object in the same namespace as this object that has been updated. |         | true      |



## SPIAccessCheck
This is basically a procedure call to check repository accessibility. Once a user creates a CR, the controller analyzes whether the repository is accessible for the user. In short, if the repository is public or the user has SPIAccessToken for this repository, the repository is accessible. In other cases (private without SPIAccessToken or does not exist), it is not accessible.
CRs are automatically deleted by the controller after some period of time (30 min by default).

### Required Fields

| Name         | Type   | Description                                  | Example | Immutable |
|--------------|--------|----------------------------------------------|---------|-----------|
| spec.repoUrl | string | URL of the repository to check accessibility |         | true      |


### Optional Fields

| Name                   | Type   | Description                                                                                                                     | Example                                                                                             | Immutable |
|------------------------|--------|---------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|-----------|
| spec.permissions       | object | The list of permissions required to this repository be considered accessible. It is used when finding matching SPIAccessTokens. | {“required”: [{“type”: “rw”, “area”: “webhooks”}], “additionalScopes: [“repo:read”, “hooks:admin”]} | true      |
| status.accessible      | bool   | Public repositories or private with existing SPIAccessToken will have this `true`. Otherwise `false`.                           |                                                                                                     | true      |
| status.accessibility   | enum   | private, public or unknown                                                                                                      |                                                                                                     | true      |
| status.type            | enum   | git                                                                                                                             |                                                                                                     | true      |
| status.serviceProvider | enum   | GitHub or Quay                                                                                                                  |                                                                                                     | true      |
| errorReason            | enum   | Detailed error reason                                                                                                           |                                                                                                     | false     |
| errorMessage           | string | Additional error message. Usually taken from a go error.                                                                        |                                                                                                     | false     |


## SPIFileContentRequest
Instances of this CRD are used to request specific file contents from the SCM repository.
After creation, it successively creates an `SPIAccessTokenBinding` and waits until it is injected.
After that it tries to read the file from the repository using the credentials obtained from secret linked with binding.
As per now, `SPIFileContentRequests` are one-time objects, that means they reflect the file content only shortly after the moment of their creation, and never tries to update or check content availability later.


### Required Fields

| Name          | Type   | Description                                                      | Example                     | Immutable |
|---------------|--------|------------------------------------------------------------------|-----------------------------|-----------|
| spec.repoUrl  | string | The URL to the repository for which the file should be obtained. | https://github.com/acme/app | true      |
| spec.filePath | string | Path to the desired file inside the repository.                  | foo/bar.txt                 | true      |


### Optional Fields

| Name                     | Type   | Description                                                                                                                                                                                                                                                                | Example                                                           | Immutable |
|--------------------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------|-----------|
| status.phase             | enum   | This can be “AwaitingTokenData” or “Delivered” or “Error”. AwaitingTokenData - initial state before the token data is supplied to the subsequent token Token object either through OAuth flow or through manual upload. “Delivered” - the file data successfully injected. |                                                                   | false     |
| status.errorMessage      | string | The details of the error                                                                                                                                                                                                                                                   | “failed to update the metadata”                                   | false     |
| status.linkedBindingName | string | name of the SPIToken Binding used for repository authentication                                                                                                                                                                                                            |                                                                   | true      |
| status.oauthUrl          | string | URL for initiating the OAuth flow copied from linked SPITokenBinding for convenience                                                                                                                                                                                       | https://spi-rest-api.acme.com/authenticate?state=kjfalasjdfl348fj | false     |
| status.uploadUrl         | string | URL for manual upload token data copied from linked SPITokenBinding for convenience                                                                                                                                                                                        |                                                                   | true      |
| status.content           | string | Encoded requested file content                                                                                                                                                                                                                                             |                                                                   | true      |
| status.contentEncoding   | string | Encoding used for file content encoding                                                                                                                                                                                                                                    | base64                                                            | true      |






