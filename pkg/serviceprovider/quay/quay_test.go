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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testValidRepoUrl = "https://quay.io/repository/redhat-appstudio/service-provider-integration-operator"

func TestMain(m *testing.M) {
	logs.InitLoggers(true, flag.CommandLine)
	os.Exit(m.Run())
}

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

func TestQuay_OAuthScopesFor(t *testing.T) {

	q := &Quay{}
	fullScopes := []string{"repo:read", "repo:write", "repo:create", "repo:admin"}

	hasScopes := func(expectedScopes []string, ps api.Permissions) func(t *testing.T) {
		return func(t *testing.T) {
			actualScopes := q.OAuthScopesFor(&ps)
			assert.Equal(t, len(expectedScopes), len(actualScopes))
			for _, s := range expectedScopes {
				assert.Contains(t, actualScopes, s)
			}
		}
	}

	hasDefault := func(ps api.Permissions) func(t *testing.T) {
		return hasScopes(fullScopes, ps)
	}

	t.Run("empty", hasDefault(api.Permissions{}))
	t.Run("read-repo", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepository,
			Type: api.PermissionTypeRead,
		},
	}}))
	t.Run("write-repo", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepository,
			Type: api.PermissionTypeWrite,
		},
	}}))
	t.Run("read-write-repo", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepository,
			Type: api.PermissionTypeReadWrite,
		},
	}}))
	t.Run("read-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepositoryMetadata,
			Type: api.PermissionTypeRead,
		},
	}}))
	t.Run("write-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepositoryMetadata,
			Type: api.PermissionTypeWrite,
		},
	}}))
	t.Run("read-write-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRepositoryMetadata,
			Type: api.PermissionTypeReadWrite,
		},
	}}))
	t.Run("additional-scopes", hasScopes(
		[]string{"repo:read", "repo:write", "repo:create", "repo:admin", "org:admin"},
		api.Permissions{AdditionalScopes: []string{"org:admin"}}))
}

func TestRequestRepoInfo(t *testing.T) {
	test := func(name string, responseFn func(req *http.Request) (*http.Response, error), expectedCode int, expectedJson map[string]interface{}, expectedError bool) {
		q := &Quay{
			httpClient: httpClientMock{doFunc: responseFn},
		}
		t.Run(name, func(t *testing.T) {
			responseCode, responseJson, err := q.requestRepoInfo(context.TODO(), "redhat-appstudio", "service-provider-integration-operator", "")
			assert.Equal(t, expectedCode, responseCode)
			assert.Equal(t, expectedJson, responseJson)
			if expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	test("forbidden access", func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: nil}, nil
	}, http.StatusForbidden, nil, false)

	test("ok response but empty body", func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: nil}, nil
	}, http.StatusOK, nil, true)

	test("response body is not valid json", func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("this does not look like json"))}, nil
	}, http.StatusOK, nil, true)

	test("ok response with json", func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{\"blabol\": \"ano prosim\"}"))}, nil
	}, http.StatusOK, map[string]interface{}{"blabol": "ano prosim"}, false)

	test("error response", func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, fmt.Errorf("fatal failure")
	}, http.StatusInternalServerError, nil, true)

	test("no response no error", func(req *http.Request) (*http.Response, error) {
		return nil, nil
	}, 0, nil, true)

	test("no response yes error", func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("fatal failure")
	}, 0, nil, true)
}

const publicRepoResponseJson = "{\"is_public\": true}"
const privateRepoResponseJson = "{\"is_public\": false}"

func TestCheckRepositoryAccessPublic(t *testing.T) {
	quay := &Quay{
		httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(publicRepoResponseJson))}, nil
		}},
		lookup: serviceprovider.GenericLookup{
			RepoHostParser: serviceprovider.RepoHostFromSchemelessUrl,
		},
	}
	k8sClient := mockK8sClient()

	accessCheck := &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl}}

	status, err := quay.CheckRepositoryAccess(context.TODO(), k8sClient, accessCheck)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
	assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
}

func TestCheckRepositoryAccess(t *testing.T) {
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeQuay),
				api.ServiceProviderHostLabel: "quay.io",
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: quayUrlBase,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})

	accessCheck := &api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ac",
			Namespace: "ac-namespace",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: testValidRepoUrl,
		},
	}

	metadataCache := serviceprovider.NewMetadataCache(cl, &serviceprovider.NeverMetadataExpirationPolicy{})
	lookupMock := serviceprovider.GenericLookup{
		RepoHostParser:      serviceprovider.RepoHostFromSchemelessUrl,
		ServiceProviderType: api.ServiceProviderTypeQuay,
		MetadataCache:       &metadataCache,
		TokenFilter: tokenFilterMock{
			matchesFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
				return true, nil
			},
		},
	}
	ts := tokenStorageMock{getFunc: func(ctx context.Context, owner *api.SPIAccessToken) *api.Token {
		return &api.Token{AccessToken: "blabol"}
	}}

	t.Run("public repository", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(publicRepoResponseJson))}, nil
			}},
			lookup:       lookupMock,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
	})

	t.Run("private repository with access", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(privateRepoResponseJson))}, nil
			}},
			lookup:       lookupMock,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityPrivate, status.Accessibility)
	})

	t.Run("non existing or private without access", func(t *testing.T) {
		check := func(status *api.SPIAccessCheckStatus, err error) {
			assert.NoError(t, err)
			assert.NotNil(t, status)
			assert.False(t, status.Accessible)
			assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
			assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
			assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		}

		t.Run("401 unauthorized", func(t *testing.T) {
			quay := &Quay{
				httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusUnauthorized}, nil
				}},
				lookup:       lookupMock,
				tokenStorage: ts,
			}

			status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)
			check(status, err)
		})

		t.Run("403 StatusForbidden", func(t *testing.T) {
			quay := &Quay{
				httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusForbidden}, nil
				}},
				lookup:       lookupMock,
				tokenStorage: ts,
			}

			status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)
			check(status, err)
		})
	})

	t.Run("not found", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusNotFound}, nil
			}},
			lookup:       lookupMock,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Equal(t, api.SPIAccessCheckErrorRepoNotFound, status.ErrorReason)
		assert.NotEmpty(t, status.ErrorMessage)
	})

	t.Run("error code from quay", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusInternalServerError}, nil
			}},
			lookup:       lookupMock,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.Error(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Equal(t, api.SPIAccessCheckErrorUnknownError, status.ErrorReason)
		assert.NotEmpty(t, status.ErrorMessage)
	})

	t.Run("error on quay api request", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("some error")
			}},
			lookup:       lookupMock,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.Error(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Equal(t, api.SPIAccessCheckErrorUnknownError, status.ErrorReason)
		assert.NotEmpty(t, status.ErrorMessage)
	})

	t.Run("bad repo url", func(t *testing.T) {
		quay := &Quay{}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: "https://quay.io"}})

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Equal(t, api.SPIAccessCheckErrorBadURL, status.ErrorReason)
		assert.NotEmpty(t, status.ErrorMessage)
	})

	t.Run("lookup failed", func(t *testing.T) {

	})
}

type httpClientMock struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (h httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return h.doFunc(req)
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}

type tokenFilterMock struct {
	matchesFunc func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error)
}

func (t tokenFilterMock) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	return t.matchesFunc(ctx, matchable, token)
}

type tokenStorageMock struct {
	getFunc func(ctx context.Context, owner *api.SPIAccessToken) *api.Token
}

func (t tokenStorageMock) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	return nil
}

func (t tokenStorageMock) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	return t.getFunc(ctx, owner), nil
}

func (t tokenStorageMock) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	return nil
}
