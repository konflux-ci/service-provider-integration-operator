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
	"context"
	"testing"

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
	gitlab := &Gitlab{}
	hasExpectedScopes := func(expectedScopes []string, permissions api.Permissions) func(t *testing.T) {
		return func(t *testing.T) {
			actualScopes := gitlab.OAuthScopesFor(&permissions)
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
