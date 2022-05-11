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

	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type metadataProvider struct {
	tokenStorage     tokenstorage.TokenStorage
	httpClient       *http.Client
	kubernetesClient client.Client
	ttl              time.Duration
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

func (p metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, err
	}

	// This method is called when we need to refresh (or obtain anew, after cache expiry) the metadata of the token.
	// Because we load all the state iteratively for Quay, this info is always empty when fresh.
	state := &TokenState{
		Repositories:  map[string]EntityRecord{},
		Organizations: map[string]EntityRecord{},
		Users:         map[string]EntityRecord{},
	}

	js, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	metadata := token.Status.TokenMetadata
	if metadata == nil {
		metadata = &api.TokenMetadata{}
		token.Status.TokenMetadata = metadata
	}

	if len(data.Username) > 0 {
		metadata.Username = data.Username
		// TODO: The scopes are going to differ per-repo, so we're going to need an SP-specific secret sync
	} else {
		metadata.Username = "$oauthtoken"
		metadata.Scopes = serviceprovider.GetAllScopes(translateToQuayScopes, &token.Spec.Permissions)
	}

	metadata.ServiceProviderState = js

	return metadata, nil
}

type RepositoryMetadata struct {
	Repository   EntityRecord
	User         EntityRecord
	Organization EntityRecord
}

// FetchRepo is the iterative version of Fetch used internally in the Quay service provider. It takes care of both
// fetching and caching of the data on per-repository basis.
func (p metadataProvider) FetchRepo(ctx context.Context, repoUrl string, token *api.SPIAccessToken) (metadata RepositoryMetadata, err error) {
	if token.Status.TokenMetadata == nil {
		return
	}

	quayState := TokenState{}
	if err = json.Unmarshal(token.Status.TokenMetadata.ServiceProviderState, &quayState); err != nil {
		return
	}

	orgOrUser, repo, _ := splitToOrganizationAndRepositoryAndVersion(repoUrl)

	var orgChanged, userChanged, repoChanged bool
	var orgRecord, userRecord, repoRecord EntityRecord

	orgRecord, orgChanged, err = p.getEntityRecord(ctx, token, orgOrUser, quayState.Organizations, fetchOrganizationRecord)
	if err != nil {
		return
	}

	userRecord, userChanged, err = p.getEntityRecord(ctx, token, orgOrUser, quayState.Users, fetchUserRecord)
	if err != nil {
		return
	}

	repoRecord, repoChanged, err = p.getEntityRecord(ctx, token, orgOrUser+"/"+repo, quayState.Repositories, fetchRepositoryRecord)
	if err != nil {
		return
	}

	if orgChanged || userChanged || repoChanged {
		if err = p.persistTokenState(ctx, token, &quayState); err != nil {
			return
		}
	}

	return RepositoryMetadata{
		Repository:   repoRecord,
		User:         userRecord,
		Organization: orgRecord,
	}, nil
}

func (p metadataProvider) getEntityRecord(ctx context.Context, token *api.SPIAccessToken, key string, cache map[string]EntityRecord, fetchFn func(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token) (*EntityRecord, error)) (rec EntityRecord, changed bool, err error) {
	rec, present := cache[key]

	if !present || time.Now().After(time.Unix(rec.LastRefreshTime, 0).Add(p.ttl)) {
		var tokenData *api.Token
		var repoRec *EntityRecord

		tokenData, err = p.tokenStorage.Get(ctx, token)
		if err != nil {
			return
		}

		repoRec, err = fetchFn(ctx, p.httpClient, key, tokenData)
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

func (p metadataProvider) persistTokenState(ctx context.Context, token *api.SPIAccessToken, tokenState *TokenState) error {
	data, err := json.Marshal(tokenState)
	if err != nil {
		return err
	}

	token.Status.TokenMetadata.ServiceProviderState = data

	return p.kubernetesClient.Status().Update(ctx, token)
}
