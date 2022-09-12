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

package infrastructure

import (
	"context"
	"fmt"
	"os"

	"github.com/kcp-dev/logicalcluster/v2"

	apisv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewKcpManager(ctx context.Context, restConfig *rest.Config, options ctrl.Options, apiExportName string) (manager.Manager, error) {
	setupLog := log.FromContext(ctx)

	setupLog.Info("Looking up virtual workspace URL")
	cfg, err := restConfigForAPIExport(ctx, restConfig, apiExportName)
	if err != nil {
		setupLog.Error(err, "error looking up virtual workspace URL")
		os.Exit(1)
	}

	setupLog.Info("Using virtual workspace URL", "url", cfg.Host)

	options.LeaderElectionConfig = restConfig
	mgr, err := kcp.NewClusterAwareManager(cfg, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP manager %w", err)
	}
	return mgr, nil
}

func IsKcp(restConfig *rest.Config) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create discovery client %w", err)
	}
	apiGroupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("failed to get server groups %w", err)
	}

	for _, group := range apiGroupList.Groups {
		if group.Name == apisv1alpha1.SchemeGroupVersion.Group {
			for _, version := range group.Versions {
				if version.Version == apisv1alpha1.SchemeGroupVersion.Version {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

var (
	missingApiExportNameError = fmt.Errorf("APIExport name must be provided")
	noVirtualWorkspaceError   = fmt.Errorf("status.virtualWorkspaces is empty")
)

func NoVirtualWorkspaceError(apiExport string) error {
	return fmt.Errorf("%w. APIExport '%s'", noVirtualWorkspaceError, apiExport)
}

// restConfigForAPIExport returns a *rest.Config properly configured to communicate with the endpoint for the
// APIExport's virtual workspace.
func restConfigForAPIExport(ctx context.Context, cfg *rest.Config, apiExportName string) (*rest.Config, error) {
	if apiExportName == "" {
		return nil, missingApiExportNameError
	}

	scheme := runtime.NewScheme()
	if err := apisv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding apis.kcp.dev/v1alpha1 to scheme: %w", err)
	}

	apiExportClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("error creating APIExport client: %w", err)
	}

	var apiExport apisv1alpha1.APIExport

	if err := apiExportClient.Get(ctx, types.NamespacedName{Name: apiExportName}, &apiExport); err != nil {
		return nil, fmt.Errorf("error getting APIExport %q: %w", apiExportName, err)
	}

	if len(apiExport.Status.VirtualWorkspaces) < 1 {
		return nil, NoVirtualWorkspaceError(apiExportName)
	}

	cfg = rest.CopyConfig(cfg)
	cfg.Host = apiExport.Status.VirtualWorkspaces[0].URL

	return cfg, nil
}

func InitKcpControllerContext(ctx context.Context, req ctrl.Request) context.Context {
	// if we're running on kcp, we need to include workspace name in context and logs
	if req.ClusterName != "" {
		ctx = logicalcluster.WithCluster(ctx, logicalcluster.New(req.ClusterName))
		ctx = log.IntoContext(ctx, log.FromContext(ctx, "clusterName", req.ClusterName))
	}

	return ctx
}
