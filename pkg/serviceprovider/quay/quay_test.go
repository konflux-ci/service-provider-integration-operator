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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"golang.org/x/oauth2"

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

var testValidRepoUrl = config.ServiceProviderTypeQuay.DefaultBaseUrl + "/repository/redhat-appstudio/service-provider-integration-operator"

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}

func TestQuayProbe_Examine(t *testing.T) {
	probe := quayProbe{}
	test := func(t *testing.T, url string, expectedMatch bool) {
		baseUrl, err := probe.Examine(nil, url)
		expectedBaseUrl := ""
		if expectedMatch {
			expectedBaseUrl = config.ServiceProviderTypeQuay.DefaultBaseUrl
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

	initializers := serviceprovider.NewInitializers().
		AddKnownInitializer(config.ServiceProviderTypeQuay, Initializer)

	fac := &serviceprovider.Factory{
		Configuration: &opconfig.OperatorConfiguration{
			TokenLookupCacheTtl: 100 * time.Hour,
			TokenMatchPolicy:    opconfig.ExactTokenPolicy,
		},
		KubernetesClient: k8sClient,
		HttpClient:       httpClient,
		Initializers:     initializers,
		TokenStorage: tokenstorage.TestTokenStorage{
			GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
				return &api.Token{
					AccessToken: "accessToken",
				}, nil
			},
		},
	}

	quay, err := newQuay(fac, &config.ServiceProviderConfiguration{})
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
						Area: api.PermissionAreaRegistry,
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
	assert.Equal(t, "unsupported permission area for Quay: 'user'", res.ScopeValidation[0].Error())
	assert.NotNil(t, res.ScopeValidation[1])
	assert.Equal(t, "unknown scope: 'blah'", res.ScopeValidation[1].Error())
	assert.NotNil(t, res.ScopeValidation[2])
	assert.Equal(t, "unsupported scope 'user:read'", res.ScopeValidation[2].Error())
}

func TestQuay_OAuthScopesFor(t *testing.T) {

	q := &Quay{
		OAuthCapability: &quayOAuthCapability{},
	}
	fullScopes := []string{"repo:read", "repo:write", "repo:create", "repo:admin"}

	hasScopes := func(expectedScopes []string, ps api.Permissions) func(t *testing.T) {
		return func(t *testing.T) {
			actualScopes := q.GetOAuthCapability().OAuthScopesFor(&ps)
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
			Area: api.PermissionAreaRegistry,
			Type: api.PermissionTypeRead,
		},
	}}))
	t.Run("write-repo", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRegistry,
			Type: api.PermissionTypeWrite,
		},
	}}))
	t.Run("read-write-repo", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRegistry,
			Type: api.PermissionTypeReadWrite,
		},
	}}))
	t.Run("read-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRegistryMetadata,
			Type: api.PermissionTypeRead,
		},
	}}))
	t.Run("write-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRegistryMetadata,
			Type: api.PermissionTypeWrite,
		},
	}}))
	t.Run("read-write-meta", hasDefault(api.Permissions{Required: []api.Permission{
		{
			Area: api.PermissionAreaRegistryMetadata,
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
				api.ServiceProviderHostLabel: config.ServiceProviderTypeQuay.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeQuay.DefaultBaseUrl,
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

	metadataCache := serviceprovider.MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &serviceprovider.NeverMetadataExpirationPolicy{},
		CacheServiceProviderState: true,
	}

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
	ts := tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{AccessToken: "blabol"}, nil
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

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: config.ServiceProviderTypeQuay.DefaultBaseUrl}})

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Equal(t, api.SPIAccessCheckErrorBadURL, status.ErrorReason)
		assert.NotEmpty(t, status.ErrorMessage)
	})

	t.Run("lookup failed public", func(t *testing.T) {
		failingLookup := lookupMock
		failingLookup.TokenFilter = tokenFilterMock{matchesFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
			return false, errors.New("intentional failure")
		}}

		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(publicRepoResponseJson))}, nil
			}},
			lookup:       failingLookup,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
		assert.Empty(t, status.ErrorMessage)
		assert.Empty(t, status.ErrorReason)
	})

	t.Run("lookup failed nonpublic", func(t *testing.T) {
		failingLookup := lookupMock
		failingLookup.TokenFilter = tokenFilterMock{matchesFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
			return false, errors.New("intentional failure")
		}}

		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(publicRepoResponseJson))}, nil
			}},
			lookup:       failingLookup,
			tokenStorage: ts,
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.NotEmpty(t, status.ErrorMessage)
		assert.Equal(t, api.SPIAccessCheckErrorTokenLookupFailed, status.ErrorReason)
	})

	t.Run("no token data on private", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusUnauthorized}, nil
			}},
			lookup: lookupMock,
			tokenStorage: tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
				return nil, nil
			}},
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
		assert.Empty(t, status.ErrorReason)
		assert.Empty(t, status.ErrorMessage)
	})

	t.Run("no token on public", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(publicRepoResponseJson))}, nil
			}},
			lookup: lookupMock,
			tokenStorage: tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
				return nil, nil
			}},
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
		assert.Empty(t, status.ErrorReason)
		assert.Empty(t, status.ErrorMessage)
	})

	t.Run("robot token on private", func(t *testing.T) {
		quay := &Quay{
			httpClient: httpClientMock{doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusUnauthorized}, nil
			}},
			lookup: lookupMock,
			tokenStorage: tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
				return &api.Token{AccessToken: "tkn", Username: "alois"}, nil
			}},
		}

		status, err := quay.CheckRepositoryAccess(context.TODO(), cl, accessCheck)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.Accessible)
		assert.Equal(t, api.SPIRepoTypeContainerRegistry, status.Type)
		assert.Equal(t, api.ServiceProviderTypeQuay, status.ServiceProvider)
		assert.Equal(t, api.SPIAccessCheckAccessibilityPrivate, status.Accessibility)
		assert.Empty(t, status.ErrorReason)
		assert.Empty(t, status.ErrorMessage)
	})
}

func TestNewQuay(t *testing.T) {
	factory := &serviceprovider.Factory{
		Configuration: &opconfig.OperatorConfiguration{
			TokenMatchPolicy: opconfig.AnyTokenPolicy,
			SharedConfiguration: config.SharedConfiguration{
				BaseUrl: "bejsjuarel",
			},
		},
	}

	t.Run("no oauth info => nil oauth capability", func(t *testing.T) {
		sp, err := newQuay(factory, &config.ServiceProviderConfiguration{ServiceProviderBaseUrl: "https://yauq.oi"})

		assert.NoError(t, err)
		assert.NotNil(t, sp)
		assert.Nil(t, sp.GetOAuthCapability())
	})

	t.Run("oauth info => oauth capability", func(t *testing.T) {
		sp, err := newQuay(factory, &config.ServiceProviderConfiguration{ServiceProviderBaseUrl: "https://baltig.moc", OAuth2Config: &oauth2.Config{ClientID: "123", ClientSecret: "456"}})

		assert.NoError(t, err)
		assert.NotNil(t, sp)
		assert.NotNil(t, sp.GetOAuthCapability())
		assert.Contains(t, sp.GetOAuthCapability().GetOAuthEndpoint(), "bejsjuarel")
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
