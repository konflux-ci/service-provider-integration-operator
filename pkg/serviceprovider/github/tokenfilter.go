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
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

type tokenFilter struct{}

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

func (t *tokenFilter) Matches(_ context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	githubState := TokenState{}
	if err := json.Unmarshal(token.Status.TokenMetadata.ServiceProviderState, &githubState); err != nil {
		return false, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	for repoUrl, rec := range githubState.AccessibleRepos {
		if string(repoUrl) == matchable.RepoUrl() && permsMatch(matchable.Permissions(), rec, token.Status.TokenMetadata.Scopes) {
			return true, nil
		}
	}

	return false, nil
}

func permsMatch(perms *api.Permissions, rec RepositoryRecord, tokenScopes []string) bool {
	requiredScopes := serviceprovider.GetAllScopes(translateToScopes, perms)

	hasScope := func(scope Scope) bool {
		for _, s := range tokenScopes {
			if Scope(s).Implies(scope) {
				return true
			}
		}

		return false
	}

	for _, s := range requiredScopes {
		scope := Scope(s)
		if !hasScope(scope) {
			return false
		}

		if !rec.ViewerPermission.Enables(scope) {
			return false
		}
	}

	return true
}
