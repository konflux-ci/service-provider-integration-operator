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

	"sigs.k8s.io/controller-runtime/pkg/log"

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
	lg := log.FromContext(ctx, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		lg.Error(err, "failed to get the token metadata")
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
		lg.Error(err, "failed to serialize the token metadata, this should not happen")
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

	lg.Info("token metadata initialized")

	return metadata, nil
}

type RepositoryMetadata struct {
	Repository   EntityRecord
	Organization EntityRecord
}

// FetchRepo is the iterative version of Fetch used internally in the Quay service provider. It takes care of both
// fetching and caching of the data on per-repository basis.
func (p metadataProvider) FetchRepo(ctx context.Context, repoUrl string, token *api.SPIAccessToken) (metadata RepositoryMetadata, err error) {
	lg := log.FromContext(ctx, "repo", repoUrl, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	lg.Info("fetching repository metadata")

	if token.Status.TokenMetadata == nil {
		lg.Info("no metadata on the token object, bailing")
		return
	}

	quayState := TokenState{}
	if err = json.Unmarshal(token.Status.TokenMetadata.ServiceProviderState, &quayState); err != nil {
		lg.Error(err, "failed to unmarshal quay token state")
		return
	}

	var tokenData *api.Token

	tokenData, err = p.tokenStorage.Get(ctx, token)
	if err != nil {
		lg.Error(err, "failed to get token data")
		return
	}
	if tokenData == nil {
		lg.Info("no token data found")
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
		var tkn string
		tkn, err = DockerLogin(log.IntoContext(ctx, lg), p.httpClient, repo, username, password)
		if err != nil {
			lg.Error(err, "failed to perform docker login")
			return LoginTokenInfo{}, err
		}

		info, err := AnalyzeLoginToken(tkn)
		if err != nil {
			lg.Error(err, "failed to analyze the docker login token")
			return LoginTokenInfo{}, err
		}

		lg.Info("used docker login to detect rw perms successfully", "repo:read",
			info.Repositories[repo].Pullable, "repo:write", info.Repositories[repo].Pushable)

		loginToken = &info

		return info, nil
	}

	var orgChanged, repoChanged bool
	var orgRecord, repoRecord EntityRecord

	orgRecord, orgChanged, err = p.getEntityRecord(log.IntoContext(ctx, lg.WithValues("entityType", "organization")), tokenData, orgOrUser, quayState.Organizations, getLoginTokenInfo, fetchOrganizationRecord)
	if err != nil {
		lg.Error(err, "failed to read the organization metadata")
		return
	}

	repoRecord, repoChanged, err = p.getEntityRecord(log.IntoContext(ctx, lg.WithValues("entityType", "repository")), tokenData, orgOrUser+"/"+repo, quayState.Repositories, getLoginTokenInfo, fetchRepositoryRecord)
	if err != nil {
		lg.Error(err, "failed to read the repository metadata")
		return
	}

	if orgChanged || repoChanged {
		if err = p.persistTokenState(ctx, token, &quayState); err != nil {
			lg.Error(err, "failed to persist the metadata changes")
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

func (p metadataProvider) getEntityRecord(ctx context.Context, tokenData *api.Token, key string, cache map[string]EntityRecord, loginInfoFn loginInfoFn, fetchFn fetchEntityRecordFn) (rec EntityRecord, changed bool, err error) {
	rec, present := cache[key]

	lg := log.FromContext(ctx)

	if !present || time.Now().After(time.Unix(rec.LastRefreshTime, 0).Add(p.ttl)) {
		lg.Info("entity metadata stale, reloading")

		var repoRec *EntityRecord

		var loginInfo LoginTokenInfo
		loginInfo, err = loginInfoFn()
		if err != nil {
			lg.Error(err, "failed to get the docker login info")
			return
		}

		repoRec, err = fetchFn(ctx, p.httpClient, key, tokenData, loginInfo)
		if err != nil {
			lg.Error(err, "failed to fetch the metadata entity")
			return
		}

		if repoRec == nil {
			repoRec = &EntityRecord{}
		}

		repoRec.LastRefreshTime = time.Now().Unix()

		cache[key] = *repoRec
		rec = *repoRec
		changed = true
		lg.Info("metadata entity fetched successfully")
	} else {
		lg.Info("requested metadata entity still valid")
	}

	return
}

func (p metadataProvider) persistTokenState(ctx context.Context, token *api.SPIAccessToken, tokenState *TokenState) error {
	lg := log.FromContext(ctx)

	data, err := json.Marshal(tokenState)
	if err != nil {
		lg.Error(err, "failed to serialize the metadata")
		return err
	}

	token.Status.TokenMetadata.ServiceProviderState = data

	return p.kubernetesClient.Status().Update(ctx, token)
}
