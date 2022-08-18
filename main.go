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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/infrastructure"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

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
	TokenLifetimeDuration          string                       `arg:"--token-ttl, env" default:"120h" help:"Access token lifetime in hours, minutes or seconds. Examples:  \"3h\",  \"5h30m40s\" etc"`
	BindingLifetimeDuration        string                       `arg:"--binding-ttl, env" default:"2h" help:"Access token binding in hours, minutes or seconds. Examples: \"3h\", \"5h30m40s\" etc"`
	VaultHost                      string                       `arg:"--vault-host, env" default:"http://spi-vault:8200" help:"Vault host URL. Default is internal kubernetes service."`
	VaultInsecureTLS               bool                         `arg:"-i, --vault-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure vault tls connection."`
	VaultAuthMethod                tokenstorage.VaultAuthMethod `arg:"--vault-auth-method, env" default:"kubernetes" help:"Authentication method to Vault token storage. Options: 'kubernetes', 'approle'."`
	VaultApproleRoleIdFilePath     string                       `arg:"--vault-roleid-filepath, env" default:"/etc/spi/role_id" help:"Used with Vault approle authentication. Filepath with role_id."`
	VaultApproleSecretIdFilePath   string                       `arg:"--vault-secretid-filepath, env" default:"/etc/spi/secret_id" help:"Used with Vault approle authentication. Filepath with secret_id."`
	VaultKubernetesSATokenFilePath string                       `arg:"--vault-k8s-sa-token-filepath, env" help:"Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development."`
	VaultKubernetesRole            string                       `arg:"--vault-k8s-role, env" default:"spi-controller-manager" help:"Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount."`
	TokenMatchPolicy               sharedConfig.TokenPolicy     `arg:"--token-match-policy, env" default:"any" help:"The policy to match the token against the binding. Options:  'any', 'exact'."`
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

	ctx := ctrl.SetupSignalHandler()

	mgr, mgrErr := createManager(ctx, args)
	if mgrErr != nil {
		setupLog.Error(mgrErr, "unable to start manager")
		os.Exit(1)
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
	cfg.TokenMatchPolicy = args.TokenMatchPolicy

	cfg.AccessTokenTtl, err = time.ParseDuration(args.TokenLifetimeDuration)
	if err != nil {
		setupLog.Error(err, "failed to parse token lifetime duration parameter")
		os.Exit(1)
	}

	cfg.AccessTokenBindingTtl, err = time.ParseDuration(args.BindingLifetimeDuration)
	if err != nil {
		setupLog.Error(err, "failed to parse binding lifetime duration parameter")
		os.Exit(1)
	}

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
		Configuration: cfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SPIAccessTokenBinding")
		os.Exit(1)
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

func createManager(ctx context.Context, args cliArgs) (manager.Manager, error) {
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

	var mgr manager.Manager
	var err error
	if isKcp, isKcpErr := infrastructure.IsKcp(restConfig); isKcpErr == nil {
		if isKcp {
			mgr, err = infrastructure.NewKcpManager(ctx, restConfig, options, args.ApiExportName)
		} else {
			mgr, err = ctrl.NewManager(restConfig, options)
		}
	} else {
		err = isKcpErr
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create manager %w", err)
	}

	return mgr, nil
}
