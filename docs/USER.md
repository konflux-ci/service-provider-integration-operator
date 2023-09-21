In this Manual we consider the main SPI use cases as well as give SPI API references for more advanced cases.    

## Table of Contents
- [Use Cases](#use-cases)
    - [Accessing the private repository](#accessing-the-private-repository) - TODO
    - [Checking permission to the particular repository](#checking-permission-to-the-particular-repository) - TODO
    - [Creating SPIAccessTokenBinding with Secret type kubernetes.io/dockerconfigjson](#creating-spiaccesstokenbinding-with-secret-type-kubernetesiodockerconfigjson)
    - [Retrieving file content from SCM repository](#retrieving-file-content-from-scm-repository)
    - [Storing username and password credentials for any provider by it's URL](#storing-username-and-password-credentials-for-any-provider-by-its-url)
    - [Uploading Access Token to SPI using Kubernetes Secret](#uploading-access-token-to-spi-using-kubernetes-secret)
    - [Providing secrets to a service account](#providing-secrets-to-a-service-account)
    - [Refreshing OAuth Access Tokens](#refreshing-oauth-access-tokens)

- [SPI OAuth Service](#spi-oauth-service)
    - [Go through the OAuth flow manually](#go-through-the-oauth-flow-manually)
    - [User Service Provider configuration](#user-service-provider-configuration)
- [Supported Service Providers](#supported-service-providers)
- [Service Provider Integration Kubernetes API (CRDs)](#service-provider-integration-kubernetes-api-crds)
    - [SPIAccessToken](#SPIAccessToken)
    - [SPIAccessTokenBinding](#SPIAccessTokenBinding)
    - [SPIAccessTokenDataUpdate](#SPIAccessTokenDataUpdate)
    - [SPIAccessCheck](#SPIAccessCheck)
    - [SPIFileContentRequest](#SPIFileContentRequest)
- [HTTP API Endpoints](#http-api-endpoints)
    - [POST /login](#post-login)
    - [GET {sp_type}/authenticate](#get-sp_typeauthenticate)
    - [GET /{sp_type}/callback](#get-sp_typecallback)
    - [POST /token/{namespace}/{name}](#post-tokennamespacename)
- [SnapshotEnvironmentBinding Controller](#SnapshotEnvironmentBinding-Controller)

# Use Cases

## Accessing the private repository

TODO

## Checking permission to the particular repository

TODO

## Creating SPIAccessTokenBinding with Secret type kubernetes.io/dockerconfigjson
An SPIAccessTokenBinding with Secret type kubernetes.io/dockerconfigjson is used when the user requires a Secret that 
enables Pods to pull images from private image registry to Kubernetes. This Secret contains so-called
Docker config.json. Kubernetes' interpretation of config.json differs from Docker's in that Kubernetes is more permissive.
That is why SPI provides options to alter the content of config.json.
You can read more about this topic [here](https://kubernetes.io/docs/concepts/containers/images/#config-json).

Users can configure the specific value of `_host_` (see example json bellow) in config.json through two annotations which can be set
in the SPIAccessTokenBinding's `spec.secret.annotations` field.

### Annotation 'spi.appstudio.redhat.com/config-json-type'

This annotation can have three values: `docker`, `kubernetes`, and `explicit`.

#### 'docker'
Specifying `docker` is the same as
not specifying any annotation at all. In this case, SPI creates the Secret with config.json in this format where the 
value of `_host_` is the URL host of the specific service provider, e.g. `quay.io` for Quay.:
```json
{
    "auths": {
        "_host_": {
            "auth": "dXNlcm5hbWU6dG9rZW4xMjM=" // base64 encoded username:token123
        }
    }
}
```
#### 'kubernetes'

Setting the annotation value to`kubernetes` means that the `_host_` will be filled with URL host and path from the
`repoUrl` of SPIAccessTokenBinding. For example Binding with this spec (ignoring other fields for simplicity):
```yaml
spec:
  repoUrl: https://quay.io/repo/spi-test
  secret:
    type: kubernetes.io/dockerconfigjson
    annotations:
      spi.appstudio.redhat.com/config-json-type: kubernetes
```
will produce this json:
```json
{
    "auths": {
        "quay.io/repo/spi-test": {
            "auth": "dXNlcm5hbWU6dG9rZW4xMjM="
        }
    }
}
```
For concrete example SPIAccessTokenBinding can take a look at this [sample](../samples/binding-kubetype-dockerconfigjson.yaml).

WARNING: Avoid using the `kubernetes` value when your SPIAccessTokenBinding's `repoUrl` contains a tag of an image, as the tag
will also be present in the `_host_` and Kubernetes may not be able to pull the image with such config.json.

#### 'explicit' and annotation spi.appstudio.redhat.com/config-json-auth-key
This is where the second annotation comes into play.
Adding the annotation `spi.appstudio.redhat.com/config-json-type: explicit` means that the `_host_` will be filled with 
explicit value specified in the `spi.appstudio.redhat.com/config-json-auth-key` annotation.

  For example Binding with this spec (ignoring other fields for simplicity):
```yaml
spec:
  secret:
    type: kubernetes.io/dockerconfigjson
    annotations:
      spi.appstudio.redhat.com/config-json-type: explicit
      spi.appstudio.redhat.com/config-json-auth-key: my.custom.host/test
```
will produce this json:
```json
{
    "auths": {
        "my.custom.host/test": {
            "auth": "dXNlcm5hbWU6dG9rZW4xMjM="
        }
    }
}
```

## Retrieving file content from SCM repository
There is dedicated controller for file content requests, which can be performed by putting
a `SPIFileContentRequest` CR in the namespace, as follows:

```
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIFileContentRequest
metadata:
  name: test-file-content-request
  namespace: default
spec:
  repoUrl: https://github.com/redhat-appstudio/service-provider-integration-operator
  filePath: hack/boilerplate.go.txt

```
Controller then generates SPIAccessTokenBinding and waiting for it to be ready/injected.
Once binding became ready, controller fetches requested content using credentials from injected secret. A successful attempt will result in base64 encoded file content
appearing in the `status.content` field on the `SPIFileContentRequest` CR.
```
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIFileContentRequest
metadata:
  name: test-file-content-request
  namespace: default
spec:
  filePath: hack/boilerplate.go.txt
  repoUrl: https://github.com/redhat-appstudio/service-provider-integration-operator
status:
  content: LyoKQ29weXJpZ2h0IDIw....==
  contentEncoding: base64
  linkedBindingName: ""
  phase: Delivered
```


At this stage, file request CR-s are intended to be single-used, so no further content refresh
or accessibility checks must be expected. A new CR instance should be used to re-request the content.

Currently, the file retrievals are limited to GitHub & GitLab repositories only, and files size up to 2 Megabytes.
Default lifetime for file content requests is 30 min and can be changed via operator configuration parameter.

## Storing username and password credentials for any provider by it's URL

It is now possible to store username + token/password credentials for nearly any
provider that may offer them (like Snyk.io etc). It is can be done in a few steps:

- Create simple SPIAccessTokenBinding, indicating provider-s URL only:
```
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIAccessTokenBinding
metadata:
 name: test-access-token-binding
 namespace: default
spec:
 repoUrl: https://snyk.io/
 secret:
   type: kubernetes.io/basic-auth
```
and push it into namespace:
```
kubectl apply -f samples/spiaccesstokenbinding.yaml --as system:serviceaccount:default:default
```

- Determine the SPIAccessTokenName:
 ```
SPI_ACCESS_TOKEN=$(kubectl get spiaccesstokenbindings test-access-token-binding -o=jsonpath='{.status.linkedAccessTokenName}')
```


- Perform manual push of the credential data:
```
curl -k -v -X POST -H "Content-Type: application/json" -H "Authorization: Bearer $(hack/get-default-sa-token.sh)" -d'{"username": "userfoo", "access_token": "4R28N79MT"}' "https://<cluster-host-or-ip>/token/default/$SPI_ACCESS_TOKEN"
```

Note that both `username` and `access_token` fields are mandatory.
`204` response indicates that credentials successfully uploaded and stored.
Please note that SPI service didn't perform any specific validity checks
for the user provided credentials, so it's assumed user provides a correct data.

## Overriding default binding lifetime
In general, bindings (and their dependent secrets) are not supposed to be long-lived. Their TTL is configured globally, 
and defaults to 2 hrs. Some tasks may require the credentials to be available for a longer time, so the creator of the binding
may override the default setting by specifying `spec.lifetime` field of the binding.
It accepts any standard time periods, like `2h30m`, `90s` etc. The negative values and values less than
60 seconds are ignored, since they did not make sense. The only exception is a `-1` value, which means infinite lifetime
of the binding.

```
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIAccessTokenBinding
metadata:
  name: test-binding
  namespace: default
spec:
  repoUrl: https://github.com/redhat-appstudio/service-provider-integration-operator
  lifetime: '-1'
```


## Uploading Access Token to SPI using Kubernetes Secret

There is an ability to upload Personal Access Token using very short living K8s Secret.
Controller recognizes the Secret by `label.spi.appstudio.redhat.com/upload-secret: token` label, gets the PAT and the Name of the SPIAccessToken associated with it, then deletes the Secret (for better security reason) and uploads the Token (which in turn updates the Status of associated SPIAccess/Token/TokenBinding).

- If you want to associate it to existed SPIAccessToken, find the name of SPIAccessToken you want to associate Access Token to like:
`TOKEN_NAME=$(kubectl get spiaccesstokenbinding/$Name-Of-SPIAccessTokenBinding -n $TARGET_NAMESPACE -o  json | jq -r .status.linkedAccessTokenName)`
- Obtain your Access Token data ($AT_DATA) from the Service Provider. For example from GitHub->Settings->Developer Settings->Personal Access Tokens
- Create Kubernetes Secret with labeled with `spi.appstudio.redhat.com/upload-secret: token` and `spi.appstudio.redhat.com/token-name: $TOKEN_NAME`:
- Field `userName` can be used to set concrete provider's username that should be associated with token.
- If you want new SPIAccessToken to be created and associate it with the Token, so the SPIAccessToken will be Ready right away, put the name of (non-existed) SPIAccessToken you want to be assigned to the `stringData.spiTokenName`  or do not have this field, in the latter case SPIAccessToken name will be randomly generated. In this case make sure you added `providerUrl` and point it to the valid, registered `URL_PROVIDER` 

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: $UPLOAD-SECRET_NAME
  labels:
    spi.appstudio.redhat.com/upload-secret: token
type: Opaque
stringData:
  spiTokenName: $TOKEN_NAME
  providerUrl: $PROVIDER_URL
  userName: $USER_NAME
  tokenData: $AT_DATA
```
After reconciliation SPIAccessToken should be filled with the Access Token metadata, it's `status.Phase` should be `Injected` and upload Secret is removed.

### Error Handling
Since the Secret is removed in any case (since point is to make the Secret live time as short as possible), there are no chance to read any status and error information we may put in it.    
So, in a case if something goes wrong the reason is written to K8s Event named with Secret name, user can can check it with `kubectl get event $UPLOAD-SECRET_NAME`.
This event is refreshed or deleted after next creation of some-named Secret or normally by Kubernetes (in 60 minutes by default)    

## Providing secrets to a service account

The access token binding (the SPIAccessTokenBinding) can optionally specify a service account(s) that the secret containing 
the credentials obtained using the binding should be linked with. This can be used to inject additional secrets to 
an existing service accounts or to create a new service accounts that should contain the secrets. The lifecycle of 
the service accounts can both be managed by the binding (the service account is created by the operator and deleted along 
with the binding) or can predate and outlive the binding.

It is possible to provide both secrets and image pull secrets to the service account.

### Linking a secret to a pre-existing service account

Using the following definition of the binding one can add a binding secret to a pre-existing service account. Note that
if the service account with the provided name doesn't exist, it is automatically created but IS NOT deleted with 
the binding (i.e. such secret outlives the binding).

```yaml
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIAccessTokenBinding
...
spec:
  secret:
    type: kubernetes.io/dockerconfigjson
    linkedTo:
    - serviceAccount:
        reference:
            name: mysa
  ...
```

Note that the secret is merely added to the list of the secrets the service account is linked to, not overwriting any 
pre-existing links. When the binding and its secret is deleted, the secret is also removed from the list of the secrets
in the service account.

### Linking a secret to a pre-existing service account as image pull secret

To use the binding secret as an image pull secret for the service account, one needs to provide the definition of 
the service account explicitly requesting linking as an image pull secret. The secret needs to have
the `dockerconfigjson` type.

```yaml
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIAccessTokenBinding
...
spec:
  secret:
    type: kubernetes.io/dockerconfigjson
    linkedTo:
    - serviceAccount:
        as: imagePullSecret
        reference:
            name: mysa
  ...
```

This also merely adds the secret to list of linked image pull secrets of the service account not overwriting any 
pre-existing ones.

### Using a managed service account

If the caller doesn't have a service account available and doesn't need the service account after the binding and 
the secret are "consumed", it is possible to mark the service account as managed. Such service accounts will be created
by the SPI operator together with the binding secret and will be deleted automatically along with the binding. To avoid
naming conflicts with pre-existing service accounts in the namespace, it is advised to use a randomized name (using
the `generateName` field).

```yaml
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIAccessTokenBinding
...
spec:
  secret:
    type: kubernetes.io/basic-auth
    linkedTo:
    - serviceAccount:
        managed:
            generateName: mysa-
  ...
```
## Refreshing OAuth Access Tokens
Supported tokens: Gitlab OAuth access tokens

If a service provider issues OAuth access tokens together with a `refresh token` and an `expiry` time, such access tokens need to be refreshed after this time.
The user can manually request a token refresh by adding the label `spi.appstudio.redhat.com/refresh-token: true` to
SPIAccessToken after the token is in the `ready` phase.

While the token is being refreshed, there might be a slight period during which the access token injected by a
SPIAccessTokenBinding (linked to SPIAccessToken) is invalid.

# Service Provider Integration OAuth Service

OAuth2 protocol is the most commonly used way that allows users to authorize applications to communicate with service providers.
`spi-oauth` to use this protocol to obtain service provider’s access tokens without the need for the user to provide us his login credentials.


This OAuth2 microservice would be responsible for:
- Initial redirection to the service provider
- Callback from the service provider
- Persistence of access token that was received from  the service provider into the permanent backend (k8s secrets or Vault)
- Handling of negative authorization and error codes
- Creation or update of SPIAccessToken
- Successful redirection at the end

Also, this service provides an HTTP API to support manual upload of tokens for service providers that have the capability to manually generate individual tokens. Like GitHub's personal access tokens or Quay's robo-accounts.

## Go through the OAuth flow manually

Let's assume that a) we're using the default namespace and that b) the default service account has permissions
to CRUD `SPIAccessToken`s and `SPIAccessTokenBinding`s in the default namespace.

This can be ensured by running:

```
kubectl apply -f hack/give-default-sa-perms-for-accesstokens.yaml
```

Let's create the `SPIAccessTokenBinding`:
```
kubectl apply -f samples/spiaccesstokenbinding.yaml --as system:serviceaccount:default:default
```

Now, we need initiate the OAuth flow. We've already created the SPI access token binding. Let's examine
it to see what access token it bound to:

```
SPI_ACCESS_TOKEN=$(kubectl get spiaccesstokenbindings test-access-token-binding -o=jsonpath='{.status.linkedAccessTokenName}')
```

Let's see if it has OAuth URL to go through:

```
OAUTH_URL=$(kubectl get spiaccesstoken $SPI_ACCESS_TOKEN -o=jsonpath='{.status.oAuthUrl}') 
```

Now let's use the bearer token of the default service account to authenticate with the OAuth service endpoint:

```
curl -v -k $OAUTH_URL 2>&1 | grep 'location: ' | cut -d' ' -f3
```

This gave us the link to the actual service provider (github) that we can use in the browser to approve and finish
the OAuth flow.

Upon returning from that OAuth flow, you should end up back on the SPI endpoint on the `.../oauth/callback` URL.
This concludes the flow and you should be able to see the secret configured in the binding with the requested data:

```
kubectl get secret token-secret -o yaml
```


# Supported Service Providers

[SPIAccessTokenBinding](#SPIAccessTokenBinding) uses permission area and permission type to infer provider-specific OAuth scopes
that the linked [SPIAccessToken](#SPIAccessToken) should have. However, each provider offers a specific set of services and OAuth scopes. Thus,
different permission areas are supported.

| Provider | Type  |Supported Token kinds   | Supported permission areas                      |
|----------|-------|------------------------|-------------------------------------------------|
| GitHub   | Git   | OAuth token, PAT*      |repository, repositoryMetadata, webhooks, user   |
| GitLab   | Git   | OAuth token, PAT       |repository, repositoryMetadata, user (read-only) |
| Quay     | Docker| Oauth, Robot account   |registry, registryMetadata                       |
| Snyk**   |  -    |Username/Password(Token)| -                                               |

* PAT - Personal Access Token
** In case of Snyk and other providers that do not support OAuth, the permission area does not matter.

## User Service Provider configuration

In situations when Service Provider configuration of SPI does not fit user's use case (like on-prem installations), one may define their own service provider configuration using a Kubernetes secret:
```yaml
...
metadata:
  labels:
    spi.appstudio.redhat.com/service-provider-type: GitHub
data:
  clientId: ...
  clientSecret: ...
  authUrl: ...
  tokenUrl: ...

```
Such secret must have label `spi.appstudio.redhat.com/service-provider-type` with value of one of our supported service provider's name (`GitHub`, `Quay`, `GitLab`).
Secret data can contain keys from template above or can be empty. If both `clientId` and `clientSecret` are set, we consider it as valid OAuth configuration and will generate OAuth URL in matching `SPIAccessTokens`. In other cases, we won't generate OAuth URL. User can always use manual token upload.

The secret must live in same namespace as `SPIAccessToken`. If matching secret is found, it is always used over SPI configuration. If format of the user's oauth configuration secret is not valid, oauth flow will fail with a descriptive error.

# Service Provider Integration Kubernetes API (CRDs)

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

| Name                              | Type     | Description                                                                                                                                                                                                                                                                                                                                     | Example                                                                                             | Immutable |
|-----------------------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|-----------|
| spec.repoUrl                      | string   | The URL to the repository for which the token should be obtained. The operator uses this information to deduce the service provider and try to match the existing access tokens with it in service-provider-specific ways. If the URL does not contain scheme, “https” scheme is assumed and the object is updated with URL having this scheme. | https://github.com/acme/app                                                                         | false     |
| spec.permissions                  | object   | The list of permissions required by this binding.                                                                                                                                                                                                                                                                                               | {“required”: [{“type”: “rw”, “area”: “webhooks”}], “additionalScopes: [“repo:read”, “hooks:admin”]} | true      |
| spec.permissions.required         | object[] | The list of permissions expressed in the SPI-specific way                                                                                                                                                                                                                                                                                       | [{“type”: “rw”, “area”: “webhooks”}]                                                                | true      |
| spec.permissions.required[].type  | enum     | “r”, “w” or “rw” (read, write or read and write)                                                                                                                                                                                                                                                                                                | “rw”                                                                                                | true      |
| spec.permissions.required[].area  | enum     | “repository”, “webhooks” or “user” - the area for which the permission applies                                                                                                                                                                                                                                                                  | “repository”                                                                                        | true      |
| spec.permissions.additionalScopes | string[] | The list of scopes (as understood by the service provider) that the token should have. Optional.                                                                                                                                                                                                                                                | [“repo:read”, “hooks:admin”]                                                                        | true      |


### Optional Fields

| Name                                                       | Type              | Description                                                                                                                                                                         | Example              | Immutable |
|------------------------------------------------------------|-------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------|-----------|
| spec.lifetime                                              | string            | Expected lifetime for given binging, which overrides default cluster-wide setting                                                                                                   | 5h10s,  '-1'         | false     |
| spec.secret.name                                           | string            | The name of the secret that should contain the token data once the data is available. If not specified, a random name is used.                                                      |                      | true      |
| spec.secret.labels                                         | map[string]string | The labels to be put on the created secret                                                                                                                                          | acme.com/for=app1    | false     |
| spec.secret.annotations                                    | map[string]string | The annotations to be put on the created secret                                                                                                                                     |                      | false     |
| spec.secret.type                                           | enum              | The type of the secret created as specified by Kubernetes. If type is not defined or is "Opaque" data is written to the "token" field or use Secret's FieldMapping to fill the data.|                      | false     |
| spec.secret.fields.token                                   | string            | The name of the key in the secret for the access token.                                                                                                                             | “access_token”       | false     |
| spec.secret.fields.name                                    | string            | The name of the key in the secret where the name of the token object should be stored.                                                                                              | “spiAccessTokenName” | true      |
| spec.secret.fields.serviceProviderUrl                      | string            | The key for the URL of the service provider that the token was obtained from.                                                                                                       | REPO_HOST            | false     |
| spec.secret.fields.serviceProviderUserName                 | string            | The key for the user name in the service provider that the token was obtained from.                                                                                                 | GITHUB_USERNAME      | false     |
| spec.secret.fields.serviceProviderUserId                   | string            | The key for the user id of the service provider that the token was obtained from.                                                                                                   | GITHUB_USERID        | false     |
| spec.secret.fields.userId                                  | string            | The key for the userId of the App Studio user that created the token in the SPI.                                                                                                    | K8S_USER             | false     |
| spec.secret.fields.expiredAfter                            | string            | The key for the token expiry date.                                                                                                                                                  | TOKEN_VALID_UNTIL    | false     |
| spec.secret.fields.scopes                                  | string            | The key for the comma-separated list of scopes that the token is valid for in the service provider                                                                                  | GITHUB_SCOPES        | false     |
| spec.secret.linkedTo                                       | array             | the list of objects the secret should be linked to. Currently only service accounts are supported.
| spec.secret.linkedTo[].serviceAccount.as                   | string            | specifies how the secret generated by the binding is linked to the service account. This can be either `secret` meaning that the secret is listed as one of the mountable secrets in the `secrets` of the service account, or `imagePullSecret` which makes the secret listed as one of the image pull secrets associated with the service account. Defaults to `secret`.
| spec.secret.linkedTo[].serviceAccount.reference.name       | string            | the name of the pre-existing service account to link with the secret.
| spec secret.linkedTo[].serviceAccount.managed.name         | string            | the name of the managed service account to link with the secret.
| spec.secret.linkedTo[].serviceAccount.managed.generateName | string            | the generate name of the service account to link with the secret. This is preferable to using `name` because it ensures that no naming conflicts will arise.
| spec.secret.linkedTo[].serviceAccount.managed.labels       | map[string]string | the additional labels to put on the service account.
| spec.secret.linkedTo[].serviceAccount.managed.annotations  | map[string]string | the additional annotations to put on the service account.
| status.phase                                               | enum              | One of AwaitingTokenData, Injected, Error                                                                                                                                           |                      | false     |
| status.errorReason                                         | enum              | Detailed error reason                                                                                                                                                               |                      | false     |
| status.errorMessage                                        | string            | Error message if phase==Error                                                                                                                                                       |                      | false     |
| status.linkedAccessTokenName                               | string            | The name of the linked SPIAccessToken object                                                                                                                                        |                      | false     |
| status.oauthUrl                                            | string            | When the phase is “AwaitingTokenData” this field contains the URL for initiating the OAuth flow.                                                                                    |                      | false     |
| status.uploadUrl                                           | string            | URL for manual upload token data                                                                                                                                                    |                      | true      |
| status.syncedObjectRef.name                                | string            | The name of the secret that contains the data of the bound token. Empty if the token is not bound (the phase is AwaitingTokenData). If not empty, this should be identical to spec. |                      | false     |


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

### SPIAccessCheck and RemoteSecrets
The original way SPIAccessCheck tried to check access to a private repository (registry) was to find a matching
SPIAccessToken and use the credentials from it to access the repository.

We have extended the functionality so that if no matching SPIAccessToken is found, it searches for a matching RemoteSecret
next (read more about RemoteSecrets [here](https://github.com/redhat-appstudio/remote-secret)).
However, there are a few constraints put on the RemoteSecret before SPIAccessCheck can use the credentials from it:
- It must be in the same namespace as SPIAccessCheck
- It must have `appstudio.redhat.com/sp.host` label where the value is host of the service provider matching that of
SPIAccessCheck's `spec.repoUrl`.
- It must already contain some secret data (DataObtained condition is true).
- RemoteSecret's `spec.secret.type` must be equal to `kubernetes.io/basic-auth`.
- It must have a deployed target in the SPIAccessCheck's namespace.
- (Optionally) RemoteSecret with the annotation `appstudio.redhat.com/sp.repository` will be prioritized if the value
is a list of comma-separated repository names and this list contains the repository name inferred from SPIAccessCheck's `spec.repoUrl.`

For example RemoteSecret yaml see [the sample](/samples/remotesecret-for-ac.yaml).

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


## HTTP API Endpoints

### POST /login
This endpoint is used to authenticate `/{sp_type}/authenticate` method with Kubernetes (SSO) bearer token in the `Authorization` header.

This endpoint sets a session cookie that is required to be present when completing the OAuth flow in the `/{sp_type}/authenticate` and `/{sp_type}/callback` endpoints.

### POST /logout
This endpoint is used to invalidate the session cookie set by the `/login` endpoint. 
It is not required to call this endpoint, as the session cookie expires in 15 minutes after the last request.

#### Response
- 200 - authorization data successfully accepted.
- 403 - provided authorization header is not valid.
-
### GET /{sp_type}/authenticate
This method is used to initiate the OAuth flow. `{sp_type}` refers to the name of the service provider.

This endpoint expects that k8s token provided by `/login` method. The token needs to enable creation of `SPIAccessTokenDataUpdate` objects in the target cluster.
The URL to this endpoint is generated by the SPI operator and can be read from the     status of the `SPIAccessTokenBinding` or `SPIAccessToken` objects.


#### Parameters
- state - The caller must supply the state query parameter which holds the OAuth flow state.
- k8s_token - the authorization token. It is HIGHLY DISCOURAGED to use this in a GET request. Use the Authorization header instead.
#### Headers
Authorization - optional, in the form `“Bearer <token>”`. Either this header, or `k8s_token` query parameter has to be provided.

#### Response
- 200 - returns an HTML page with meta http-equiv tag that redirects the caller to the corresponding service provider to perform the OAuth flow.
- 403 - if the authorization token is not correct or supplied

### GET /{sp_type}/callback
This is the endpoint to which the user is redirected from the service provider when the OAuth flow is completed. As such, this endpoint is not meant for direct consumption by any direct caller. Instead, the service provider redirects the clients to this endpoint upon completion of the OAuth flow.

#### Response
- 200 - an HTML page shown upon successful OR ERRONEOUS completion of the flow. The page shows a human-readable description of the result.

### POST /token/{namespace}/{name}
This endpoint is used to manually upload the token data for an existing SPIAccessToken object. This endpoint is authenticated using a Kubernetes (SSO) bearer token in the Authorization header.

#### Path Parameters
- namespace - the namespace of the SPIAccessToken object to update with the data
- name - the name of the SPIAccessToken object to update with the data

#### Headers
- Authorization - mandatory, in the form `“Bearer <token>”`.
#### Body
This endpoint accepts a JSON object with the following structure:

```json
{
        "access_token": "string value of the access token", 
        "username": "service provider username",
        "token_type": "the type of the token", // currently ignored
        "refresh_token": "string value of the refresh token", // currently ignored
        "expiry": 42 // the date when the token expires represented as timestamp, currently ignored         
}
```
#### Response
- 204 - when the data is processed successfully
- 403 - on authorization error

## SnapshotEnvironmentBinding Controller
The SnapshotEnvironmentBinding controller is a part of the SPI Operator. The responsibility of the controller is to watch
SnapshotEnvironmentBindings and infer which RemoteSecret needs to have target added or removed.

RemoteSecrets belonging to a certain Environment (which means they are candidates for updating their targets) are
identified by `appstudio.redhat.com/environment` label or annotation.

When RemoteSecret has the label set, the value is interpreted as Environment name. When instead the RemoteSecret has annotation
with such a key, the value is interpreted as comma-separated list of Environment names. This distinction is made because
annotations are permitted longer values. If neither label nor annotation is set, the RemoteSecret is considered to belong
to every environment. The annotation takes precedence over the label. In other words, specifying both label and
annotation is the same as just specifying annotation. See examples below:

```yaml
apiVersion: appstudio.redhat.com/v1beta1
kind: RemoteSecret
metadata:
  labels:
    appstudio.redhat.com/environment: prod # This RemoteSecret belongs to the `prod` Environment
```

```yaml
apiVersion: appstudio.redhat.com/v1beta1
kind: RemoteSecret
metadata:
  annotations:
    appstudio.redhat.com/environment: prod,test,gama # This RemoteSecret belongs to the `prod`, `test`, and `gama` Environment
  # adding a label with the same key would have no effect
```

