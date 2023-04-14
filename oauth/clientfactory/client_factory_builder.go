//
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

package clientfactory

import (
	"fmt"
	"net/url"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	cli "github.com/redhat-appstudio/service-provider-integration-operator/cmd/oauth/oauthcli"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	authz "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	certutil "k8s.io/client-go/util/cert"

	"net/http"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8sClientFactoryBuilder allows to construct different types of K8S client factories
type K8sClientFactoryBuilder struct {
	Args cli.OAuthServiceCliArgs
}

func (r K8sClientFactoryBuilder) CreateInClusterClientFactory() (clientFactory kubernetesclient.K8sClientFactory, err error) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)
	clientOptions, errClientOptions := clientOptions(mapper)
	if errClientOptions != nil {
		return nil, errClientOptions
	}
	if r.Args.KubeConfig != "" {
		restConfig, err := clientcmd.BuildConfigFromFlags("", r.Args.KubeConfig)
		setInsecure(r.Args, restConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create rest configuration: %w", err)
		}
		return InClusterK8sClientFactory{ClientOptions: clientOptions, RestConfig: restConfig}, nil
	}
	restConfig, err := rest.InClusterConfig()
	setInsecure(r.Args, restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
	}
	return InClusterK8sClientFactory{ClientOptions: clientOptions, RestConfig: restConfig}, nil
}

func (r K8sClientFactoryBuilder) CreateUserAuthClientFactory() (clientFactory kubernetesclient.K8sClientFactory, err error) {
	// we can't use the default dynamic rest mapper, because we don't have a token that would enable us to connect
	// to the cluster just yet. Therefore, we need to list all the resources that we are ever going to query using our
	// client here thus making the mapper not reach out to the target cluster at all.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(authz.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), meta.RESTScopeRoot)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)
	clientOptions, errClientOptions := clientOptions(mapper)
	if errClientOptions != nil {
		return nil, errClientOptions
	}

	if r.Args.ApiServer == "" {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
		}
		setInsecure(r.Args, cfg)
		AugmentConfiguration(cfg)
		return UserAuthK8sClientFactory{ClientOptions: clientOptions, RestConfig: cfg}, nil
	}

	// here we're essentially replicating what is done in rest.InClusterConfig() but we're using our own
	// configuration - this is to support going through an alternative API server to the one we're running with...
	// Note that we're NOT adding the Token or the TokenFile to the configuration here. This is supposed to be
	// handled on per-request basis...
	cfg := &rest.Config{}

	tlsConfig := rest.TLSClientConfig{}

	if r.Args.ApiServerCAPath != "" {
		// rest.InClusterConfig is doing this most possibly only for early error handling so let's do the same
		if _, err := certutil.NewPool(r.Args.ApiServerCAPath); err != nil {
			return nil, fmt.Errorf("expected to load root CA config from %s, but got err: %w", r.Args.ApiServerCAPath, err)
		} else {
			tlsConfig.CAFile = r.Args.ApiServerCAPath
		}
	}

	cfg.TLSClientConfig = tlsConfig
	setInsecure(r.Args, cfg)
	AugmentConfiguration(cfg)
	apiServerUrl, err := url.Parse(r.Args.ApiServer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the API server URL: %w", err)
	}
	cfg.Host = "https://" + apiServerUrl.Host

	return WorkspaceAwareK8sClientFactory{
		ClientOptions: clientOptions,
		RestConfig:    cfg,
		ApiServer:     r.Args.ApiServer,
		HTTPClient: &http.Client{
			Transport: httptransport.HttpMetricCollectingRoundTripper{
				RoundTripper: http.DefaultTransport}}}, nil
}

func setInsecure(args cli.OAuthServiceCliArgs, cfg *rest.Config) {
	// insecure mode only allowed when the trusted root certificate is not specified...
	if args.KubeInsecureTLS && args.ApiServerCAPath == "" {
		cfg.Insecure = true
	}
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
