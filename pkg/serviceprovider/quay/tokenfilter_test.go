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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(api.AddToScheme(scheme))
}

func TestMatches_RobotToken(t *testing.T) {
	now := time.Now().Unix()

	t.Run("use cached data", func(t *testing.T) {
		state, err := json.Marshal(TokenState{
			Repositories: map[string]EntityRecord{
				"repo/img": {
					PossessedScopes: []Scope{ScopeRepoRead},
					LastRefreshTime: now,
				},
			},
			Organizations: map[string]EntityRecord{
				"repo": {
					PossessedScopes: []Scope{},
					LastRefreshTime: now,
				},
			},
		})
		assert.NoError(t, err)

		token := api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenSpec{
				Permissions: api.Permissions{
					AdditionalScopes: []string{string(ScopeRepoRead)},
				},
				ServiceProviderUrl: config.ServiceProviderTypeQuay.DefaultBaseUrl,
			},
			Status: api.SPIAccessTokenStatus{
				Phase:        api.SPIAccessTokenPhaseReady,
				ErrorReason:  "",
				ErrorMessage: "",
				OAuthUrl:     "",
				TokenMetadata: &api.TokenMetadata{
					Username:             "alois",
					UserId:               "42",
					Scopes:               nil,
					ServiceProviderState: state,
					LastRefreshTime:      0,
				},
			},
		}

		binding := api.SPIAccessTokenBinding{
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "https://quay.io/repo/img:latest",
				Permissions: api.Permissions{
					AdditionalScopes: []string{"repo:read"},
				},
			},
		}

		kubernetesClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&token).
			Build()

		httpClient := &http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				assert.Fail(t, "This test should be using the cached data")
				return nil, fmt.Errorf("no request to quay should be necessary")
			}),
		}

		filter := tokenFilter{
			metadataProvider: &metadataProvider{
				kubernetesClient: kubernetesClient,
				httpClient:       httpClient,
				tokenStorage: tokenstorage.TestTokenStorage{
					StoreImpl: func(ctx context.Context, token *api.SPIAccessToken, token2 *api.Token) error {
						return nil
					},
					GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
						if token.Name == "token" {
							return &api.Token{
								AccessToken: "asdf",
							}, nil
						}
						return nil, nil
					},
					DeleteImpl: func(ctx context.Context, token *api.SPIAccessToken) error {
						return nil
					},
				},
				ttl: 1 * time.Hour,
			},
		}

		res, err := filter.Matches(context.TODO(), &binding, &token)
		assert.NoError(t, err)
		assert.True(t, res)
	})
}
