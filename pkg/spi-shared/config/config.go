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
	"io"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ServiceProviderType string

const (
	ServiceProviderTypeGitHub ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay   ServiceProviderType = "Quay"
	DefaultVaultHost          string              = "http://spi-vault:8200"
)

// PersistedConfiguration is the on-disk format of the configuration that references other files for shared secret
// and the used kube config. It can be Inflate-d into a Configuration that has these files loaded in memory for easier
// consumption.
type PersistedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration `yaml:"serviceProviders"`

	//KubernetesAuthAudiences is the list of audiences used when performing the token reviews with the Kubernetes API.
	// Can be left empty if not needed.
	KubernetesAuthAudiences []string `yaml:"kubernetesAuthAudiences,omitempty"`

	// SharedSecret is secret value used for signing the JWT keys.
	SharedSecret string `yaml:"sharedSecret"`

	// BaseUrl is the URL on which the OAuth service is deployed.
	BaseUrl string `yaml:"baseUrl"`

	// TokenLookupCacheTtl is the time the token lookup results are considered valid. This string expresses the
	// duration as string accepted by the time.ParseDuration function (e.g. "5m", "1h30m", "5s", etc.). The default
	// is 1h (1 hour).
	TokenLookupCacheTtl string `yaml:"tokenLookupCacheTtl"`

	// VaultHost is url to Vault storage. Default `http://spi-vault:8200` which is default spi Vault service name for
	// kubernetes deployments.
	VaultHost string `yaml:"vaultHost"`

	// AccessCheckTtl is the time after that SPIAccessCheck CR will be deleted by operator. This string expresses the
	// duration as string accepted by the time.ParseDuration function (e.g. "5m", "1h30m", "5s", etc.). The default
	// is 30m (30 minutes).
	AccessCheckTtl string `yaml:"accessCheckTtl"`
}

// Configuration contains the specification of the known service providers as well as other configuration data shared
// between the SPI OAuth service and the SPI operator
type Configuration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration

	// BaseUrl is the URL on which the OAuth service is deployed. It is used to compose the redirect URLs for the
	// service providers in the form of `${BASE_URL}/${SP_TYPE}/callback` (e.g. my-host/github/callback).
	BaseUrl string

	//KubernetesAuthAudiences is the list of audiences used when performing the token reviews with the Kubernetes API.
	// Can be left empty if not needed.
	KubernetesAuthAudiences []string

	// SharedSecret is the secret value used for signing the JWT keys used as OAuth state.
	SharedSecret []byte

	// TokenLookupCacheTtl is the time for which the lookup cache results are considered valid
	TokenLookupCacheTtl time.Duration

	// VaultHost url to vault storage.
	VaultHost string

	// ServiceAccountTokenFilePath file with service account token. It is used for Vault kubernetes auth.
	// No need to set when running in pod, but can be useful when running outside, like local dev.
	// It is set with `SA_TOKEN_PATH` environment variable.
	ServiceAccountTokenFilePath string

	// AccessCheckTtl is time after that SPIAccessCheck CR will be deleted.
	AccessCheckTtl time.Duration
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
func (c PersistedConfiguration) inflate() (Configuration, error) {
	conf := Configuration{}

	if c.VaultHost == "" {
		conf.VaultHost = DefaultVaultHost
	} else {
		conf.VaultHost = c.VaultHost
	}

	var parseErr error
	conf.TokenLookupCacheTtl, parseErr = parseDuration(c.TokenLookupCacheTtl, "1h")
	if parseErr != nil {
		return conf, parseErr
	}

	conf.AccessCheckTtl, parseErr = parseDuration(c.AccessCheckTtl, "30m")
	if parseErr != nil {
		return conf, parseErr
	}

	if saTokenPath, ok := os.LookupEnv("SA_TOKEN_PATH"); ok {
		conf.ServiceAccountTokenFilePath = saTokenPath
	}

	conf.KubernetesAuthAudiences = c.KubernetesAuthAudiences
	conf.ServiceProviders = c.ServiceProviders
	conf.SharedSecret = []byte(c.SharedSecret)
	conf.BaseUrl = c.BaseUrl
	return conf, nil
}

func parseDuration(timeString string, defaultValue string) (time.Duration, error) {
	if timeString == "" {
		timeString = defaultValue
	}
	return time.ParseDuration(timeString)
}

func LoadFrom(configFile string) (Configuration, error) {
	cfg := Configuration{}
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
// initialized with no need to call the Configuration.ParseFiles() method anymore.
func loadFrom(path string) (PersistedConfiguration, error) {
	file, err := os.Open(path)
	if err != nil {
		return PersistedConfiguration{}, err
	}
	defer file.Close()

	return readFrom(file)
}

// readFrom reads the configuration from the provided reader. Note that the returned configuration is fully initialized
// with no need to call the Configuration.ParseFiles() method anymore.
func readFrom(rdr io.Reader) (PersistedConfiguration, error) {
	conf := PersistedConfiguration{}

	bytes, err := ioutil.ReadAll(rdr)
	if err != nil {
		return conf, err
	}

	if err := yaml.Unmarshal(bytes, &conf); err != nil {
		return conf, err
	}

	return conf, err
}
