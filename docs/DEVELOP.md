## Building & Testing
This project provides a `Makefile` to run all the usual development tasks. If you simply run `make` without any arguments, you'll get a list of available "targets".

To build the project one needs to invoke (builds both `operator` and `oauth` binaries into `bin/` folder):

```
make build
```

To test the code (WARNING: tests require a running cluster in the kubectl context):

```
make test
```
To run individual unit tests, you can use the normal `go test` workflow.

There are also many integration tests that are also run by `make test`.

The integration tests run with a `testenv` Kubernetes API server so they cannot be run simply by `go test`. You can run individual integration tests using 
```
make itest focus="..."
```
where the value of `focus` is the description of the Ginkgo integration test you want to run.

To build the docker images of the operator and oauth service one can run:

```
make docker-build
```

This will make a docker images called `quay.io/redhat-appstudio/service-provider-integration-operator:next` and `quay.io/redhat-appstudio/service-provider-integration-oauth:next` which might or might not be what you want.
To override the name of the image build, specify it in the `SPI_IMG_BASE` and/or `TAG_NAME` environment variable, e.g.:

```
make docker-build SPI_IMG_BASE=quay.io/acme TAG_NAME=bugfix
```

To push the images to an image repository one can use:

```
make docker-push
```

The image being pushed can again be modified using the environment variable:
```
make docker-push SPI_IMG_BASE=quay.io/acme TAG_NAME=bugfix
```

To set precise image names, one can use `SPIO_IMG` for operator image and `SPIS_IMG` for oauth image (see [Makefile](Makefile) for more details).

Before you push a PR to the repository, it is recommended to run an overall validity check of the codebase. This will
run the formatting check, static code analysis and all the tests:

```
make check
```
If you don't want to merely check that everything is OK but also make the modifications automatically, if necessary, you can, instead of `make check`, run:

```
make ready
```

which will automatically format and lint the code, update the `go.mod` and `go.sum` files and run tests. As such, this goal may modify the contents of the repository.

### Running out of cluster
There is a dedicated make target to run the operator locally:

```
make run
```

This will also deploy RBAC setup and the CRDs into the cluster and will run the operator locally with the permissions of the deployed service account as configure in the Kustomize files in the `config` directory.

To run the operator with the permissions of the currently active kubectl context, use:

```
make run_as_current_user
```
To run the OAuth service locally, one can use:

```
make run_oauth
```

### Running in cluster
Again, there is a dedicated make target to deploy the operator with OAuth service into the cluster:
```
make deploy_openshift       # OpenShift with Vault tokenstorage
make deploy_openshift_aws   # OpenShift with AWS tokenstorage
make deploy_minikube        # minikube with Vault tokenstorage
make deploy_minikube_aws    # minikube with AWS tokenstorage
```

## Debugging

It is possible to debug the operator using `dlv` or some IDE like `vscode`. Just point the debugger of your choice to `main.go` as the main program file and remember to configure the environment variables for the correct/intended function of the operator.

## Manual testing with custom images

This assumes the current working directory is your local checkout of this repository.

If on Minikube, we first need to enable the ingress addon (skip this step, obviously, if you're working with OpenShift or other Kubernetes distribution):
```
minikube addons enable ingress
```

Then we can install our CRDs:

```
make install
```

Next, we're ready to build and push the custom operator and oauth images:
```
make docker-build docker-push SPI_IMG_BASE=<MY-CUSTOM-IMAGE-BASE> TAG_NAME=<MY-CUSTOM-TAG-NAME>
```

Next step is to deploy the operator and oauth service along with all other Kubernetes objects to the cluster.
This step assumes that you also want to use a custom image for both SPI OAuth service and Operator. If you want to use the
default one, specify just the `SPIS_IMG` or `SPIO_IMG` env var below.

On OpenShift use:
```
make deploy_openshift SPI_IMG_BASE=<MY-CUSTOM-IMAGE-BASE> TAG_NAME=<MY-CUSTOM-TAG-NAME>
```

On Minikube use:
```
make deploy_minikube SPI_IMG_BASE=<MY-CUSTOM-IMAGE-BASE> TAG_NAME=<MY-CUSTOM-TAG-NAME>
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

### Requirements on the Service Providers
For the OAuth workflow to work, SPI needs to be registered as an Oauth application within all service providers that it will need to interact with.
Note that the integration also includes the “redirect_uri”, i.e. the target URL to which the OAuth flow will be redirected upon completion.
In addition, the SPI REST API will need to be configured with the `Client ID` and `Client Secret` for each such OAuth app in every Service Provider.
