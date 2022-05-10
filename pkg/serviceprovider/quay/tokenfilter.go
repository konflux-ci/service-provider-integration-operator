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
	"encoding/json"
	"net/http"
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type tokenFilter struct {
	kubernetesClient client.Client
	httpClient       *http.Client
	tokenStorage     tokenstorage.TokenStorage
	ttl              time.Duration
}

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

func (t *tokenFilter) Matches(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken) (bool, error) {
	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	quayState := TokenState{}
	if err := json.Unmarshal(token.Status.TokenMetadata.ServiceProviderState, &quayState); err != nil {
		return false, err
	}

	orgOrUser, repo, _ := splitToOrganizationAndRepositoryAndVersion(binding.Spec.RepoUrl)

	var orgChanged, userChanged, repoChanged bool
	var orgRecord, userRecord, repoRecord EntityRecord
	var err error

	orgRecord, orgChanged, err = t.getEntityRecord(ctx, token, orgOrUser, quayState.Organizations, FetchOrganizationRecord)
	if err != nil {
		return false, err
	}

	userRecord, userChanged, err = t.getEntityRecord(ctx, token, orgOrUser, quayState.Users, FetchUserRecord)
	if err != nil {
		return false, err
	}

	repoRecord, repoChanged, err = t.getEntityRecord(ctx, token, orgOrUser+"/"+repo, quayState.Repositories, FetchRepositoryRecord)
	if err != nil {
		return false, err
	}

	if orgChanged || userChanged || repoChanged {
		if err = t.persistTokenState(ctx, token, &quayState); err != nil {
			return false, err
		}
	}

	requiredScopes := serviceprovider.GetAllScopes(translateToQuayScopes, &binding.Spec.Permissions)

	for _, s := range requiredScopes {
		requiredScope := Scope(s)

		var testedRecord EntityRecord

		switch requiredScope {
		case ScopeUserRead, ScopeUserAdmin:
			testedRecord = userRecord
		case ScopeOrgAdmin:
			testedRecord = orgRecord
		default:
			testedRecord = repoRecord
		}

		if !requiredScope.IsIncluded(testedRecord.PossessedScopes) {
			return false, nil
		}
	}

	return true, nil
}

func (t *tokenFilter) getEntityRecord(ctx context.Context, token *api.SPIAccessToken, key string, cache map[string]EntityRecord, fetchFn func(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token) (*EntityRecord, error)) (rec EntityRecord, changed bool, err error) {
	rec, present := cache[key]

	if !present || time.Now().After(time.Unix(rec.LastRefreshTime, 0).Add(t.ttl)) {
		var tokenData *api.Token
		var repoRec *EntityRecord

		tokenData, err = t.tokenStorage.Get(ctx, token)
		if err != nil {
			return
		}

		repoRec, err = fetchFn(ctx, t.httpClient, key, tokenData)
		if err != nil {
			return
		}

		if repoRec == nil {
			repoRec = &EntityRecord{}
		}

		repoRec.LastRefreshTime = time.Now().Unix()

		cache[key] = *repoRec
		rec = *repoRec
		changed = true
	}

	return
}

func (t *tokenFilter) persistTokenState(ctx context.Context, token *api.SPIAccessToken, tokenState *TokenState) error {
	data, err := json.Marshal(tokenState)
	if err != nil {
		return err
	}

	token.Status.TokenMetadata.ServiceProviderState = data

	return t.kubernetesClient.Status().Update(ctx, token)
}
