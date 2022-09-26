# service-provider-integration-operator
[![Code Coverage Report](https://github.com/redhat-appstudio/service-provider-integration-operator/actions/workflows/codecov.yaml/badge.svg)](https://github.com/redhat-appstudio/service-provider-integration-operator/actions/workflows/codecov.yaml)
[![codecov](https://codecov.io/gh/redhat-appstudio/service-provider-integration-operator/branch/main/graph/badge.svg?token=EH16HO2RHP)](https://codecov.io/gh/redhat-appstudio/service-provider-integration-operator)

A Kubernetes controller/operator that manages service provider integration tasks 

## Building & Testing
This project provides a `Makefile` to run all the usual development tasks. If you simply run `make` without any arguments, you'll get a list of available "targets".

To build the project one needs to invoke:

```
make build
```

To test the code:

```
make test
```

To build the docker image of the operator one can run:

```
make docker-build
```

This will make a docker image called `controller:latest` which might or might not be what you want. To override the name of the image build, specify it in the `SPIO_IMG` environment variable, e.g.:

```
SPIO_IMG=quay.io/acme/spio:42 make docker-build
```

To push the image to an image repository one can use:

```
make docker-push
```

The image being pushed can again be modified using the environment variable:
```
SPIO_IMG=quay.io/acme/spio:42 make docker-push
```

Before you push a PR to the repository, it is recommended to run an overall validity check of the codebase. This will
run the formatting check, static code analysis and all the tests:

```
make check
```

### KCP
We generate `APIResourceSchema`s and `APIExport` for KCP from our CRDs using `./hack/generate-kcp-api.sh` script or `make manifests-kcp`.
These have to be updated on every CRD change and committed. `APIResourceSchema` is immutable in KCP, so we version it by putting datetime as a name prefix.


## Configuration

It is expected by the Kustomize deployment that this configuration lives in a Secret in the same namespaces as SPI.
Name of the secret should be `spi-shared-configuration-file` with this configuration yaml under `config.yaml` key.

This is basic configuration that is mandatory to run SPI Operator and OAuth services. [See config.go](pkg/spi-shared/config/config.go) for details (`persistedConfiguration`).

```yaml
serviceProviders:
- type: <service_provider_type>
  clientId: <service_provider_client_id>
  clientSecret: <service_provider_secret>
```

 - `<service_provider_type>` - type of the service provider. This must be one of the supported values: GitHub, Quay
 - `<service_provider_client_id>` - client ID of the OAuth application
 - `<service_provider_secret>` - client secret of the OAuth application that the SPI uses to access the service provider


_To create OAuth application at GitHub, follow [GitHub - Creating an OAuth App](https://docs.github.com/en/developers/apps/building-oauth-apps/creating-an-oauth-app)_

The rest of the configuration is applied using the environment variables or command line arguments.

In addition to the secret, there are 3 configmaps that contain the configuration for operator and oauth service.

| ConfigMap | Applicable to |
|-----------|---------------|
| `spi-shared-environment-config` | operator and oauth service |
| `spi-controller-manager-environment-config` | operator |
| `spi-oauth-service-environment-config` | oauth service |

The `spi-shared-environment-config` is bound to both the operator and oauth service and is therefore best used for
configuration options that should have the same value in both deployments.

#### Common configuration parameters
The `shared-environment-config` config map contains configuration options that will be applied to both the operator and
oauth service (if you look in the tables of the individual config options for operator and oauth service, you will see
that a lot of them are common). This especially makes sense for configuration that needs to have the same value in both
deployments, e.g. `BASEURL` and `VAULTHOST` are two examples of values that should be the same for both.

To change the VAULTHOST for both the oauth service and the operator, one can do:
```
kubectl -n spi-system patch configmap spi-shared-environment-config --type=merge -p '{"data": {"VAULTHOST": "https://vault.somewhere.else"}}'
kubectl -n spi-system rollout restart deployment/spi-controller-manager deployment/spi-oauth-service
```

This is the full set of parameters that are common for both the operator and the oauth service (note that while the
command line arguments and the environment variable names are the same, it may not make sense for both the operator
and oauth service to have the same value for them):

| Command argument                                      | Environment variable           | Default               | Description                                                                                                                                                                                                                        |
|-------------------------------------------------------|--------------------------------|-----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --base-url                                            | BASEURL                        |                       | This is the publicly accessible URL on which the SPI OAuth service is reachable. Note that this is not just a hostname, it is a full URL including a scheme, e.g. "https://acme.com/spi"                                           |
| --config-file                                         | CONFIGFILE                     | /etc/spi/config.yaml  | The location of the configuration file.                                                                                                                                                                                            |
| --metrics-bind-address                                | METRICSADDR                    | :8080                 | The address the metric endpoint binds to.                                                                                                                                                                                          |
| --health-probe-bind-address HEALTH-PROBE-BIND-ADDRESS | PROBEADDR                      | :8081                 | The address the probe endpoint binds to.                                                                                                                                                                                           |
| --vault-host                                          | VAULTHOST                      | http://spi-vault:8200 | Vault host URL. Default is internal kubernetes service.                                                                                                                                                                            |
| --vault-insecure-tls                                  | VAULTINSECURETLS               | false                 | Whether is allowed or not insecure vault tls connection.                                                                                                                                                                           |
| --vault-auth-method                                   | VAULTAUTHMETHOD                | approle               | Authentication method to Vault token storage. Options: 'kubernetes', 'approle'.                                                                                                                                                    |
| --vault-roleid-filepath                               | VAULTAPPROLEROLEIDFILEPATH     | /etc/spi/role_id      | Used with Vault approle authentication. Filepath with role_id.                                                                                                                                                                     |
| --vault-secretid-filepath                             | VAULTAPPROLESECRETIDFILEPATH   | /etc/spi/secret_id    | Used with Vault approle authentication. Filepath with secret_id.                                                                                                                                                                   |
| --vault-k8s-sa-token-filepath                         | VAULTKUBERNETESSATOKENFILEPATH |                       | Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development. |
| --vault-k8s-role                                      | VAULTKUBERNETESROLE            |                       | Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount.                                                                                                                                   |
| --zap-devel                                           | ZAPDEVEL                       | false                 | Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)                                                            |
| --zap-encoder                                         | ZAPENCODER                     |                       | Zap log encoding (‘json’ or ‘console’)                                                                                                                                                                                             |
| --zap-log-level                                       | ZAPLOGLEVEL                    |                       | Zap Level to configure the verbosity of logging.                                                                                                                                                                                   |
| --zap-stacktrace-level                                | ZAPSTACKTRACELEVEL             |                       | Zap Level at and above which stacktraces are captured.                                                                                                                                                                             |
| --zap-time-encoding                                   | ZAPTIMEENCODING                | iso8601               | Format of the time in the log. One of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano.                                                                                                                             |

### Operator configuration parameters

This table only contains the configuration parameters specific to the operator. All the common configuration parameters
are also applicable to the operator. The configmap for operator-specific configuration is called 
`spi-controller-manager-environment-config`.

| Command argument        | Environment variable        | Default | Description                                                                                                                                                                      |
|-------------------------|-----------------------------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --leader-elect          | ENABLELEADERELECTION        | false   | Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.                                                            |
| --metadata-cache-ttl    | TOKENMETADATACACHETTL       | 1h      | The maximum age of the token metadata cache. To reduce the load on the service providers, SPI only refreshes the metadata of the tokens when determined stale by this parameter. |
| --token-ttl             | TOKENLIFETIMEDURATION       | 120h    | Access token lifetime in hours, minutes or seconds. Examples:  "3h",  "5h30m40s" etc.                                                                                            |
| --binding-ttl           | BINDINGLIFETIMEDURATION     | 2h      | Access token binding lifetime in hours, minutes or seconds. Examples: "3h", "5h30m40s" etc.                                                                                      |
| --access-check-ttl      | ACCESSCHECKLIFETIMEDURATION | 30m     | Access check lifetime in hours, minutes or seconds.                                                                                                                              |
| --token-match-policy    | TOKENMATCHPOLICY            | any     | The policy to match the token against the binding. Options:  'any', 'exact'."`                                                                                                   |
| --kcp-api-export-name   | APIEXPORTNAME               | spi     | SPI ApiExport name used in KCP environment to configure controller with virtual workspace.                                                                                       |
| --deletion-grace-period | DELETIONGRACEPERIOD         | 2s      | The grace period between a condition for deleting a binding or token is satisfied and the token or binding actually being deleted.

### OAuth service configuration parameters

This table only contains the configuration parameters specific to the oauth service. All the common configuration parameters
are also applicable to the oauth service. The configmap for oauth-service-specific configuration is called
`spi-oauth-service-environment-config`.

| Command argument              | Environment variable           | Default                                                         | Description                                                                                                                                                                                                                        |
|-------------------------------|--------------------------------|-----------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --service-addr                | SERVICEADDR                    | 0.0.0.0:8000                                                    | Service address to listen on.                                                                                                                                                                                                      |
| --allowed-origins             | ALLOWEDORIGINS                 | https://console.dev.redhat.com,https://prod.foo.redhat.com:1337 | Comma-separated list of domains allowed for cross-domain requests.                                                                                                                                                                 |
| --kubeconfig                  | KUBECONFIG                     |                                                                 | KUBE-CONFIG.                                                                                                                                                                                                                       |
| --kube-insecure-tls           | KUBEINSECURETLS                | false                                                           | Whether is allowed or not insecure kubernetes tls connection.                                                                                                                                                                      |
| --api-server                  | API_SERVER                     |                                                                 | Host:port of the Kubernetes API server to use when handling HTTP requests.                                                                                                                                                         |
| --ca-path                     | API_SERVER_CA_PATH             |                                                                 | The path to the CA certificate to use when connecting to the Kubernetes API server.                                                                                                                                                |

## Vault

Vault instance is deployed together with SPI components. `make deploy` or `make deploy_minikube` configures it automatically. 
For other deployments, like [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) run `./hack/vault-init.sh` manually.

There are couple of support scripts to work with Vault
 - `./hack/vault-init.sh` - Initialize and configure Vault instance
 - `./hack/vault-generate-template.sh` - generates deployment yamls from [vault-helm](https://github.com/hashicorp/vault-helm). These should be commited in this repository.
 - injected in vault pod `/vault/userconfig/scripts/poststart.sh` - unseal vault storage. Runs automatically after pod startup.
 - injected in vault pod `/vault/userconfig/scripts/root.sh` - vault login as root with generated root token. Can be used for manual configuration.

## Running
It is possible to run the operator both in and out of a Kubernetes cluster.

### Out of cluster
There is a dedicated make target to run the operator locally:

```
make run
```

This will also deploy RBAC setup and the CRDs into the cluster and will run the operator locally with the permissions of the deployed service account as configure in the Kustomize files in the `config` directory.

To run the operator with the permissions of the currently active kubectl context, use:

```
make run_as_current_user
```

### In cluster
Again, there is a dedicated make target to deploy the operator into the cluster:

For OpenShift, use:

```
make deploy
```

For Kubernetes, use:
```
make deploy_k8s
```

Once deployed, several manual modifications need to be made. See the below section about manual testing
with custom images for details.

## Debugging

It is possible to debug the operator using `dlv` or some IDE like `vscode`. Just point the debugger of your choice to `main.go` as the main program file and remember to configure the environment variables for the correct/intended function of the operator.

The `launch.json` file for `vscode` is included in the repository so you should be all set if using that IDE. Just make sure to run `make prepare` before debugging.

## Manual testing with custom images

This assumes the current working directory is your local checkout of this repository.

First, we need to enable the ingress addon (skip this step, obviously, if you're working with OpenShift):
```
minikube addons enable ingress
```

Then we can install our CRDs:

```
make install
```

Next, we're ready to build and push the custom operator image:
```
make docker-build docker-push SPIO_IMG=<MY-CUSTOM-OPERATOR-IMAGE>
```

Next step is to deploy the operator and oauth service along with all other Kubernetes objects to the cluster.
This step assumes that you also want to use a custom image of the SPI OAuth service. If you want to use the
default one, just don't specify the `SPIS_IMG` env var below.

On OpenShift use:
```
make deploy SPIO_IMG=<MY-CUSTOM-OPERATOR-IMAGE> SPIS_IMG=<MY-CUSTOM-OAUTH-SERVICE-IMAGE>
```

On Kubernetes/Minikube use:
```
make deploy_k8s SPIO_IMG=<MY-CUSTOM-OPERATOR-IMAGE> SPIS_IMG=<MY-CUSTOM-OAUTH-SERVICE-IMAGE>
```

Next, comes the manual part. We need to set the external domain of the ingress/route of the OAuth service and reconfigure
the OAuth service and operator to know about it:

On Minikube, we can use `nip.io` to set the hostname like this:
```
SPI_HOST="spi.$(minikube ip).nip.io"
kubectl -n spi-system patch ingress spi-oauth-ingress --type=json --patch '[{"op": "replace", "path": "/spec/rules/0/host", "value": "'$SPI_HOST'"}]'
```

On Kubernetes, the host of the ingress needs to be set be the means appropriate to your cluster environment.

On OpenShift, you merely need to note down the hostname of your route.

In either case, store the hostname of your ingress/route in the `$SPI_HOST` environment variable

Also, note down the client id and client secret of the OAuth application in Github that you want SPI to act as
and store the in the `CLIENT_ID` and `CLIENT_SECRET` env vars respectively.

Next, we need to reconfigure the oauth service and operator. Both are configured using a single configmap:

```
SPI_CONFIGMAP=$(kubectl -n spi-system get configmap -l app.kubernetes.io/part-of=service-provider-integration-operator | grep spi-oauth-config | cut -f1 -d' ')
kubectl -n spi-system patch configmap $SPI_CONFIGMAP --type=json --patch '[{"op": "replace", "path": "/data/config.yaml", "value": "'"$(kubectl -n spi-system get configmap $SPI_CONFIGMAP -o jsonpath='{.data.config\.yaml}' | yq -y 'setpath(["baseUrl"]; "https://'$SPI_HOST'")' | yq -y 'setpath(["serviceProviders", 0, "clientId"]; "'$CLIENT_ID'")' | yq -y 'setpath(["serviceProviders", 0, "clientSecret"]; "'$CLIENT_SECRET'")' | sed ':a;N;$!ba;s/\n/\\n/g')"'"}]'
```

All that is left for the setup is to restart the oauth service and operator to load the new configuration:
```
kubectl -n spi-system scale deployment spi-controller-manager spi-oauth-service --replicas=0
kubectl -n spi-system scale deployment spi-controller-manager spi-oauth-service --replicas=1
```

### Go through the OAuth flow manually

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

Now, we need initiate the OAuth flow. We've already created the spi access token binding. Let's examine
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

Upon returning from that OAuth flow, you should end up back on the SPI endpoint on the `.../github/callback` URL.
This concludes the flow and you should be able to see the secret configured in the binding with the requested data:

```
kubectl get secret token-secret -o yaml
```


## Supported providers and token kinds

The currently supported list of providers and token kinds is following:

### Github

 - OAuth token via user authentication flow
 - Personal access token via manual upload

### Quay

  - OAuth token via user authentication flow
  - Robot account credentials via manual upload

### Snyk & other providers that do not support OAuth

 - Manual credentials upload only with both username and token required.
   Single token per namespace per host is stored.
   See details below.


## Storing username and password credentials for any provider by it's URL

  It is now possible to store username + token/password credentials for nearly any
  provider that may offer them (like Snyk.io etc). It is can be done in a few steps:
  
 - Create simple SPIAccessTokenBinging, indicating provider-s URL only:
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