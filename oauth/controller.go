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
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
)

// Controller implements the OAuth flow. There are specific implementations for each service provider type. These
// are usually instances of the commonController with service-provider-specific configuration.
type Controller interface {
	// Authenticate handles the initial OAuth request. It should validate that the request is authenticated in Kubernetes
	// compose the authenticated OAuth state and return a redirect to the service-provider OAuth endpoint with the state.
	Authenticate(w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo)

	// Callback finishes the OAuth flow. It handles the final redirect from the OAuth flow of the service provider.
	Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo)
}

// oauthFinishResult is an enum listing the possible results of authentication during the commonController.finishOAuthExchange
// method.
type oauthFinishResult int

const (
	oauthFinishAuthenticated oauthFinishResult = iota
	oauthFinishK8sAuthRequired
	oauthFinishError
)

var (
	errServiceProviderAlreadyInitialized = errors.New("service provider already initialized")
)

func InitController(lg *logr.Logger, spType config.ServiceProviderType, cfg RouterConfiguration, defaultBaseUrlHost string, defaultEndpoint oauth2.Endpoint) (Controller, error) {
	// use the notifying token storage to automatically inform the cluster about changes in the token storage
	ts := &tokenstorage.NotifyingTokenStorage{
		Client:       cfg.K8sClient,
		TokenStorage: cfg.TokenStorage,
	}

	controller := &commonController{
		K8sClient:               cfg.K8sClient,
		TokenStorage:            ts,
		BaseUrl:                 cfg.BaseUrl,
		Authenticator:           cfg.Authenticator,
		StateStorage:            cfg.StateStorage,
		RedirectTemplate:        cfg.RedirectTemplate,
		ServiceProviderInstance: map[string]oauthConfiguration{},
		ServiceProviderType:     spType,
	}

	for _, sp := range cfg.ServiceProviders {
		if sp.ServiceProviderType != spType {
			continue
		}

		baseUrl := defaultBaseUrlHost
		if sp.ServiceProviderBaseUrl != "" {
			baseUrlParsed, parseUrlErr := url.Parse(sp.ServiceProviderBaseUrl)
			if parseUrlErr != nil {
				return nil, fmt.Errorf("failed to parse service provider url: %w", parseUrlErr)
			}
			baseUrl = baseUrlParsed.Host
		}

		lg.Info("initializing service provider controller", "type", sp.ServiceProviderType, "url", baseUrl)
		if _, alreadyHasBaseUrl := controller.ServiceProviderInstance[baseUrl]; alreadyHasBaseUrl {
			return nil, fmt.Errorf("%w '%s' base url '%s'", errServiceProviderAlreadyInitialized, spType, baseUrl)
		}

		endpoint := defaultEndpoint
		if sp.ServiceProviderBaseUrl != "" {
			endpoint = createDefaultEndpoint(sp.ServiceProviderBaseUrl)
		}

		controller.ServiceProviderInstance[baseUrl] = oauthConfiguration{
			Config:   sp,
			Endpoint: endpoint,
		}
	}

	return controller, nil
}
