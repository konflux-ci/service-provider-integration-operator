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
	} else {
		metadata.Username = "$oauthtoken"
	}

	metadata.ServiceProviderState = js

	return metadata, nil
}

type RepositoryMetadata struct {
	Repository   EntityRecord
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

	var tokenData *api.Token

	tokenData, err = p.tokenStorage.Get(ctx, token)
	if err != nil {
		return
	}

	orgOrUser, repo, _ := splitToOrganizationAndRepositoryAndVersion(repoUrl)

	// enable the lazy one-time login to docker in the subsequent calls
	var loginToken *LoginTokenInfo
	getLoginTokenInfo := func() (LoginTokenInfo, error) {
		if loginToken != nil {
			return *loginToken, nil
		}

		username, password := getUsernameAndPasswordFromTokenData(tokenData)
		var loginToken string
		loginToken, err = DockerLogin(ctx, p.httpClient, repo, username, password)
		if err != nil {
			return LoginTokenInfo{}, err
		}

		info, err := AnalyzeLoginToken(loginToken)
		if err != nil {
			return LoginTokenInfo{}, err
		}

		return info, nil
	}

	var orgChanged, repoChanged bool
	var orgRecord, repoRecord EntityRecord

	orgRecord, orgChanged, err = p.getEntityRecord(ctx, token, orgOrUser, quayState.Organizations, getLoginTokenInfo, fetchOrganizationRecord)
	if err != nil {
		return
	}

	repoRecord, repoChanged, err = p.getEntityRecord(ctx, token, orgOrUser+"/"+repo, quayState.Repositories, getLoginTokenInfo, fetchRepositoryRecord)
	if err != nil {
		return
	}

	if orgChanged || repoChanged {
		if err = p.persistTokenState(ctx, token, &quayState); err != nil {
			return
		}
	}

	return RepositoryMetadata{
		Repository:   repoRecord,
		Organization: orgRecord,
	}, nil
}

// helper types for getEntityRecord parameters
type loginInfoFn func() (LoginTokenInfo, error)
type fetchEntityRecordFn func(ctx context.Context, cl *http.Client, repoUrl string, tokenData *api.Token, info LoginTokenInfo) (*EntityRecord, error)

func (p metadataProvider) getEntityRecord(ctx context.Context, token *api.SPIAccessToken, key string, cache map[string]EntityRecord, loginInfoFn loginInfoFn, fetchFn fetchEntityRecordFn) (rec EntityRecord, changed bool, err error) {
	rec, present := cache[key]

	if !present || time.Now().After(time.Unix(rec.LastRefreshTime, 0).Add(p.ttl)) {
		var tokenData *api.Token
		var repoRec *EntityRecord

		tokenData, err = p.tokenStorage.Get(ctx, token)
		if err != nil {
			return
		}
		if tokenData == nil {
			// the data is stale or not present, and we have no way of figuring out the new data because
			// we don't have a token
			rec = EntityRecord{}
			return
		}

		var loginInfo LoginTokenInfo
		loginInfo, err = loginInfoFn()
		if err != nil {
			return
		}

		repoRec, err = fetchFn(ctx, p.httpClient, key, tokenData, loginInfo)
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
