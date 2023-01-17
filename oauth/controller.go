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
	"fmt"
	"net/http"
	"net/url"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func InitController(ctx context.Context, spType config.ServiceProviderType, cfg RouterConfiguration) (Controller, error) {
	lg := log.FromContext(ctx)

	// use the notifying token storage to automatically inform the cluster about changes in the token storage
	ts := &tokenstorage.NotifyingTokenStorage{
		Client:       cfg.K8sClient,
		TokenStorage: cfg.TokenStorage,
	}

	controller := &commonController{
		OAuthServiceConfiguration: cfg.OAuthServiceConfiguration,
		K8sClient:                 cfg.K8sClient,
		TokenStorage:              ts,
		Authenticator:             cfg.Authenticator,
		StateStorage:              cfg.StateStorage,
		RedirectTemplate:          cfg.RedirectTemplate,
		ServiceProviderType:       spType,
	}

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

		lg.Info("initializing service provider controller", "type", sp.ServiceProviderType.Name, "url", spHost)
	}

	return controller, nil
}
