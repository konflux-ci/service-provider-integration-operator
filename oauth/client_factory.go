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

package oauth

import (
	"fmt"
	"net/url"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	authz "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthenticatingClient is just a typedef that advertises that it is safe to use the WithAuthIntoContext or
// WithAuthFromRequestIntoContext functions with clients having this type.
type AuthenticatingClient client.Client

type ClientFactoryConfig struct {
	KubeInsecureTLS bool
	KubeConfig      string
	ApiServer       string
	ApiServerCAPath string
}

// CreateUserAuthClient creates a new client based on the provided configuration. We use this client for k8s requests with user's token (e.g.: SelfSubjectAccessReview).
// Note that configuration is potentially modified during the call.
func CreateUserAuthClient(args *ClientFactoryConfig) (AuthenticatingClient, error) {
	// we can't use the default dynamic rest mapper, because we don't have a token that would enable us to connect
	// to the cluster just yet. Therefore, we need to list all the resources that we are ever going to query using our
	// client here thus making the mapper not reach out to the target cluster at all.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(authz.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), meta.RESTScopeRoot)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)

	return createClient(args, mapper, true)
}

// CreateInClusterClient creates a new client based on the provided configuration. We use this client for k8s requests with ServiceAccount (e.g.: reading configuration secrets).
func CreateInClusterClient(args *ClientFactoryConfig) (client.Client, error) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	return createClient(args, mapper, false)
}

func createClient(args *ClientFactoryConfig, mapper meta.RESTMapper, userAuthentication bool) (client.Client, error) {
	kubeconfig, err := baseKubernetesConfig(args, userAuthentication)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes incluster config: %w", err)
	}

	kubeconfig, err = customizeKubeconfig(kubeconfig, args, userAuthentication)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes incluster config: %w", err)
	}

	clientOptions, errClientOptions := clientOptions(mapper)
	if errClientOptions != nil {
		return nil, errClientOptions
	}

	cl, err := client.New(kubeconfig, *clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}

	return cl, nil
}

func customizeKubeconfig(kubeconfig *rest.Config, args *ClientFactoryConfig, userAuthentication bool) (*rest.Config, error) {
	// insecure mode only allowed when the trusted root certificate is not specified...
	if args.KubeInsecureTLS && kubeconfig.TLSClientConfig.CAFile == "" {
		kubeconfig.Insecure = true
	}

	if userAuthentication {
		AugmentConfiguration(kubeconfig)
	}

	return kubeconfig, nil
}

func clientOptions(mapper meta.RESTMapper) (*client.Options, error) {
	options := &client.Options{
		Mapper: mapper,
		Scheme: runtime.NewScheme(),
	}

	if err := corev1.AddToScheme(options.Scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}

	if err := v1beta1.AddToScheme(options.Scheme); err != nil {
		return nil, fmt.Errorf("failed to add api to the scheme: %w", err)
	}

	if err := authz.AddToScheme(options.Scheme); err != nil {
		return nil, fmt.Errorf("failed to add authz to the scheme: %w", err)
	}

	return options, nil
}

// baseKubernetesConfig returns proper configuration based on given cli args. `userAuthentication=true` allows setting apiserver url in cli args
// and enables using bearer token in authorization header to authenticate kubernetes requests. This is used for requests we're doing on behalf of end-user.
// If `kubeconfig` is set in cli args, it is always used.
func baseKubernetesConfig(args *ClientFactoryConfig, userAuthentication bool) (*rest.Config, error) {
	if args.KubeConfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", args.KubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create rest configuration: %w", err)
		}

		return cfg, nil
	} else if userAuthentication && args.ApiServer != "" {
		// here we're essentially replicating what is done in rest.InClusterConfig() but we're using our own
		// configuration - this is to support going through an alternative API server to the one we're running with...
		// Note that we're NOT adding the Token or the TokenFile to the configuration here. This is supposed to be
		// handled on per-request basis...
		cfg := &rest.Config{}

		apiServerUrl, err := url.Parse(args.ApiServer)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the API server URL: %w", err)
		}

		cfg.Host = "https://" + apiServerUrl.Host

		tlsConfig := rest.TLSClientConfig{}

		if args.ApiServerCAPath != "" {
			// rest.InClusterConfig is doing this most possibly only for early error handling so let's do the same
			if _, err := certutil.NewPool(args.ApiServerCAPath); err != nil {
				return nil, fmt.Errorf("expected to load root CA config from %s, but got err: %w", args.ApiServerCAPath, err)
			} else {
				tlsConfig.CAFile = args.ApiServerCAPath
			}
		}

		cfg.TLSClientConfig = tlsConfig

		return cfg, nil
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
		}
		return cfg, nil
	}
}
