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
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var unknownServiceProviderError = errors.New("unknown service provider")

type Router struct {
	controllers map[config.ServiceProviderType]Controller

	stateStorage *StateStorage
}

type CallbackRoute struct {
	router *Router
}

type AuthenticateRoute struct {
	router *Router
}

type RouterConfiguration struct {
	OAuthServiceConfiguration
	Authenticator    *Authenticator
	StateStorage     *StateStorage
	K8sClient        client.Client
	TokenStorage     tokenstorage.TokenStorage
	RedirectTemplate *template.Template
}

type serviceProviderDefaults struct {
	spType   config.ServiceProviderType
	endpoint oauth2.Endpoint
	baseUrl  string
}

// these are default values for all service providers we support
var spDefaults = []serviceProviderDefaults{
	{
		spType:   config.ServiceProviderTypeGitHub,
		endpoint: github.Endpoint,
		baseUrl:  githubUrlBaseHost,
	},
	{
		spType:   config.ServiceProviderTypeQuay,
		endpoint: quayEndpoint,
		baseUrl:  quayUrlBaseHost,
	},
	{
		spType:   config.ServiceProviderTypeGitLab,
		endpoint: gitlabEndpoint,
		baseUrl:  gitlabUrlBaseHost,
	},
}

func NewRouter(lg *logr.Logger, cfg RouterConfiguration) (*Router, error) {
	router := &Router{
		controllers:  map[config.ServiceProviderType]Controller{},
		stateStorage: cfg.StateStorage,
	}

	for _, sp := range spDefaults {
		if controller, initControllerErr := InitController(lg, sp.spType, cfg, sp.baseUrl, sp.endpoint); initControllerErr == nil {
			router.controllers[sp.spType] = controller
		} else {
			return nil, fmt.Errorf("failed to initialize controller '%s': %w", sp.spType, initControllerErr)
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
			return nil, nil, err
		}
	} else {
		stateString = req.FormValue("state")
	}

	state := &oauthstate.OAuthInfo{}
	err = oauthstate.ParseInto(stateString, state)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse state string: %w", err)
	}

	controller := r.controllers[state.ServiceProviderType]
	if controller == nil {
		return nil, nil, fmt.Errorf("%w: type '%s', base URL '%s'", unknownServiceProviderError, state.ServiceProviderType, state.ServiceProviderUrl)
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
}

func (r *AuthenticateRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, false)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	ctrl.Authenticate(wrt, req, state)
}
