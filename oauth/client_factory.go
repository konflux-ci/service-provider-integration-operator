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
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	errUnparseableResponse  = errors.New("unable to parse workspace  API response for namespace")
	errWorkspaceNotFound    = errors.New("target workspace not found for namespace")
	errClientInstanceNotSet = errors.New("client instance does not set")
)

type K8sClientFactory interface {
	CreateClient(ctx context.Context) (client.Client, error)
}

type InClusterK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
}

type WorkspaceAwareK8sClientFactory struct {
	ClientOptions    *client.Options
	RestConfig       *rest.Config
	ApiServer        string
	WorkspaceApiPath string
	HTTPClient       rest.HTTPClient
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
	wsEndpoint, err := url.JoinPath(w.ApiServer, strings.TrimPrefix(w.WorkspaceApiPath, "/"))
	if err != nil {
		lg.Error(err, "failed to create the workspace API path", "api", w.ApiServer, "path", w.WorkspaceApiPath)
		return nil, fmt.Errorf("failed to create the workspace API path from URL %s and path %s: %w", w.ApiServer, w.WorkspaceApiPath, err)
	}
	req, reqErr := http.NewRequestWithContext(ctx, "GET", wsEndpoint, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to create request for the workspace API", "url", wsEndpoint)
		return nil, fmt.Errorf("error while constructing HTTP request for workspace context to %s: %w", wsEndpoint, reqErr)
	}
	resp, err := w.HTTPClient.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the workspace API", "url", wsEndpoint)
		return nil, fmt.Errorf("error performing HTTP request for workspace context to %s: %w", wsEndpoint, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			lg.Error(err, "Failed to close response body doing workspace fetch")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		lg.Info("unexpected return code for workspace api", "url", wsEndpoint, "code", resp.StatusCode)
		return nil, fmt.Errorf("bad status (%d) when performing HTTP request for workspace context to %v: %w", resp.StatusCode, wsEndpoint, err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		lg.Error(err, "failed to read the workspace API response", "error", err.Error())
		return nil, fmt.Errorf("failed to read the workspace API response: %w", err)
	}

	wsList := &v1alpha1.WorkspaceList{}
	if err := json.Unmarshal(bodyBytes, wsList); err != nil {
		return nil, fmt.Errorf("%w '%s': %s", errUnparseableResponse, namespace, err.Error())
	}
	for _, ws := range wsList.Items {
		for _, ns := range ws.Status.Namespaces {
			if ns.Name == namespace {
				w.RestConfig.Host, err = url.JoinPath(w.RestConfig.Host, "workspaces", ws.Name)
				if err != nil {
					lg.Error(err, "failed to create the K8S API path", "api", w.RestConfig.Host, "workspace", ws.Name)
					return nil, fmt.Errorf("failed to create the K8S API path from URL %s and workspace %s: %w", w.RestConfig.Host, ws.Name, err)
				}
				return doCreateClient(w.RestConfig, *w.ClientOptions)
			}
		}
	}
	return nil, fmt.Errorf("%w: %s", errWorkspaceNotFound, namespace)
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
