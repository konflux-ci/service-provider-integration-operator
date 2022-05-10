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
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

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

func TestMetadataProvider_ReadRobotUsername(t *testing.T) {

	var localTs = tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				Username:     "user1",
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}

	mp := metadataProvider{
		tokenStorage: &localTs,
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
	assert.Equal(t, "user1", data.Username)
	assert.Empty(t, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)
}

func TestMetadataProvider_FetchUserAndRWPerms(t *testing.T) {

	mp := metadataProvider{
		tokenStorage: &ts,
	}

	tkn := api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{Required: []api.Permission{
				{
					Type: api.PermissionTypeReadWrite,
					Area: api.PermissionAreaRepository,
				},
				{
					Area: api.PermissionAreaUser,
					Type: api.PermissionTypeReadWrite,
				},
			}},
		},
	}
	data, err := mp.Fetch(context.TODO(), &tkn)
	assert.NoError(t, err)

	assert.NotNil(t, data)
	assert.Equal(t, "$oauthtoken", data.Username)
	assert.ElementsMatch(t, []string{"repo:read", "repo:write", "user:admin"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)
}

func TestMetadataProvider_FetchUserAndROPerms(t *testing.T) {

	mp := metadataProvider{
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
	assert.ElementsMatch(t, []string{"repo:read"}, data.Scopes)
	assert.NotEmpty(t, data.ServiceProviderState)
}
