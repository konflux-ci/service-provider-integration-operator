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
)

type ServiceProviderConfiguration struct {
	ClientId               string              `yaml:"clientId"`
	ClientSecret           string              `yaml:"clientSecret"`
	RedirectUrl            string              `yaml:"redirectUrl"`
	ServiceProviderType    ServiceProviderType `yaml:"type"`
	ServiceProviderBaseUrl string              `yaml:"baseUrl,omitempty"`
}

// Configuration contains the specification of the known service providers as well as other configuration data shared
// between the SPI OAuth service and the SPI operator
type Configuration struct {
	// ServiceProviders is the list of configuration options for the individual service providers
	ServiceProviders []ServiceProviderConfiguration

	// SharedSecret is the secret value used for signing the JWT keys. This is configured by specifying the
	// `sharedSecretFile` in the persisted configuration with the path to the file containing the secret.
	SharedSecret []byte

	//KubernetesAuthAudiences is the list of audiences used when performing the token reviews with the Kubernetes API.
	// Can be left empty if not needed.
	KubernetesAuthAudiences []string

	KubernetesClientConfiguration rest.Config
}

// KubernetesClient is the kubernetes client to be used. This is either the in-cluster client or the client with
// configuration obtained from the file on `kubeConfigPath`. If no `kubeConfigPath` is provided in the persisted
// configuration, the in-cluster client is assumed.
func (c Configuration) KubernetesClient(opts client.Options) (client.Client, error) {
	return client.New(&c.KubernetesClientConfiguration, opts)
}

func (c Configuration) KubernetesClientset() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(&c.KubernetesClientConfiguration)
}

type persistedConfiguration struct {
	ServiceProviders        []ServiceProviderConfiguration `yaml:"serviceProviders"`
	SharedSecretFile        string                         `yaml:"sharedSecretFile"`
	KubeConfigPath          string                         `yaml:"kubeConfigPath,omitempty"`
	KubernetesAuthAudiences []string                       `yaml:"kubernetesAuthAudiences,omitempty"`
}

func LoadFrom(path string) (Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return Configuration{}, err
	}
	defer file.Close()

	return ReadFrom(file)
}

func ReadFrom(rdr io.Reader) (Configuration, error) {
	cfg := persistedConfiguration{}

	ret := Configuration{}

	bytes, err := ioutil.ReadAll(rdr)
	if err != nil {
		return ret, err
	}

	if err := yaml.Unmarshal(bytes, &cfg); err != nil {
		return ret, err
	}

	kcfg, err := readKubeConfig(cfg.KubeConfigPath)
	if err != nil {
		return ret, err
	}

	ret.KubernetesClientConfiguration = *kcfg
	ret.ServiceProviders = cfg.ServiceProviders
	ret.KubernetesAuthAudiences = cfg.KubernetesAuthAudiences

	ret.SharedSecret, err = ioutil.ReadFile(cfg.SharedSecretFile)

	return ret, err
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
