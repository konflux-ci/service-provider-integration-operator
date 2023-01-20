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

package serviceprovider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}

func TestGetAllScopesUniqueValues(t *testing.T) {
	translateToScopes := func(permission api.Permission) []string {
		return []string{string(permission.Type), string(permission.Area)}
	}

	perms := &api.Permissions{
		Required: []api.Permission{
			{
				Type: "a",
				Area: "b",
			},
			{
				Type: "a",
				Area: "c",
			},
		},
		AdditionalScopes: []string{"a", "b", "d", "e"},
	}

	scopes := GetAllScopes(translateToScopes, perms)

	expected := []string{"a", "b", "c", "d", "e"}
	for _, e := range expected {
		assert.Contains(t, scopes, e)
	}
	assert.Len(t, scopes, len(expected))
}

func TestDefaultMapToken(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		m := DefaultMapToken(&api.SPIAccessToken{}, &api.Token{})
		assert.Empty(t, m.Token)
		assert.Empty(t, m.Name)
		assert.Empty(t, m.Scopes)
		assert.Empty(t, m.UserId)
		assert.Empty(t, m.ServiceProviderUrl)
		assert.Empty(t, m.ServiceProviderUserName)
		assert.Empty(t, m.ServiceProviderUserId)
		assert.Empty(t, m.UserId)
		assert.NotNil(t, m.ExpiredAfter)
		assert.Equal(t, uint64(0), *m.ExpiredAfter)
	})

	t.Run("with data", func(t *testing.T) {
		m := DefaultMapToken(&api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: "objectname",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "service://provider",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					Username: "username",
					UserId:   "42",
					Scopes:   []string{"a", "b", "c"},
				},
			},
		}, &api.Token{
			Username:    "realusername",
			AccessToken: "access token",
			Expiry:      15,
		})
		assert.Equal(t, "access token", m.Token)
		assert.Equal(t, "objectname", m.Name)
		assert.Equal(t, []string{"a", "b", "c"}, m.Scopes)
		assert.Empty(t, m.UserId)
		assert.Equal(t, "service://provider", m.ServiceProviderUrl)
		assert.Equal(t, "username", m.ServiceProviderUserName)
		assert.Equal(t, "42", m.ServiceProviderUserId)
		assert.Empty(t, m.UserId)
		assert.NotNil(t, m.ExpiredAfter)
		assert.Equal(t, uint64(15), *m.ExpiredAfter)
	})
}

func TestFromRepoUrl(t *testing.T) {
	mockSP := struct {
		ServiceProvider
	}{}

	mockInit := Initializer{
		Probe: struct {
			ProbeFunc
		}{
			ProbeFunc: func(cl *http.Client, url string) (string, error) {
				return "https://base-url.com", nil
			},
		},
		Constructor: struct {
			ConstructorFunc
		}{
			ConstructorFunc: func(factory *Factory, _ *config.ServiceProviderConfiguration) (ServiceProvider, error) {
				return mockSP, nil
			},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

	initializers := NewInitializers().
		AddKnownInitializer(config.ServiceProviderTypeQuay, mockInit)

	fact := Factory{
		Configuration:    &opconfig.OperatorConfiguration{},
		KubernetesClient: cl,
		HttpClient:       nil,
		TokenStorage:     nil,
		Initializers:     initializers,
	}

	config.SupportedServiceProviderTypes = []config.ServiceProviderType{config.ServiceProviderTypeQuay}

	sp, err := fact.FromRepoUrl(context.TODO(), "quay.com/namespace/repo", "namespace")
	assert.NoError(t, err)
	assert.Equal(t, mockSP, sp)
}

func TestCreateHostCredentialsProvider(t *testing.T) {
	mockSP := struct {
		ServiceProvider
	}{}
	mockInit := Initializer{
		Probe: struct {
			ProbeFunc
		}{
			ProbeFunc: func(cl *http.Client, url string) (string, error) {
				return "https://base-url.com", nil
			},
		},
		Constructor: struct {
			ConstructorFunc
		}{
			ConstructorFunc: func(factory *Factory, _ *config.ServiceProviderConfiguration) (ServiceProvider, error) {
				return mockSP, nil
			},
		},
	}

	t.Run("created ok", func(t *testing.T) {
		f := Factory{
			Initializers: NewInitializers().AddKnownInitializer(config.ServiceProviderTypeHostCredentials, mockInit),
		}

		repoUrl, _ := url.Parse("https://blabol.sp/hey/there")

		sp, err := f.createHostCredentialsProvider(repoUrl)

		assert.NotNil(t, sp)
		assert.NoError(t, err)
	})

	t.Run("missing initializer", func(t *testing.T) {
		f := Factory{
			Initializers: NewInitializers().AddKnownInitializer(config.ServiceProviderTypeQuay, mockInit),
		}

		repoUrl, _ := url.Parse("https://blabol.sp/hey/there")

		sp, err := f.createHostCredentialsProvider(repoUrl)

		assert.Nil(t, sp)
		assert.Error(t, err)
	})

	t.Run("failed constructor", func(t *testing.T) {
		mockInit := Initializer{
			Probe: struct {
				ProbeFunc
			}{
				ProbeFunc: func(cl *http.Client, url string) (string, error) {
					return "https://base-url.com", nil
				},
			},
			Constructor: struct {
				ConstructorFunc
			}{
				ConstructorFunc: func(factory *Factory, _ *config.ServiceProviderConfiguration) (ServiceProvider, error) {
					return nil, fmt.Errorf("fial")
				},
			},
		}

		f := Factory{
			Initializers: NewInitializers().AddKnownInitializer(config.ServiceProviderTypeHostCredentials, mockInit),
		}

		repoUrl, _ := url.Parse("https://blabol.sp/hey/there")

		sp, err := f.createHostCredentialsProvider(repoUrl)

		assert.Nil(t, sp)
		assert.Error(t, err)
	})
}

// func TestInitializeServiceProvider(t *testing.T) {
// 	factory := Factory{}
// 	urlWithProtocol := "https://with.service.url"
// 	urlWoutProtocol := "without.service.url"

// 	test := func(repoUrl string, expectedSPBaseUrl string, baseUrls []string) {
// 		mockSP := struct {
// 			ServiceProvider
// 		}{}

// 		initializer := Initializer{Probe: struct {
// 			ProbeFunc
// 		}{
// 			ProbeFunc: func(cl *http.Client, url string) (string, error) {
// 				assert.FailNow(t, "should not be called")
// 				return "", nil
// 			},
// 		}, Constructor: struct {
// 			ConstructorFunc
// 		}{
// 			ConstructorFunc: func(factory *Factory, baseUrl string, spConfig *config.ServiceProviderConfiguration) (ServiceProvider, error) {
// 				assert.Equal(t, expectedSPBaseUrl, baseUrl)
// 				return mockSP, nil
// 			},
// 		}}

// 		// t.Run("should create service provider with base URL: "+expectedSPBaseUrl, func(t *testing.T) {
// 		// 	sp := factory.initializeServiceProvider(&initializer, repoUrl, baseUrls)
// 		// 	assert.NotNil(t, sp)
// 		// 	assert.Equal(t, mockSP, sp)
// 		// })
// 	}

// 	test("with.service.url/repo/path", urlWithProtocol, []string{urlWithProtocol})
// 	test("https://with.service.url/with/repo/path", urlWithProtocol, []string{urlWithProtocol})

// 	test("without.service.url/with/path", "https://"+urlWoutProtocol, []string{urlWoutProtocol})
// 	test("https://without.service.url/with/path", "https://"+urlWoutProtocol, []string{urlWoutProtocol})

// 	initializer := Initializer{Probe: struct {
// 		ProbeFunc
// 	}{
// 		ProbeFunc: func(cl *http.Client, url string) (string, error) {
// 			return "", fmt.Errorf("no urls matching found")
// 		},
// 	}, Constructor: struct {
// 		ConstructorFunc
// 	}{
// 		ConstructorFunc: func(factory *Factory, baseUrl string, spConfig *config.ServiceProviderConfiguration) (ServiceProvider, error) {
// 			assert.FailNow(t, "should not be called")
// 			return nil, nil
// 		},
// 	}}
// 	sp := factory.initializeServiceProvider(&initializer, "another.service.url/with/path", []string{urlWithProtocol, urlWoutProtocol})
// 	assert.Nil(t, sp)
// }
