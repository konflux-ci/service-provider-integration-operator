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
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	gitlab := &Gitlab{}
	validationResult, err := gitlab.Validate(context.TODO(), &api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeWrite,
						Area: api.PermissionAreaUser,
					},
					{
						Type: api.PermissionTypeRead,
						Area: api.PermissionAreaWebhooks,
					},
					{
						Type: api.PermissionTypeReadWrite,
						Area: api.PermissionAreaRepository,
					},
				},
				AdditionalScopes: []string{string(ScopeSudo), string(ScopeProfile), "darth", string(ScopeWriteRepository), "vader"},
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, 4, len(validationResult.ScopeValidation))
	assert.ErrorIs(t, validationResult.ScopeValidation[0], unsupportedUserWritePermissionError)
	assert.ErrorIs(t, validationResult.ScopeValidation[1], unsupportedAreaError)
	assert.ErrorContains(t, validationResult.ScopeValidation[1], string(api.PermissionAreaWebhooks))
	assert.ErrorIs(t, validationResult.ScopeValidation[2], unsupportedScopeError)
	assert.ErrorContains(t, validationResult.ScopeValidation[2], "darth")
	assert.ErrorIs(t, validationResult.ScopeValidation[3], unsupportedScopeError)
	assert.ErrorContains(t, validationResult.ScopeValidation[3], "vader")
}

func TestOAuthScopesFor(t *testing.T) {
	gitlab := &Gitlab{
		oauthCapability: &gitlabOAuthCapability{},
	}
	hasExpectedScopes := func(expectedScopes []string, permissions api.Permissions) func(t *testing.T) {
		return func(t *testing.T) {
			actualScopes := gitlab.GetOAuthCapability().OAuthScopesFor(&permissions)
			assert.Equal(t, len(expectedScopes), len(actualScopes))
			for _, s := range expectedScopes {
				assert.Contains(t, actualScopes, s)
			}
		}
	}

	t.Run("read repository",
		hasExpectedScopes([]string{string(ScopeReadRepository), string(ScopeReadUser)},
			api.Permissions{Required: []api.Permission{
				{
					Area: api.PermissionAreaRepository,
					Type: api.PermissionTypeRead,
				},
			}}))

	t.Run("write repository and registry",
		hasExpectedScopes([]string{string(ScopeWriteRepository), string(ScopeWriteRegistry), string(ScopeReadUser)},
			api.Permissions{Required: []api.Permission{
				{
					Area: api.PermissionAreaRepository,
					Type: api.PermissionTypeWrite,
				},
				{
					Area: api.PermissionAreaRegistry,
					Type: api.PermissionTypeWrite,
				},
			}}))

	additionalScopes := []string{string(ScopeSudo), string(ScopeApi), string(ScopeReadApi), string(ScopeReadUser)}
	t.Run("read user and additional scopes", hasExpectedScopes(
		additionalScopes,
		api.Permissions{Required: []api.Permission{{
			Type: api.PermissionTypeRead,
			Area: api.PermissionAreaUser,
		}},
			AdditionalScopes: additionalScopes}))
}

func TestIsPublicRepo(t *testing.T) {
	t.Run("http request fails", func(t *testing.T) {
		expectedError := errors.New("expected error")
		gitlab := Gitlab{httpClient: &http.Client{
			Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
				return nil, expectedError
			}),
		}}

		spiAccessCheck := &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: "gitlab.test"}}
		isPublic, err := gitlab.isPublicRepo(context.TODO(), spiAccessCheck)
		assert.False(t, isPublic)
		assert.ErrorIs(t, err, expectedError)
	})

	test := func(statusCode int, expected bool) {
		t.Run(fmt.Sprintf("should return %t for http status code %d", expected, statusCode), func(t *testing.T) {
			gitlab := Gitlab{httpClient: &http.Client{
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: statusCode,
					}, nil
				}),
			}}

			spiAccessCheck := &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: "gitlab.test"}}
			isPublic, err := gitlab.isPublicRepo(context.TODO(), spiAccessCheck)
			assert.NoError(t, err)
			assert.Equal(t, expected, isPublic)
		})
	}

	test(200, true)
	test(404, false)
	test(402, false)
	test(401, false)
}

func TestCheckRepositoryAccess(t *testing.T) {
	k8sClient := mockK8sClient()
	gitlab := mockGitlab(k8sClient, http.StatusOK, "", nil, nil)

	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}

	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitLab, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
}

func TestCheckRepositoryAccess_FailWithGithubHttp(t *testing.T) {
	k8sClient := mockK8sClient()
	gitlab := mockGitlab(k8sClient, http.StatusServiceUnavailable, "", fmt.Errorf("fail to talk to github api"), nil)

	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}

	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &ac)

	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestCheckRepositoryAccess_Private(t *testing.T) {
	k8sClient := mockK8sClient()
	gitlab := mockGitlab(k8sClient, http.StatusNotFound, "", nil, nil)
	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}

	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitLab, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
}

func TestCheckRepositoryAccess_LookupFailing_With_PublicRepo(t *testing.T) {
	k8sClient := mockK8sClient()
	gitlab := mockGitlab(k8sClient, http.StatusOK, "", nil, errors.New("expected error"))

	accessCheck := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}
	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &accessCheck)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitLab, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
	assert.Empty(t, status.ErrorReason)
	assert.Empty(t, status.ErrorMessage)
}

func TestCheckRepositoryAccess_LookupFailing_With_PrivateRepo(t *testing.T) {
	k8sClient := mockK8sClient()
	expectedError := errors.New("expected error")
	gitlab := mockGitlab(k8sClient, http.StatusNotFound, "", nil, expectedError)

	accessCheck := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}
	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &accessCheck)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitLab, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
	assert.Equal(t, api.SPIAccessCheckErrorTokenLookupFailed, status.ErrorReason)
	assert.Contains(t, status.ErrorMessage, expectedError.Error())
}

func TestCheckRepositoryAccess_With_MatchingTokens(t *testing.T) {
	k8sClient := mockK8sClient()
	gitlab := mockGitlab(k8sClient, http.StatusOK, "", nil, nil)
	ac := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "access-check",
			Namespace: "ac-namespace",
		},
		Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl + "/namespace/repo"},
	}
	status, err := gitlab.CheckRepositoryAccess(context.TODO(), k8sClient, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
}

func mockGitlab(cl client.Client, returnCode int, body string, responseError error, lookupError error) *Gitlab {
	metadataCache := serviceprovider.MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &serviceprovider.NeverMetadataExpirationPolicy{},
		CacheServiceProviderState: true,
	}
	tokenStorageMock := tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{AccessToken: "access_tolkien"}, nil
	}}

	httpClientMock := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: returnCode,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewBuffer([]byte(body))),
				Request:    r,
			}, responseError
		}),
	}

	return &Gitlab{
		Configuration: &opconfig.OperatorConfiguration{SharedConfiguration: config.SharedConfiguration{BaseUrl: "https://test.url"}},
		httpClient:    httpClientMock,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitLab,
			MetadataCache:       &metadataCache,
			TokenFilter: struct {
				serviceprovider.TokenFilterFunc
			}{
				TokenFilterFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
					return true, lookupError
				},
			},
			RepoHostParser: serviceprovider.RepoHostFromUrl,
		},
		tokenStorage: tokenStorageMock,
		glClientBuilder: gitlabClientBuilder{
			httpClient:   httpClientMock,
			tokenStorage: tokenStorageMock,
		},
	}
}

func mockK8sClient() client.WithWatch {
	token := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "accessCheck-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitLab),
				api.ServiceProviderHostLabel: config.ServiceProviderTypeGitLab.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeGitLab.DefaultBaseUrl,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	}

	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(token).Build()
}

func TestNewGitlab(t *testing.T) {
	factory := &serviceprovider.Factory{
		Configuration: &opconfig.OperatorConfiguration{
			TokenMatchPolicy: opconfig.AnyTokenPolicy,
			SharedConfiguration: config.SharedConfiguration{
				BaseUrl: "bejsjuarel",
			},
		},
	}

	t.Run("no oauth info => nil oauth capability", func(t *testing.T) {
		sp, err := newGitlab(factory, &config.ServiceProviderConfiguration{ServiceProviderBaseUrl: "https://baltig.moc"})

		assert.NoError(t, err)
		assert.NotNil(t, sp)
		assert.Nil(t, sp.GetOAuthCapability())
	})

	t.Run("oauth info => oauth capability", func(t *testing.T) {
		sp, err := newGitlab(factory, &config.ServiceProviderConfiguration{ServiceProviderBaseUrl: "https://baltig.moc", OAuth2Config: &oauth2.Config{ClientID: "123", ClientSecret: "456"}})

		assert.NoError(t, err)
		assert.NotNil(t, sp)
		assert.NotNil(t, sp.GetOAuthCapability())
		assert.Contains(t, sp.GetOAuthCapability().GetOAuthEndpoint(), "bejsjuarel")
	})
}
