/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/kcp"

	"github.com/alexflint/go-arg"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceproviders"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	corev1 "k8s.io/api/core/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	appstudiov1beta1 "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	//+kubebuilder:scaffold:imports
	apisv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"

	sharedConfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appstudiov1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type cliArgs struct {
	MetricsAddr                    string                       `arg:"-m, --metrics-bind-address, env" default:":8080" help:"The address the metric endpoint binds to."`
	ProbeAddr                      string                       `arg:"-h, --health-probe-bind-address, env" default:":8081" help:"The address the probe endpoint binds to."`
	EnableLeaderElection           bool                         `arg:"-l, --leader-elect, env" default:"false" help:"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager."`
	ConfigFile                     string                       `arg:"-c, --config-file, env" default:"/etc/spi/config.yaml" help:"The location of the configuration file."`
	VaultHost                      string                       `arg:"--vault-host, env" default:"http://spi-vault:8200" help:"Vault host URL. Default is internal kubernetes service."`
	VaultInsecureTLS               bool                         `arg:"-i, --vault-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure vault tls connection."`
	VaultAuthMethod                tokenstorage.VaultAuthMethod `arg:"--vault-auth-method, env" default:"kubernetes" help:"Authentication method to Vault token storage. Options: 'kubernetes', 'approle'."`
	VaultApproleRoleIdFilePath     string                       `arg:"--vault-roleid-filepath, env" default:"/etc/spi/role_id" help:"Used with Vault approle authentication. Filepath with role_id."`
	VaultApproleSecretIdFilePath   string                       `arg:"--vault-secretid-filepath, env" default:"/etc/spi/secret_id" help:"Used with Vault approle authentication. Filepath with secret_id."`
	VaultKubernetesSATokenFilePath string                       `arg:"--vault-k8s-sa-token-filepath, env" help:"Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development."`
	VaultKubernetesRole            string                       `arg:"--vault-k8s-role, env" default:"spi-controller-manager" help:"Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount."`
	ZapDevel                       bool                         `arg:"-d, --zap-devel, env" default:"false" help:"Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)"`
	ZapEncoder                     string                       `arg:"-e, --zap-encoder, env" default:"" help:"Zap log encoding (‘json’ or ‘console’)"`
	ZapLogLevel                    string                       `arg:"-v, --zap-log-level, env" default:"" help:"Zap Level to configure the verbosity of logging"`
	ZapStackTraceLevel             string                       `arg:"-s, --zap-stacktrace-level, env" default:"" help:"Zap Level at and above which stacktraces are captured"`
	ZapTimeEncoding                string                       `arg:"-t, --zap-time-encoding, env" default:"iso8601" help:"one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'"`
	ApiExportName                  string                       `arg:"--kcp-api-export-name, env" default:"spi" help:"SPI ApiExport name used in KCP environment to configure controller with virtual workspace."`
}

func main() {
	args := cliArgs{}
	arg.MustParse(&args)
	logs.InitLoggers(args.ZapDevel, args.ZapEncoder, args.ZapLogLevel, args.ZapStackTraceLevel, args.ZapTimeEncoding)

	setupLog.Info("Starting SPI operator with environment", "env", os.Environ(), "configuration", &args)
	if err := config.ValidateEnv(); err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	var mgr ctrl.Manager
	var err error
	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     args.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: args.ProbeAddr,
		LeaderElection:         args.EnableLeaderElection,
		LeaderElectionID:       "f5c55e16.appstudio.redhat.org",
		Logger:                 ctrl.Log,
	}
	restConfig := ctrl.GetConfigOrDie()

	if kcpAPIsGroupPresent(restConfig) {
		setupLog.Info("Looking up virtual workspace URL")
		cfg, err := restConfigForAPIExport(ctx, restConfig, args.ApiExportName)
		if err != nil {
			setupLog.Error(err, "error looking up virtual workspace URL")
			os.Exit(1)
		}

		setupLog.Info("Using virtual workspace URL", "url", cfg.Host)

		options.LeaderElectionConfig = restConfig
		mgr, err = kcp.NewClusterAwareManager(cfg, options)
		if err != nil {
			setupLog.Error(err, "unable to start cluster aware manager")
			os.Exit(1)
		}
	} else {
		mgr, err = ctrl.NewManager(restConfig, options)
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}
	}

	cfg, err := sharedConfig.LoadFrom(args.ConfigFile)
	if err != nil {
		setupLog.Error(err, "Failed to load the configuration")
		os.Exit(1)
	}

	strg, err := tokenstorage.NewVaultStorage(&tokenstorage.VaultStorageConfig{
		Host:                        args.VaultHost,
		AuthType:                    args.VaultAuthMethod,
		Insecure:                    args.VaultInsecureTLS,
		Role:                        args.VaultKubernetesRole,
		ServiceAccountTokenFilePath: args.VaultKubernetesSATokenFilePath,
		RoleIdFilePath:              args.VaultApproleRoleIdFilePath,
		SecretIdFilePath:            args.VaultApproleSecretIdFilePath,
	})
	if err != nil {
		setupLog.Error(err, "failed to initialize the token storage")
		os.Exit(1)
	}

	if config.RunControllers() {
		if err = (&controllers.SPIAccessTokenReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			TokenStorage: strg,
			ServiceProviderFactory: serviceprovider.Factory{
				Configuration:    cfg,
				KubernetesClient: mgr.GetClient(),
				HttpClient:       http.DefaultClient,
				Initializers:     serviceproviders.KnownInitializers(),
				TokenStorage:     strg,
			},
			Configuration: cfg,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "SPIAccessToken")
			os.Exit(1)
		}
		if err = (&controllers.SPIAccessTokenBindingReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			TokenStorage: strg,
			ServiceProviderFactory: serviceprovider.Factory{
				Configuration:    cfg,
				KubernetesClient: mgr.GetClient(),
				HttpClient:       http.DefaultClient,
				Initializers:     serviceproviders.KnownInitializers(),
				TokenStorage:     strg,
			},
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "SPIAccessTokenBinding")
			os.Exit(1)
		}
	} else {
		setupLog.Info("CRD controllers inactive")
	}

	if err = (&controllers.SPIAccessCheckReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		ServiceProviderFactory: serviceprovider.Factory{
			Configuration:    cfg,
			KubernetesClient: mgr.GetClient(),
			HttpClient:       http.DefaultClient,
			Initializers:     serviceproviders.KnownInitializers(),
			TokenStorage:     strg,
		},
		Configuration: cfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SPIAccessCheck")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// restConfigForAPIExport returns a *rest.Config properly configured to communicate with the endpoint for the
// APIExport's virtual workspace.
func restConfigForAPIExport(ctx context.Context, cfg *rest.Config, apiExportName string) (*rest.Config, error) {
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
			return nil, fmt.Errorf("no APIExport found")
		}
		if len(exports.Items) > 1 {
			return nil, fmt.Errorf("more than one APIExport found")
		}
		apiExport = exports.Items[0]
	}

	if len(apiExport.Status.VirtualWorkspaces) < 1 {
		return nil, fmt.Errorf("APIExport %q status.virtualWorkspaces is empty", apiExportName)
	}

	cfg = rest.CopyConfig(cfg)
	cfg.Host = apiExport.Status.VirtualWorkspaces[0].URL

	return cfg, nil
}

func kcpAPIsGroupPresent(restConfig *rest.Config) bool {
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
