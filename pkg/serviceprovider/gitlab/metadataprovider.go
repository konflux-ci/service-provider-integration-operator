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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/hashicorp/go-retryablehttp"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type metadataProvider struct {
	tokenStorage    tokenstorage.TokenStorage
	httpClient      *http.Client
	glClientBuilder gitlabClientBuilder
	baseUrl         string
}

var _ serviceprovider.MetadataProvider = (*metadataProvider)(nil)

const gitlabOAuthTokenInfoPath = "/oauth/token/info"
const gitlabPATInfoPath = "/personal_access_tokens/self"

func (p metadataProvider) Fetch(ctx context.Context, token *api.SPIAccessToken) (*api.TokenMetadata, error) {
	lg := log.FromContext(ctx, "tokenName", token.Name, "tokenNamespace", token.Namespace)

	data, err := p.tokenStorage.Get(ctx, token)
	if err != nil {
		lg.Error(err, "failed to get the token metadata")
		return nil, fmt.Errorf("failed to get the token metadata: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	state := &TokenState{}

	glClient, err := p.glClientBuilder.createGitlabAuthClient(ctx, token, p.baseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated GitLab client: %w", err)
	}

	username, userId, err := p.fetchUser(ctx, glClient)
	if err != nil {
		return nil, err
	}
	scopes, err := p.fetchScopes(ctx, glClient)
	if err != nil {
		return nil, err
	}

	lg.V(logs.DebugLevel).Info("fetched user metadata from GitLab", "login", username, "userid", userId, "scopes", scopes)

	// TODO: implement TokenState
	encodedState, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("error marshalling the state: %w", err)
	}

	metadata := &api.TokenMetadata{}
	metadata.UserId = userId
	metadata.Username = username
	metadata.Scopes = scopes
	metadata.ServiceProviderState = encodedState

	return metadata, nil
}

func (p metadataProvider) fetchUser(ctx context.Context, gitlabClient *gitlab.Client) (userName string, userId string, err error) {
	lg := log.FromContext(ctx)
	usr, resp, err := gitlabClient.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		lg.Error(err, "error during fetching user metadata from GitLab")
		err = fmt.Errorf("failed to fetch user metadate from GitLab: %w", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		lg.Error(err, "error during fetching user metadata from GitLab", "status", resp.StatusCode)
		return "", "", nil
	}

	return usr.Username, strconv.FormatInt(int64(usr.ID), 10), nil
}

func (p metadataProvider) fetchScopes(ctx context.Context, gitlabClient *gitlab.Client) ([]string, error) {
	lg := log.FromContext(ctx)

	tokenInfoResponse := struct {
		Scopes []string `json:"scope"`
	}{}
	oauthInfoRequest, err := retryablehttp.NewRequestWithContext(ctx, "GET", p.baseUrl+gitlabOAuthTokenInfoPath, nil)
	if err != nil {
		return nil, err
	}

	oauthInfoResponse, err := gitlabClient.Do(oauthInfoRequest, &tokenInfoResponse)
	if err != nil {
		lg.Error(err, "error during fetching token scopes from GitLab")
		return nil, err
	}

	if oauthInfoResponse.StatusCode == http.StatusOK {
		return tokenInfoResponse.Scopes, nil
	}

	if oauthInfoResponse.StatusCode != http.StatusUnauthorized {
		lg.Error(err, "non-ok status during fetching token scopes from GitLab", "status", oauthInfoResponse.StatusCode)
		return nil, nil
	}

	// We are going to try to get scopes with PAT API since OAuth API did not accept the token
	patInfoRequest, err := retryablehttp.NewRequestWithContext(ctx, "GET", p.baseUrl+gitlabPATInfoPath, nil)
	if err != nil {
		return nil, err
	}

	patInfoResponse, err := gitlabClient.Do(patInfoRequest, &tokenInfoResponse)
	if err != nil {
		return nil, err
	}

	if patInfoResponse.StatusCode != http.StatusOK {
		// We give up on finding out the scopes at this point and just return empty slice.
		// In the future we can figure out scopes by making request for different resource
		// similarly to Quay.
		return []string{}, nil
	}

	return tokenInfoResponse.Scopes, nil
}
