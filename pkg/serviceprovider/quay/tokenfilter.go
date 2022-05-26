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

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

type tokenFilter struct {
	metadataProvider *metadataProvider
}

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

func (t *tokenFilter) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	lg := log.FromContext(ctx, "matchableUrl", matchable.RepoUrl())

	lg.Info("matching", "token", token.Name)
	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	rec, err := t.metadataProvider.FetchRepo(ctx, matchable.RepoUrl(), token)
	if err != nil {
		lg.Error(err, "failed to fetch token metadata")
		return false, err
	}

	requiredScopes := serviceprovider.GetAllScopes(translateToQuayScopes, matchable.Permissions())

	for _, s := range requiredScopes {
		requiredScope := Scope(s)

		var testedRecord EntityRecord

		switch requiredScope {
		case ScopeOrgAdmin:
			testedRecord = rec.Organization
		default:
			testedRecord = rec.Repository
		}

		if !requiredScope.IsIncluded(testedRecord.PossessedScopes) {
			return false, nil
		}
	}

	return true, nil
}
