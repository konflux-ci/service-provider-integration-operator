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
	"html/template"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

var errUnknownServiceProviderType = errors.New("unknown service provider type")

// Router holds service provider controllers and is responsible for providing matching controller for incoming requests.
type Router struct {
	controllers map[config.ServiceProviderName]Controller

	stateStorage StateStorage
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
		controllers:  map[config.ServiceProviderName]Controller{},
		stateStorage: cfg.StateStorage,
	}

	for _, sp := range spDefaults {
		if controller, initControllerErr := InitController(ctx, sp, cfg); initControllerErr == nil {
			router.controllers[sp.Name] = controller
		} else {
			return nil, fmt.Errorf("failed to initialize controller '%s': %w", sp.Name, initControllerErr)
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

	controller := r.controllers[state.ServiceProviderName]
	if controller == nil {
		return nil, nil, fmt.Errorf("%w: type '%s', base URL '%s'", errUnknownServiceProviderType, state.ServiceProviderName, state.ServiceProviderUrl)
	}

	return controller, state, nil
}

func (r *CallbackRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, true)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	ctrl.Callback(req.Context(), wrt, req, state)

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

	ctrl.Authenticate(wrt, req, state)
}
