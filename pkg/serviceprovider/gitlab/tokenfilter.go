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

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

type tokenFilter struct{}

func (t tokenFilter) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	// We are currently matching only by scopes.
	// This will change in the future when TokenState for GitLab tokens will be implemented.

	lg := log.FromContext(ctx, "matchableUrl", matchable.RepoUrl())
	lg.Info("matching", "token", token.Name)

	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	requiredScopes := serviceprovider.GetAllScopes(translateToGitlabScopes, matchable.Permissions())

	hasScope := func(scope Scope) bool {
		for _, s := range token.Status.TokenMetadata.Scopes {
			if Scope(s).Implies(scope) {
				return true
			}
		}
		return false
	}
	for _, s := range requiredScopes {
		scope := Scope(s)
		if !hasScope(scope) {
			return false, nil
		}
	}

	return true, nil
}
