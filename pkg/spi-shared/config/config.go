// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ServiceProviderType string

const (
	ServiceProviderTypeGitHub          ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay            ServiceProviderType = "Quay"
	ServiceProviderTypeHostCredentials ServiceProviderType = "HostCredentials"
)

type VaultAuthMethod string

const (
	VaultAuthMethodKubernetes VaultAuthMethod = "kubernetes"
	VaultAuthMethodApprole    VaultAuthMethod = "approle"
)

// SharedCliArgs are the command line arguments and environment variable definitions understood by the configuration
// infrastructure shared between the operator and the oauth service.
type SharedCliArgs struct {
	MetricsAddr                    string          `arg:"-m, --metrics-bind-address, env" default:":8080" help:"The address the metric endpoint binds to."`
	ProbeAddr                      string          `arg:"-h, --health-probe-bind-address, env" default:":8081" help:"The address the probe endpoint binds to."`
	EnableLeaderElection           bool            `arg:"-l, --leader-elect, env" default:"false" help:"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager."`
	ConfigFile                     string          `arg:"-c, --config-file, env" default:"/etc/spi/config.yaml" help:"The location of the configuration file."`
	BaseUrl                        string          `arg:"-b, --base-url, env" help:"The externally accessible URL on which the OAuth service is listening. This is used to construct manual-upload and OAuth URLs"`
	VaultHost                      string          `arg:"--vault-host, env" default:"http://spi-vault:8200" help:"Vault host URL. Default is internal kubernetes service."`
	VaultInsecureTLS               bool            `arg:"-i, --vault-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure vault tls connection."`
	VaultAuthMethod                VaultAuthMethod `arg:"--vault-auth-method, env" default:"kubernetes" help:"Authentication method to Vault token storage. Options: 'kubernetes', 'approle'."`
	VaultApproleRoleIdFilePath     string          `arg:"--vault-roleid-filepath, env" default:"/etc/spi/role_id" help:"Used with Vault approle authentication. Filepath with role_id."`
	VaultApproleSecretIdFilePath   string          `arg:"--vault-secretid-filepath, env" default:"/etc/spi/secret_id" help:"Used with Vault approle authentication. Filepath with secret_id."`
	VaultKubernetesSATokenFilePath string          `arg:"--vault-k8s-sa-token-filepath, env" help:"Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development."`
	VaultKubernetesRole            string          `arg:"--vault-k8s-role, env" default:"spi-controller-manager" help:"Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount."`
	ZapDevel                       bool            `arg:"-d, --zap-devel, env" default:"false" help:"Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)"`
	ZapEncoder                     string          `arg:"-e, --zap-encoder, env" default:"" help:"Zap log encoding (‘json’ or ‘console’)"`
	ZapLogLevel                    string          `arg:"-v, --zap-log-level, env" default:"" help:"Zap Level to configure the verbosity of logging"`
	ZapStackTraceLevel             string          `arg:"-s, --zap-stacktrace-level, env" default:"" help:"Zap Level at and above which stacktraces are captured"`
	ZapTimeEncoding                string          `arg:"-t, --zap-time-encoding, env" default:"iso8601" help:"one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'"`
}

// persistedConfiguration is the on-disk format of the configuration that references other files for shared secret
// and the used kube config. It can be Inflate-d into a SharedConfiguration that has these files loaded in memory for easier
// consumption.
type persistedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration `yaml:"serviceProviders"`

	// SharedSecret is secret value used for signing the JWT keys.
	SharedSecret string `yaml:"sharedSecret"`

	// BaseUrl is the URL on which the OAuth service is deployed.
	BaseUrl string `yaml:"baseUrl"`
}

// SharedConfiguration contains the specification of the known service providers as well as other configuration data shared
// between the SPI OAuth service and the SPI operator
type SharedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration

	// BaseUrl is the URL on which the OAuth service is deployed. It is used to compose the redirect URLs for the
	// service providers in the form of `${BASE_URL}/${SP_TYPE}/callback` (e.g. my-host/github/callback).
	BaseUrl string

	// SharedSecret is the secret value used for signing the JWT keys used as OAuth state.
	SharedSecret []byte
}

// ServiceProviderConfiguration contains configuration for a single service provider configured with the SPI. This
// mainly contains config.yaml of the OAuth application within the service provider.
type ServiceProviderConfiguration struct {
	// ClientId is the client ID of the OAuth application that the SPI uses to access the service provider.
	ClientId string `yaml:"clientId"`

	// ClientSecret is the client secret of the OAuth application that the SPI uses to access the service provider.
	ClientSecret string `yaml:"clientSecret"`

	// ServiceProviderType is the type of the service provider. This must be one of the supported values: GitHub, Quay
	ServiceProviderType ServiceProviderType `yaml:"type"`

	// ServiceProviderBaseUrl is the base URL of the service provider. This can be omitted for certain service provider
	// types, like GitHub that only can have 1 well-known base URL.
	ServiceProviderBaseUrl string `yaml:"baseUrl,omitempty"`

	// Extra is the extra configuration required for some service providers to be able to uniquely identify them. E.g.
	// for Quay, we require to know the organization for which the OAuth application is defined for.
	Extra map[string]string `yaml:"extra,omitempty"`
}

// inflate loads the files specified in the persisted configuration and returns a fully initialized configuration
// struct.
func (c persistedConfiguration) inflate() (SharedConfiguration, error) {
	conf := SharedConfiguration{}

	conf.ServiceProviders = c.ServiceProviders
	conf.SharedSecret = []byte(c.SharedSecret)
	conf.BaseUrl = c.BaseUrl
	return conf, nil
}

func ParseDuration(timeString string, defaultValue string) (time.Duration, error) {
	if timeString == "" {
		timeString = defaultValue
	}

	dur, err := time.ParseDuration(timeString)
	if err != nil {
		err = fmt.Errorf("error parsing duration in config file: %w", err)
	}

	return dur, err
}

func LoadFrom(configFile string) (SharedConfiguration, error) {
	cfg := SharedConfiguration{}
	pcfg, err := loadFrom(configFile)
	if err != nil {
		return cfg, err
	}

	cfg, err = pcfg.inflate()
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

// loadFrom loads the configuration from the provided file-system path. Note that the returned configuration is fully
// initialized with no need to call the SharedConfiguration.ParseFiles() method anymore.
func loadFrom(path string) (persistedConfiguration, error) {
	file, err := os.Open(path)
	if err != nil {
		return persistedConfiguration{}, fmt.Errorf("error opening the config file from %s: %w", path, err)
	}
	defer file.Close()

	return readFrom(file)
}

// readFrom reads the configuration from the provided reader. Note that the returned configuration is fully initialized
// with no need to call the SharedConfiguration.ParseFiles() method anymore.
func readFrom(rdr io.Reader) (persistedConfiguration, error) {
	conf := persistedConfiguration{}

	bytes, err := io.ReadAll(rdr)
	if err != nil {
		return conf, fmt.Errorf("error reading the config file: %w", err)
	}

	if err := yaml.Unmarshal(bytes, &conf); err != nil {
		return conf, fmt.Errorf("error parsing the config file as YAML: %w", err)
	}

	return conf, nil
}
