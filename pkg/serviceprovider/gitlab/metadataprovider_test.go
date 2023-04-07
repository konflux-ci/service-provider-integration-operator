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

package gitlab

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

var tokenStorageGet = func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
	return &api.Token{
		AccessToken:  "token",
		TokenType:    "Grizzly Bear-er",
		RefreshToken: "refresh-token",
		Expiry:       0,
	}, nil
}

func TestFetch_Success(t *testing.T) {
	httpClient := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/user") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "user42"}`))),
				}, nil
			}
			if strings.Contains(r.URL.String(), gitlabOAuthTokenInfoPath) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"scope": ["write_repository", "read_registry"]}`))),
				}, nil
			}

			return nil, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}
	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpClient,
			tokenStorage: ts,
		},
		httpClient:   httpClient,
		tokenStorage: &ts,
	}

	token := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &token, false)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "42", data.UserId)
	assert.Equal(t, "user42", data.Username)
	assert.Equal(t, []string{"write_repository", "read_registry"}, data.Scopes)
	assert.Empty(t, data.ServiceProviderState)
}

func TestFetch_Success_PatScopes(t *testing.T) {
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/user") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "user42"}`))),
				}, nil
			}
			if strings.Contains(r.URL.String(), gitlabOAuthTokenInfoPath) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"error": "invalid_token"}`))),
				}, nil
			}
			if strings.Contains(r.URL.String(), gitlabPatInfoPath) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"scopes": ["write_repository", "read_registry"]}`))),
				}, nil
			}
			return nil, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}

	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpCl,
			tokenStorage: ts,
		},
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	token := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &token, true)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "42", data.UserId)
	assert.Equal(t, "user42", data.Username)
	assert.Equal(t, []string{"write_repository", "read_registry"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)
}

func TestFetch_Fail_User(t *testing.T) {
	expectedError := errors.New("success is the progressive realization of a worthy ideal")
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/user") {
				return nil, expectedError
			}
			return nil, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}
	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpCl,
			tokenStorage: ts,
		},
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn, false)
	assert.Error(t, err, expectedError)
	assert.Nil(t, data)
}

func TestFetch_Fail_Scopes(t *testing.T) {
	expectedError := errors.New("be the change that you wish to see in the world")
	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/user") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "user42"}`))),
				}, nil
			}
			if strings.Contains(r.URL.String(), gitlabOAuthTokenInfoPath) || strings.Contains(r.URL.String(), gitlabPatInfoPath) {
				return nil, expectedError
			}
			return nil, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: tokenStorageGet,
	}

	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpCl,
			tokenStorage: ts,
		},
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{}
	data, err := mp.Fetch(context.TODO(), &tkn, false)
	assert.Error(t, err, expectedError)
	assert.Nil(t, data)
}

func TestMetadataProvider_Fetch_state_handling(t *testing.T) {
	additionalRequests := make([]string, 0)

	httpCl := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v4/" {
				// this request is made by the gitlab client to establish rate limiting. We have no control over it doing
				// that.
				return &http.Response{StatusCode: 200}, nil
			} else if strings.Contains(r.URL.String(), "/user") {
				return &http.Response{
					StatusCode: 200,
					Header: map[string][]string{
						// the letter case is important here, http client is sensitive to this
						"X-Oauth-Scopes": {"a, b, c, d"},
					},
					Body: io.NopCloser(bytes.NewBuffer([]byte(`{"id": 42, "username": "test_user"}`))),
				}, nil
			} else if strings.Contains(r.URL.String(), gitlabPatInfoPath) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"scopes": ["write_repository", "read_registry"]}`))),
				}, nil
			} else if strings.Contains(r.URL.String(), gitlabOAuthTokenInfoPath) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"error": "invalid_token"}`))),
				}, nil
			} else {
				additionalRequests = append(additionalRequests, r.URL.String())
				return &http.Response{StatusCode: 200}, nil
			}
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   httpCl,
		tokenStorage: ts,
	}

	mp := metadataProvider{
		glClientBuilder: gitlabClientBuilder,
		httpClient:      httpCl,
		tokenStorage:    &ts,
	}

	t.Run("nostate", func(t *testing.T) {
		tkn := api.SPIAccessToken{}
		data, err := mp.Fetch(context.TODO(), &tkn, false)
		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Empty(t, additionalRequests)
		assert.Nil(t, data.ServiceProviderState)
		assert.Equal(t, "42", data.UserId)
		assert.Equal(t, "test_user", data.Username)
		assert.Equal(t, []string{"write_repository", "read_registry"}, data.Scopes)
	})

	t.Run("state", func(t *testing.T) {
		tkn := api.SPIAccessToken{}
		data, err := mp.Fetch(context.TODO(), &tkn, true)
		assert.NoError(t, err)
		assert.NotNil(t, data)
		// currently, we do no additional requests, but that's possibly going to change in the future
		assert.Empty(t, additionalRequests)
		assert.NotNil(t, data.ServiceProviderState)
		assert.Equal(t, "42", data.UserId)
		assert.Equal(t, "test_user", data.Username)
		assert.Equal(t, []string{"write_repository", "read_registry"}, data.Scopes)
	})
}
