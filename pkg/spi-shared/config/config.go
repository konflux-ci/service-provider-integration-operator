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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type spiInstanceIdContextKeyType struct{}

var SPIInstanceIdContextKey = spiInstanceIdContextKeyType{}

type ServiceProviderName string
type ServiceProviderType struct {
	Name                 ServiceProviderName
	DefaultOAuthEndpoint oauth2.Endpoint // default oauth endpoint of the service provider
	DefaultHost          string          // default host of the service provider. ex.: `github.com`
	DefaultBaseUrl       string          // default base url of the service provider, typically scheme+host. ex: `https://github.com`
}

// all service provider types we support, including default values
//
// Note: HostCredentials service provider does not belong here because it's not defined service provider
// that can be configured in any form.
var SupportedServiceProviderTypes []ServiceProviderType = []ServiceProviderType{
	ServiceProviderTypeGitHub,
	ServiceProviderTypeGitLab,
	ServiceProviderTypeQuay,
}

// HostCredentials service provider is used for service provider URLs that we don't support (are not in list of SupportedServiceProviderTypes).
// We can still provide limited functionality for them like manual token upload.
var ServiceProviderTypeHostCredentials ServiceProviderType = ServiceProviderType{
	Name: "HostCredentials",
}

const (
	MetricsNamespace = "redhat_appstudio"
	MetricsSubsystem = "spi"
)

// persistedConfiguration is the on-disk format of the configuration that references other files for shared secret
// and the used kube config. It can be Inflate-d into a SharedConfiguration that has these files loaded in memory for easier
// consumption.
type persistedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []persistedServiceProviderConfiguration `yaml:"serviceProviders"  validate:"omitempty,dive"`
}

// ServiceProviderConfiguration contains configuration for a single service provider configured with the SPI. This
// mainly contains config.yaml of the OAuth application within the service provider.
type persistedServiceProviderConfiguration struct {
	// ClientId is the client ID of the OAuth application that the SPI uses to access the service provider.
	ClientId string `yaml:"clientId"`

	// ClientSecret is the client secret of the OAuth application that the SPI uses to access the service provider.
	ClientSecret string `yaml:"clientSecret"`

	// ServiceProviderName is the type of the service provider. This must be one of the supported values: GitHub, Quay, GitLab
	ServiceProviderName ServiceProviderName `yaml:"type"`

	// ServiceProviderBaseUrl is the base URL of the service provider. This can be omitted for certain service provider
	// types, like GitHub that only can have 1 well-known base URL.
	ServiceProviderBaseUrl string `yaml:"baseUrl,omitempty" validate:"omitempty,https_only"`

	// Extra is the extra configuration required for some service providers to be able to uniquely identify them.
	Extra map[string]string `yaml:"extra,omitempty"`
}

// SharedConfiguration contains the specification of the known service providers as well as other configuration data shared
// between the SPI OAuth service and the SPI operator
type SharedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration `validate:"omitempty,dive"`

	// BaseUrl is the URL on which the OAuth service is deployed. It is used to compose the redirect URLs for the
	// service providers in the form of `${BASE_URL}/oauth/callback` (e.g. my-host/oauth/callback).
	BaseUrl string `validate:"required,https_only"`
}

// ServiceProviderConfiguration contains configuration for a single service provider configured with the SPI. This
// mainly contains config.yaml of the OAuth application within the service provider.
type ServiceProviderConfiguration struct {
	// ServiceProviderType is the type of the service provider.
	ServiceProviderType ServiceProviderType

	// ServiceProviderBaseUrl is the base URL of the service provider. This can be omitted for certain service provider
	// types, like GitHub that only can have 1 well-known base URL.
	ServiceProviderBaseUrl string `validate:"omitempty,https_only"`

	// Extra is the extra configuration required for some service providers to be able to uniquely identify them.
	Extra map[string]string

	// OAuth2Config holds oauth2 configuration of the service provider.
	// It can be nil in case provider does not support OAuth or we don't have it configured.
	OAuth2Config *oauth2.Config
}

// convert converts persisted configuration into the SharedConfiguration instance.
func (persistedConfig persistedConfiguration) convert() (*SharedConfiguration, error) {
	conf := SharedConfiguration{
		ServiceProviders: []ServiceProviderConfiguration{},
	}

	for _, sp := range persistedConfig.ServiceProviders {
		spType, err := GetServiceProviderTypeByName(sp.ServiceProviderName)
		if err != nil {
			return nil, err
		}

		newSp := ServiceProviderConfiguration{
			ServiceProviderType:    spType,
			ServiceProviderBaseUrl: sp.ServiceProviderBaseUrl,
			Extra:                  sp.Extra,
		}

		if sp.ClientId != "" && sp.ClientSecret != "" {
			newSp.OAuth2Config = &oauth2.Config{
				ClientID:     sp.ClientId,
				ClientSecret: sp.ClientSecret,
				Endpoint:     spType.DefaultOAuthEndpoint,
			}
		}

		// if we don't have url defined in configuration, we set default one
		if sp.ServiceProviderBaseUrl == "" {
			newSp.ServiceProviderBaseUrl = spType.DefaultBaseUrl
		}

		conf.ServiceProviders = append(conf.ServiceProviders, newSp)
	}

	// if we don't have all supported service providers explicitly configured by config file, we add it with default values without OAuth configuration
	for _, spDefault := range SupportedServiceProviderTypes {
		explicitlyConfigured := false
		for _, sp := range conf.ServiceProviders {
			if sp.ServiceProviderType.Name == spDefault.Name {
				explicitlyConfigured = true
				break
			}
		}

		if !explicitlyConfigured {
			conf.ServiceProviders = append(conf.ServiceProviders, ServiceProviderConfiguration{
				ServiceProviderType:    spDefault,
				ServiceProviderBaseUrl: spDefault.DefaultBaseUrl,
			})
		}
	}

	return &conf, nil
}

func LoadFrom(configFilePath, baseUrl string) (SharedConfiguration, error) {
	pcfg, err := loadFrom(configFilePath)
	if err != nil {
		return SharedConfiguration{}, err
	}

	cfg, err := pcfg.convert()
	if err != nil {
		return SharedConfiguration{}, err
	}

	cfg.BaseUrl = strings.TrimSuffix(baseUrl, "/")
	err = ValidateStruct(cfg)
	if err != nil {
		return SharedConfiguration{}, fmt.Errorf("service configuration validation failed: %w", err)
	}
	return *cfg, nil
}

// loadFrom loads the configuration from the provided file-system path. Note that the returned configuration is fully
// initialized with no need to call the SharedConfiguration.ParseFiles() method anymore.
func loadFrom(path string) (persistedConfiguration, error) {
	file, err := os.Open(path) // #nosec:G304, path param and file is controlled by operator deployment
	if err != nil {
		return persistedConfiguration{}, fmt.Errorf("error opening the config file from %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()

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

var errUnknownServiceProvider = errors.New("haven't found service provider in supported list")

func GetServiceProviderTypeByName(name ServiceProviderName) (ServiceProviderType, error) {
	for _, sp := range SupportedServiceProviderTypes {
		if sp.Name == name {
			return sp, nil
		}
	}
	return ServiceProviderType{}, fmt.Errorf("serviceprovider '%s': %w", name, errUnknownServiceProvider)
}
