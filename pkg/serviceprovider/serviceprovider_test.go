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
	"net/http"
	"os"
	"testing"

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
			ConstructorFunc: func(factory *Factory, baseUrl string) (ServiceProvider, error) {
				return mockSP, nil
			},
		},
		SupportsManualUploadOnlyMode: true,
	}

	fact := Factory{
		Configuration:    &opconfig.OperatorConfiguration{},
		KubernetesClient: nil,
		HttpClient:       nil,
		Initializers: map[config.ServiceProviderType]Initializer{
			config.ServiceProviderTypeQuay: mockInit,
		},
		TokenStorage: nil,
	}

	sp, err := fact.FromRepoUrl(context.TODO(), "quay.com/namespace/repo")
	assert.NoError(t, err)
	assert.Equal(t, mockSP, sp)
}
