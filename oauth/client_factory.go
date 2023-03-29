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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/codeready-toolchain/api/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
	KubeInsecureTLS        bool
	KubeConfig             string
	ApiServer              string
	ApiServerCAPath        string
	ApiServerWorkspacePath string
}

type ClientFactory struct {
	FactoryConfig ClientFactoryConfig
	HTTPClient    rest.HTTPClient
}

// CreateUserAuthClientForNamespace creates a new client based on the provided configuration. We use this client for k8s requests with user's token (e.g.: SelfSubjectAccessReview).
// Note that configuration is potentially modified during the call.
func (f ClientFactory) CreateUserAuthClientForNamespace(ctx context.Context, namespace string) (AuthenticatingClient, error) {
	// we can't use the default dynamic rest mapper, because we don't have a token that would enable us to connect
	// to the cluster just yet. Therefore, we need to list all the resources that we are ever going to query using our
	// client here thus making the mapper not reach out to the target cluster at all.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(authz.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), meta.RESTScopeRoot)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)

	return f.createClient(ctx, mapper, namespace, true)
}

// CreateInClusterClient creates a new client based on the provided configuration. We use this client for k8s requests with ServiceAccount (e.g.: reading configuration secrets).
func (f ClientFactory) CreateInClusterClient() (client.Client, error) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	return f.createClient(context.TODO(), mapper, "", false)
}

func (f ClientFactory) createClient(ctx context.Context, mapper meta.RESTMapper, namespace string, userAuthentication bool) (client.Client, error) {
	kubeconfig, err := baseKubernetesConfig(ctx, &f.FactoryConfig, namespace, &f.HTTPClient, userAuthentication)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes incluster config: %w", err)
	}

	kubeconfig, err = customizeKubeconfig(kubeconfig, &f.FactoryConfig, userAuthentication)
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
func baseKubernetesConfig(ctx context.Context, args *ClientFactoryConfig, namespace string, httpClient *rest.HTTPClient, userAuthentication bool) (*rest.Config, error) {
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

		if namespace != "" {
			wsPath, err := calculateWorkspaceSubpath(ctx, args, *httpClient, namespace)
			if err != nil {
				return nil, fmt.Errorf("failed to prepare workspace context for client in namespace %s: %w", namespace, err)
			}
			cfg.APIPath = wsPath
		}

		return cfg, nil
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
		}
		return cfg, nil
	}
}

// calculateWorkspaceSubpath fetches all accessible workspaces for user and finds out one that given namespace belongs to,
// and constructs API URL sub-path
func calculateWorkspaceSubpath(ctx context.Context, args *ClientFactoryConfig, client rest.HTTPClient, namespace string) (string, error) {
	lg := log.FromContext(ctx)
	wsApiPath, _ := strings.CutPrefix(args.ApiServerWorkspacePath, "/")
	wsEndpoint := path.Join(args.ApiServer, wsApiPath)
	req, reqErr := http.NewRequestWithContext(ctx, "GET", wsEndpoint, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to create request for the workspace API", "url", wsEndpoint)
		return "", fmt.Errorf("error while constructing HTTP request for workspace context to %s: %w", wsEndpoint, reqErr)
	}
	resp, err := client.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the workspace API", "url", wsEndpoint)
		return "", fmt.Errorf("error performing HTTP request for workspace context to %v: %w", wsEndpoint, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "Failed to close response body doing workspace fetch")
		}
	}()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Print(err.Error())
		}
		wsList := &v1alpha1.WorkspaceList{}
		json.Unmarshal(bodyBytes, wsList)
		for _, ws := range wsList.Items {
			for _, ns := range ws.Status.Namespaces {
				if ns.Name == namespace {
					return path.Join("workspaces", ws.Name), nil
				}
			}
		}
		return "", fmt.Errorf("target workspace not found for namespace %s", namespace)
	} else {
		lg.Info("unexpected return code for workspace api", "url", wsEndpoint, "code", resp.StatusCode)
		return "", fmt.Errorf("bad status (%d) when performing HTTP request for workspace context to %v: %w", resp.StatusCode, wsEndpoint, err)
	}

}
