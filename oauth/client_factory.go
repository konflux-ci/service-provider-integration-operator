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
	"path"
	"strings"

	"github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	errUnparseableResponse = errors.New("unable to parse workspace  API response for namespace")
	errWorkspaceNotFound   = errors.New("target workspace not found for namespace")
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
	if !ok {
		// no namespace, return simple client
		cl, err := client.New(w.RestConfig, *w.ClientOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
		}
		return cl, nil
	}
	lg := log.FromContext(ctx)
	wsEndpoint := path.Join(w.ApiServer, strings.TrimPrefix(w.WorkspaceApiPath, "/"))
	req, reqErr := http.NewRequestWithContext(ctx, "GET", wsEndpoint, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to create request for the workspace API", "url", wsEndpoint)
		return nil, fmt.Errorf("error while constructing HTTP request for workspace context to %s: %w", wsEndpoint, reqErr)
	}
	resp, err := w.HTTPClient.Do(req)
	if err != nil {
		lg.Error(err, "failed to request the workspace API", "url", wsEndpoint)
		return nil, fmt.Errorf("error performing HTTP request for workspace context to %v: %w", wsEndpoint, err)
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
		if err := json.Unmarshal(bodyBytes, wsList); err != nil {
			return nil, fmt.Errorf("%w: %s", errUnparseableResponse, namespace)
		}
		for _, ws := range wsList.Items {
			for _, ns := range ws.Status.Namespaces {
				if ns.Name == namespace {
					w.RestConfig.APIPath = path.Join("workspaces", ws.Name)
					cl, err := client.New(w.RestConfig, *w.ClientOptions)
					if err != nil {
						return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
					}
					return cl, nil
				}
			}
		}
		return nil, fmt.Errorf("%w: %s", errWorkspaceNotFound, namespace)
	} else {
		lg.Info("unexpected return code for workspace api", "url", wsEndpoint, "code", resp.StatusCode)
		return nil, fmt.Errorf("bad status (%d) when performing HTTP request for workspace context to %v: %w", resp.StatusCode, wsEndpoint, err)
	}

}

func (u UserAuthK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {

	cl, err := client.New(u.RestConfig, *u.ClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}
	return cl, nil
}

func (i InClusterK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	cl, err := client.New(i.RestConfig, *i.ClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}
	return cl, nil
}
