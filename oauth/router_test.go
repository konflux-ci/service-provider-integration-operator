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
	"net/http"
	"net/url"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/serviceprovider"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2/github"
)

var testSpDefaults = []ServiceProviderDefaults{
	{
		SpType:   config.ServiceProviderTypeGitHub,
		Endpoint: github.Endpoint,
		UrlHost:  serviceprovider.GithubSaasHost,
	},
	{
		SpType:   config.ServiceProviderTypeQuay,
		Endpoint: serviceprovider.QuayEndpoint,
		UrlHost:  serviceprovider.QuaySaasHost,
	},
	{
		SpType:   config.ServiceProviderTypeGitLab,
		Endpoint: serviceprovider.GitlabEndpoint,
		UrlHost:  serviceprovider.GitlabSaasHost,
	},
}

func TestNewRouter(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.PersistedServiceProviderConfiguration{
						{
							ClientId:               "abc",
							ClientSecret:           "cde",
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "https://test.sp",
						},
						{
							ClientId:               "abc",
							ClientSecret:           "cde",
							ServiceProviderType:    config.ServiceProviderTypeQuay,
							ServiceProviderBaseUrl: "https://test.sp",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, testSpDefaults)

		assert.NotNil(t, router)
		assert.NoError(t, err)
		assert.Equal(t, len(testSpDefaults), len(router.controllers))
	})

	t.Run("fail with invalid sp url", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.PersistedServiceProviderConfiguration{
						{
							ClientId:               "abc",
							ClientSecret:           "cde",
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: ":::",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, testSpDefaults)

		assert.Nil(t, router)
		assert.Error(t, err)
	})

	t.Run("fail when multiple sp for same url", func(t *testing.T) {
		cfg := RouterConfiguration{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					BaseUrl: "http://spi",
					ServiceProviders: []config.PersistedServiceProviderConfiguration{
						{
							ClientId:               "abc",
							ClientSecret:           "cde",
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "test.sp",
						},
						{
							ClientId:               "123",
							ClientSecret:           "234",
							ServiceProviderType:    config.ServiceProviderTypeGitHub,
							ServiceProviderBaseUrl: "test.sp",
						},
					},
				},
			},
		}

		router, err := NewRouter(context.TODO(), cfg, testSpDefaults)

		assert.Nil(t, router)
		assert.Error(t, err)
	})
}

func TestFindController(t *testing.T) {
	cfg := RouterConfiguration{
		StateStorage: &StateStorage{},
		OAuthServiceConfiguration: OAuthServiceConfiguration{
			SharedConfiguration: config.SharedConfiguration{
				BaseUrl: "http://spi",
				ServiceProviders: []config.PersistedServiceProviderConfiguration{
					{
						ClientId:               "abc",
						ClientSecret:           "cde",
						ServiceProviderType:    config.ServiceProviderTypeGitHub,
						ServiceProviderBaseUrl: "https://test.sp",
					},
					{
						ClientId:               "abc",
						ClientSecret:           "cde",
						ServiceProviderType:    config.ServiceProviderTypeQuay,
						ServiceProviderBaseUrl: "https://test.sp",
					},
				},
			},
		},
	}

	router, err := NewRouter(context.TODO(), cfg, testSpDefaults)
	assert.NoError(t, err)

	t.Run("fail when request unknown service provider", func(t *testing.T) {
		spDefaults := []ServiceProviderDefaults{
			{
				SpType:   config.ServiceProviderTypeGitHub,
				Endpoint: github.Endpoint,
				UrlHost:  serviceprovider.GithubSaasHost,
			},
			{
				SpType:   config.ServiceProviderTypeQuay,
				Endpoint: serviceprovider.QuayEndpoint,
				UrlHost:  serviceprovider.QuaySaasHost,
			},
		}

		router, err := NewRouter(context.TODO(), cfg, spDefaults)
		assert.NoError(t, err)

		statestring, stateErr := oauthstate.Encode(&oauthstate.OAuthInfo{
			ServiceProviderType: config.ServiceProviderTypeGitLab,
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
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		})
		assert.NoError(t, stateErr)

		testReq, reqErr := http.NewRequest(http.MethodGet, "http://test", nil)
		assert.NoError(t, reqErr)
		testReq.Form = url.Values{"state": []string{statestring}}

		controller, state, err := router.findController(testReq, false)

		assert.NotNil(t, controller)
		assert.NotNil(t, state)
		assert.Equal(t, state.ServiceProviderType, config.ServiceProviderTypeGitHub)
		assert.NoError(t, err)
	})
}
