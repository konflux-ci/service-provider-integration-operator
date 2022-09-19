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
	"strings"

	"gopkg.in/yaml.v3"
)

type ServiceProviderType string

const (
	ServiceProviderTypeGitHub          ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay            ServiceProviderType = "Quay"
	ServiceProviderTypeHostCredentials ServiceProviderType = "HostCredentials"
)

// LoggingCliArgs define the command line arguments for configuring the logging using Zap.
type LoggingCliArgs struct {
	ZapDevel           bool   `arg:"--zap-devel, env" default:"false" help:"Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)"`
	ZapEncoder         string `arg:"--zap-encoder, env" default:"" help:"Zap log encoding (‘json’ or ‘console’)"`
	ZapLogLevel        string `arg:"--zap-log-level, env" default:"" help:"Zap Level to configure the verbosity of logging"`
	ZapStackTraceLevel string `arg:"--zap-stacktrace-level, env" default:"" help:"Zap Level at and above which stacktraces are captured"`
	ZapTimeEncoding    string `arg:"--zap-time-encoding, env" default:"iso8601" help:"one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'"`
}

// CommonCliArgs are the command line arguments and environment variable definitions understood by the configuration
// infrastructure shared between the operator and the oauth service.
type CommonCliArgs struct {
	MetricsAddr string `arg:"--metrics-bind-address, env" default:"127.0.0.1:8080" help:"The address the metric endpoint binds to."`
	ProbeAddr   string `arg:"--health-probe-bind-address, env" default:":8081" help:"The address the probe endpoint binds to."`
	ConfigFile  string `arg:"--config-file, env" default:"/etc/spi/config.yaml" help:"The location of the configuration file."`
	BaseUrl     string `arg:"--base-url, env" help:"The externally accessible URL on which the OAuth service is listening. This is used to construct manual-upload and OAuth URLs"`
}

// persistedConfiguration is the on-disk format of the configuration that references other files for shared secret
// and the used kube config. It can be Inflate-d into a SharedConfiguration that has these files loaded in memory for easier
// consumption.
type persistedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration `yaml:"serviceProviders"`
}

// SharedConfiguration contains the specification of the known service providers as well as other configuration data shared
// between the SPI OAuth service and the SPI operator
type SharedConfiguration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration

	// BaseUrl is the URL on which the OAuth service is deployed. It is used to compose the redirect URLs for the
	// service providers in the form of `${BASE_URL}/${SP_TYPE}/callback` (e.g. my-host/github/callback).
	BaseUrl string
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

// convert converts persisted configuration into the SharedConfiguration instance.
func (c persistedConfiguration) convert() SharedConfiguration {
	conf := SharedConfiguration{}

	conf.ServiceProviders = c.ServiceProviders
	return conf
}

func LoadFrom(args *CommonCliArgs) (SharedConfiguration, error) {
	pcfg, err := loadFrom(args.ConfigFile)
	if err != nil {
		return SharedConfiguration{}, err
	}

	cfg := pcfg.convert()
	cfg.BaseUrl = strings.TrimSuffix(args.BaseUrl, "/")

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
