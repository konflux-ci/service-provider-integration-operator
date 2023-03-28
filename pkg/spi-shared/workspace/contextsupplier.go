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

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type key int

const (
	wsKey key = iota
)

type ContextSupplier struct {
	ApiServerUrl string
	HTTPClient   rest.HTTPClient
}

// NewContext calculates and puts a workspace name into a context.
// Note that inbound context MUST contain an Auth info (i.e. K8S token set) to have access to the API
func (c ContextSupplier) NewContext(ctx context.Context, namespace string) (context.Context, error) {
	if c.ApiServerUrl == "" {
		return ctx, nil // No API to call, assuming we're in non workspace-aware environment
	}
	workspace, err := c.calculateWorkspace(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to detect workspace name from namespace %s: %w", namespace, err)
	}
	return context.WithValue(ctx, wsKey, workspace), nil
}

// FromContext extracts a workspace name from the context
func FromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(wsKey).(string)
	return s, ok
}

func (c ContextSupplier) calculateWorkspace(ctx context.Context, namespace string) (string, error) {
	lg := log.FromContext(ctx)
	wsEndpoint := path.Join(c.ApiServerUrl, "apis/toolchain.dev.openshift.com/v1alpha1/workspaces")
	req, reqErr := http.NewRequestWithContext(ctx, "GET", wsEndpoint, nil)
	if reqErr != nil {
		lg.Error(reqErr, "failed to create request for the workspace API", "url", wsEndpoint)
		return "", fmt.Errorf("error while constructing HTTP request for workspace context to %s: %w", wsEndpoint, reqErr)
	}
	resp, err := c.HTTPClient.Do(req)
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
					return ws.Name, nil
				}
			}
		}
		return "", fmt.Errorf("target workspace not found for namespace %s", namespace)
	} else {
		lg.Info("unexpected return code for workspace api", "url", wsEndpoint, "code", resp.StatusCode)
		return "", fmt.Errorf("bad status (%d) when performing HTTP request for workspace context to %v: %w", resp.StatusCode, wsEndpoint, err)
	}

}
