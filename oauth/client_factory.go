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
	"errors"
	"fmt"
	"github.com/codeready-toolchain/api/api/v1alpha1"
	"io"
	"k8s.io/client-go/rest"
	"net/http"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const wsApiPath = "/apis/toolchain.dev.openshift.com/v1alpha1/workspaces"

var (
	errUnableToParseWorkspaceResponse = errors.New("unable to parse response from workspace API requested for namespace")
	errWorkspaceNotFound              = errors.New("target workspace not found for namespace")
	errClientInstanceNotSet           = errors.New("client instance does not set")
)

type K8sClientFactory interface {
	CreateClient(ctx context.Context) (client.Client, error)
}

type InClusterK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
}

type WorkspaceAwareK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
	ApiServer     string
	HTTPClient    rest.HTTPClient
}

type UserAuthK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
}

func (w WorkspaceAwareK8sClientFactory) CreateClient(ctx context.Context) (client.Client, error) {
	namespace, ok := NamespaceFromContext(ctx)
	if !ok { // no namespace, return simple client
		return doCreateClient(w.RestConfig, *w.ClientOptions)
	}
	lg := log.FromContext(ctx)

	wsName, err := fetchWorkspaceName(ctx, w.ApiServer, namespace, w.HTTPClient)
	if err != nil {
		lg.Error(err, "failed to fetch workspace name via API:", "api", w.RestConfig.Host, "workspace", wsName, "error", err.Error())
		return nil, fmt.Errorf("unable to fetch workspace name via API: %w", err)
	}
	w.RestConfig.Host, err = url.JoinPath(w.RestConfig.Host, "workspaces", wsName)
	if err != nil {
		lg.Error(err, "failed to create the K8S API path", "api", w.RestConfig.Host, "workspace", wsName)
		return nil, fmt.Errorf("failed to create the K8S API path from URL %s and workspace %s: %w", w.RestConfig.Host, wsName, err)
	}
	return doCreateClient(w.RestConfig, *w.ClientOptions)
}

func (u UserAuthK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	return doCreateClient(u.RestConfig, *u.ClientOptions)
}

func (i InClusterK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	return doCreateClient(i.RestConfig, *i.ClientOptions)
}

func doCreateClient(cfg *rest.Config, opts client.Options) (client.Client, error) {
	cl, err := client.New(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}
	return cl, nil
}

// SingleInstanceClientFactory client instance holding factory impl, used in tests only
type SingleInstanceClientFactory struct {
	client client.Client
}

func (c SingleInstanceClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	if c.client != nil {
		return c.client, nil
	} else {
		return nil, errClientInstanceNotSet
	}
}

func fetchWorkspaceName(ctx context.Context, apiServer, namespace string, client rest.HTTPClient) (string, error) {
	lg := log.FromContext(ctx)
	wsEndpoint := apiServer + wsApiPath
	req, reqErr := http.NewRequestWithContext(ctx, "GET", wsEndpoint, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to create request for the workspace API", "url", wsEndpoint)
		return "", fmt.Errorf("error while constructing HTTP request for workspace context to %s: %w", wsEndpoint, reqErr)
	}
	resp, err := client.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the workspace API", "url", wsEndpoint)
		return "", fmt.Errorf("error performing HTTP request for workspace context to %s: %w", wsEndpoint, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "Failed to close response body doing workspace fetch")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		lg.Info("unexpected return code for workspace api", "url", wsEndpoint, "code", resp.StatusCode)
		return "", fmt.Errorf("bad status (%d) when performing HTTP request for workspace context to %v: %w", resp.StatusCode, wsEndpoint, err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		lg.Error(err, "failed to read the workspace API response", "error", err.Error())
		return "", fmt.Errorf("failed to read the workspace API response: %w", err)
	}

	wsList := &v1alpha1.WorkspaceList{}
	if err := json.Unmarshal(bodyBytes, wsList); err != nil {
		return "", fmt.Errorf("%w '%s': %s", errUnableToParseWorkspaceResponse, namespace, err.Error())
	}
	for _, ws := range wsList.Items {
		for _, ns := range ws.Status.Namespaces {
			if ns.Name == namespace {
				return ws.Name, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", errWorkspaceNotFound, namespace)
}
