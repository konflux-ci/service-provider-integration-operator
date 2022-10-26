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
	"fmt"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestMatches(t *testing.T) {
	tf := &tokenFilter{}

	t.Run("match with no metadata", func(t *testing.T) {
		res, err := tf.Matches(context.TODO(),
			&api.SPIAccessTokenBinding{},
			&api.SPIAccessToken{})
		assert.NoError(t, err)
		assert.False(t, res)
	})

	test := func(t *testing.T, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, expectedMatch bool) {
		t.Run(fmt.Sprintf("match should be %t by required scopes", expectedMatch), func(t *testing.T) {
			res, err := tf.Matches(context.TODO(), binding, token)
			assert.NoError(t, err)
			assert.Equal(t, expectedMatch, res)
		})
	}

	binding := &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "some-gitlab-repo",
			Permissions: api.Permissions{
				Required: []api.Permission{
					{
						Type: api.PermissionTypeWrite,
						Area: api.PermissionAreaRepository,
					},
					{
						Type: api.PermissionTypeRead,
						Area: api.PermissionAreaRegistry,
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
				Scopes:               []string{string(ScopeWriteRepository)},
				ServiceProviderState: []byte(""),
			},
		},
	}

	matchingToken := &api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{},
		Status: api.SPIAccessTokenStatus{
			TokenMetadata: &api.TokenMetadata{
				Username:             "you",
				UserId:               "42",
				Scopes:               []string{string(ScopeWriteRepository), string(ScopeReadRegistry)},
				ServiceProviderState: []byte(""),
			},
		},
	}

	matchingToken2 := &api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{},
		Status: api.SPIAccessTokenStatus{
			TokenMetadata: &api.TokenMetadata{
				Username:             "you",
				UserId:               "42",
				Scopes:               []string{string(ScopeApi)},
				ServiceProviderState: []byte(""),
			},
		},
	}

	test(t, binding, nonMatchingToken, false)
	test(t, binding, matchingToken, true)
	test(t, binding, matchingToken2, true)
}
