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
	"net/http"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testValidRepoUrl = "https://quay.io/repository/redhat-appstudio/service-provider-integration-operator"

func TestQuayProbe_Examine(t *testing.T) {
	probe := quayProbe{}
	test := func(t *testing.T, url string, expectedMatch bool) {
		baseUrl, err := probe.Examine(nil, url)
		expectedBaseUrl := ""
		if expectedMatch {
			expectedBaseUrl = "https://quay.io"
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedBaseUrl, baseUrl)
	}

	test(t, "quay.io/name/repo", true)
	test(t, "https://quay.io/name/repo", true)
	test(t, "https://github.com/name/repo", false)
	test(t, "github.com/name/repo", false)
}

func TestCheckAccessNotImplementedYetError(t *testing.T) {
	cl := mockK8sClient()
	quay := mockQuay(cl, http.StatusNotFound, nil)
	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := quay.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, api.SPIAccessCheckErrorNotImplemented, status.ErrorReason)
}

func TestMapToken(t *testing.T) {
	k8sClient := mockK8sClient()
	httpClient := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return nil, nil
		}),
	}

	fac := &serviceprovider.Factory{
		Configuration: config.Configuration{
			TokenLookupCacheTtl: 100 * time.Hour,
		},
		KubernetesClient: k8sClient,
		HttpClient:       httpClient,
		Initializers: map[config.ServiceProviderType]serviceprovider.Initializer{
			config.ServiceProviderTypeQuay: Initializer,
		},
		TokenStorage: tokenstorage.TestTokenStorage{
			GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
				return &api.Token{
					AccessToken: "accessToken",
				}, nil
			},
		},
	}

	quay, err := newQuay(fac, "")
	assert.NoError(t, err)

	now := time.Now().Unix()

	state := TokenState{
		Repositories: map[string]EntityRecord{
			"org/repo": {
				LastRefreshTime: now,
				PossessedScopes: []Scope{ScopeRepoAdmin},
			},
		},
		Organizations: map[string]EntityRecord{
			"org": {
				LastRefreshTime: now,
				PossessedScopes: []Scope{},
			},
		},
	}

	stateBytes, err := json.Marshal(&state)
	assert.NoError(t, err)
	mapper, err := quay.MapToken(context.TODO(), &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "quay.io/org/repo:latest",
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeRead,
						Area: api.PermissionAreaUser,
					},
					{
						Type: api.PermissionTypeReadWrite,
						Area: api.PermissionAreaRepository,
					},
				},
				AdditionalScopes: []string{string(ScopeOrgAdmin)},
			},
			Secret: api.SecretSpec{},
		},
	}, &api.SPIAccessToken{
		Status: api.SPIAccessTokenStatus{
			TokenMetadata: &api.TokenMetadata{
				Username:             "alois",
				UserId:               "42",
				Scopes:               nil,
				ServiceProviderState: stateBytes,
			},
		},
	}, &api.Token{
		AccessToken: "access_token",
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(mapper.Scopes))
	assert.Contains(t, mapper.Scopes, string(ScopeRepoAdmin))
}

func TestValidate(t *testing.T) {
	q := &Quay{}

	res, err := q.Validate(context.TODO(), &api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeRead,
						Area: api.PermissionAreaUser,
					},
				},
				AdditionalScopes: []string{"blah", "user:read", "repo:read", "push", "pull"},
			},
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, 3, len(res.ScopeValidation))
	assert.NotNil(t, res.ScopeValidation[0])
	assert.Equal(t, "user-related permissions are not supported for Quay", res.ScopeValidation[0].Error())
	assert.NotNil(t, res.ScopeValidation[1])
	assert.Equal(t, "unknown scope: 'blah'", res.ScopeValidation[1].Error())
	assert.NotNil(t, res.ScopeValidation[2])
	assert.Equal(t, "unsupported scope 'user:read'", res.ScopeValidation[2].Error())
}

func TestQuay_TranslateToScopes(t *testing.T) {
	repoR := api.Permission{
		Area: api.PermissionAreaRepository,
		Type: api.PermissionTypeRead,
	}
	repoW := api.Permission{
		Area: api.PermissionAreaRepository,
		Type: api.PermissionTypeWrite,
	}
	repoRW := api.Permission{
		Area: api.PermissionAreaRepository,
		Type: api.PermissionTypeReadWrite,
	}
	repoMR := api.Permission{
		Area: api.PermissionAreaRepositoryMetadata,
		Type: api.PermissionTypeRead,
	}
	repoMW := api.Permission{
		Area: api.PermissionAreaRepositoryMetadata,
		Type: api.PermissionTypeWrite,
	}
	repoMRW := api.Permission{
		Area: api.PermissionAreaRepositoryMetadata,
		Type: api.PermissionTypeReadWrite,
	}

	q := &Quay{}

	assert.Equal(t, []string{"repo:read"}, q.TranslateToScopes(repoR))
	assert.Equal(t, []string{"repo:write"}, q.TranslateToScopes(repoW))
	assert.Equal(t, []string{"repo:read", "repo:write"}, q.TranslateToScopes(repoRW))
	assert.Equal(t, []string{"repo:read"}, q.TranslateToScopes(repoMR))
	assert.Equal(t, []string{"repo:write"}, q.TranslateToScopes(repoMW))
	assert.Equal(t, []string{"repo:read", "repo:write"}, q.TranslateToScopes(repoMRW))
}

type httpClientMock struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (h httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return h.doFunc(req)
}

type tokenFilterMock struct {
	matchesFunc func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error)
}

func (t tokenFilterMock) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	return t.matchesFunc(ctx, matchable, token)
}

func mockQuay(cl client.Client, returnCode int, httpErr error) *Quay {
	metadataCache := serviceprovider.NewMetadataCache(cl, &serviceprovider.NeverMetadataExpirationPolicy{})
	return &Quay{
		httpClient: httpClientMock{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: returnCode}, httpErr
			},
		},
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitHub,
			MetadataCache:       &metadataCache,
			TokenFilter: tokenFilterMock{
				matchesFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
					return true, nil
				},
			},
		},
	}
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
