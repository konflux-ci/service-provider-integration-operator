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

package quay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

func TestMetadataProvider_Fetch(t *testing.T) {

	t.Run("returns nil when no token data", func(t *testing.T) {
		ts := tokenstorage.TestTokenStorage{
			GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
				return nil, nil
			},
		}

		mp := metadataProvider{
			tokenStorage: &ts,
		}

		tkn := api.SPIAccessToken{
			Spec:   api.SPIAccessTokenSpec{},
			Status: api.SPIAccessTokenStatus{},
		}

		data, err := mp.Fetch(context.TODO(), &tkn)

		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("initializes state", func(t *testing.T) {
		test := func(t *testing.T, ts tokenstorage.TokenStorage, expectedUsername string) {
			mp := metadataProvider{
				tokenStorage: ts,
			}

			tkn := api.SPIAccessToken{
				Spec:   api.SPIAccessTokenSpec{},
				Status: api.SPIAccessTokenStatus{},
			}

			data, err := mp.Fetch(context.TODO(), &tkn)

			assert.NoError(t, err)
			assert.NotNil(t, data)
			assert.Equal(t, expectedUsername, data.Username)
			assert.NotNil(t, data.ServiceProviderState)

			state := &TokenState{}
			assert.NoError(t, json.Unmarshal(data.ServiceProviderState, state))

			assert.NotNil(t, state.Organizations)
			assert.Empty(t, state.Organizations)
			assert.NotNil(t, state.Repositories)
			assert.Empty(t, state.Repositories)
		}

		t.Run("using oauth token", func(t *testing.T) {
			ts := tokenstorage.TestTokenStorage{
				GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
					return &api.Token{
						AccessToken: "token",
					}, nil
				},
			}

			test(t, &ts, "$oauthtoken")
		})

		t.Run("using robot token", func(t *testing.T) {
			ts := tokenstorage.TestTokenStorage{
				GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
					return &api.Token{
						AccessToken: "token",
						Username:    "alois",
					}, nil
				},
			}

			test(t, &ts, "alois")
		})
	})
}

func TestMetadataProvider_FetchRepo(t *testing.T) {
	refreshTime := time.Now().Add(-1 * time.Hour).Unix()

	state := TokenState{
		Repositories: map[string]EntityRecord{
			"org/repo": {
				LastRefreshTime: refreshTime,
				PossessedScopes: []Scope{ScopeRepoRead, ScopeRepoWrite},
			},
		},
		Organizations: map[string]EntityRecord{
			"org": {
				LastRefreshTime: refreshTime,
				PossessedScopes: []Scope{ScopeOrgAdmin},
			},
		},
	}

	stateBytes, err := json.Marshal(&state)
	assert.NoError(t, err)

	failingHttpClient := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			assert.Fail(t, "No outbound traffic should happen")
			return nil, nil
		}),
	}

	t.Run("fetch from cache", func(t *testing.T) {
		token := &api.SPIAccessToken{
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					ServiceProviderState: stateBytes,
				},
			},
		}

		k8sClient := fake.NewClientBuilder().Build()

		mp := metadataProvider{
			tokenStorage: tokenstorage.TestTokenStorage{
				GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
					return &api.Token{
						AccessToken: "token",
					}, nil
				},
			},
			httpClient:       failingHttpClient,
			kubernetesClient: k8sClient,
			ttl:              10 * time.Hour,
		}

		repoMetadata, err := mp.FetchRepo(context.TODO(), "quay.io/org/repo:latest", token)
		assert.NoError(t, err)

		assert.Equal(t, []Scope{ScopeRepoRead, ScopeRepoWrite}, repoMetadata.Repository.PossessedScopes)
		assert.Equal(t, []Scope{ScopeOrgAdmin}, repoMetadata.Organization.PossessedScopes)
	})

	t.Run("attempted fetch with missing initial metadata", func(t *testing.T) {
		token := &api.SPIAccessToken{
			Status: api.SPIAccessTokenStatus{},
		}

		k8sClient := fake.NewClientBuilder().Build()

		mp := metadataProvider{
			tokenStorage:     tokenstorage.TestTokenStorage{},
			httpClient:       failingHttpClient,
			kubernetesClient: k8sClient,
			ttl:              10 * time.Hour,
		}

		repoMetadata, err := mp.FetchRepo(context.TODO(), "quay.io/org/repo:latest", token)
		assert.NoError(t, err)

		assert.Empty(t, repoMetadata.Repository.PossessedScopes)
		assert.Empty(t, repoMetadata.Organization.PossessedScopes)
	})

	t.Run("attempted fetch with missing token", func(t *testing.T) {
		token := &api.SPIAccessToken{
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					ServiceProviderState: stateBytes,
				},
			},
		}

		k8sClient := fake.NewClientBuilder().Build()

		mp := metadataProvider{
			tokenStorage: tokenstorage.TestTokenStorage{
				GetImpl: func(_ context.Context, _ *api.SPIAccessToken) (*api.Token, error) {
					// this is the default case, but let's be explicit here
					return nil, nil
				},
			},
			httpClient:       failingHttpClient,
			kubernetesClient: k8sClient,
			ttl:              0,
		}

		repoMetadata, err := mp.FetchRepo(context.TODO(), "quay.io/not-our-org/repo:latest", token)
		assert.NoError(t, err)

		assert.Empty(t, repoMetadata.Repository.PossessedScopes)
		assert.Empty(t, repoMetadata.Organization.PossessedScopes)
	})

	t.Run("fetch from quay", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "quay.io" {
					assert.Fail(t, "only traffic to quay.io should happen")
					return nil, nil
				}

				auth := r.Header.Get("Authorization")

				var bearerAuthExpected, basicAuthExpected bool
				var res *http.Response

				switch r.URL.Path {
				case "/api/v1/organization/org/robots", "/api/v1/user/robots", "/api/v1/user/",
					"/api/v1/repository/org/repo/notification/":
					res = &http.Response{StatusCode: 200}
					bearerAuthExpected = true
				case "/api/v1/repository":
					res = &http.Response{StatusCode: 400}
					bearerAuthExpected = true
				case "/api/v1/repository/org/repo":
					bearerAuthExpected = true
					if r.Method == "PUT" {
						res = &http.Response{StatusCode: 200}
					} else if r.Method == "GET" {
						res = &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"description": "asdf"}`))}
					} else {
						assert.Fail(t, "unexpected method in %s", r.URL.String())
					}
				case "/v2/auth":
					assert.Equal(t, "repository:org/repo:push,pull", r.URL.Query().Get("scope"))
					// this returns a fake JWT token giving push and pull access to "org/repo" repository to a test+test user
					res = &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJxdWF5IiwiYXVkIjoicXVheS5pbyIsIm5iZiI6MTY1MjEwNTQ1OCwiaWF0IjoxNjUyMTA1NDU4LCJleHAiOjE2NTIxMDkwNTgsInN1YiI6InRlc3QrdGVzdCIsImFjY2VzcyI6W3sidHlwZSI6InJlcG9zaXRvcnkiLCJuYW1lIjoib3JnL3JlcG8iLCJhY3Rpb25zIjpbInB1c2giLCJwdWxsIl19XSwiY29udGV4dCI6eyJ2ZXJzaW9uIjoyLCJlbnRpdHlfa2luZCI6InJvYm90IiwiZW50aXR5X3JlZmVyZW5jZSI6InRlc3QrdGVzdCIsImtpbmQiOiJ1c2VyIiwidXNlciI6InRlc3QrdGVzdCIsImNvbS5hcG9zdGlsbGUucm9vdHMiOnsidW5ob29rL3VuaG9vay10dW5uZWwiOiIkZGlzYWJsZWQifSwiY29tLmFwb3N0aWxsZS5yb290IjoiJGRpc2FibGVkIn19.6bdjhEosHqNjlsvyiaKUxqWm6mF98EPLs08jJBtXcNA"}`))}
					basicAuthExpected = true
				default:
					assert.Fail(t, "unexpected quay request", "url: %s", r.URL.String())
				}

				if bearerAuthExpected && auth != "Bearer token" {
					assert.Fail(t, "unexpected bearer token", "auth header: `%s`, url: %s", auth, r.URL)
				}

				// Base64 encoded "$oauthtoken:token" is what docker login expects in this case
				if basicAuthExpected && auth != "Basic JG9hdXRodG9rZW46dG9rZW4=" {
					assert.Fail(t, "unexpected basic auth", "auth header: `%s`, url: %s", auth, r.URL)
				}

				return res, nil
			}),
		}

		scheme := runtime.NewScheme()
		utilruntime.Must(api.AddToScheme(scheme))

		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					ServiceProviderState: stateBytes,
				},
			},
		}

		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(token).Build()

		mp := metadataProvider{
			tokenStorage: tokenstorage.TestTokenStorage{
				GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
					return &api.Token{
						AccessToken: "token",
					}, nil
				},
			},
			httpClient:       httpClient,
			kubernetesClient: k8sClient,
			ttl:              0, // make sure the cache is stale
		}

		repoMetadata, err := mp.FetchRepo(context.TODO(), "quay.io/org/repo:latest", token)
		assert.NoError(t, err)

		// the http client responses give us all the permissions (unlike the initial state which didn't give org and user perms)
		assert.Equal(t, []Scope{ScopeRepoRead, ScopePull, ScopeRepoWrite, ScopePush, ScopeRepoAdmin, ScopeRepoCreate}, repoMetadata.Repository.PossessedScopes)
		assert.Equal(t, []Scope{ScopeOrgAdmin}, repoMetadata.Organization.PossessedScopes)
	})
}

func TestMetadataProvider_ShouldRecoverFromTokenWithOldStateFormat(t *testing.T) {

	t.Run("fetch from cache", func(t *testing.T) {
		//ServiceProviderState in an old format that doesn't contain TokenState.Repositories or TokenState.Organizations maps
		rawDecodedText, err := base64.StdEncoding.DecodeString("eyJBY2Nlc3NpYmxlUmVwb3MiOnt9LCJSZW1vdGVVc2VybmFtZSI6InNidWRod2FyIn0=")
		assert.NoError(t, err)

		token := &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					ServiceProviderState: rawDecodedText,
				},
			},
		}

		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(token).Build()
		httpClient := &http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "quay.io" {
					assert.Fail(t, "only traffic to quay.io should happen")
					return nil, nil
				}

				auth := r.Header.Get("Authorization")

				var bearerAuthExpected, basicAuthExpected bool
				var res *http.Response

				switch r.URL.Path {
				case "/api/v1/organization/org/robots", "/api/v1/user/robots", "/api/v1/user/",
					"/api/v1/repository/org/repo/notification/":
					res = &http.Response{StatusCode: 200}
					bearerAuthExpected = true
				case "/api/v1/repository":
					res = &http.Response{StatusCode: 400}
					bearerAuthExpected = true
				case "/api/v1/repository/org/repo":
					bearerAuthExpected = true
					if r.Method == "PUT" {
						res = &http.Response{StatusCode: 200}
					} else if r.Method == "GET" {
						res = &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"description": "asdf"}`))}
					} else {
						assert.Fail(t, "unexpected method in %s", r.URL.String())
					}
				case "/v2/auth":
					assert.Equal(t, "repository:org/repo:push,pull", r.URL.Query().Get("scope"))
					// this returns a fake JWT token giving push and pull access to "org/repo" repository to a test+test user
					res = &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJxdWF5IiwiYXVkIjoicXVheS5pbyIsIm5iZiI6MTY1MjEwNTQ1OCwiaWF0IjoxNjUyMTA1NDU4LCJleHAiOjE2NTIxMDkwNTgsInN1YiI6InRlc3QrdGVzdCIsImFjY2VzcyI6W3sidHlwZSI6InJlcG9zaXRvcnkiLCJuYW1lIjoib3JnL3JlcG8iLCJhY3Rpb25zIjpbInB1c2giLCJwdWxsIl19XSwiY29udGV4dCI6eyJ2ZXJzaW9uIjoyLCJlbnRpdHlfa2luZCI6InJvYm90IiwiZW50aXR5X3JlZmVyZW5jZSI6InRlc3QrdGVzdCIsImtpbmQiOiJ1c2VyIiwidXNlciI6InRlc3QrdGVzdCIsImNvbS5hcG9zdGlsbGUucm9vdHMiOnsidW5ob29rL3VuaG9vay10dW5uZWwiOiIkZGlzYWJsZWQifSwiY29tLmFwb3N0aWxsZS5yb290IjoiJGRpc2FibGVkIn19.6bdjhEosHqNjlsvyiaKUxqWm6mF98EPLs08jJBtXcNA"}`))}
					basicAuthExpected = true
				default:
					assert.Fail(t, "unexpected quay request", "url: %s", r.URL.String())
				}

				if bearerAuthExpected && auth != "Bearer token" {
					assert.Fail(t, "unexpected bearer token", "auth header: `%s`, url: %s", auth, r.URL)
				}

				// Base64 encoded "$oauthtoken:token" is what docker login expects in this case
				if basicAuthExpected && auth != "Basic JG9hdXRodG9rZW46dG9rZW4=" {
					assert.Fail(t, "unexpected basic auth", "auth header: `%s`, url: %s", auth, r.URL)
				}

				return res, nil
			}),
		}
		mp := metadataProvider{
			tokenStorage: tokenstorage.TestTokenStorage{
				GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
					return &api.Token{
						AccessToken: "token",
					}, nil
				},
			},
			httpClient:       httpClient,
			kubernetesClient: k8sClient,
			ttl:              10 * time.Hour,
		}

		repoMetadata, err := mp.FetchRepo(context.TODO(), "quay.io/org/repo:latest", token)
		assert.NoError(t, err)

		assert.Equal(t, []Scope{ScopeRepoRead, ScopePull, ScopeRepoWrite, ScopePush, ScopeRepoAdmin, ScopeRepoCreate}, repoMetadata.Repository.PossessedScopes)
		assert.Equal(t, []Scope{ScopeOrgAdmin}, repoMetadata.Organization.PossessedScopes)
	})

}
