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

package main

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/alexflint/go-arg"
	"github.com/gorilla/mux"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	authz "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	args := oauth.OAuthServiceCliArgs{}
	arg.MustParse(&args)

	logs.InitLoggers(args.ZapDevel, args.ZapEncoder, args.ZapLogLevel, args.ZapStackTraceLevel, args.ZapTimeEncoding)

	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("Starting OAuth service with environment", "env", os.Environ(), "configuration", &args)

	cfg, err := oauth.LoadOAuthServiceConfiguration(args)
	if err != nil {
		setupLog.Error(err, "failed to initialize the configuration")
		os.Exit(1)
	}

	kubeConfig, err := kubernetesConfig(&args)
	if err != nil {
		setupLog.Error(err, "failed to create kubernetes configuration")
		os.Exit(1)
	}

	go metrics.ServeMetrics(context.Background(), args.MetricsAddr)
	router := mux.NewRouter()

	// insecure mode only allowed when the trusted root certificate is not specified...
	if args.KubeInsecureTLS && kubeConfig.TLSClientConfig.CAFile == "" {
		kubeConfig.Insecure = true
	}

	// we can't use the default dynamic rest mapper, because we don't have a token that would enable us to connect
	// to the cluster just yet. Therefore, we need to list all the resources that we are ever going to query using our
	// client here thus making the mapper not reach out to the target cluster at all.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(authz.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), meta.RESTScopeRoot)
	//	mapper.Add(auth.SchemeGroupVersion.WithKind("TokenReview"), meta.RESTScopeRoot)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessTokenDataUpdate"), meta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	cl, err := oauth.CreateClient(kubeConfig, client.Options{
		Mapper: mapper,
	})

	if err != nil {
		setupLog.Error(err, "failed to create kubernetes client")
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
		MetricsRegisterer:           metrics.Registry,
	})
	if err != nil {
		setupLog.Error(err, "failed to create token storage interface")
		os.Exit(1)
	}

	if err := strg.Initialize(context.Background()); err != nil {
		setupLog.Error(err, "failed to login to token storage")
		os.Exit(1)
	}

	tokenUploader := oauth.SpiTokenUploader{
		K8sClient: cl,
		Storage: tokenstorage.NotifyingTokenStorage{
			Client:       cl,
			TokenStorage: strg,
		},
	}

	// the session has 15 minutes timeout and stale sessions are cleaned every 5 minutes
	sessionManager := scs.New()
	sessionManager.Store = memstore.NewWithCleanupInterval(5 * time.Minute)
	sessionManager.IdleTimeout = 15 * time.Minute
	sessionManager.Cookie.Name = "appstudio_spi_session"
	sessionManager.Cookie.SameSite = http.SameSiteNoneMode
	sessionManager.Cookie.Secure = true
	authenticator := oauth.NewAuthenticator(sessionManager, cl)
	stateStorage := oauth.NewStateStorage(sessionManager)
	//static routes first
	router.HandleFunc("/health", oauth.OkHandler).Methods("GET")
	router.HandleFunc("/ready", oauth.OkHandler).Methods("GET")
	router.HandleFunc("/callback_success", oauth.CallbackSuccessHandler).Methods("GET")
	router.HandleFunc("/login", authenticator.Login).Methods("POST")
	router.NewRoute().Path("/{type}/callback").Queries("error", "", "error_description", "").HandlerFunc(oauth.CallbackErrorHandler)
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(oauth.HandleUpload(&tokenUploader)).Methods("POST")

	redirectTpl, err := template.ParseFiles("static/redirect_notice.html")
	if err != nil {
		setupLog.Error(err, "failed to parse the redirect notice HTML template")
		os.Exit(1)
	}

	for _, sp := range cfg.ServiceProviders {
		setupLog.V(1).Info("initializing service provider controller", "type", sp.ServiceProviderType, "url", sp.ServiceProviderBaseUrl)

		controller, err := oauth.FromConfiguration(cfg, sp, authenticator, stateStorage, cl, strg, redirectTpl)
		if err != nil {
			setupLog.Error(err, "failed to initialize controller")
		}

		prefix := strings.ToLower(string(sp.ServiceProviderType))
		router.Handle(fmt.Sprintf("/%s/authenticate", prefix), http.HandlerFunc(controller.Authenticate)).Methods("GET", "POST")
		router.Handle(fmt.Sprintf("/%s/callback", prefix), oauth.NewCompletedFlowMetricHandler(sp, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			controller.Callback(r.Context(), w, r)
		}))).Methods("GET")
	}
	setupLog.Info("Starting the server", "Addr", args.ServiceAddr)
	server := &http.Server{
		Addr: args.ServiceAddr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout:      time.Second * 15,
		ReadTimeout:       time.Second * 15,
		ReadHeaderTimeout: time.Second * 15,
		IdleTimeout:       time.Second * 60,
		Handler:           sessionManager.LoadAndSave(oauth.MiddlewareHandler(metrics.Registry, strings.Split(args.AllowedOrigins, ","), router)),
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := server.ListenAndServe(); err != nil {
			setupLog.Error(err, "failed to start the HTTP server")
		}
	}()
	setupLog.Info("Server is up and running")
	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Waiting for SIGINT (kill -2)
	<-stop
	setupLog.Info("Server got interrupt signal, going to gracefully shutdown the server", "signal", stop)
	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	if err := server.Shutdown(ctx); err != nil {
		setupLog.Error(err, "OAuth server shutdown failed")
		os.Exit(1)
	}
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	setupLog.Info("OAuth server exited properly")
	os.Exit(0)
}

func kubernetesConfig(args *oauth.OAuthServiceCliArgs) (*rest.Config, error) {
	if args.KubeConfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", args.KubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create rest configuration: %w", err)
		}

		return cfg, nil
	} else if args.ApiServer != "" {
		// here we're essentially replicating what is done in rest.InClusterConfig() but we're using our own
		// configuration - this is to support going through an alternative API server to the one we're running with...
		// Note that we're NOT adding the Token or the TokenFile to the configuration here. This is supposed to be
		// handled on per-request basis...
		cfg := rest.Config{}

		apiServerUrl, err := url.Parse(args.ApiServer)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the API server URL: %w", err)
		}

		cfg.Host = "https://" + net.JoinHostPort(apiServerUrl.Hostname(), apiServerUrl.Port())

		tlsConfig := rest.TLSClientConfig{}

		if args.ApiServerCAPath != "" {
			// rest.InClusterConfig is doing this most possibly only for early error handling so let's do the same
			if _, err := certutil.NewPool(args.ApiServerCAPath); err != nil {
				return nil, fmt.Errorf("expected to load root CA config from %s, but got err: %w", args.ApiServerCAPath, err)
			} else {
				tlsConfig.CAFile = args.ApiServerCAPath
			}
		}

		cfg.TLSClientConfig = tlsConfig

		return &cfg, nil
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize in-cluster config: %w", err)
		}
		return cfg, nil
	}
}
