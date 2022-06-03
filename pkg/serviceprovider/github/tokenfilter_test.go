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

package github

import (
	"context"
	"encoding/json"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestTokenFilter_Matches(t *testing.T) {
	tf := &tokenFilter{}

	t.Run("no metadata", func(t *testing.T) {
		res, err := tf.Matches(context.TODO(),
			&api.SPIAccessTokenBinding{},
			&api.SPIAccessToken{})
		assert.NoError(t, err)
		assert.False(t, res)
	})

	test := func(t *testing.T, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, expectedMatch bool) {
		res, err := tf.Matches(context.TODO(), binding, token)
		assert.NoError(t, err)
		assert.Equal(t, expectedMatch, res)
	}

	t.Run("by repo", func(t *testing.T) {
		ts, err := json.Marshal(&TokenState{
			AccessibleRepos: map[RepositoryUrl]RepositoryRecord{
				"my-repo": {ViewerPermission: ViewerPermissionAdmin},
			},
		})
		assert.NoError(t, err)

		test(t,
			&api.SPIAccessTokenBinding{
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl:     "my-repo",
					Permissions: api.Permissions{},
					Secret:      api.SecretSpec{},
				},
			},
			&api.SPIAccessToken{
				Spec: api.SPIAccessTokenSpec{},
				Status: api.SPIAccessTokenStatus{
					TokenMetadata: &api.TokenMetadata{
						Username:             "you",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: ts,
					},
				},
			},
			true)
	})

	t.Run("by repo and scopes", func(t *testing.T) {
		ts, err := json.Marshal(&TokenState{
			AccessibleRepos: map[RepositoryUrl]RepositoryRecord{
				"my-repo": {ViewerPermission: ViewerPermissionAdmin},
			},
		})
		assert.NoError(t, err)

		binding := &api.SPIAccessTokenBinding{
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "my-repo",
				Permissions: api.Permissions{
					Required: []api.Permission{
						{
							Type: api.PermissionTypeWrite,
							Area: api.PermissionAreaRepository,
						},
					},
				},
			},
		}

		nonMatchingToken := &api.SPIAccessToken{
			Spec: api.SPIAccessTokenSpec{},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					Username:             "you",
					UserId:               "42",
					Scopes:               []string{},
					ServiceProviderState: ts,
				},
			},
		}

		matchingToken := &api.SPIAccessToken{
			Spec: api.SPIAccessTokenSpec{},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					Username:             "you",
					UserId:               "42",
					Scopes:               []string{"repo"},
					ServiceProviderState: ts,
				},
			},
		}

		test(t, binding, matchingToken, true)
		test(t, binding, nonMatchingToken, false)
	})
}
