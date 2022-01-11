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
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"gopkg.in/yaml.v3"
)

type ServiceProviderType string

const (
	ServiceProviderTypeGitHub ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay   ServiceProviderType = "Quay"
	baseUrlEnv                string              = "OAUTH_BASE_URL"
	sharedSecretEnv           string              = "OAUTH_SHARED_SECRET"
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

	// KubeConfigPath is the path to the kube/config file. If left empty, the in-cluster configuration is assumed.
	KubeConfigPath string `yaml:"kubeConfigPath,omitempty"`
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

	// KubernetesClientConfiguration is the kubernetes client configuration to use.
	KubernetesClientConfiguration *rest.Config

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

// KubernetesClient creates a new kubernetes client based on the config. This is either the in-cluster client or
// the client with configuration obtained from the file on `kubeConfigPath`. If no `kubeConfigPath` is provided in the
// persisted configuration, the in-cluster client is assumed.
func (c Configuration) KubernetesClient(opts client.Options) (client.Client, error) {
	return client.New(c.KubernetesClientConfiguration, opts)
}

// Inflate loads the files specified in the persisted configuration and returns a fully initialized configuration
// struct.
func (c PersistedConfiguration) Inflate() (Configuration, error) {
	conf := Configuration{}
	kcfg, err := readKubeConfig(c.KubeConfigPath)
	if err != nil {
		return conf, err
	}

	if val, ok := os.LookupEnv(baseUrlEnv); !ok {
		return conf, fmt.Errorf("'%v' env must be set", baseUrlEnv)
	} else {
		conf.BaseUrl = val
	}

	if val, ok := os.LookupEnv(sharedSecretEnv); !ok {
		return conf, fmt.Errorf("'%v' env must be set", sharedSecretEnv)
	} else {
		conf.SharedSecret = []byte(val)
	}

	conf.KubernetesClientConfiguration = kcfg
	conf.KubernetesAuthAudiences = c.KubernetesAuthAudiences
	conf.ServiceProviders = c.ServiceProviders

	fmt.Printf("using configuration '%+v'", conf)

	return conf, err
}

// KubernetesClientset creates a new kubernetes client. As with `KubernetesClient()` method, this is either created using
// the supplied configuration or the in-cluster config.yaml is used if no explicit configuration file path is provided in the
// persisted configuration.
func (c Configuration) KubernetesClientset() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(c.KubernetesClientConfiguration)
}

// LoadFrom loads the configuration from the provided file-system path. Note that the returned configuration is fully
// initialized with no need to call the Configuration.ParseFiles() method anymore.
func LoadFrom(path string) (PersistedConfiguration, error) {
	file, err := os.Open(path)
	if err != nil {
		return PersistedConfiguration{}, err
	}
	defer file.Close()

	return ReadFrom(file)
}

// ReadFrom reads the configuration from the provided reader. Note that the returned configuration is fully initialized
// with no need to call the Configuration.ParseFiles() method anymore.
func ReadFrom(rdr io.Reader) (PersistedConfiguration, error) {
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

func readKubeConfig(loc string) (*rest.Config, error) {
	if loc == "" {
		return rest.InClusterConfig()
	} else {
		content, err := ioutil.ReadFile(loc)
		if err != nil {
			return nil, err
		}
		return clientcmd.RESTConfigFromKubeConfig(content)
	}
}
