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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/redhat-appstudio/service-provider-integration-operator/cmd"
	cli "github.com/redhat-appstudio/service-provider-integration-operator/cmd/oauth/oauthcli"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/alexedwards/scs/v2/memstore"
	"github.com/alexflint/go-arg"
	"github.com/gorilla/mux"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth"
	oauth2 "github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	args := cli.OAuthServiceCliArgs{}
	arg.MustParse(&args)

	logs.InitLoggers(args.ZapDevel, args.ZapEncoder, args.ZapLogLevel, args.ZapStackTraceLevel, args.ZapTimeEncoding)

	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("Starting OAuth service with environment", "env", os.Environ(), "configuration", &args)

	var err error
	err = config.SetupCustomValidations(config.CustomValidationOptions{AllowInsecureURLs: args.AllowInsecureURLs})
	if err != nil {
		setupLog.Error(err, "failed to initialize the validators")
		os.Exit(1)
	}

	cfg, err := loadOAuthServiceConfiguration(args)
	if err != nil {
		setupLog.Error(err, "failed to initialize the configuration")
		os.Exit(1)
	}

	go metrics.ServeMetrics(context.Background(), args.MetricsAddr)
	router := mux.NewRouter()

	clientFactoryConfig := createClientFactoryConfig(args)

	userAuthClient, errUserAuthClient := oauth.CreateUserAuthClient(&clientFactoryConfig)
	if errUserAuthClient != nil {
		setupLog.Error(errUserAuthClient, "failed to create user auth kubernetes client")
		os.Exit(1)
	}

	inClusterK8sClient, errK8sClient := oauth.CreateInClusterClient(&clientFactoryConfig)
	if errK8sClient != nil {
		setupLog.Error(errK8sClient, "failed to create ServiceAccount k8s client")
		os.Exit(1)
	}

	strg, err := cmd.InitTokenStorage(context.Background(), &args.CommonCliArgs)
	if err != nil {
		setupLog.Error(err, "failed to initialize the token storage")
		os.Exit(1)
	}

	tokenUploader := oauth.SpiTokenUploader{
		K8sClient: userAuthClient,
		Storage: tokenstorage.NotifyingTokenStorage{
			Client:       userAuthClient,
			TokenStorage: strg,
		},
	}

	// the session has 15 minutes timeout and stale sessions are cleaned every 5 minutes
	sessionManager := scs.New()
	sessionManager.Store = memstore.NewWithCleanupInterval(5 * time.Minute)
	sessionManager.IdleTimeout = 15 * time.Minute
	sessionManager.Lifetime = time.Hour
	sessionManager.Cookie.Persist = false
	sessionManager.Cookie.Name = "appstudio_spi_session"
	sessionManager.Cookie.SameSite = http.SameSiteNoneMode
	sessionManager.Cookie.Secure = true
	authenticator := oauth.NewAuthenticator(sessionManager, userAuthClient)
	stateStorage := oauth.NewStateStorage(sessionManager)

	// service state routes
	router.HandleFunc("/health", oauth.OkHandler).Methods("GET")
	router.HandleFunc("/ready", oauth.OkHandler).Methods("GET")

	// auth
	router.HandleFunc("/login", authenticator.Login).Methods("POST")
	router.HandleFunc("/logout", authenticator.Logout).Methods("POST")

	// token upload
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(oauth.HandleUpload(&tokenUploader)).Methods("POST")

	// oauth
	redirectTpl, templateErr := template.ParseFiles("static/redirect_notice.html")
	if templateErr != nil {
		setupLog.Error(templateErr, "failed to parse the redirect notice HTML template")
		os.Exit(1)
	}
	routerCfg := oauth.RouterConfiguration{
		OAuthServiceConfiguration: cfg,
		Authenticator:             authenticator,
		StateStorage:              stateStorage,
		UserAuthK8sClient:         userAuthClient,
		InClusterK8sClient:        inClusterK8sClient,
		TokenStorage:              strg,
		RedirectTemplate:          redirectTpl,
	}
	oauthRouter, routerErr := oauth.NewRouter(context.Background(), routerCfg, config.SupportedServiceProviderTypes)
	if routerErr != nil {
		setupLog.Error(routerErr, "failed to initialize oauth router")
		os.Exit(1)
	}

	router.Handle("/callback_success", oauth.CSPHandler(oauth.CallbackSuccessHandler())).Methods("GET")
	router.NewRoute().Path(oauth2.CallBackRoutePath).Queries("error", "", "error_description", "").Handler(oauth.CSPHandler(oauth.CallbackErrorHandler()))
	router.NewRoute().Path(oauth2.CallBackRoutePath).Handler(oauthRouter.Callback())
	router.NewRoute().Path(oauth2.AuthenticateRoutePath).Handler(oauth.CSPHandler(oauthRouter.Authenticate()))

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

func loadOAuthServiceConfiguration(args cli.OAuthServiceCliArgs) (oauth.OAuthServiceConfiguration, error) {
	baseCfg, err := config.LoadFrom(args.ConfigFile, args.BaseUrl)
	if err != nil {
		return oauth.OAuthServiceConfiguration{}, fmt.Errorf("failed to load the configuration from file %s: %w", args.ConfigFile, err)
	}

	return oauth.OAuthServiceConfiguration{SharedConfiguration: baseCfg}, nil
}

func createClientFactoryConfig(args cli.OAuthServiceCliArgs) oauth.ClientFactoryConfig {
	return oauth.ClientFactoryConfig{
		KubeConfig:      args.KubeConfig,
		KubeInsecureTLS: args.KubeInsecureTLS,
		ApiServer:       args.ApiServer,
		ApiServerCAPath: args.ApiServerCAPath,
	}
}
