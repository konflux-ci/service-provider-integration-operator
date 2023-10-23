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

package oauth

import (
	"context"
	"errors"
	"fmt"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"
	"golang.org/x/oauth2"
	"html/template"
	"net/http"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

var (
	errUnknownServiceProviderType = errors.New("unknown service provider type")
	errNoOAuthConfiguration       = errors.New("no OAuth configuration found for the service provider")
	errConfigNoOAuth              = errors.New("found sp configuration has no OAuth information. This should never happen, because Operator should find same configuration and thus not generate OAuth URL and we should not get to the OAuth service at all")
)

// Router holds service provider controllers and is responsible for providing matching controller for incoming requests.
type Router struct {
	controller      Controller
	spiSyncStrategy SPIAccessTokenSyncStrategy
	rsSyncStrategy  RemoteSecretSyncStrategy
	OAuthServiceConfiguration

	stateStorage       StateStorage
	InClusterK8sClient client.Client
}

// CallbackRoute route for /oauth/callback requests
type CallbackRoute struct {
	router *Router
}

// AuthenticateRoute route for /oauth/authenticate requests
type AuthenticateRoute struct {
	router *Router
}

// RouterConfiguration configuration needed to create new Router
type RouterConfiguration struct {
	OAuthServiceConfiguration
	Authenticator      *Authenticator
	StateStorage       StateStorage
	ClientFactory      kubernetesclient.K8sClientFactory
	InClusterK8sClient client.Client
	TokenStorage       tokenstorage.TokenStorage
	RedirectTemplate   *template.Template
}

func NewRouter(ctx context.Context, cfg RouterConfiguration, spDefaults []config.ServiceProviderType) (*Router, error) {
	router := &Router{
		controller:   InitController(cfg),
		stateStorage: cfg.StateStorage,
		spiSyncStrategy: SPIAccessTokenSyncStrategy{
			ClientFactory: cfg.ClientFactory,
			TokenStorage:  cfg.TokenStorage,
		},
		rsSyncStrategy: RemoteSecretSyncStrategy{
			ClientFactory: cfg.ClientFactory,
		},
	}

	initializedServiceProviders := map[string]bool{}

	for _, spType := range spDefaults {
		for _, sp := range cfg.ServiceProviders {
			if sp.ServiceProviderType.Name != spType.Name {
				continue
			}

			spHost := spType.DefaultHost
			if sp.ServiceProviderBaseUrl != "" {
				baseUrlParsed, parseUrlErr := url.Parse(sp.ServiceProviderBaseUrl)
				if parseUrlErr != nil {
					return nil, fmt.Errorf("failed to parse service provider url: %w", parseUrlErr)
				}
				spHost = baseUrlParsed.Host
			}

			if _, alreadyInitialized := initializedServiceProviders[spHost]; alreadyInitialized {
				return nil, fmt.Errorf("%w '%s' base url '%s'", errMultipleConfigsForSameHost, spType.Name, spHost)
			} else {
				initializedServiceProviders[spHost] = true
				log.FromContext(ctx).Info("initializing service provider", "type", sp.ServiceProviderType.Name, "url", spHost)
			}
		}
	}

	return router, nil
}

func (r *Router) Callback() *CallbackRoute {
	return &CallbackRoute{router: r}
}

func (r *Router) Authenticate() *AuthenticateRoute {
	return &AuthenticateRoute{router: r}
}

func (r *Router) findController(req *http.Request, veiled bool) (Controller, *oauthstate.OAuthInfo, error) {
	var stateString string
	var err error

	if veiled {
		stateString, err = r.stateStorage.UnveilState(req.Context(), req)
		if err != nil {
			return nil, nil, fmt.Errorf("unknown or invalid state: %w", err)
		}
	} else {
		stateString = req.FormValue("state")
	}

	state := &oauthstate.OAuthInfo{}
	err = oauthstate.ParseInto(stateString, state)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse state string: %w", err)
	}

	if state.IsRemoteSecret {
		r.controller.setSyncStrategy(r.rsSyncStrategy)
	} else {
		r.controller.setSyncStrategy(r.spiSyncStrategy)
	}

	return r.controller, state, nil
}

// redirectUrl constructs the URL to the callback endpoint so that it can be handled by this controller.
func (r *Router) redirectUrl() string {
	if r.OAuthServiceConfiguration.RedirectProxyUrl != "" {
		return r.OAuthServiceConfiguration.RedirectProxyUrl
	}
	return strings.TrimSuffix(r.OAuthServiceConfiguration.BaseUrl, "/") + oauth.CallBackRoutePath
}

// findOauthConfig is responsible for getting oauth configuration of service provider.
// Currently, this can be configured with labeled secret living in namespace together with SPIAccessToken.
// If no such secret is found, global configuration of oauth service is used.
func (r *Router) findOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
	spUrl, urlParseErr := url.Parse(info.ServiceProviderUrl)
	if urlParseErr != nil {
		return nil, fmt.Errorf("failed to parse serviceprovider url: %w", urlParseErr)
	}

	spType, err := config.GetServiceProviderTypeByName(info.ServiceProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to find service provider type: %w", urlParseErr)
	}

	// first try to find configuration in user's secret
	spConfig, errSpConfig := config.SpConfigFromUserSecret(ctx, r.InClusterK8sClient, info.TokenNamespace, spType, spUrl)
	if errSpConfig != nil {
		return nil, fmt.Errorf("failed to create service provider config from user config secret: %w", errSpConfig)
	}

	redirectUrl := r.redirectUrl()
	if spConfig != nil {
		if spConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("user config error: %w", errConfigNoOAuth)
		}
		oauthConfig := spConfig.OAuth2Config
		oauthConfig.RedirectURL = redirectUrl
		return oauthConfig, nil
	}

	// if we don't have user's config, we
	if globalSpConfig := config.SpConfigFromGlobalConfig(&r.SharedConfiguration, spType, config.GetBaseUrl(spUrl)); globalSpConfig != nil {
		if globalSpConfig.OAuth2Config == nil {
			return nil, fmt.Errorf("global config error: %w", errConfigNoOAuth)
		}
		oauthConfig := globalSpConfig.OAuth2Config
		oauthConfig.RedirectURL = redirectUrl
		return oauthConfig, nil
	}

	return nil, fmt.Errorf("%w '%s' url: '%s'", errNoOAuthConfiguration, info.ServiceProviderName, info.ServiceProviderUrl)
}

func (r *CallbackRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, true)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	oauthConfig, err := r.router.findOauthConfig(req.Context(), state)
	if err != nil {
		return
	}
	ctrl.Callback(req.Context(), wrt, req, state, oauthConfig)
	veiledAt, err := r.router.stateStorage.StateVeiledAt(req.Context(), req)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
	}

	FlowCompleteTimeMetric.WithLabelValues(string(state.ServiceProviderName), state.ServiceProviderUrl).Observe(time.Since(veiledAt).Seconds())
}

func (r *AuthenticateRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, false)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	oauthConfig, err := r.router.findOauthConfig(req.Context(), state)
	if err != nil {
		return
	}

	ctrl.Authenticate(wrt, req, state, oauthConfig)
}
