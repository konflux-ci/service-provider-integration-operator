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
	"net"
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

// CreateUserAuthClient creates a new client based on the provided configuration. Note that configuration is potentially
// modified during the call.
func CreateUserAuthClient(args *OAuthServiceCliArgs) (AuthenticatingClient, error) {
	kubeConfig, err := kubernetesConfig(args, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes user authenticated config: %w", err)
	}

	// insecure mode only allowed when the trusted root certificate is not specified...
	if args.KubeInsecureTLS && kubeConfig.TLSClientConfig.CAFile == "" {
		kubeConfig.Insecure = true
	}

	// we can't use the default dynamic rest mapper, because we don't have a token that would enable us to connect
	// to the cluster just yet. Therefore, we need to list all the resources that we are ever going to query using our
	// client here thus making the mapper not reach out to the target cluster at all.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(authz.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), meta.RESTScopeRoot)
	//	mapper.Add(auth.SchemeGroupVersion.WithKind("TokenReview"), meta.RESTScopeRoot)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)

	return createClient(kubeConfig, client.Options{
		Mapper: mapper,
	})
}

func CreateServiceAccountClient(args *OAuthServiceCliArgs) (client.Client, error) {
	kubeConfig, err := kubernetesConfig(args, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes user authenticated config: %w", err)
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	kubeConfig, errKubeConfig := rest.InClusterConfig()
	if errKubeConfig != nil {
		return nil, fmt.Errorf("failed to create incluster kubeconfig: %w", errKubeConfig)
	}

	return createClient(kubeConfig, client.Options{
		Mapper: mapper,
	})
}

func createClient(cfg *rest.Config, options client.Options) (client.Client, error) {
	var err error
	scheme := options.Scheme
	if scheme == nil {
		scheme = runtime.NewScheme()
		options.Scheme = scheme
	}

	if err = corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}

	if err = v1beta1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add api to the scheme: %w", err)
	}

	if err = authz.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add authz to the scheme: %w", err)
	}

	AugmentConfiguration(cfg)
	cl, err := client.New(cfg, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}

	return cl, nil
}

// TODO comment
func kubernetesConfig(args *OAuthServiceCliArgs, userconfig bool) (*rest.Config, error) {
	if args.KubeConfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", args.KubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create rest configuration: %w", err)
		}

		return cfg, nil
	} else if userconfig && args.ApiServer != "" {
		// here we're essentially replicating what is done in rest.InClusterConfig() but we're using our own
		// configuration - this is to support going through an alternative API server to the one we're running with...
		// Note that we're NOT adding the Token or the TokenFile to the configuration here. This is supposed to be
		// handled on per-request basis...
		cfg := rest.Config{}

		apiServerUrl, err := url.Parse(args.ApiServer)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the API server URL: %w", err)
		}

		cfg.Host = "https://" + net.JoinHostPort(apiServerUrl.Hostname(), apiServerUrl.Port())

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

		return &cfg, nil
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
		}
		return cfg, nil
	}
}
