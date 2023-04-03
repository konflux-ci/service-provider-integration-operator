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
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/alexflint/go-arg"
	"github.com/redhat-appstudio/service-provider-integration-operator/cmd"
	cli "github.com/redhat-appstudio/service-provider-integration-operator/cmd/operator/operatorcli"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/github"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/gitlab"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/hostcredentials"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/quay"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	sharedconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
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
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	initializers = serviceprovider.NewInitializers()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appstudiov1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	initServiceProviders()
}

func initServiceProviders() {
	initializers.
		AddKnownInitializer(sharedconfig.ServiceProviderTypeGitHub, github.Initializer).
		AddKnownInitializer(sharedconfig.ServiceProviderTypeGitLab, gitlab.Initializer).
		AddKnownInitializer(sharedconfig.ServiceProviderTypeQuay, quay.Initializer).
		AddKnownInitializer(sharedconfig.ServiceProviderTypeHostCredentials, hostcredentials.Initializer)
}

func main() {
	args := cli.OperatorCliArgs{}
	arg.MustParse(&args)
	logs.InitLoggers(args.ZapDevel, args.ZapEncoder, args.ZapLogLevel, args.ZapStackTraceLevel, args.ZapTimeEncoding)

	var err error
	err = sharedconfig.SetupCustomValidations(sharedconfig.CustomValidationOptions{AllowInsecureURLs: args.AllowInsecureURLs})
	if err != nil {
		setupLog.Error(err, "failed to initialize the validators")
		os.Exit(1)
	}

	setupLog.Info("Starting SPI operator with environment", "env", os.Environ(), "configuration", &args)

	ctx := ctrl.SetupSignalHandler()
	ctx = context.WithValue(ctx, config.SPIInstanceIdContextKey, args.CommonCliArgs.SPIInstanceId)
	ctx = log.IntoContext(ctx, ctrl.Log.WithValues("spiInstanceId", args.CommonCliArgs.SPIInstanceId))

	mgr, mgrErr := createManager(args)
	if mgrErr != nil {
		setupLog.Error(mgrErr, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := LoadFrom(&args)
	if err != nil {
		setupLog.Error(err, "Failed to load the configuration")
		os.Exit(1)
	}

	strg, err := cmd.InitTokenStorage(ctx, &args.CommonCliArgs)
	if err != nil {
		setupLog.Error(err, "failed to initialize the token storage")
		os.Exit(1)
	}

	if err = controllers.SetupAllReconcilers(mgr, &cfg, strg, initializers); err != nil {
		setupLog.Error(err, "failed to set up the controllers")
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

func LoadFrom(args *cli.OperatorCliArgs) (opconfig.OperatorConfiguration, error) {
	baseCfg, err := sharedconfig.LoadFrom(args.ConfigFile, args.BaseUrl)
	if err != nil {
		return opconfig.OperatorConfiguration{}, fmt.Errorf("failed to load the configuration file from %s: %w", args.ConfigFile, err)
	}
	ret := opconfig.OperatorConfiguration{SharedConfiguration: baseCfg}

	ret.TokenLookupCacheTtl = args.TokenMetadataCacheTtl
	ret.AccessCheckTtl = args.AccessCheckLifetimeDuration
	ret.AccessTokenTtl = args.TokenLifetimeDuration
	ret.AccessTokenBindingTtl = args.BindingLifetimeDuration
	ret.FileContentRequestTtl = args.FileRequestLifetimeDuration
	ret.TokenMatchPolicy = args.TokenMatchPolicy
	ret.DeletionGracePeriod = args.DeletionGracePeriod
	ret.MaxFileDownloadSize = args.MaxFileDownloadSize
	ret.EnableTokenUpload = args.EnableTokenUpload

	return ret, nil
}

func createManager(args cli.OperatorCliArgs) (manager.Manager, error) {
	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     args.MetricsAddr,
		HealthProbeBindAddress: args.ProbeAddr,
		LeaderElection:         args.EnableLeaderElection,
		LeaderElectionID:       "f5c55e16.appstudio.redhat.org",
		Logger:                 ctrl.Log,
	}
	restConfig := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(restConfig, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager %w", err)
	}

	return mgr, nil
}
