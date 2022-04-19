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
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/mshaposhnik/service-provider-integration-operator/pkg/spi-shared/util"

	api "github.com/mshaposhnik/service-provider-integration-operator/api/v1beta1"
	"github.com/mshaposhnik/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

var httpCl = &http.Client{
	Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL == quayUserApiEndpoint {
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`{"anonymous": false, "username": "test_user"}`))),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 404,
			}, nil
		}
	}),
}

var ts = tokenstorage.TestTokenStorage{
	GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{
			AccessToken:  "access",
			TokenType:    "fake",
			RefreshToken: "refresh",
			Expiry:       0,
		}, nil
	},
}

func TestMetadataProvider_FetchRW(t *testing.T) {

	mp := metadataProvider{
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{Required: []api.Permission{
				{
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaRepository,
				},
			}},
		},
	}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "$oauthtoken", data.Username)
	assert.ElementsMatch(t, []string{"repo:read", "repo:write", "user:read"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)

	tokenState := &TokenState{}
	assert.NoError(t, json.Unmarshal(data.ServiceProviderState, tokenState))
	assert.Equal(t, "test_user", tokenState.RemoteUsername)
}

func TestMetadataProvider_FetchRO(t *testing.T) {

	mp := metadataProvider{
		httpClient:   httpCl,
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{Required: []api.Permission{
				{
					Type: api.PermissionTypeRead,
					Area: api.PermissionAreaRepository,
				},
			}},
		},
	}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "$oauthtoken", data.Username)
	assert.ElementsMatch(t, []string{"repo:read", "user:read"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)

	tokenState := &TokenState{}
	assert.NoError(t, json.Unmarshal(data.ServiceProviderState, tokenState))
	assert.Equal(t, "test_user", tokenState.RemoteUsername)
}
