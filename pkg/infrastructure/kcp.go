package infrastructure

import (
	"context"
	"fmt"
	apisv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"os"
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
	return kcp.NewClusterAwareManager(cfg, options)
}

func IsKcp(ctx context.Context, restConfig *rest.Config) bool {
	setupLog := log.FromContext(ctx)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "failed to create discovery client")
		os.Exit(1)
	}
	apiGroupList, err := discoveryClient.ServerGroups()
	if err != nil {
		setupLog.Error(err, "failed to get server groups")
		os.Exit(1)
	}

	for _, group := range apiGroupList.Groups {
		if group.Name == apisv1alpha1.SchemeGroupVersion.Group {
			for _, version := range group.Versions {
				if version.Version == apisv1alpha1.SchemeGroupVersion.Version {
					return true
				}
			}
		}
	}
	return false
}

var (
	noApiExportFoundError    = fmt.Errorf("no APIExport found")
	moreApiExportsFoundError = fmt.Errorf("more than one APIExport found")
	noVirtualWorkspaceError  = fmt.Errorf("status.virtualWorkspaces is empty")
)

func NoVirtualWorkspaceError(apiExport string) error {
	return fmt.Errorf("%w. APIExport '%s'", noVirtualWorkspaceError, apiExport)
}

// restConfigForAPIExport returns a *rest.Config properly configured to communicate with the endpoint for the
// APIExport's virtual workspace.
func restConfigForAPIExport(ctx context.Context, cfg *rest.Config, apiExportName string) (*rest.Config, error) {
	setupLog := log.FromContext(ctx)

	scheme := runtime.NewScheme()
	if err := apisv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding apis.kcp.dev/v1alpha1 to scheme: %w", err)
	}

	apiExportClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("error creating APIExport client: %w", err)
	}

	var apiExport apisv1alpha1.APIExport

	if apiExportName != "" {
		if err := apiExportClient.Get(ctx, types.NamespacedName{Name: apiExportName}, &apiExport); err != nil {
			return nil, fmt.Errorf("error getting APIExport %q: %w", apiExportName, err)
		}
	} else {
		setupLog.Info("api-export-name is empty - listing")
		exports := &apisv1alpha1.APIExportList{}
		if err := apiExportClient.List(ctx, exports); err != nil {
			return nil, fmt.Errorf("error listing APIExports: %w", err)
		}
		if len(exports.Items) == 0 {
			return nil, noApiExportFoundError
		}
		if len(exports.Items) > 1 {
			return nil, moreApiExportsFoundError
		}
		apiExport = exports.Items[0]
	}

	if len(apiExport.Status.VirtualWorkspaces) < 1 {
		return nil, NoVirtualWorkspaceError(apiExportName)
	}

	cfg = rest.CopyConfig(cfg)
	cfg.Host = apiExport.Status.VirtualWorkspaces[0].URL

	return cfg, nil
}
