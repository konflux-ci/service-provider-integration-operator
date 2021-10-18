# service-provider-integration-operator
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

## Configuration
The operator is currently configured using environment variables.

* `SPI_URL` - **mandatory** the URL to the REST API of the Service Provider Integration service to which the operator talks to receive the tokens
* `SPI_BEARER_TOKEN_FILE` - *optional* the location of the file with the bearer token used to authenticate with the SPI REST API. This
is autodetected when running in-cluster but can be useful to explicitly specify when running out of cluster when debugging.

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

Note that it is required to configure the operator using the environment variables as mentioned above.
### In cluster
Again, there is a dedicated make target to deploy the operator into the cluster:

```
make deploy
```

Once deployed, make sure to edit the operator Deployment in the `spi-system` namespace and set the `SPI_URL` to a URL on which the SPI REST API
can be reached.
## Debugging

It is possible to debug the operator using `dlv` or some IDE like `vscode`. Just point the debugger of your choice to `main.go` as the main program file and remember to configure the environment variables for the correct/intended function of the operator.

The `launch.json` file for `vscode` is included in the repository so you should be all set if using that IDE. Just make sure to run `make prepare` before debugging.
