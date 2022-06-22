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

### KCP
We generate `APIResourceSchema`s and `APIExport` for KCP from our CRDs using `./hack/generate-kcp-api.sh` script or `make manifests-kcp`.
These have to be updated on every CRD change and committed. `APIResourceSchema` is immutable in KCP, so we version it by putting datetime as a name prefix.


## Configuration

It is expected by the Kustomize deployment that this configuration lives in a Secret in the same namespaces as SPI.
Name of the secret should be `oauth-config` with this configuration yaml under `config.yaml` key.

This is basic configuration that is mandatory to run SPI Operator and OAuth services. [See config.go](pkg/spi-shared/config/config.go) for details (`PersistedConfiguration` and `ServiceProviderConfiguration`).

```yaml
sharedSecret: <jwt_sign_secret>
serviceProviders:
- type: <service_provider_type>
  clientId: <service_provider_client_id>
  clientSecret: <service_provider_secret>
baseUrl: <oauth_base_url>
vaultHost: <vault_url>
```

 - `<jwt_sign_secret>` - secret value used for signing the JWT keys
 - `<service_provider_type>` - type of the service provider. This must be one of the supported values: GitHub, Quay
 - `<service_provider_client_id>` - client ID of the OAuth application
 - `<service_provider_secret>` - client secret of the OAuth application that the SPI uses to access the service provider
 - `<oauth_base_url>` - URL on which the OAuth service is deployed
 - `<vault_url>` - Optional. URL to Vault token storage. Default works with deployment scripts. Useful for local development.


_To create OAuth application at GitHub, follow [GitHub - Creating an OAuth App](https://docs.github.com/en/developers/apps/building-oauth-apps/creating-an-oauth-app)_

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


### SPI Logging concept.

It is important to keep consistency between components in terms of the ways and form of how
the events are logged. It makes the development and support of these components much easier.

Since SPI is OpenShift/Kubernetes oriented framework we are inspired by [Operator SDK Logging](
https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/) and [structured logging](
https://www.client9.com/structured-logging-in-golang/) methods. That means usage of [logr](https://pkg.go.dev/github.com/go-logr/logr) interface
to log and [zap](https://pkg.go.dev/github.com/go-logr/zapr) as a backend.


#### How to use
```
func (r *SPIAccessTokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
....
	lg := log.FromContext(ctx)
	lg.Info("Reconciling")
....
}	
```
```
func CallbackErrorHandler(w http.ResponseWriter, r *http.Request) {
	lg := log.FromContext(r.Context())
	lg.Info("CallbackErrorHandler")
	....
}
```
#### Log levels
INFO inclide
- outgoing network connections
- security-related events
  
DEBUG include
  - level 1
    - TODO
      - level 2
    - TODO

#### Structural messages
As described here [structured logging](
https://www.client9.com/structured-logging-in-golang/) methods it is recommended to use structural logs like 
```
// vargargs style
lg.Info("user not found", "userid", 1234)
```
The output is a list of key-value pairs followed by the static message: `userid=1234 msg="user not found"`

Instead of
```
lg.Info(fmt.Sprintf("user %d wasn't found", 1234))
// user 1234 wasn't found
```

#### How to configure in local tests
TODO
#### How to configure in IDE test
TODO
#### How to configure on test cluster
TODO
#### How to configure on staging.
TODO
