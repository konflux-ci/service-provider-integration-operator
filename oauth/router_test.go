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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prometheusTest "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

var state = &oauthstate.OAuthInfo{
	TokenName:           "mytoken",
	TokenNamespace:      IT.Namespace,
	Scopes:              []string{"a", "b"},
	ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
	ServiceProviderUrl:  "http://spi",
}

func TestNewRouter(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "abc",
								ClientSecret: "cde",
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "https://test.sp",
						},
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "abc",
								ClientSecret: "cde",
							},
							ServiceProviderType:    config.ServiceProviderTypeQuay,
							ServiceProviderBaseUrl: "https://test.sp",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, config.SupportedServiceProviderTypes)

		assert.NotNil(t, router)
		assert.NoError(t, err)
		assert.Equal(t, len(config.SupportedServiceProviderTypes), len(router.controllers))
	})

	t.Run("fail with invalid sp url", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "abc",
								ClientSecret: "cde",
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: ":::",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, config.SupportedServiceProviderTypes)

		assert.Nil(t, router)
		assert.Error(t, err)
	})

	t.Run("fail when multiple sp for same url", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "abc",
								ClientSecret: "cde",
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "test.sp",
						},
						{
							OAuth2Config: &oauth2.Config{
								ClientID:     "123",
								ClientSecret: "234",
							},
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "test.sp",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, config.SupportedServiceProviderTypes)

		assert.Nil(t, router)
		assert.Error(t, err)
	})
}

func TestFindController(t *testing.T) {
	cfg := RouterConfiguration{
		StateStorage: &SessionStateStorage{},
		OAuthServiceConfiguration: OAuthServiceConfiguration{
			SharedConfiguration: config.SharedConfiguration{
				BaseUrl: "http://spi",
				ServiceProviders: []config.ServiceProviderConfiguration{
					{
						OAuth2Config: &oauth2.Config{
							ClientID:     "abc",
							ClientSecret: "cde",
						},
						ServiceProviderType:    config.ServiceProviderTypeGitHub,
						ServiceProviderBaseUrl: "https://test.sp",
					},
					{
						OAuth2Config: &oauth2.Config{
							ClientID:     "abc",
							ClientSecret: "cde",
						},
						ServiceProviderType:    config.ServiceProviderTypeQuay,
						ServiceProviderBaseUrl: "https://test.sp",
					},
				},
			},
		},
	}

	router, err := NewRouter(context.TODO(), cfg, config.SupportedServiceProviderTypes)
	assert.NoError(t, err)

	t.Run("fail when request unknown service provider", func(t *testing.T) {
		spDefaults := []config.ServiceProviderType{config.ServiceProviderTypeQuay, config.ServiceProviderTypeGitHub}

		router, err := NewRouter(context.TODO(), cfg, spDefaults)
		assert.NoError(t, err)

		statestring, stateErr := oauthstate.Encode(&oauthstate.OAuthInfo{
			ServiceProviderName: config.ServiceProviderTypeGitLab.Name,
		})
		assert.NoError(t, stateErr)

		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)
		testReq.Form = url.Values{"state": []string{statestring}}

		controller, state, err := router.findController(testReq, false)

		assert.Nil(t, controller)
		assert.Nil(t, state)
		assert.Error(t, err)
	})

	t.Run("fail with empty request", func(t *testing.T) {
		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)

		controller, state, err := router.findController(testReq, false)

		assert.Nil(t, controller)
		assert.Nil(t, state)
		assert.Error(t, err)
	})

	t.Run("fail with empty request veiled", func(t *testing.T) {
		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)

		controller, state, err := router.findController(testReq, true)

		assert.Nil(t, controller)
		assert.Nil(t, state)
		assert.Error(t, err)
	})

	t.Run("fail with empty request veiled", func(t *testing.T) {
		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)

		controller, state, err := router.findController(testReq, true)

		assert.Nil(t, controller)
		assert.Nil(t, state)
		assert.Error(t, err)
	})

	t.Run("ok find sp type", func(t *testing.T) {
		statestring, stateErr := oauthstate.Encode(&oauthstate.OAuthInfo{
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
		})
		assert.NoError(t, stateErr)

		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)
		testReq.Form = url.Values{"state": []string{statestring}}

		controller, state, err := router.findController(testReq, false)

		assert.NotNil(t, controller)
		assert.NotNil(t, state)
		assert.Equal(t, config.ServiceProviderTypeGitHub.Name, state.ServiceProviderName)
		assert.NoError(t, err)
	})
}

func TestCallbackRoute(t *testing.T) {

	t.Run("OAuth flow metrics", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		FlowCompleteTimeMetric.Reset()
		registry.MustRegister(FlowCompleteTimeMetric)
		encoded, _ := oauthstate.Encode(state)
		router, _ := NewTestRouter(SimpleStateStorage{vailState: "abcde", state: encoded, vailAt: time.Now()}, map[config.ServiceProviderName]Controller{
			config.ServiceProviderTypeGitHub.Name: NopController{},
		})
		req, _ := http.NewRequest("GET", "callback?state=abcde", nil)
		rr := httptest.NewRecorder()

		router.Callback().ServeHTTP(rr, req)

		count, err := prometheusTest.GatherAndCount(registry, "redhat_appstudio_spi_oauth_flow_complete_time_seconds")
		assert.Equal(t, 1, count)
		assert.NoError(t, err)
		problems, err := prometheusTest.GatherAndLint(registry, "redhat_appstudio_spi_oauth_flow_complete_time_seconds")

		assert.NoError(t, err)
		assert.Equal(t, 0, len(problems), fmt.Sprintf("Unexpected lint problems:  %s ", problems))

	})
}

// Creates new Router with predefined StateStorage and ready to use map of Controller.
func NewTestRouter(stateStorage StateStorage, controllers map[config.ServiceProviderName]Controller) (*Router, error) {
	return &Router{
		controllers:  controllers,
		stateStorage: stateStorage,
	}, nil
}
