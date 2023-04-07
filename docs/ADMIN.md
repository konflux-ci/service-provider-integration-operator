## Installation
TODO Explain production configuration

## Configuration

It is expected by the Kustomize deployment that this configuration lives in a Secret in the same namespaces as SPI.
Name of the secret should be `shared-configuration-file` with this configuration yaml under `config.yaml` key.

This is basic configuration that is mandatory to run SPI Operator and OAuth services. [See config.go](pkg/spi-shared/config/config.go) for details (`persistedConfiguration`).

```yaml
serviceProviders:
- type: <service_provider_type>
  clientId: <service_provider_client_id>
  clientSecret: <service_provider_secret>
  baseUrl: <service_provider_url>
```

- `<service_provider_type>` - type of the service provider. This must be one of the supported values: `GitHub`, `Quay`, `GitLab`
- `<service_provider_client_id>` - client ID of the OAuth application
- `<service_provider_secret>` - client secret of the OAuth application that the SPI uses to access the service provider
- `<service_provider_url>` - optional field used for service providers running on custom domains (other than public saas). Example: `https://my-gitlab-sp.io`

_Note: See [Configuring Service Providers](#configuring-service-providers) for configuration on service provider side._

The rest of the configuration is applied using the environment variables or command line arguments.

In addition to the secret, there are 3 configmaps that contain the configuration for operator and oauth service.

| ConfigMap                                   | Applicable to              |
|---------------------------------------------|----------------------------|
| `spi-shared-environment-config`             | operator and oauth service |
| `spi-controller-manager-environment-config` | operator                   |
| `spi-oauth-service-environment-config`      | oauth service              |

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

| Command argument                                      | Environment variable           | Default                  | Description                                                                                                                                                                                                                        |
|-------------------------------------------------------|--------------------------------|--------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --base-url                                            | BASEURL                        |                          | This is the publicly accessible URL on which the SPI OAuth service is reachable. Note that this is not just a hostname, it is a full URL including a scheme, e.g. "https://acme.com/spi"                                           |
| --config-file                                         | CONFIGFILE                     | /etc/spi/config.yaml     | The location of the configuration file.                                                                                                                                                                                            |
| --spi-instance-id                                     | SPIINSTANCEID                  | spi-1                    | ID of this SPI instance. Used to avoid conflicts when multiple SPI instances uses shared resources (e.g. secretstorage). |
| --metrics-bind-address                                | METRICSADDR                    | 127.0.0.1:8080           | The address the metric endpoint binds to. Note: While this is the default from the operator binary point of view, the metrics are still available externally through the authorized endpoint provided by kube-rbac-proxy           |
| --allow-insecure-urls                                 | ALLOWINSECUREURLS              | false                    | Whether it is allowed or not to use insecure (http) URLs in service provider or token storage configurations.                                                                                                                      |
| --health-probe-bind-address HEALTH-PROBE-BIND-ADDRESS | PROBEADDR                      | :8081                    | The address the probe endpoint binds to.                                                                                                                                                                                           |
| --tokenstorage                                        | TOKENSTORAGE                   | vault                    | The type of the token storage. Supported types: 'vault', 'aws' (experimental)                                                                                                                                                      |
| --vault-host                                          | VAULTHOST                      | http://spi-vault:8200    | Vault host URL. Default is internal kubernetes service.                                                                                                                                                                            |
| --vault-insecure-tls                                  | VAULTINSECURETLS               | false                    | Whether is allowed or not insecure vault tls connection.                                                                                                                                                                           |
| --vault-auth-method                                   | VAULTAUTHMETHOD                | approle                  | Authentication method to Vault token storage. Options: 'kubernetes', 'approle'.                                                                                                                                                    |
| --vault-roleid-filepath                               | VAULTAPPROLEROLEIDFILEPATH     | /etc/spi/role_id         | Used with Vault approle authentication. Filepath with role_id.                                                                                                                                                                     |
| --vault-secretid-filepath                             | VAULTAPPROLESECRETIDFILEPATH   | /etc/spi/secret_id       | Used with Vault approle authentication. Filepath with secret_id.                                                                                                                                                                   |
| --vault-k8s-sa-token-filepath                         | VAULTKUBERNETESSATOKENFILEPATH |                          | Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development. |
| --vault-k8s-role                                      | VAULTKUBERNETESROLE            |                          | Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount.                                                                                                                                   |
| --vault-data-path-prefix                              | VAULTDATAPATHPREFIX            | spi                      | Path prefix in Vault token storage under which all SPI data will be stored. No leading or trailing '/' should be used, it will be trimmed.                                                                                         |
| --aws-config-filepath                                 | AWS_CONFIG_FILE                | /etc/spi/aws/config      | Filepath to AWS configuration file                                                                                                                                                                                                 |
| --aws-credentials-filepath                            | AWS_CREDENTIALS_FILE           | /etc/spi/aws/credentials | Filepath to AWS credentials file                                                                                                                                                                                                   |
| --zap-devel                                           | ZAPDEVEL                       | false                    | Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)                                                            |
| --zap-encoder                                         | ZAPENCODER                     |                          | Zap log encoding (‘json’ or ‘console’)                                                                                                                                                                                             |
| --zap-log-level                                       | ZAPLOGLEVEL                    |                          | Zap Level to configure the verbosity of logging.                                                                                                                                                                                   |
| --zap-stacktrace-level                                | ZAPSTACKTRACELEVEL             |                          | Zap Level at and above which stacktraces are captured.                                                                                                                                                                             |
| --zap-time-encoding                                   | ZAPTIMEENCODING                | iso8601                  | Format of the time in the log. One of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano.                                                                                                                             |

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
| --file-request-ttl      | FILEREQUESTLIFETIMEDURATION | 30m     | File content request lifetime in hours, minutes or seconds.                                                                                                                      |
| --token-match-policy    | TOKENMATCHPOLICY            | any     | The policy to match the token against the binding. Options:  'any', 'exact'."`                                                                                                   |
| --deletion-grace-period | DELETIONGRACEPERIOD         | 2s      | The grace period between a condition for deleting a binding or token is satisfied and the token or binding actually being deleted.                                               |

### OAuth service configuration parameters

This table only contains the configuration parameters specific to the oauth service. All the common configuration parameters
are also applicable to the oauth service. The configmap for oauth-service-specific configuration is called
`spi-oauth-service-environment-config`.

| Command argument    | Environment variable | Default                                                                                                                     | Description                                                                         |
|---------------------|----------------------|-----------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| --service-addr      | SERVICEADDR          | 0.0.0.0:8000                                                                                                                | Service address to listen on.                                                       |
| --allowed-origins   | ALLOWEDORIGINS       | https://console.redhat.com,https://console.stage.redhat.com,https://console.dev.redhat.com,https://prod.foo.redhat.com:1337 | Comma-separated list of domains allowed for cross-domain requests.                  |
| --kubeconfig        | KUBECONFIG           |                                                                                                                             | KUBE-CONFIG.                                                                        |
| --kube-insecure-tls | KUBEINSECURETLS      | false                                                                                                                       | Whether is allowed or not insecure kubernetes tls connection.                       |
| --api-server        | API_SERVER           |                                                                                                                             | Host:port of the Kubernetes API server to use when handling HTTP requests.          |
| --ca-path           | API_SERVER_CA_PATH   |                                                                                                                             | The path to the CA certificate to use when connecting to the Kubernetes API server. |

## [Configuring Service Providers](#configuring-service-providers)

OAuth requires to create OAuth Application on Service Provider side. Service providers usually require to set:
 - __application homepage URL__: URL of the root of main Route/Ingress to SPI OAuth Service. For minikube: `https://spi.<minikube_ip>.nip.io`, for openshift: `https://<name>-<namespace>.<cluster_host>`. Example for minikube: `https://spi.192.168.64.166.nip.io`.
 - __application callback URL__: homepage URL + `/oauth/callback`. Example for minikube: `https://spi.192.168.64.166.nip.io/oauth/callback`.

### GitHub
To create OAuth application follow [GitHub - Creating an OAuth App](https://docs.github.com/en/developers/apps/building-oauth-apps/creating-an-oauth-app).

### GitLab
To create OAuth application follow [Configure GitLab as an OAuth 2.0 authentication identity provider](https://docs.gitlab.com/ee/integration/oauth_provider.html).

When creating the OAuth application on GitLab, it is required to choose a set of scopes that the application __can ask for__. SPIAccessTokenBindings ask for different
scopes depending on the `spec.permissions.required` and `spec.permissions.additionalScopes`. For additional information about permissions format, see [SPIAccessTokenBinding](docs/USER.md).

The table below defines what GitLab scopes are required based on permissions of an SPIAccessTokenBinding.
All described scopes should be permitted to avoid integration issues with SPI.

| Scope            | Permissions Area | Permission Types | Description                                                         |
|------------------|------------------|------------------|---------------------------------------------------------------------|
| read_user        | every area       | every type       | Every SPIAccessTokenBinding needs this scope to read user metadata. |
| read_repository  | "repository"     | "r"              | Grants read-only access to repositories on private projects.        |
| write_repository | "repository"     | "w", "rw"        | Grants read-write access to repositories on private projects        |

## Token Storage
### Vault

Vault is default token storage. Vault instance is deployed together with SPI components. `make deploy_minikube` or `make deploy_openshift` configures it automatically.
For other deployments, like [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) run `./hack/vault-init.sh` manually.

There are couple of support scripts to work with Vault
- `./hack/vault-init.sh` - Initialize and configure Vault instance.
  - To change path prefix for the SPI data (default is `spi`), set `SPI_DATA_PATH_PREFIX` environment variable. Value must be without leading and trailing slashes (e.g.: `SPI_DATA_PATH_PREFIX=all/spi/tokens/here`). To configure Vault path prefix in SPI see `--vault-data-path-prefix` SPI property.
- `./hack/vault-generate-template.sh` - generates deployment yamls from [vault-helm](https://github.com/hashicorp/vault-helm). These should be commited in this repository.
- injected in vault pod `/vault/userconfig/scripts/poststart.sh` - unseal vault storage. Runs automatically after pod startup.
- injected in vault pod `/vault/userconfig/scripts/root.sh` - vault login as root with generated root token. Can be used for manual configuration.

### AWS Secrets Manager (experimental)

_Warning: AWS Secrets Manager as SPI Token storage is currently in experimental phase of implementation. Usage is not recommended for production use, implementation can change with backward breaking changes anytime without any further notice._

To enable AWS Secrets Manager as SPI token storage, set `--tokenstorage=aws`. `make deploy_minikube_aws` or `make deploy_openshift_aws` configures it automatically.

SPI require 2 AWS configuration files, `config` and `credentials`. These can be set with `--aws-config-filepath` and `--aws-credentials-filepath`.

_Note: If you've used AWS cli locally, AWS configuration files should be at `~/.aws/config` and `~/.aws/credentials`. To create the secret, use `./hack/aws-create-credentials-secret.sh`_

## [Service Level Objectives monitoring](#service-level-objectives-monitoring)

 There is a defined list of Service Level Objectives (SLO-s), for which SPI service should collect indicator metrics, 
 and expose them on its monitoring framework. It is dedicated Grafana dashboard, containing only those metrics which are defined
 as a SLI/SLOs for the SPI service.

The key indicators and desired objectives are explained below: 

 - `Token update time` It is a time delay between the moment when the token is created or updated in the Vault storage and
the moment when this change is processed by the operator and reflected on the K8S, i.e. the `SPIAccessTokenDataUpdate` being reconciled and applied.
The expected SLO for this metric is that at least 90% of the tokens should be processed in less than 1 second.


 - `Token metadata reconcile time` It's a measure of time which it takes for the operator to retrieve the token metadata from the service provider.
We collect and show it separately per each service provider (GitHub, GitLab, Quay). The expected objective is to fit 90% of requests into 1 second of processing time. 


 - `5xx errors rate` It's a per-service provider ratio of "overall vs 5xx" HTTP calls (typically, to the SP-s API), expressed in percentages.
Expected SLO is to have less than 0.1% of requests with 5xx status code responses per 24h time period.


 - `OAuth flow completion time` This metric counts up the time needed for the user to successfully pass through the OAuth flow on the service provider side UI, 
i.e. the period from when we send him to the OAuth login page to when he is returning to the callback. 
That gives us an idea of how clear is process for the user, the amount of time needed for him to understand which permission is given, what scopes are requested, etc.
The desired objective is to have most OAuth completion times under 30 seconds. 