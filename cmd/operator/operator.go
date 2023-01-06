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
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/alexflint/go-arg"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
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

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
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

func main() {
	args := opconfig.OperatorCliArgs{}
	arg.MustParse(&args)
	logs.InitLoggers(args.ZapDevel, args.ZapEncoder, args.ZapLogLevel, args.ZapStackTraceLevel, args.ZapTimeEncoding)

	setupLog.Info("Starting SPI operator with environment", "env", os.Environ(), "configuration", &args)

	ctx := ctrl.SetupSignalHandler()
	ctx = log.IntoContext(ctx, ctrl.Log)

	mgr, mgrErr := createManager(args)
	if mgrErr != nil {
		setupLog.Error(mgrErr, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := opconfig.LoadFrom(&args)
	if err != nil {
		setupLog.Error(err, "Failed to load the configuration")
		os.Exit(1)
	}

	vaultConfig := tokenstorage.VaultStorageConfigFromCliArgs(&args.VaultCliArgs)
	// use the same metrics registry as the controller-runtime
	vaultConfig.MetricsRegisterer = metrics.Registry
	strg, err := tokenstorage.NewVaultStorage(vaultConfig)
	if err != nil {
		setupLog.Error(err, "failed to initialize the token storage")
		os.Exit(1)
	}

	if err = strg.Initialize(ctx); err != nil {
		setupLog.Error(err, "failed to log in to the token storage")
		os.Exit(1)
	}

	if err = controllers.SetupAllReconcilers(mgr, &cfg, strg, serviceproviders.KnownInitializers()); err != nil {
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

func createManager(args opconfig.OperatorCliArgs) (manager.Manager, error) {
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

	mgr, err := ctrl.NewManager(restConfig, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager %w", err)
	}

	return mgr, nil
}
